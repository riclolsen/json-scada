/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

// Commands: commandsQueue inserts for this connection become go-sunspec
// WritePoint calls (FC16 of the point's registers, after a read of the model
// that checks its header and scale factor), then the command document is
// acknowledged. Only top-level points of a model can be written.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/drv"
	"modbus-go/internal/mbsec"

	"github.com/otfabric/go-sunspec"
	"github.com/riclolsen/json-scada/src/go-common/jscommands"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func fieldStr(doc bson.M, key string) string {
	if doc[key] == nil {
		return ""
	}
	return drv.AnyString(doc[key])
}

func (c *Connection) ack(ctx context.Context, doc bson.M, ok bool, desc string) {
	if c.Commands == nil {
		return
	}
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := c.Commands.Ack(actx, doc["_id"], ok, desc); err != nil {
		c.log.Error(fmt.Sprintf("command ack update failed: %v", err))
		return
	}
	res := "NAK"
	if ok {
		res = "ACK"
	}
	jslog.Log(jslog.LevelBasic, "%s: command %s -> %s (%s)", c.Cfg.Name(), fieldStr(doc, "tag"), res, desc)
}

// HandleCommand executes one commandsQueue document.
func (c *Connection) HandleCommand(ctx context.Context, doc bson.M) {
	if !c.Cfg.Link.CommandsEnabled {
		c.log.Info("command ignored, commands are disabled for this connection")
		return
	}
	if c.Active != nil && !c.Active() {
		c.log.Info("command ignored, this node is not the active one")
		return
	}
	if c.expired(doc) {
		c.ack(ctx, doc, false, "command expired")
		return
	}
	if c.currentClient() == nil || !c.Connected() {
		c.ack(ctx, doc, false, "connection down")
		return
	}
	ok, desc := c.executeCommand(ctx, doc)
	c.ack(ctx, doc, ok, desc)
}

// expired reports (and logs as a security event) a command older than the
// expiry, so a restart or a long wait for the link does not replay it.
func (c *Connection) expired(doc bson.M) bool {
	ts := jsmongo.GetTime(doc, "timeTag")
	if ts.IsZero() || time.Since(ts) <= jscommands.DefaultExpiry {
		return false
	}
	c.secCommandStale(doc, ts)
	return true
}

// errNotSent stops a command in the gate before it reaches the device.
type errNotSent struct{ desc string }

func (e errNotSent) Error() string { return e.desc }

func (c *Connection) executeCommand(ctx context.Context, doc bson.M) (bool, string) {
	refuse := func(kind, desc string) (bool, string) {
		c.secCommandRefused(doc, kind, desc, nil)
		return false, desc
	}
	a, err := ParseAddress(fieldStr(doc, "protocolSourceObjectAddress"))
	if err != nil {
		return refuse(mbsec.KindCmdObjectUnknown, err.Error())
	}
	unit := drv.ToNum(doc["protocolSourceCommonAddress"], doc["protocolSourceCommonAddress"] != nil, 1)
	if math.IsNaN(unit) || unit != math.Trunc(unit) || unit < 1 || unit > 255 {
		return refuse(mbsec.KindCmdObjectUnknown, "invalid unit id "+fieldStr(doc, "protocolSourceCommonAddress"))
	}
	c.mu.Lock()
	us := c.units[int(unit)]
	c.mu.Unlock()
	if us == nil {
		return refuse(mbsec.KindCmdObjectUnknown, fmt.Sprintf("no SunSpec device discovered at unit %d", int(unit)))
	}
	inst, ok := instanceOf(us.dev.Discovery, a)
	if !ok || inst.Schema == nil || inst.Schema.Group == nil {
		return refuse(mbsec.KindCmdObjectUnknown, fmt.Sprintf("model %s not in the device's chain", a.String()))
	}
	if !a.TopLevel() {
		return refuse(mbsec.KindCmdRejected, "only top-level points of a model can be written")
	}
	pm := inst.Schema.Group.Point(a.Point)
	if pm == nil {
		return refuse(mbsec.KindCmdObjectUnknown, fmt.Sprintf("no point %s in model %d", a.Point, a.ModelID))
	}
	num := drv.ToNum(doc["value"], doc["value"] != nil, math.NaN())
	var val any = num
	numeric := !isStringType(pm.Type)
	if !numeric {
		s, ok := doc["valueString"].(string)
		if !ok {
			return refuse(mbsec.KindCmdRejected, "a string point needs valueString")
		}
		val = s
	} else if math.IsNaN(num) || math.IsInf(num, 0) {
		return refuse(mbsec.KindCmdRejected, "invalid value for a numeric point: "+fieldStr(doc, "value"))
	}
	c.secCommand(ctx, doc, num, numeric)

	var p *sunspec.DecodedPoint
	var seq uint64
	err = c.exec(ctx, true, false, func(ctx context.Context) error {
		// The gate may have waited for a poll: check again what HandleCommand checked.
		if c.Active != nil && !c.Active() {
			return errNotSent{"this node is no longer the active one"}
		}
		if c.expired(doc) {
			return errNotSent{"command expired"}
		}
		c.mu.Lock()
		same := c.units[int(unit)] == us
		c.mu.Unlock()
		if !c.Connected() || us.dev.Client != c.currentClient() {
			return errNotSent{"connection down"}
		}
		if !same {
			return errNotSent{"the device was discovered again, command not sent"}
		}
		var e error
		seq = c.nextSeq()
		p, e = us.dev.WritePoint(ctx, inst, a.Point, val)
		return e
	})
	var ns errNotSent
	if errors.As(err, &ns) {
		return false, ns.desc
	}
	if err != nil {
		if isException(err) || linkError(err) {
			c.countError(err)
		}
		switch {
		case errors.Is(err, sunspec.ErrModelChanged):
			c.mu.Lock()
			us.rediscover = true
			c.mu.Unlock()
			return refuse(mbsec.KindCmdRejected, "the device's model chain changed, discovering again")
		case errors.Is(err, sunspec.ErrPointNotFound):
			return refuse(mbsec.KindCmdObjectUnknown, err.Error())
		case isException(err):
			c.secCommandRefused(doc, "", "", err)
		case !linkError(err):
			// not writable, a value the encoder refuses, a scale factor not usable
			return refuse(mbsec.KindCmdRejected, err.Error())
		}
		return false, clientapp.ErrText(err)
	}
	c.stat(func(s *Stats) { s.Writes++ })
	c.publishWritten(int(unit), a, p, seq)
	return true, "OK"
}

// publishWritten publishes the value the device reports after a write to
// the supervised tags of the point.
func (c *Connection) publishWritten(unit int, a Address, p *sunspec.DecodedPoint, seq uint64) {
	if p == nil {
		return
	}
	key := a.String()
	c.mu.Lock()
	var targets []*binding
	for _, b := range c.bindings {
		if b.Unit == unit && b.Addr.String() == key {
			targets = append(targets, b)
		}
	}
	c.mu.Unlock()
	if len(targets) == 0 {
		return
	}
	u := newUpdater(c.Cfg.ConnNumber())
	for _, b := range targets {
		c.publish(u, b, PointValue(p), "3", false, seq)
	}
	wctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.flush(wctx, u); err != nil {
		jslog.Log(jslog.LevelBasic, "%s: realtimeData write error: %v", c.Cfg.Name(), err)
	}
}
