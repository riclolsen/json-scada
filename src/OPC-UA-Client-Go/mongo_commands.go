/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
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
// commandsQueue picks up inserts, and each command is either a Write of the
// Value attribute or, when the ASDU says "method", a Call.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jscommands"
	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// commandExpiry is how old a queued command may be before it is refused.
const commandExpiry = jscommands.DefaultExpiry

// commandTimeout is the TimeoutHint the C# driver puts on the write.
const commandTimeout = 10 * time.Second

// commandsLoop watches commandsQueue for inserted commands.
func commandsLoop(ctx context.Context, cfg jsconfig.Config, conns []*OPCUAConnection) {
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

func watchCommands(ctx context.Context, db *mongo.Database, collCmds *mongo.Collection, conns []*OPCUAConnection) error {
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
		// parity: commands are executed by the active node only.
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
func handleCommand(ctx context.Context, collCmds *mongo.Collection, conns []*OPCUAConnection, doc bson.M) {
	if doc == nil {
		return
	}
	connNumber := jsmongo.GetInt(doc, "protocolSourceConnectionNumber", 0)
	jslog.Log(jslog.LevelDetailed, "MongoDB CMD CS - Looking for connection %d...", connNumber)

	conn := connByNumber(conns, connNumber)
	if conn == nil {
		return // not a connection of this driver instance
	}

	docID := doc["_id"]
	address := jsmongo.GetString(doc, "protocolSourceObjectAddress", "")
	asdu := jsmongo.GetString(doc, "protocolSourceASDU", "")
	value := jsmongo.GetDouble(doc, "value", 0)
	valueString := jsmongo.GetString(doc, "valueString", "")

	// Expired: the operator's intent is stale, so refuse rather than act.
	age := time.Since(jsmongo.GetTime(doc, "timeTag"))
	if age > commandExpiry {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Address %s value %v Command Timeout Expired, %v Seconds old",
			conn.Name, address, value, age.Seconds())
		cancelCommand(ctx, collCmds, docID, "expired")
		return
	}

	cli := conn.Client()
	if cli == nil || cli.State() != opcua.Connected || !conn.CommandsEnabled {
		reason := "not connected"
		what := " Not Connected"
		if !conn.CommandsEnabled {
			reason = "commands disabled"
			what = " Commands Disabled"
		}
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s Address %s value %v%s", conn.Name, address, value, what)
		cancelCommand(ctx, collCmds, docID, reason)
		return
	}

	// An OPC UA method is invoked through the Call service, not written as
	// a Value attribute.
	if strings.EqualFold(asdu, "method") {
		callMethod(ctx, collCmds, conn, cli, docID, address, valueString)
		return
	}

	writeValue(ctx, collCmds, conn, cli, docID, address, asdu, value, valueString)
}

// callMethod resolves the object owning a method and calls it.
func callMethod(ctx context.Context, collCmds *mongo.Collection, conn *OPCUAConnection, cli *opcua.Client, docID any, address, valueString string) {
	ok := false
	resultDescription := ""

	err := func() error {
		methodID, err := ua.ParseNodeID(address)
		if err != nil {
			return err
		}

		objectID, err := methodParent(ctx, cli, methodID)
		if err != nil {
			return err
		}

		args, err := methodArguments(valueString)
		if err != nil {
			return err
		}

		callCtx, cancel := context.WithTimeout(ctx, commandTimeout)
		defer cancel()
		res, err := cli.Call(callCtx, &ua.CallMethodRequest{
			ObjectID:       objectID,
			MethodID:       methodID,
			InputArguments: args,
		})
		if err != nil {
			return err
		}
		if !statusIsGood(res.StatusCode) {
			return errors.New(statusCodeName(res.StatusCode))
		}

		ok = true
		if len(res.OutputArguments) == 0 {
			resultDescription = "OK"
		} else {
			outs := make([]string, 0, len(res.OutputArguments))
			for _, o := range res.OutputArguments {
				outs = append(outs, fmt.Sprint(o.Value()))
			}
			resultDescription = "OK: " + strings.Join(outs, ",")
		}
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Method called: %s - %s", conn.Name, address, resultDescription)
		return nil
	}()

	if err != nil {
		ok = false
		resultDescription = err.Error()
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Method call error: %v", conn.Name, err)
	}

	ackCommand(ctx, collCmds, docID, ok, resultDescription)
}

// methodParent finds the object a method belongs to, which the Call service
// requires alongside the method itself.
func methodParent(ctx context.Context, cli *opcua.Client, methodID *ua.NodeID) (*ua.NodeID, error) {
	browseCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	resp, err := cli.Browse(browseCtx, &ua.BrowseRequest{
		View:                          &ua.ViewDescription{ViewID: ua.NewTwoByteNodeID(0)},
		RequestedMaxReferencesPerNode: 0,
		NodesToBrowse: []*ua.BrowseDescription{{
			NodeID:          methodID,
			BrowseDirection: ua.BrowseDirectionInverse,
			ReferenceTypeID: ua.NewNumericNodeID(0, id.HierarchicalReferences),
			IncludeSubtypes: true,
			NodeClassMask:   uint32(ua.NodeClassObject),
			ResultMask:      uint32(ua.BrowseResultMaskAll),
		}},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 || len(resp.Results[0].References) == 0 {
		return nil, errors.New("could not resolve parent object for method")
	}
	return ua.NewNodeIDFromExpandedNodeID(resp.Results[0].References[0].NodeID), nil
}

// methodArguments types the optional input arguments, given as a JSON array
// in valueString. Typing is best effort, the same ladder the C# driver uses:
// bool, then integer, then float, then string.
func methodArguments(valueString string) ([]*ua.Variant, error) {
	if strings.TrimSpace(valueString) == "" {
		return nil, nil
	}
	var raw []any
	if err := json.Unmarshal([]byte(valueString), &raw); err != nil {
		// parity: a valueString that is not a JSON array is simply no
		// arguments, not an error.
		return nil, nil
	}

	args := make([]*ua.Variant, 0, len(raw))
	for _, el := range raw {
		var v *ua.Variant
		var err error
		switch t := el.(type) {
		case bool:
			v, err = ua.NewVariant(t)
		case float64:
			if t == float64(int64(t)) {
				v, err = ua.NewVariant(int64(t))
			} else {
				v, err = ua.NewVariant(t)
			}
		case string:
			v, err = ua.NewVariant(t)
		default:
			v, err = ua.NewVariant(fmt.Sprint(t))
		}
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	return args, nil
}

// writeValue converts the command to the type the node expects and writes
// it to the Value attribute.
func writeValue(ctx context.Context, collCmds *mongo.Collection, conn *OPCUAConnection, cli *opcua.Client, docID any, address, asdu string, value float64, valueString string) {
	nodeID, err := ua.ParseNodeID(address)
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Type conversion error! %v", conn.Name, err)
		cancelCommand(ctx, collCmds, docID, "type conversion error")
		return
	}

	variant, reason, err := commandVariant(asdu, value, valueString)
	if reason != "" {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - %s", conn.Name, reason)
		cancelCommand(ctx, collCmds, docID, reason)
		return
	}
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Type conversion error! %v", conn.Name, err)
		cancelCommand(ctx, collCmds, docID, "type conversion error")
		return
	}
	// No branch matched the ASDU: refuse rather than write a null value.
	if variant == nil {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Unsupported command ASDU '%s', ignoring.", conn.Name, asdu)
		cancelCommand(ctx, collCmds, docID, "unsupported command type")
		return
	}

	jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Writing node...", conn.Name)

	writeCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	resp, err := cli.Write(writeCtx, &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{{
			NodeID:      nodeID,
			AttributeID: ua.AttributeIDValue,
			// deviation D7: only the value bit is set. The C# driver also
			// sends the status code and both timestamps, which many servers
			// reject with BadWriteNotSupported.
			Value: &ua.DataValue{EncodingMask: ua.DataValueValue, Value: variant},
		}},
	})

	ok := false
	resultDescription := "no result returned"
	switch {
	case err != nil:
		resultDescription = err.Error()
	case len(resp.Results) > 0:
		resultDescription = statusCodeName(resp.Results[0])
		ok = statusIsGood(resp.Results[0])
	}

	jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Address: %s value: %v valueString: %s - Command delivered - %s",
		conn.Name, address, value, valueString, resultDescription)

	ackCommand(ctx, collCmds, docID, ok, resultDescription)
}

