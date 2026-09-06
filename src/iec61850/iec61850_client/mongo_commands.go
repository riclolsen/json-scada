/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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

// Command execution. Port of MongoCommands.cs: a change stream on
// commandsQueue picks up inserts, and each command is either a plain MMS
// write (any functional constraint other than CO) or a control operation
// (CO), after which the queue document is acknowledged.

package main

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jscommands"
	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// commandExpiry is how old a queued command may be before it is refused.
//
// deviation D1: the C# driver tests the seconds component of the elapsed
// time, which wraps every minute and lets much older commands through. This
// uses the total elapsed time, like every other json-scada driver.
const commandExpiry = 10 * time.Second

// commandsLoop watches commandsQueue for inserted commands.
func commandsLoop(ctx context.Context, cfg jsconfig.Config, conns []*Iec61850Connection) {
	for ctx.Err() == nil {
		cli, _, err := jsmongo.ConnectAndPing(cfg)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception MongoCmd")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		db := cli.Database(cfg.MongoDatabaseName)
		collCmds := db.Collection(jsmongo.CommandsQueueCollectionName)

		if err := watchCommands(ctx, db, collCmds, conns); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelNoLog, "Exception MongoCmd")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(3 * time.Second)
		}
		_ = cli.Disconnect(context.Background())
	}
}

func watchCommands(ctx context.Context, db *mongo.Database, collCmds *mongo.Collection, conns []*Iec61850Connection) error {
	if err := jsmongo.Ping(db, 1*time.Second); err != nil {
		return err
	}

	pipeline := jscommands.InsertOnlyPipeline()
	cs, err := collCmds.Watch(ctx, pipeline, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		return err
	}
	defer cs.Close(context.Background())

	jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - Start listening for commands via changestream...")

	for cs.Next(ctx) {
		var ev struct {
			FullDocument bson.M `bson:"fullDocument"`
		}
		if err := cs.Decode(&ev); err != nil {
			jslog.Log(jslog.LevelDetailed, "MongoDB CMD CS - decode: %v", err)
			continue
		}
		if !redundancy.Active() {
			continue
		}
		handleCommand(ctx, collCmds, conns, ev.FullDocument)
	}
	if err := cs.Err(); err != nil {
		return err
	}
	return errors.New("command change stream closed")
}

// handleCommand validates one queued command and dispatches it.
func handleCommand(ctx context.Context, collCmds *mongo.Collection, conns []*Iec61850Connection, doc bson.M) {
	if doc == nil {
		return
	}
	connNumber := jsmongo.GetInt(doc, "protocolSourceConnectionNumber", 0)
	jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - Looking for connection %d...", connNumber)

	var conn *Iec61850Connection
	for _, c := range conns {
		if c.ProtocolConnectionNumber == connNumber {
			conn = c
			break
		}
	}
	if conn == nil {
		return // not for a connection of this driver instance
	}

	id := doc["_id"]
	objAddr := strings.TrimSpace(jsmongo.GetString(doc, "protocolSourceObjectAddress", ""))
	commonAddr := strings.ToUpper(strings.TrimSpace(jsmongo.GetString(doc, "protocolSourceCommonAddress", "")))
	value := jsmongo.GetDouble(doc, "value", 0)
	useSBO := jsmongo.GetBool(doc, "protocolSourceCommandUseSBO", false)
	timeTag := jsmongo.GetTime(doc, "timeTag")

	if !timeTag.IsZero() {
		if elapsed := time.Since(timeTag); elapsed > commandExpiry {
			jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s -  Address %s value %v Expired, %d Seconds old",
				conn.Name, objAddr, value, int(elapsed.Seconds()))
			cancelCommand(ctx, collCmds, id, "expired")
			return
		}
	}

	entry := conn.Entry(objAddr + commonAddr)
	cli := conn.Client()
	connected := cli != nil && cli.State() == mms.StateConnected

	if !connected || !conn.CommandsEnabled || entry == nil {
		reason := "command not found!"
		switch {
		case !conn.CommandsEnabled:
			reason = "commands disabled"
		case entry != nil:
			reason = "not connected"
		}
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s OA %s value %v %s", conn.Name, objAddr, value, capitalizeReason(reason))
		cancelCommand(ctx, collCmds, id, reason)
		return
	}

	jslog.Log(jslog.LevelNoLog, "%s Control %s Value %v", conn.Name, entry.Path, value)

	ok, abort := dispatchCommand(ctx, conn, entry, value, useSBO)
	if abort {
		// The C# driver leaves the queue document untouched when it cannot
		// reach the object at all.
		return
	}

	jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s -  Address: %s - Command delivered - ", conn.Name, objAddr)
	ackCommand(ctx, collCmds, id, ok, "")
}

