/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

// Control handling: an IEC 61850 control operation becomes a commandsQueue
// document, which JSON-SCADA routes back to the driver that owns the source
// point (IEC 61850-90-2 control pass-through).

package main

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// commandQueue holds commands between the MMS handler and the inserter, so
// the handler never blocks on MongoDB.
var commandQueue struct {
	mu    sync.Mutex
	items []bson.M
}

func enqueueCommand(doc bson.M) {
	commandQueue.mu.Lock()
	commandQueue.items = append(commandQueue.items, doc)
	commandQueue.mu.Unlock()
}

func dequeueCommand() (bson.M, bool) {
	commandQueue.mu.Lock()
	defer commandQueue.mu.Unlock()
	if len(commandQueue.items) == 0 {
		return nil, false
	}
	d := commandQueue.items[0]
	commandQueue.items = commandQueue.items[1:]
	return d, true
}

// installControlHandlers registers a handler per command point.
func installControlHandlers(g *Gateway) {
	if !g.conn.CommandsEnabled {
		jslog.Log(jslog.LevelBasic, "Commands are disabled for this connection - no control handlers installed.")
		return
	}
	count := 0
	for ref, mp := range g.built.ByCtlObjRef {
		point := mp
		g.srv.OnControl(model.ObjectReference(ref), func(ctx *server.ControlCtx) model.AddCause {
			return g.handleControl(point, ctx)
		})
		count++
	}
	jslog.Log(jslog.LevelBasic, "Installed control handlers for %d command point(s).", count)
}

// handleControl validates a control operation and queues the command.
func (g *Gateway) handleControl(mp *MappedPoint, ctx *server.ControlCtx) model.AddCause {
	if !g.conn.CommandsEnabled {
		jslog.Log(jslog.LevelBasic, "Control refused for %s: commands are disabled for this connection.", mp.Tag)
		return model.AddCauseBlockedByMode
	}
	if ctx.Select {
		// The selection is accepted; only an operate produces a command.
		return model.AddCauseNone
	}

	value, valueString, ok := controlValue(mp.Kind, ctx.Value)
	if !ok {
		jslog.Log(jslog.LevelNoLog, "Control value conversion error for %s", mp.Tag)
		return model.AddCauseInconsistentParameters
	}
	value, valueString = convertCommandValue(mp, value, valueString)

	enqueueCommand(bson.M{
		"protocolSourceConnectionNumber": mp.SrcConnectionNumber,
		"protocolSourceCommonAddress":    mp.SrcCommonAddress,
		"protocolSourceObjectAddress":    mp.SrcObjectAddress,
		"protocolSourceASDU":             mp.SrcASDU,
		"protocolSourceCommandDuration":  mp.SrcCommandDuration,
		"protocolSourceCommandUseSBO":    mp.SrcCommandUseSBO,
		"pointKey":                       mp.PointKey,
		"tag":                            mp.Tag,
		"value":                          value,
		"valueString":                    valueString,
		"originatorUserName": "IEC61850 connection: " +
			strconv.Itoa(g.conn.ProtocolConnectionNumber) + " " + g.conn.Name,
		"originatorIpAddress": ctx.Peer,
		"timeTag":             bson.NewDateTimeFromTime(time.Now().UTC()),
	})
	jslog.Log(jslog.LevelBasic, "Command queued: %s = %s (from %s)", mp.Tag, valueString, ctx.Peer)

	// Fire and forget: JSON-SCADA routes and acknowledges asynchronously.
	return model.AddCauseNone
}

// convertCommandValue applies the tag's kconv1/kconv2 to the value that goes
// into commandsQueue, so the driver that owns the source point receives the
// value in its own scale (same rules as the lib60870 server drivers).
//
//   - digital: the value is forced to 1/0, and inverted when kconv1 is -1.
//   - analog: value * kconv1 + kconv2.
//   - anything else (string, json): left untouched.
func convertCommandValue(mp *MappedPoint, value float64, valueString string) (float64, string) {
	switch mp.Type {
	case "digital":
		on := value != 0
		if mp.Kconv1 == -1 {
			on = !on
		}
		if on {
			return 1, "true"
		}
		return 0, "false"
	case "analog":
		v := value*mp.Kconv1 + mp.Kconv2
		return v, formatFloat(v)
	}
	return value, valueString
}

// controlValue converts a ctlVal into the value pair a commandsQueue
// document carries.
func controlValue(kind PointKind, v *mms.Value) (float64, string, bool) {
	if v == nil {
		return 0, "", false
	}
	switch kind {
	case KindSPC:
		if v.Type() != mms.TypeBoolean {
			return 0, "", false
		}
		if v.Bool() {
			return 1, "true", true
		}
		return 0, "false", true
	case KindAPC:
		// An analogue control carries an AnalogueValue: {f} or {i}.
		f, ok := analogueValue(v)
		if !ok {
			return 0, "", false
		}
		return f, formatFloat(f), true
	case KindINC:
		if v.Type() != mms.TypeInteger && v.Type() != mms.TypeUnsigned {
			return 0, "", false
		}
		n := float64(v.Int64())
		return n, strconv.FormatInt(int64(n), 10), true
	}
	switch v.Type() {
	case mms.TypeBoolean:
		if v.Bool() {
			return 1, "true", true
		}
		return 0, "false", true
	case mms.TypeInteger, mms.TypeUnsigned:
		n := float64(v.Int64())
		return n, strconv.FormatInt(int64(n), 10), true
	case mms.TypeFloat32, mms.TypeFloat64:
		return v.Float64(), formatFloat(v.Float64()), true
	}
	return 0, "", false
}

// analogueValue extracts the number from an AnalogueValue structure, or
// from a bare number when a client sends one.
func analogueValue(v *mms.Value) (float64, bool) {
	switch v.Type() {
	case mms.TypeStructure:
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			if e == nil {
				continue
			}
			switch e.Type() {
			case mms.TypeFloat32, mms.TypeFloat64:
				return e.Float64(), true
			case mms.TypeInteger, mms.TypeUnsigned:
				return float64(e.Int64()), true
			}
		}
	case mms.TypeFloat32, mms.TypeFloat64:
		return v.Float64(), true
	case mms.TypeInteger, mms.TypeUnsigned:
		return float64(v.Int64()), true
	}
	return 0, false
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'G', -1, 64)
}

// commandInserterLoop persists queued commands into commandsQueue.
func commandInserterLoop(ctx context.Context, cfg jsconfig.Config) {
	var coll *mongo.Collection
	var cli *mongo.Client

	for ctx.Err() == nil {
		if coll == nil {
			c, _, err := jsmongo.ConnectAndPing(cfg)
			if err != nil {
				time.Sleep(3 * time.Second)
				continue
			}
			cli = c
			coll = cli.Database(cfg.MongoDatabaseName).Collection(jsmongo.CommandsQueueCollectionName)
		}

		doc, ok := dequeueCommand()
		if !ok {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		insCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := coll.InsertOne(insCtx, doc)
		cancel()
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "commandsQueue insert error: %v", err)
			// Put it back at the head and retry with a fresh connection.
			commandQueue.mu.Lock()
			commandQueue.items = append([]bson.M{doc}, commandQueue.items...)
			commandQueue.mu.Unlock()
			if cli != nil {
				_ = cli.Disconnect(context.Background())
			}
			cli, coll = nil, nil
			time.Sleep(1 * time.Second)
		}
	}
	if cli != nil {
		_ = cli.Disconnect(context.Background())
	}
}
