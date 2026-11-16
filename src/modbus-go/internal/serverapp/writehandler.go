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

// write is one point a master write reaches, with the raw value it carries.
type write struct {
	p   *Point
	raw mbcore.Value
}

// CoilWrite handles FC5.
func (w *WriteHandler) CoilWrite(unit, addr int, on bool, remote string) error {
	return w.run([]write{{w.pm.LookupWrite(unit, mbcore.AreaCoil, addr).Word, boolValue(on)}}, remote)
}

// MultiCoilWrite handles FC15: each mapped coil in the range is written.
func (w *WriteHandler) MultiCoilWrite(unit, start int, bits []bool, remote string) error {
	var ws []write
	for i, b := range bits {
		if p := w.pm.LookupWrite(unit, mbcore.AreaCoil, start+i).Word; p != nil {
			ws = append(ws, write{p, boolValue(b)})
		}
	}
	if len(ws) == 0 {
		return modbus.ErrIllegalDataAddress
	}
	return w.run(ws, remote)
}

// registerTargets picks who a whole-register write at one address reaches: a
// word command point, else the register bit command points, else the
// supervised word point, else the supervised bit points.
func registerTargets(s WriteSlot) (word *Point, bits bool) {
	switch {
	case s.Word != nil && s.Word.IsCommand:
		return s.Word, false
	case s.bitCommands():
		return nil, true
	case s.Word != nil:
		return s.Word, false
	}
	return nil, s.HasBits()
}

// bitWrites are the writes of a register value to the register bit points;
// with commandsOnly, supervised bit points are left out.
func bitWrites(s WriteSlot, value uint16, commandsOnly bool) []write {
	var ws []write
	for bit, p := range s.Bits {
		if p != nil && (p.IsCommand || !commandsOnly) {
			ws = append(ws, write{p, mbcore.Num(float64((value >> uint(bit)) & 1))})
		}
	}
	return ws
}

// RegisterWrite handles FC6. A value of several registers cannot be written
// with it: the write must start at the point and the point occupy one register.
func (w *WriteHandler) RegisterWrite(unit, addr int, value uint16, remote string) error {
	s := w.pm.LookupWrite(unit, mbcore.AreaHolding, addr)
	word, bits := registerTargets(s)
	switch {
	case word != nil:
		if word.Offset != addr || word.Span != 1 {
			jslog.Log(jslog.LevelBasic, "%s: write from %s to %s refused: FC6 writes one register of a %d-register %s",
				w.cfg.Name, remote, word.Tag, word.Span, word.Asdu.Label())
			return modbus.ErrIllegalDataAddress
		}
		raw, err := w.decode(word, []uint16{value}, remote)
		if err != nil {
			return err
		}
		return w.run([]write{{word, raw}}, remote)
	case bits:
		return w.run(bitWrites(s, value, s.bitCommands()), remote)
	}
	return modbus.ErrIllegalDataAddress
}

// MultiRegisterWrite handles FC16: the block is split into the points it fully
// covers, and the register bit points of the registers it writes; a value
// written only in part is skipped.
func (w *WriteHandler) MultiRegisterWrite(unit, start int, regs []uint16, remote string) error {
	var ws []write
	mapped := false
	seen := map[*Point]bool{}
	for i, r := range regs {
		s := w.pm.LookupWrite(unit, mbcore.AreaHolding, start+i)
		word, bits := registerTargets(s)
		switch {
		case word != nil:
			mapped = true
			if seen[word] {
				continue
			}
			seen[word] = true
			rel := word.Offset - start
			if rel < 0 || rel+word.Span > len(regs) {
				jslog.Log(jslog.LevelDetailed, "%s: write from %s to %s skipped: written only in part", w.cfg.Name, remote, word.Tag)
				continue
			}
			raw, err := w.decode(word, regs[rel:rel+word.Span], remote)
			if err != nil {
				return err
			}
			ws = append(ws, write{word, raw})
		case bits:
			mapped = true
			ws = append(ws, bitWrites(s, r, s.bitCommands())...)
		}
	}
	if !mapped || len(ws) == 0 {
		return modbus.ErrIllegalDataAddress
	}
	return w.run(ws, remote)
}

// MaskWrite handles FC22 to register bit points. Each bit the masks change
// (andMask bit 0) is written with (current AND andMask) OR (orMask AND NOT
// andMask), as the standard defines it, the current bit taken from the served
// register. A mask that changes no mapped bit is refused.
func (w *WriteHandler) MaskWrite(unit, addr int, andMask, orMask uint16, remote string) error {
	s := w.pm.LookupWrite(unit, mbcore.AreaHolding, addr)
	var cur uint16
	if regs, ok := w.pm.ReadRegisters(unit, mbcore.AreaHolding, addr, 1); ok {
		cur = regs[0]
	}
	res := (cur & andMask) | (orMask &^ andMask)
	var ws []write
	for bit, p := range s.Bits {
		if p != nil && andMask&(1<<uint(bit)) == 0 {
			ws = append(ws, write{p, boolValue((res>>uint(bit))&1 == 1)})
		}
	}
	if len(ws) == 0 {
		return modbus.ErrIllegalDataAddress
	}
	return w.run(ws, remote)
}

func boolValue(on bool) mbcore.Value {
	if on {
		return mbcore.Num(1)
	}
	return mbcore.Num(0)
}

func (w *WriteHandler) decode(p *Point, regs []uint16, remote string) (mbcore.Value, error) {
	raw, err := DecodeWrittenValue(p, regs)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s: write from %s to %s failed to decode as %s: %v", w.cfg.Name, remote, p.Tag, p.Asdu.Type, err)
		return mbcore.Value{}, modbus.ErrIllegalDataValue
	}
	return raw, nil
}

// check refuses a write the connection does not take.
func (w *WriteHandler) check(p *Point, remote string) error {
	if p == nil {
		return modbus.ErrIllegalDataAddress
	}
	if !w.cfg.CommandsEnabled {
		jslog.Log(jslog.LevelBasic, "%s: write from %s to %s refused: commands are disabled for this connection", w.cfg.Name, remote, p.Tag)
		return modbus.ErrIllegalFunction
	}
	if !p.IsCommand && !w.cfg.AllowWritesToSupervised {
		jslog.Log(jslog.LevelBasic, "%s: write from %s to %s refused: point is supervised, not a command "+
			"(set allowWritesToSupervised to permit direct writes)", w.cfg.Name, remote, p.Tag)
		return modbus.ErrIllegalFunction
	}
	return nil
}

// run checks every write first, so a refused one leaves the others undone,
// then stores or relays each.
func (w *WriteHandler) run(ws []write, remote string) error {
	if len(ws) == 0 {
		return modbus.ErrIllegalDataAddress
	}
	for _, x := range ws {
		if err := w.check(x.p, remote); err != nil {
			return err
		}
	}
	for _, x := range ws {
		if x.p.IsCommand {
			w.relay(x.p, x.raw, remote)
		} else {
			w.pm.StoreWritten(x.p, x.raw)
			jslog.Log(jslog.LevelDetailed, "%s: write from %s stored in supervised %s raw=%s", w.cfg.Name, remote, x.p.Tag, x.raw.String())
		}
	}
	return nil
}

// relay inserts the command of a checked write.
func (w *WriteHandler) relay(p *Point, raw mbcore.Value, remote string) {
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