// capitalizeReason renders the cancel reason the way the C# log line does.
func capitalizeReason(reason string) string {
	switch reason {
	case "commands disabled":
		return "Commands Disabled"
	case "not connected":
		return "Not connected"
	}
	return "Command not found!"
}

func cancelCommand(ctx context.Context, collCmds *mongo.Collection, id any, reason string) {
	if err := jscommands.Cancel(ctx, collCmds, id, reason); err != nil {
		jslog.Log(jslog.LevelDetailed, "MongoDB CMD CS - update: %v", err)
	}
}

// ackCommand records the outcome of a command that reached the IED.
func ackCommand(ctx context.Context, collCmds *mongo.Collection, id any, ok bool, resultDescription string) {
	if err := jscommands.Ack(ctx, collCmds, id, ok, resultDescription); err != nil {
		jslog.Log(jslog.LevelDetailed, "MongoDB CMD CS - update: %v", err)
	}
}

// dispatchCommand sends one command to the IED. It returns whether the
// command succeeded, and whether the queue document should be left alone.
func dispatchCommand(ctx context.Context, conn *Iec61850Connection, entry *Iec61850Entry, value float64, useSBO bool) (ok bool, abort bool) {
	reqCtx, cancel := context.WithTimeout(ctx, conn.requestTimeout())
	defer cancel()

	if entry.FC != model.CO {
		return writeValueCommand(reqCtx, conn, entry, value)
	}
	return controlCommand(reqCtx, conn, entry, value, useSBO)
}

// writeValueCommand performs a plain MMS write, choosing the written type
// from the type the object currently has.
func writeValueCommand(ctx context.Context, conn *Iec61850Connection, entry *Iec61850Entry, value float64) (bool, bool) {
	cli := conn.Client()
	ref := model.ObjectReference(entry.Path)

	current, err := cli.Read(ctx, ref, entry.FC)
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "%s Writable object not found! %s", conn.Name, entry.Path)
		jslog.Log(jslog.LevelNoLog, "%v", err)
		return false, true
	}

	var out *mms.Value
	switch current.Type() {
	case mms.TypeBoolean:
		out = mms.NewBool(value != 0)
	case mms.TypeUnsigned:
		out = mms.NewUint32(uint32(value))
	case mms.TypeInteger:
		out = mms.NewInt64(int64(value))
	case mms.TypeFloat32:
		out = mms.NewFloat32(float32(value))
	case mms.TypeFloat64:
		out = mms.NewFloat64(value)
	case mms.TypeVisibleString:
		out = mms.NewVisibleString(formatDouble(value))
	case mms.TypeMMSString:
		out = mms.NewMMSString(formatDouble(value))
	case mms.TypeBitString:
		bs := mms.NewBitString(current.BitLen())
		setBitsFromUint32(bs, uint32(value))
		out = bs
	case mms.TypeUTCTime:
		out = mms.NewUTCTime(time.UnixMilli(int64(value)).UTC(), mms.TimeAccuracy(10))
	case mms.TypeBinaryTime:
		out = mms.NewBinaryTime(time.UnixMilli(int64(value)).UTC())
	case mms.TypeOctetString:
		b := make([]byte, len(current.Bytes()))
		if len(b) == 0 {
			b = make([]byte, 1)
		}
		b[0] = byte(uint32(value) % 256)
		out = mms.NewOctetString(b)
	default:
		jslog.Log(jslog.LevelNoLog, "%s Writable object of unsupported type! %s", conn.Name, entry.Path)
		return false, false
	}

	if err := cli.Write(ctx, ref, entry.FC, out); err != nil {
		jslog.Log(jslog.LevelNoLog, "%s Write failed! %s - %v", conn.Name, entry.Path, err)
		return false, false
	}
	return true, false
}

// setBitsFromUint32 fills a bit string the way libiec61850's
// BitStringFromUInt32 does: bit i of the integer goes to bit i.
func setBitsFromUint32(bs *mms.Value, v uint32) {
	for i := 0; i < bs.BitLen(); i++ {
		bs.SetBit(i, v&1 == 1)
		v >>= 1
	}
}

