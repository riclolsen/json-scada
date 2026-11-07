/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

// Commands: commandsQueue inserts for this connection become Modbus writes
// (FC5/6/16/22, or FC3+FC6 read-modify-write), then the command document is
// acknowledged. Ported from src/modbus/src/client/commands.ts.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"

	"github.com/otfabric/go-modbus"
	"github.com/riclolsen/json-scada/src/go-common/jscommands"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CommandStore is where commands come from and where their outcome goes.
type CommandStore interface {
	// Watch calls onInsert for each command inserted for the connection until
	// ctx ends, re-establishing the watch after an error.
	Watch(ctx context.Context, connNumber float64, onInsert func(doc bson.M))
	// Ack records the outcome: delivered, ack, ackTimeTag, resultDescription.
	Ack(ctx context.Context, id any, ok bool, resultDescription string) error
}

// MongoCommands is the commandsQueue collection.
type MongoCommands struct {
	Coll *mongo.Collection
	Name string // connection name, for log lines
}

// Watch implements CommandStore with a change stream on inserts.
func (m MongoCommands) Watch(ctx context.Context, connNumber float64, onInsert func(doc bson.M)) {
	pipeline := mongo.Pipeline{bson.D{{Key: "$match", Value: bson.D{
		{Key: "operationType", Value: "insert"},
		{Key: "fullDocument.protocolSourceConnectionNumber", Value: connNumber},
	}}}}
	for ctx.Err() == nil {
		cs, err := m.Coll.Watch(ctx, pipeline, options.ChangeStream().SetFullDocument(options.UpdateLookup))
		if err != nil {
			if ctx.Err() == nil {
				jslog.Log(jslog.LevelBasic, "%s: command watch failed: %v", m.Name, err)
			}
		} else {
			for cs.Next(ctx) {
				var ev struct {
					FullDocument bson.M `bson:"fullDocument"`
				}
				if err := cs.Decode(&ev); err == nil && ev.FullDocument != nil {
					onInsert(ev.FullDocument)
				}
			}
			if err := cs.Err(); err != nil && ctx.Err() == nil {
				jslog.Log(jslog.LevelBasic, "%s: command stream error: %v", m.Name, err)
			}
			_ = cs.Close(context.Background())
		}
		if !sleepCtx(ctx, 5*time.Second) {
			return
		}
	}
}

// Ack implements CommandStore.
func (m MongoCommands) Ack(ctx context.Context, id any, ok bool, desc string) error {
	return jscommands.Ack(ctx, m.Coll, id, ok, desc)
}

// HandleCommand executes one commandsQueue document.
func (c *Connection) HandleCommand(ctx context.Context, doc bson.M) {
	if c.log.DebugEnabled() {
		c.log.Debug(fmt.Sprintf("command received tag=%s addr=%s asdu=%s unit=%s value=%s",
			fieldStr(doc, "tag"), fieldStr(doc, "protocolSourceObjectAddress"), fieldStr(doc, "protocolSourceASDU"),
			fieldStr(doc, "protocolSourceCommonAddress"), fieldStr(doc, "value")))
	}
	if !c.Cfg.CommandsEnabled {
		c.log.Info("command ignored, commands are disabled for this connection")
		return
	}
	if c.Active != nil && !c.Active() {
		c.log.Info("command ignored, this node is not the active one")
		return
	}
	// Ignore stale commands, so a restart does not replay them.
	if ts := jsmongo.GetTime(doc, "timeTag"); !ts.IsZero() && time.Since(ts) > jscommands.DefaultExpiry {
		c.ack(ctx, doc, false, "command expired")
		return
	}
	cl := c.currentClient()
	if cl == nil || !c.Connected() {
		c.ack(ctx, doc, false, "connection down")
		return
	}
	ok, desc := c.executeCommand(ctx, cl, doc)
	c.ack(ctx, doc, ok, desc)
}

func fieldStr(doc bson.M, k string) string {
	v, ok := doc[k]
	if !ok || v == nil {
		return ""
	}
	return drv.AnyString(v)
}