// commandVariant converts a queued command to the OPC UA type named by its
// ASDU. A non-empty reason means the command must be cancelled with that
// exact cancelReason.
func commandVariant(asdu string, value float64, valueString string) (v *ua.Variant, reason string, err error) {
	// An ASDU with exactly one '[' names an array type, e.g. "double[]".
	if strings.Count(asdu, "[") == 1 {
		return arrayVariant(asdu, valueString)
	}

	switch strings.ToLower(strings.TrimSpace(asdu)) {
	case "boolean":
		v, err = ua.NewVariant(value != 0.0)
	case "sbyte":
		v, err = ua.NewVariant(int8(value))
	case "byte":
		v, err = ua.NewVariant(byte(value))
	case "int16":
		v, err = ua.NewVariant(int16(value))
	case "uint16":
		v, err = ua.NewVariant(uint16(value))
	case "integer", "int32":
		v, err = ua.NewVariant(int32(value))
	case "uint32":
		v, err = ua.NewVariant(uint32(value))
	case "int64":
		v, err = ua.NewVariant(int64(value))
	case "uint64":
		v, err = ua.NewVariant(uint64(value))
	case "float":
		v, err = ua.NewVariant(float32(value))
	case "double":
		v, err = ua.NewVariant(value)
	case "datetime":
		// Acquisition publishes datetimes as Unix milliseconds; convert
		// back the same way.
		v, err = ua.NewVariant(time.UnixMilli(int64(value)).UTC())
	case "string", "bytestring", "localizedtext", "qualifiedname",
		"nodeid", "guid", "expandednodeid", "xmlelement":
		v, err = ua.NewVariant(valueString)
	case "extensionobject", "numericrange", "variant", "diagnosticinfo", "datavalue":
		// There is no way to build these from a plain number or string.
		return nil, "", fmt.Errorf("writing complex type '%s' is not supported", asdu)
	}
	return v, "", err
}

