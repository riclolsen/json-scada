/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

// The go-modbus RequestHandler: reads are served from the PointMap, writes go
// to the WriteHandler. Every exception answered is logged as a request error
// and counted, as the Node.js server stack did.

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbnet"

	"github.com/otfabric/go-modbus"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// counters are the request statistics.
type counters struct {
	reads, writes, exceptions atomic.Int64
}

// Handler implements modbus.RequestHandler and modbus.MaskWriteHandler.
type Handler struct {
	cfg   Config
	pm    *PointMap
	wh    *WriteHandler
	addrs *mbnet.AddrMap
	st    *counters
}

// handlerError marks an exception the handler chose (already logged and
// counted), so the metrics hook skips it.
type handlerError struct{ err error }

func (e handlerError) Error() string { return e.err.Error() }
func (e handlerError) Unwrap() error { return e.err }

func (h *Handler) remote(clientAddr string) string { return h.addrs.Resolve(clientAddr) }

// gate applies strictUnitId: an unserved unit gets no answer at all.
func (h *Handler) gate(unit uint8, remote string) error {
	if unit != 0 && h.cfg.StrictUnitID && !h.pm.AcceptsUnit(int(unit)) {
		jslog.Log(jslog.LevelDetailed, "%s: [%s] request ignored: unit id %d not served (strictUnitId is enabled)", h.cfg.Name, remote, unit)
		return modbus.ErrNoResponse
	}
	return nil
}

func exceptionCode(err error) int {
	switch {
	case errors.Is(err, modbus.ErrIllegalFunction):
		return mbcore.ExIllegalFunction
	case errors.Is(err, modbus.ErrIllegalDataAddress):
		return mbcore.ExIllegalDataAddress
	case errors.Is(err, modbus.ErrIllegalDataValue):
		return mbcore.ExIllegalDataValue
	}
	return mbcore.ExServerDeviceFailure
}

// fail logs and counts an exception answer.
func (h *Handler) fail(remote string, unit uint8, pdu []byte, err error) error {
	if err == nil {
		return nil
	}
	h.st.exceptions.Add(1)
	jslog.Log(jslog.LevelBasic, "%s: [%s] request error: unit=%d %s -> exception %s",
		h.cfg.Name, remote, unit, mbcore.DescribeRequest(pdu), mbcore.ExceptionName(exceptionCode(err)))
	return handlerError{err}
}

func u16(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }

func readPDU(fc byte, addr, qty uint16) []byte {
	return append(append([]byte{fc}, u16(addr)...), u16(qty)...)
}

func regsPDU(fc byte, addr uint16, regs []uint16) []byte {
	if fc == mbcore.FCWriteSingleRegister && len(regs) == 1 {
		return append(append([]byte{fc}, u16(addr)...), u16(regs[0])...)
	}
	p := append(append([]byte{fc}, u16(addr)...), u16(uint16(len(regs)))...)
	p = append(p, byte(2*len(regs)))
	for _, r := range regs {
		p = append(p, u16(r)...)
	}
	return p
}

func coilsPDU(fc byte, addr uint16, bits []bool) []byte {
	if fc == mbcore.FCWriteSingleCoil && len(bits) == 1 {
		v := uint16(0)
		if bits[0] {
			v = 0xff00
		}
		return append(append([]byte{fc}, u16(addr)...), u16(v)...)
	}
	p := append(append([]byte{fc}, u16(addr)...), u16(uint16(len(bits)))...)
	data := make([]byte, (len(bits)+7)/8)
	for i, b := range bits {
		if b {
			data[i/8] |= 1 << uint(i%8)
		}
	}
	return append(append(p, byte(len(data))), data...)
}

// HandleCoils serves FC1, FC5 and FC15.
func (h *Handler) HandleCoils(_ context.Context, req *modbus.CoilsRequest) ([]bool, error) {
	remote := h.remote(req.ClientAddr)
	if err := h.gate(req.UnitID, remote); err != nil {
		return nil, err
	}
	fc := byte(req.FunctionCode)
	if !req.IsWrite {
		h.st.reads.Add(1)
		bits, ok := h.pm.ReadBits(int(req.UnitID), mbcore.AreaCoil, int(req.Addr), int(req.Quantity))
		if !ok {
			return nil, h.fail(remote, req.UnitID, readPDU(fc, req.Addr, req.Quantity), modbus.ErrIllegalDataAddress)
		}
		return bits, nil
	}
	h.st.writes.Add(1)
	var err error
	if fc == mbcore.FCWriteSingleCoil && len(req.Args) == 1 {
		err = h.wh.CoilWrite(int(req.UnitID), int(req.Addr), req.Args[0], remote)
	} else {
		err = h.wh.MultiCoilWrite(int(req.UnitID), int(req.Addr), req.Args, remote)
	}
	return nil, h.fail(remote, req.UnitID, coilsPDU(fc, req.Addr, req.Args), err)
}