// controlCommand operates a controllable object, following its control
// model. The library runs the select step of an SBO model itself, reusing
// the control number across the sequence.
func controlCommand(ctx context.Context, conn *Iec61850Connection, entry *Iec61850Entry, value float64, useSBO bool) (bool, bool) {
	cli := conn.Client()
	ref := model.ObjectReference(entry.Path)

	co, err := cli.ControlFor(ctx, ref)
	if err != nil || co == nil {
		jslog.Log(jslog.LevelNoLog, "%s Control object not found! %s", conn.Name, entry.Path)
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "%s Control object exception! %s - %v", conn.Name, entry.Path, err)
		}
		return false, true
	}

	ctlModel := co.Model()
	ctlType, err := co.CtlValType(ctx)
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "%s Control object exception! %s - %v", conn.Name, entry.Path, err)
		return false, true
	}
	jslog.Log(jslog.LevelNoLog, "%s %s has control model %s", conn.Name, entry.Path, ctlModel)
	jslog.Log(jslog.LevelNoLog, "%s  type of ctlVal: %s", conn.Name, mmsTypeName(ctlType))

	if ctlModel == model.CtlStatusOnly {
		jslog.Log(jslog.LevelNoLog, "%s Control is status-only!", conn.Name)
		return false, false
	}

	ctlVal, err := buildCtlVal(ctx, co, ctlType, value)
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "%s Unsupported Command Type!", conn.Name)
		return false, false
	}

	opts := []client.ControlOption{
		client.WithOriginator(model.OrCatStationControl, "JsonScada"),
		client.WithInterlockCheck(true),
		client.WithSynchroCheck(true),
		client.WithTest(false),
	}

	// The C# driver chooses between select and select-with-value from the
	// tag's useSBO flag. The library picks the form the control model
	// prescribes, which is what an IED expects; the flag only forces
	// select-with-value on a normal-security SBO object.
	if ctlModel == model.CtlSBONormal && useSBO {
		jslog.Log(jslog.LevelNoLog, "%s Selecting with value...", conn.Name)
		if err := co.SelectWithValue(ctx, ctlVal, opts...); err != nil {
			jslog.Log(jslog.LevelNoLog, "%s Select with value failed!", conn.Name)
			logControlError(conn, err)
			return false, false
		}
		jslog.Log(jslog.LevelNoLog, "%s Selected successfully!", conn.Name)
		time.Sleep(100 * time.Millisecond)
		// The selection is open, so the operate below must not select
		// again; it keeps the control number of the select.
		opts = append(opts, client.WithModel(model.CtlDirectNormal))
	} else if ctlModel.HasSelect() {
		jslog.Log(jslog.LevelNoLog, "%s Selecting...", conn.Name)
	}

	if err := co.Operate(ctx, ctlVal, opts...); err != nil {
		jslog.Log(jslog.LevelNoLog, "%s Operate failed!", conn.Name)
		logControlError(conn, err)
		return false, false
	}
	jslog.Log(jslog.LevelNoLog, "%s Operated successfully!", conn.Name)
	return true, false
}

// buildCtlVal produces the ctlVal an object expects for a numeric command
// value. Unlike the C# driver, integer, float and double-point controls get
// a correctly typed value rather than a boolean (deviation D2).
func buildCtlVal(ctx context.Context, co *client.ControlObject, ctlType mms.Type, value float64) (*mms.Value, error) {
	switch ctlType {
	case mms.TypeBoolean:
		return mms.NewBool(value != 0), nil
	case mms.TypeInteger:
		return mms.NewInt32(int32(value)), nil
	case mms.TypeUnsigned:
		return mms.NewUint32(uint32(value)), nil
	case mms.TypeFloat32:
		return mms.NewFloat32(float32(value)), nil
	case mms.TypeFloat64:
		return mms.NewFloat64(value), nil
	case mms.TypeBitString:
		// A double-point control (DPC) takes a two-bit position.
		if value != 0 {
			return model.DbposOn.Value(), nil
		}
		return model.DbposOff.Value(), nil
	case mms.TypeStructure:
		// An analogue control (APC) takes an AnalogueValue, whose members
		// say whether the server wants an integer or a float.
		spec, err := co.CtlValSpec(ctx)
		if err != nil {
			return nil, err
		}
		for _, comp := range spec.Components {
			switch comp.Name {
			case "f":
				return mms.NewStructure(mms.NewFloat32(float32(value))), nil
			case "i":
				return mms.NewStructure(mms.NewInt32(int32(value))), nil
			}
		}
		return nil, errors.New("unsupported analogue control value")
	}
	return nil, errors.New("unsupported control value type " + strconv.Itoa(int(ctlType)))
}

// logControlError prints the error and additional cause of a rejected
// control the way the C# driver does.
func logControlError(conn *Iec61850Connection, err error) {
	var ce *client.ControlError
	if errors.As(err, &ce) {
		jslog.Log(jslog.LevelNoLog, "%s Error: %v", conn.Name, ce.Err)
		jslog.Log(jslog.LevelNoLog, "%s Addit.Cause: %s", conn.Name, ce.AddCause)
		return
	}
	jslog.Log(jslog.LevelNoLog, "%s Error: %v", conn.Name, err)
}