// arrayVariant converts a JSON array in valueString to an array of the
// element type named by the ASDU.
func arrayVariant(asdu, valueString string) (*ua.Variant, string, error) {
	if valueString == "" {
		return nil, "empty array json error", nil
	}
	var raw []any
	if err := json.Unmarshal([]byte(valueString), &raw); err != nil {
		return nil, "array invalid json format error", nil
	}

	elemType := strings.ToLower(strings.TrimSpace(strings.Split(asdu, "[")[0]))

	num := func(i int) (float64, error) {
		f, ok := raw[i].(float64)
		if !ok {
			return 0, fmt.Errorf("element %d of the array is %T, not a number", i, raw[i])
		}
		return f, nil
	}

	var v *ua.Variant
	var err error
	switch elemType {
	case "datetime":
		// parity: array elements are ISO-8601 strings here, while a scalar
		// datetime travels as Unix milliseconds.
		a := make([]time.Time, len(raw))
		for i := range raw {
			s, ok := raw[i].(string)
			if !ok {
				return nil, "", fmt.Errorf("element %d of the array is %T, not a timestamp", i, raw[i])
			}
			t, perr := time.Parse(time.RFC3339Nano, s)
			if perr != nil {
				return nil, "", perr
			}
			a[i] = t
		}
		v, err = ua.NewVariant(a)
	case "int16":
		a := make([]int16, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = int16(f)
		}
		v, err = ua.NewVariant(a)
	case "uint16":
		a := make([]uint16, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = uint16(f)
		}
		v, err = ua.NewVariant(a)
	case "uint32":
		a := make([]uint32, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = uint32(f)
		}
		v, err = ua.NewVariant(a)
	case "int64":
		a := make([]int64, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = int64(f)
		}
		v, err = ua.NewVariant(a)
	case "uint64":
		a := make([]uint64, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = uint64(f)
		}
		v, err = ua.NewVariant(a)
	case "float":
		a := make([]float32, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = float32(f)
		}
		v, err = ua.NewVariant(a)
	case "double":
		a := make([]float64, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = f
		}
		v, err = ua.NewVariant(a)
	case "int32", "integer":
		a := make([]int32, len(raw))
		for i := range raw {
			f, e := num(i)
			if e != nil {
				return nil, "", e
			}
			a[i] = int32(f)
		}
		v, err = ua.NewVariant(a)
	case "boolean":
		a := make([]bool, len(raw))
		for i := range raw {
			b, ok := raw[i].(bool)
			if !ok {
				return nil, "", fmt.Errorf("element %d of the array is %T, not a boolean", i, raw[i])
			}
			a[i] = b
		}
		v, err = ua.NewVariant(a)
	case "string", "bytestring", "localizedtext", "qualifiedname",
		"nodeid", "guid", "expandednodeid", "xmlelement":
		a := make([]string, len(raw))
		for i := range raw {
			s, ok := raw[i].(string)
			if !ok {
				return nil, "", fmt.Errorf("element %d of the array is %T, not a string", i, raw[i])
			}
			a[i] = s
		}
		v, err = ua.NewVariant(a)
	default:
		return nil, "", fmt.Errorf("unsupported array type: %s", elemType)
	}
	return v, "", err
}

// cancelCommand marks a command refused before it reached the server.
func cancelCommand(ctx context.Context, collCmds *mongo.Collection, docID any, reason string) {
	if err := jscommands.Cancel(ctx, collCmds, docID, reason); err != nil {
		jslog.Log(jslog.LevelDetailed, "MongoDB CMD CS - cancel update: %v", err)
	}
}

// ackCommand records the outcome of a command that reached the server.
func ackCommand(ctx context.Context, collCmds *mongo.Collection, docID any, ok bool, resultDescription string) {
	if err := jscommands.Ack(ctx, collCmds, docID, ok, resultDescription); err != nil {
		jslog.Log(jslog.LevelDetailed, "MongoDB CMD CS - ack update: %v", err)
	}
}