// HandleDiscreteInputs serves FC2.
func (h *Handler) HandleDiscreteInputs(_ context.Context, req *modbus.DiscreteInputsRequest) ([]bool, error) {
	remote := h.remote(req.ClientAddr)
	if err := h.gate(req.UnitID, remote); err != nil {
		return nil, err
	}
	h.st.reads.Add(1)
	bits, ok := h.pm.ReadBits(int(req.UnitID), mbcore.AreaDiscrete, int(req.Addr), int(req.Quantity))
	if !ok {
		return nil, h.fail(remote, req.UnitID, readPDU(byte(req.FunctionCode), req.Addr, req.Quantity), modbus.ErrIllegalDataAddress)
	}
	return bits, nil
}

// HandleHoldingRegisters serves FC3, FC6 and FC16.
func (h *Handler) HandleHoldingRegisters(_ context.Context, req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	remote := h.remote(req.ClientAddr)
	if err := h.gate(req.UnitID, remote); err != nil {
		return nil, err
	}
	fc := byte(req.FunctionCode)
	if !req.IsWrite {
		h.st.reads.Add(1)
		regs, ok := h.pm.ReadRegisters(int(req.UnitID), mbcore.AreaHolding, int(req.Addr), int(req.Quantity))
		if !ok {
			return nil, h.fail(remote, req.UnitID, readPDU(fc, req.Addr, req.Quantity), modbus.ErrIllegalDataAddress)
		}
		return regs, nil
	}
	h.st.writes.Add(1)
	var err error
	if fc == mbcore.FCWriteSingleRegister && len(req.Args) == 1 {
		err = h.wh.RegisterWrite(int(req.UnitID), int(req.Addr), req.Args[0], remote)
	} else {
		err = h.wh.MultiRegisterWrite(int(req.UnitID), int(req.Addr), req.Args, remote)
	}
	return nil, h.fail(remote, req.UnitID, regsPDU(fc, req.Addr, req.Args), err)
}

// HandleInputRegisters serves FC4.
func (h *Handler) HandleInputRegisters(_ context.Context, req *modbus.InputRegistersRequest) ([]uint16, error) {
	remote := h.remote(req.ClientAddr)
	if err := h.gate(req.UnitID, remote); err != nil {
		return nil, err
	}
	h.st.reads.Add(1)
	regs, ok := h.pm.ReadRegisters(int(req.UnitID), mbcore.AreaInput, int(req.Addr), int(req.Quantity))
	if !ok {
		return nil, h.fail(remote, req.UnitID, readPDU(byte(req.FunctionCode), req.Addr, req.Quantity), modbus.ErrIllegalDataAddress)
	}
	return regs, nil
}

// HandleMaskWrite serves FC22.
func (h *Handler) HandleMaskWrite(_ context.Context, req *modbus.MaskWriteRequest) error {
	remote := h.remote(req.ClientAddr)
	if err := h.gate(req.UnitID, remote); err != nil {
		return err
	}
	h.st.writes.Add(1)
	err := h.wh.MaskWrite(int(req.UnitID), int(req.Addr), req.AndMask, req.OrMask, remote)
	pdu := append(append(append([]byte{mbcore.FCMaskWriteRegister}, u16(req.Addr)...), u16(req.AndMask)...), u16(req.OrMask)...)
	return h.fail(remote, req.UnitID, pdu, err)
}

// metrics reports the exceptions the library answers on its own: function
// codes this server does not serve, quantities out of range.
type metrics struct {
	name string
	st   *counters
}

func (m metrics) OnRequest(uint8, modbus.FunctionCode)                 {}
func (m metrics) OnResponse(uint8, modbus.FunctionCode, time.Duration) {}
func (m metrics) OnError(unit uint8, fc modbus.FunctionCode, _ time.Duration, err error) {
	var he handlerError
	if errors.As(err, &he) || errors.Is(err, modbus.ErrNoResponse) {
		return
	}
	m.st.exceptions.Add(1)
	jslog.Log(jslog.LevelBasic, "%s: request error: unit=%d %s -> %s",
		m.name, unit, mbcore.FCName(int(fc)), describeLibError(err))
}

func describeLibError(err error) string {
	if errors.Is(err, modbus.ErrProtocolError) {
		return "protocol error, link closed"
	}
	return fmt.Sprintf("exception %s", mbcore.ExceptionName(exceptionCode(err)))
}
