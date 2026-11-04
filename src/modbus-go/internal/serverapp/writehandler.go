/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

// Relays master writes into JSON-SCADA commands. A write to a mapped command
// point becomes a commandsQueue insert carrying the point's protocolSource*
// routing, as the UI does, so the client driver owning the point dispatches it
// to the field device. Ported from write-handler.ts.

import (
	"context"
	"time"

	"modbus-go/internal/mbcore"

	"github.com/otfabric/go-modbus"
	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// CommandSink receives relayed commands.
type CommandSink interface {
	Insert(ctx context.Context, doc bson.D) error
}

// MongoCommands is the commandsQueue collection.
type MongoCommands struct{ Coll *mongo.Collection }

// Insert implements CommandSink.
func (m MongoCommands) Insert(ctx context.Context, doc bson.D) error {
	_, err := m.Coll.InsertOne(ctx, doc)
	return err
}

// WriteHandler turns writes into commands.
type WriteHandler struct {
	cfg  Config
	sink CommandSink
	pm   *PointMap
}

// NewWriteHandler binds a handler to its map and command sink.
func NewWriteHandler(cfg Config, sink CommandSink, pm *PointMap) *WriteHandler {
	return &WriteHandler{cfg: cfg, sink: sink, pm: pm}
}

// CoilWrite handles FC5.
func (w *WriteHandler) CoilWrite(unit, addr int, on bool, remote string) error {
	return w.relay(w.pm.LookupWrite(unit, mbcore.AreaCoil, addr), boolValue(on), remote)
}

// RegisterWrite handles FC6.
func (w *WriteHandler) RegisterWrite(unit, addr int, value uint16, remote string) error {
	p := w.pm.LookupWrite(unit, mbcore.AreaHolding, addr)
	if p == nil {
		return modbus.ErrIllegalDataAddress
	}
	return w.relayDecoded(p, []uint16{value}, remote)
}

// MultiCoilWrite handles FC15: each mapped coil starting in the span is relayed.
func (w *WriteHandler) MultiCoilWrite(unit, start int, bits []bool, remote string) error {
	mapped := false
	for i, b := range bits {
		p := w.pm.LookupWrite(unit, mbcore.AreaCoil, start+i)
		if p != nil && p.Offset == start+i {
			mapped = true
			if err := w.relay(p, boolValue(b), remote); err != nil {
				return err
			}
		}
	}
	if !mapped {
		return modbus.ErrIllegalDataAddress
	}
	return nil
}

// MultiRegisterWrite handles FC16: the block is split into the points it fully
// covers; a point written only partly is skipped.
func (w *WriteHandler) MultiRegisterWrite(unit, start int, regs []uint16, remote string) error {
	var covered []*Point
	seen := map[*Point]bool{}
	for i := range regs {
		if p := w.pm.LookupWrite(unit, mbcore.AreaHolding, start+i); p != nil && !seen[p] {
			seen[p] = true
			covered = append(covered, p)
		}
	}
	if len(covered) == 0 {
		return modbus.ErrIllegalDataAddress
	}
	for _, p := range covered {
		rel := p.Offset - start
		if rel < 0 || rel+p.Span > len(regs) {
			continue
		}
		if err := w.relayDecoded(p, regs[rel:rel+p.Span], remote); err != nil {
			return err
		}
	}
	return nil
}

// MaskWrite handles FC22 to a bit-in-register command point. The resulting bit
// is (current AND andMask) OR (orMask AND NOT andMask), as the standard
// defines it, with the current bit taken from the served register.
func (w *WriteHandler) MaskWrite(unit, addr int, andMask, orMask uint16, remote string) error {
	p := w.pm.LookupWrite(unit, mbcore.AreaHolding, addr)
	if p == nil || !p.HasBit() {
		return modbus.ErrIllegalDataAddress
	}
	var cur uint16
	if regs, ok := w.pm.ReadRegisters(unit, mbcore.AreaHolding, addr, 1); ok {
		cur = regs[0]
	}
	res := (cur & andMask) | (orMask &^ andMask)
	return w.relay(p, boolValue((res>>uint(p.Bit))&1 == 1), remote)
}

func boolValue(on bool) mbcore.Value {
	if on {
		return mbcore.Num(1)
	}
	return mbcore.Num(0)
}

func (w *WriteHandler) relayDecoded(p *Point, regs []uint16, remote string) error {
	raw, err := DecodeWrittenValue(p, regs)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s: write from %s to %s failed to decode as %s: %v", w.cfg.Name, remote, p.Tag, p.Asdu.Type, err)
		return modbus.ErrIllegalDataValue
	}
	return w.relay(p, raw, remote)
}

func (w *WriteHandler) relay(p *Point, raw mbcore.Value, remote string) error {
	if p == nil {
		return modbus.ErrIllegalDataAddress
	}
	if !w.cfg.CommandsEnabled {
		jslog.Log(jslog.LevelBasic, "%s: write from %s to %s refused: commands are disabled for this connection", w.cfg.Name, remote, p.Tag)
		return modbus.ErrIllegalFunction
	}
	if !p.IsCommand {
		if w.cfg.AllowWritesToSupervised {
			w.pm.StoreWritten(p, raw)
			return nil
		}
		jslog.Log(jslog.LevelBasic, "%s: write from %s to %s refused: point is supervised, not a command "+
			"(set allowWritesToSupervised to permit direct writes)", w.cfg.Name, remote, p.Tag)
		return modbus.ErrIllegalFunction
	}
	value, valueString := RouteCommandValue(p, raw)
	jslog.Log(jslog.LevelDebug, "%s: routing write from %s as command %s type=%s raw=%s -> value=%s",
		w.cfg.Name, remote, p.Tag, p.TagType, raw.String(), mbcore.JSNumberString(value))
	// Fire and forget: Modbus cannot wait for end-to-end confirmation.
	doc := CommandDoc(w.cfg, p, value, valueString, remote, time.Now())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := w.sink.Insert(ctx, doc); err != nil {
			jslog.Log(jslog.LevelBasic, "%s: command insert failed for %s: %v", w.cfg.Name, p.Tag, err)
			return
		}
		jslog.Log(jslog.LevelBasic, "%s: relayed write to command %s value=%s from %s", w.cfg.Name, p.Tag, mbcore.JSNumberString(value), remote)
	}()
	return nil
}

// CommandDoc is the commandsQueue document of a relayed write, in the Node.js
// driver's field order.
func CommandDoc(cfg Config, p *Point, value float64, valueString, remote string, now time.Time) bson.D {
	connNum := 0.0
	if p.CmdConnectionNumber != nil {
		connNum = *p.CmdConnectionNumber
	}
	var ca any
	if p.CmdCommonAddress != nil {
		ca = *p.CmdCommonAddress
	}
	return bson.D{
		{Key: "protocolSourceConnectionNumber", Value: connNum},
		{Key: "protocolSourceCommonAddress", Value: ca},
		{Key: "protocolSourceObjectAddress", Value: p.CmdObjectAddress},
		{Key: "protocolSourceASDU", Value: p.CmdAsdu},
		{Key: "protocolSourceCommandDuration", Value: 0.0},
		{Key: "protocolSourceCommandUseSBO", Value: false},
		{Key: "pointKey", Value: p.PointKey},
		{Key: "tag", Value: p.Tag},
		{Key: "timeTag", Value: now},
		{Key: "value", Value: value},
		{Key: "valueString", Value: valueString},
		{Key: "originatorUserName", Value: "Protocol connection: " + cfg.Name},
		{Key: "originatorIpAddress", Value: remote},
	}
}