func (c *Connection) executeCommand(ctx context.Context, cl *modbus.Client, doc bson.M) (bool, string) {
	addr, err := mbcore.ParseObjectAddress(fieldStr(doc, "protocolSourceObjectAddress"), c.Cfg.UseModiconAddresses)
	if err != nil {
		return false, err.Error()
	}
	asduStr, _ := doc["protocolSourceASDU"].(string)
	asdu, err := mbcore.ParseAsdu(asduStr, addr.Area, c.Cfg.Defaults())
	if err != nil {
		return false, err.Error()
	}
	ca, hasCA := doc["protocolSourceCommonAddress"]
	unit := clampUnitDefault1(jsNumber(ca, hasCA))
	val, hasVal := doc["value"]
	if !hasVal || val == nil {
		val, hasVal = 0.0, true
	}
	rawValue := jsNumber(val, hasVal)
	kconv1 := c.lookupKconv(doc, "kconv1", 1)
	kconv2 := c.lookupKconv(doc, "kconv2", 0)

	broadcast := unit == 0
	var pdu []byte
	var op func(ctx context.Context) error
	u8, a16 := uint8(unit), uint16(addr.Offset)

	switch {
	case mbcore.IsBitArea(addr.Area) || asdu.Type == mbcore.TypeBool:
		on := rawValue != 0
		if kconv1 == -1 {
			on = !on
		}
		pdu = []byte{mbcore.FCWriteSingleCoil, byte(a16 >> 8), byte(a16), 0, 0}
		if on {
			pdu[3] = 0xff
		}
		op = func(ctx context.Context) error { return cl.WriteCoil(ctx, u8, a16, on) }
	case addr.HasBit():
		on := rawValue != 0
		bit := uint(addr.Bit)
		if c.Cfg.UseMaskWrite || broadcast { // a broadcast cannot be read back
			andMask := ^uint16(1<<bit) & 0xffff
			orMask := uint16(0)
			if on {
				orMask = 1 << bit
			}
			pdu = []byte{mbcore.FCMaskWriteRegister, byte(a16 >> 8), byte(a16), byte(andMask >> 8), byte(andMask), byte(orMask >> 8), byte(orMask)}
			op = func(ctx context.Context) error { return cl.MaskWriteRegister(ctx, u8, a16, andMask, orMask) }
			break
		}
		// Read-modify-write fallback: FC3 then FC6.
		var word uint16
		rd := readRequestPDU(mbcore.FCReadHoldingRegisters, addr.Offset, 1)
		err := c.exec(ctx, true, unit, rd, func(ctx context.Context) error {
			var e error
			word, e = cl.ReadRegister(ctx, u8, a16, modbus.HoldingRegister)
			return e
		})
		if err != nil {
			return false, ErrText(err)
		}
		if on {
			word |= 1 << bit
		} else {
			word &^= 1 << bit
		}
		pdu = []byte{mbcore.FCWriteSingleRegister, byte(a16 >> 8), byte(a16), byte(word >> 8), byte(word)}
		w := word
		op = func(ctx context.Context) error { return cl.WriteRegister(ctx, u8, a16, w) }
	default:
		// Inverse engineering scaling: raw = (value - kconv2) / kconv1.
		v := mbcore.Num(rawValue)
		if asdu.Type == mbcore.TypeString {
			s, _ := doc["valueString"].(string)
			v = mbcore.Str(s)
		} else if kconv1 != 0 {
			v = mbcore.Num((rawValue - kconv2) / kconv1)
		}
		wire, err := mbcore.Encode(asdu.Type, v, asdu.Perm, asdu.Str)
		if err != nil {
			return false, err.Error()
		}
		if asdu.RegCount == 1 && asdu.Type != mbcore.TypeString {
			w := uint16(wire[0])<<8 | uint16(wire[1])
			pdu = []byte{mbcore.FCWriteSingleRegister, byte(a16 >> 8), byte(a16), wire[0], wire[1]}
			op = func(ctx context.Context) error { return cl.WriteRegister(ctx, u8, a16, w) }
		} else {
			regs := make([]uint16, len(wire)/2)
			for i := range regs {
				regs[i] = uint16(wire[2*i])<<8 | uint16(wire[2*i+1])
			}
			pdu = append([]byte{mbcore.FCWriteMultipleRegisters, byte(a16 >> 8), byte(a16), byte(len(regs) >> 8), byte(len(regs)), byte(len(wire))}, wire...)
			op = func(ctx context.Context) error { return cl.WriteRegisters(ctx, u8, a16, regs) }
		}
	}

	c.log.Debug(fmt.Sprintf("dispatching command %s unit=%d", mbcore.DescribeRequest(pdu), unit))
	if broadcast {
		// Unit 0: the devices execute and do not answer. Do not wait for the
		// full timeout and do not retry.
		bctx, cancel := context.WithTimeout(ctx, broadcastWait(c.Cfg))
		defer cancel()
		c.gate.acquire(true)
		err := op(bctx)
		sleepCtx(ctx, msDur(c.Cfg.InterRequestDelayMs))
		c.gate.release()
		if err != nil && !errors.Is(err, modbus.ErrRequestTimedOut) && !errors.Is(err, context.DeadlineExceeded) {
			return false, ErrText(err)
		}
		return true, "OK (broadcast, no confirmation)"
	}
	if err := c.exec(ctx, true, unit, pdu, op); err != nil {
		return false, ErrText(err)
	}
	return true, "OK"
}

// broadcastWait is how long a broadcast write is given on the wire: the RTU
// turnaround delay, or 100 ms, never more than the response timeout.
func broadcastWait(cfg Config) time.Duration {
	d := 100 * time.Millisecond
	if t := msDur(cfg.TimeoutMs); t > 0 && t < d {
		d = t
	}
	return d
}

// lookupKconv takes the scaling from the supervised binding whose _id is the
// command's pointKey, then from the command document, then the default.
func (c *Connection) lookupKconv(doc bson.M, field string, def float64) float64 {
	if pk, ok := doc["pointKey"]; ok && pk != nil {
		if b, ok := c.BindingByKey(drv.AnyString(pk)); ok {
			if field == "kconv1" {
				return b.Kconv1
			}
			return b.Kconv2
		}
	}
	v, ok := doc[field]
	f := jsNumber(v, ok)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return def
	}
	return f
}

func (c *Connection) ack(ctx context.Context, doc bson.M, ok bool, desc string) {
	if c.Commands == nil {
		return
	}
	actx := context.WithoutCancel(ctx)
	if err := c.Commands.Ack(actx, doc["_id"], ok, desc); err != nil {
		jslog.Log(jslog.LevelBasic, "%s: command ack update failed: %v", c.Cfg.Name, err)
		return
	}
	name := doc["tag"]
	if name == nil {
		name = doc["_id"]
	}
	res := "NAK"
	if ok {
		res = "ACK"
	}
	jslog.Log(jslog.LevelBasic, "%s: command %s -> %s (%s)", c.Cfg.Name, drv.AnyString(name), res, desc)
}
