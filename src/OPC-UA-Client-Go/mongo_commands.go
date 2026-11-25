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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
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

// commandQueueSize bounds the commands waiting for one connection.
const commandQueueSize = 256

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

	// deviation D33: each connection runs its commands on its own worker, in
	// arrival order. One slow or dead server used to hold this loop for up to
	// 20 s per command while every other connection's commands aged past the
	// expiry.
	workers := map[*OPCUAConnection]chan bson.M{}
	var running sync.WaitGroup
	wctx, stopWorkers := context.WithCancel(ctx)
	defer running.Wait() // runs after stopWorkers: let in-flight commands report
	defer stopWorkers()

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
		conn := connByNumber(conns, jsmongo.GetInt(ev.FullDocument, "protocolSourceConnectionNumber", 0))
		if conn == nil {
			continue // not a connection of this driver instance
		}
		queue, ok := workers[conn]
		if !ok {
			queue = make(chan bson.M, commandQueueSize)
			workers[conn] = queue
			running.Add(1)
			go func() {
				defer running.Done()
				for {
					select {
					case <-wctx.Done():
						return
					case doc := <-queue:
						handleCommand(ctx, collCmds, conns, doc)
					}
				}
			}()
		}
		select {
		case queue <- ev.FullDocument:
		default:
			jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - too many commands waiting, refusing one", conn.Name)
			conn.secCommandRefused(kindFloodCommands, ev.FullDocument,
				fmt.Sprintf("more than %d commands waiting for the server, refused", commandQueueSize), "")
			cancelCommand(ctx, collCmds, ev.FullDocument["_id"], "busy")
		}
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
		conn.secCommandRefused(kindCmdStaleTime, doc,
			fmt.Sprintf("command older than its expiry (%.0f s), not sent", age.Seconds()), "")
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
			conn.secCommandRefused(kindCmdRejected, doc, "commands are disabled on the connection", "")
		}
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s Address %s value %v%s", conn.Name, address, value, what)
		cancelCommand(ctx, collCmds, docID, reason)
		return
	}

	conn.secCommand(collCmds.Database().Collection(jsmongo.RealtimeDataCollectionName), doc, asdu, value)

	// An OPC UA method is invoked through the Call service, not written as
	// a Value attribute.
	if strings.EqualFold(asdu, "method") {
		callMethod(ctx, collCmds, conn, cli, doc, address, valueString)
		return
	}

	writeValue(ctx, collCmds, conn, cli, doc, address, asdu, value, valueString)
}

// callMethod resolves the object owning a method and calls it.
func callMethod(ctx context.Context, collCmds *mongo.Collection, conn *OPCUAConnection, cli *opcua.Client, doc bson.M, address, valueString string) {
	docID := doc["_id"]
	ok := false
	resultDescription := ""
	expired := false

	err := func() error {
		methodID, err := ua.ParseNodeID(address)
		if err != nil {
			conn.secCommandRefused(kindCmdObjectUnknown, doc, "the command's address is not a valid node id", "")
			return err
		}

		objectID, err := methodParent(ctx, cli, methodID)
		if errors.Is(err, errNoMethodParent) {
			conn.secCommandRefused(kindCmdObjectUnknown, doc, "the method has no parent object on the server", "")
			return err
		}
		if err != nil {
			conn.secCommandFailed(doc, "Browse", err)
			return err
		}

		args, err := typedMethodArguments(valueString, methodInputTypes(ctx, cli, methodID))
		if err != nil {
			conn.secCommandRefused(kindCmdWrongType, doc, "the method's arguments: "+err.Error(), "")
			return err
		}

		// The lookups above take requests to the server: the command may
		// have expired meanwhile.
		if commandExpired(conn, doc) {
			expired = true
			return nil
		}

		callCtx, cancel := context.WithTimeout(ctx, commandTimeout)
		defer cancel()
		res, err := cli.Call(callCtx, &ua.CallMethodRequest{
			ObjectID:       objectID,
			MethodID:       methodID,
			InputArguments: args,
		})
		outcome := ""
		switch {
		case err != nil:
			outcome = err.Error()
		default:
			outcome = statusCodeName(res.StatusCode)
		}
		// Reported after the dispatch: telling what the method is takes
		// requests to the server, which must not delay the command.
		go conn.secMethodCall(cli, doc, methodID, objectID, len(args), outcome)
		if err != nil {
			conn.secCommandFailed(doc, "Call", err)
			return err
		}
		if !statusIsGood(res.StatusCode) {
			conn.secCommandAnswer(doc, "Call", res.StatusCode)
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

	if expired {
		cancelCommand(ctx, collCmds, docID, "expired")
		return
	}
	if err != nil {
		ok = false
		resultDescription = err.Error()
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Method call error: %v", conn.Name, err)
	}

	ackCommand(ctx, collCmds, docID, ok, resultDescription)
}

// methodParent finds the object a method belongs to, which the Call service
// requires alongside the method itself.
// errNoMethodParent: the server answered, and the method has no parent
// object.
var errNoMethodParent = errors.New("could not resolve parent object for method")

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
		return nil, errNoMethodParent
	}
	return ua.NewNodeIDFromExpandedNodeID(resp.Results[0].References[0].NodeID), nil
}

// methodInputTypes reads the data types of a method's input arguments from
// its InputArguments property (best effort: nil when the server does not say).
func methodInputTypes(ctx context.Context, cli *opcua.Client, methodID *ua.NodeID) []uint32 {
	readCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	resp, err := cli.Browse(readCtx, &ua.BrowseRequest{
		View: &ua.ViewDescription{ViewID: ua.NewTwoByteNodeID(0)},
		NodesToBrowse: []*ua.BrowseDescription{{
			NodeID:          methodID,
			BrowseDirection: ua.BrowseDirectionForward,
			ReferenceTypeID: ua.NewNumericNodeID(0, id.HasProperty),
			IncludeSubtypes: true,
			NodeClassMask:   uint32(ua.NodeClassVariable),
			ResultMask:      uint32(ua.BrowseResultMaskAll),
		}},
	})
	if err != nil || len(resp.Results) == 0 {
		return nil
	}
	for _, ref := range resp.Results[0].References {
		if ref.BrowseName == nil || ref.BrowseName.Name != "InputArguments" {
			continue
		}
		v, err := cli.Node(ua.NewNodeIDFromExpandedNodeID(ref.NodeID)).Value(readCtx)
		if err != nil || v == nil {
			return nil
		}
		list, ok := v.Value().([]*ua.ExtensionObject)
		if !ok {
			return nil
		}
		types := make([]uint32, len(list))
		for i, eo := range list {
			if eo == nil {
				continue
			}
			if a, ok := eo.Value.(*ua.Argument); ok && a != nil && a.DataType != nil &&
				a.DataType.Namespace() == 0 && a.ValueRank < 0 {
				types[i] = a.DataType.IntID()
			}
		}
		return types
	}
	return nil
}

// methodArguments is typedMethodArguments without knowing the types.
func methodArguments(valueString string) ([]*ua.Variant, error) {
	return typedMethodArguments(valueString, nil)
}

// typedMethodArguments builds the optional input arguments, given as a JSON
// array in valueString.
//
// deviation D34: when the server declares the argument types, each element
// is converted to its declared type (a whole number for an Int32 parameter
// must not travel as an Int64, which servers refuse). Without a declared
// type, or for types not handled here, typing is best effort, the ladder the
// C# driver used: bool, then integer, then float, then string.
func typedMethodArguments(valueString string, types []uint32) ([]*ua.Variant, error) {
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
	for i, el := range raw {
		var v *ua.Variant
		var err error
		if i < len(types) && types[i] != 0 {
			if tv, handled, terr := typedArgument(el, types[i]); terr != nil {
				return nil, fmt.Errorf("argument %d: %w", i, terr)
			} else if handled {
				args = append(args, tv)
				continue
			}
		}
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

// typedArgument converts one JSON element to the built-in type with the given
// namespace-0 id. handled is false for types this does not cover.
func typedArgument(el any, typeID uint32) (v *ua.Variant, handled bool, err error) {
	num := func() (float64, error) {
		switch t := el.(type) {
		case float64:
			return t, nil
		case bool:
			if t {
				return 1, nil
			}
			return 0, nil
		case string:
			return strconv.ParseFloat(t, 64)
		}
		return 0, fmt.Errorf("%v is not a number", el)
	}
	if typeID == id.Boolean {
		switch t := el.(type) {
		case bool:
			v, err = ua.NewVariant(t)
		default:
			f, nerr := num()
			if nerr != nil {
				return nil, true, nerr
			}
			v, err = ua.NewVariant(f != 0)
		}
		return v, true, err
	}
	if typeID == id.String {
		s, isString := el.(string)
		if !isString {
			s = fmt.Sprint(el)
		}
		v, err = ua.NewVariant(s)
		return v, true, err
	}
	if typeID == id.ByteString {
		s, isString := el.(string)
		if !isString {
			return nil, true, fmt.Errorf("%v is not a base64 string", el)
		}
		b, derr := base64.StdEncoding.DecodeString(s)
		if derr != nil {
			return nil, true, derr
		}
		v, err = ua.NewVariant(b)
		return v, true, err
	}

	switch typeID {
	case id.SByte, id.Byte, id.Int16, id.UInt16, id.Int32, id.UInt32, id.Int64, id.UInt64, id.Float, id.Double:
	default:
		return nil, false, nil
	}
	f, nerr := num()
	if nerr != nil {
		return nil, true, nerr
	}
	switch typeID {
	case id.SByte:
		v, err = variantOf(toInteger[int8](f, "SByte", -128, 128))
	case id.Byte:
		v, err = variantOf(toInteger[uint8](f, "Byte", 0, 256))
	case id.Int16:
		v, err = variantOf(toInteger[int16](f, "Int16", -32768, 32768))
	case id.UInt16:
		v, err = variantOf(toInteger[uint16](f, "UInt16", 0, 65536))
	case id.Int32:
		v, err = variantOf(toInteger[int32](f, "Int32", -2147483648, 2147483648))
	case id.UInt32:
		v, err = variantOf(toInteger[uint32](f, "UInt32", 0, 4294967296))
	case id.Int64:
		v, err = variantOf(toInteger[int64](f, "Int64", -9223372036854775808, 9223372036854775808))
	case id.UInt64:
		v, err = variantOf(toInteger[uint64](f, "UInt64", 0, 18446744073709551616))
	case id.Float:
		v, err = variantOf(toFloat32(f))
	case id.Double:
		v, err = ua.NewVariant(f)
	}
	return v, true, err
}

// writeValue converts the command to the type the node expects and writes
// it to the Value attribute.
func writeValue(ctx context.Context, collCmds *mongo.Collection, conn *OPCUAConnection, cli *opcua.Client, doc bson.M, address, asdu string, value float64, valueString string) {
	docID := doc["_id"]
	nodeID, err := ua.ParseNodeID(address)
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Type conversion error! %v", conn.Name, err)
		conn.secCommandRefused(kindCmdObjectUnknown, doc, "the command's address is not a valid node id", "")
		cancelCommand(ctx, collCmds, docID, "type conversion error")
		return
	}

	variant, reason, err := commandVariant(asdu, value, valueString)
	if reason != "" {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - %s", conn.Name, reason)
		conn.secCommandRefused(kindCmdWrongType, doc, "command not sent: "+reason, "")
		cancelCommand(ctx, collCmds, docID, reason)
		return
	}
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Type conversion error! %v", conn.Name, err)
		conn.secCommandRefused(kindCmdWrongType, doc, "command not sent: the value does not convert to "+asdu, "")
		cancelCommand(ctx, collCmds, docID, "type conversion error")
		return
	}
	// No branch matched the ASDU: refuse rather than write a null value.
	if variant == nil {
		jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Unsupported command ASDU '%s', ignoring.", conn.Name, asdu)
		conn.secCommandRefused(kindCmdWrongType, doc, "command not sent: unsupported command type "+asdu, "")
		cancelCommand(ctx, collCmds, docID, "unsupported command type")
		return
	}

	if commandExpired(conn, doc) {
		cancelCommand(ctx, collCmds, docID, "expired")
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
		conn.secCommandFailed(doc, "Write", err)
	case len(resp.Results) > 0:
		resultDescription = statusCodeName(resp.Results[0])
		ok = statusIsGood(resp.Results[0])
		conn.secCommandAnswer(doc, "Write", resp.Results[0])
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

	// Integer commands are rounded half to even and refused when they do not
	// fit the type, like .NET's Convert.ToXxx; see commandconv.go.
	switch strings.ToLower(strings.TrimSpace(asdu)) {
	case "boolean":
		v, err = ua.NewVariant(value != 0.0)
	case "sbyte":
		v, err = variantOf(toInteger[int8](value, "SByte", -128, 128))
	case "byte":
		v, err = variantOf(toInteger[uint8](value, "Byte", 0, 256))
	case "int16":
		v, err = variantOf(toInteger[int16](value, "Int16", -32768, 32768))
	case "uint16":
		v, err = variantOf(toInteger[uint16](value, "UInt16", 0, 65536))
	case "integer", "int32":
		v, err = variantOf(toInteger[int32](value, "Int32", -2147483648, 2147483648))
	case "uint32":
		v, err = variantOf(toInteger[uint32](value, "UInt32", 0, 4294967296))
	case "int64":
		v, err = variantOf(toInteger[int64](value, "Int64", -9223372036854775808, 9223372036854775808))
	case "uint64":
		v, err = variantOf(toInteger[uint64](value, "UInt64", 0, 18446744073709551616))
	case "float":
		v, err = variantOf(toFloat32(value))
	case "double":
		v, err = ua.NewVariant(value)
	case "datetime":
		// Acquisition publishes datetimes as Unix milliseconds; convert
		// back the same way.
		v, err = variantOf(toDateTime(value))
	case "string":
		v, err = ua.NewVariant(valueString)
	case "xmlelement":
		v, err = ua.NewVariant(ua.XMLElement(valueString))
	case "bytestring", "localizedtext", "qualifiedname", "nodeid", "guid", "expandednodeid":
		// deviation D34: sent as the type the ASDU names, in the textual form
		// acquisition publishes (base64, "ns:Name", a node id string). A
		// String variant is refused by servers with BadTypeMismatch.
		v, err = stringFamilyVariant(strings.ToLower(strings.TrimSpace(asdu)), valueString)
	case "extensionobject", "numericrange", "variant", "diagnosticinfo", "datavalue":
		// There is no way to build these from a plain number or string.
		return nil, "", fmt.Errorf("writing complex type '%s' is not supported", asdu)
	}
	return v, "", err
}

// stringFamilyVariant builds the variant of a scalar whose command value is a
// string: the inverse of what convertOPCValue publishes.
func stringFamilyVariant(asdu, s string) (*ua.Variant, error) {
	switch asdu {
	case "bytestring":
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("not valid base64: %w", err)
		}
		return ua.NewVariant(b)
	case "nodeid":
		n, err := ua.ParseNodeID(s)
		if err != nil {
			return nil, err
		}
		return ua.NewVariant(n)
	case "expandednodeid":
		n, err := ua.ParseNodeID(s)
		if err != nil {
			return nil, err
		}
		return ua.NewVariant(ua.NewExpandedNodeID(n, "", 0))
	case "guid":
		g := ua.NewGUID(s)
		if g == nil {
			return nil, fmt.Errorf("%q is not a GUID", s)
		}
		return ua.NewVariant(g)
	case "localizedtext":
		return ua.NewVariant(&ua.LocalizedText{EncodingMask: ua.LocalizedTextText, Text: s})
	default: // qualifiedname, published as "ns:Name" (just "Name" for ns 0)
		ns, name := uint16(0), s
		if i := strings.Index(s, ":"); i > 0 {
			if n, err := strconv.ParseUint(s[:i], 10, 16); err == nil {
				ns, name = uint16(n), s[i+1:]
			}
		}
		return ua.NewVariant(&ua.QualifiedName{NamespaceIndex: ns, Name: name})
	}
}

// arrayVariant converts a JSON array in valueString to an array of the
// element type named by the ASDU.
func arrayVariant(asdu, valueString string) (*ua.Variant, string, error) {
	if valueString == "" {
		return nil, "empty array json error", nil
	}
	raw, ok := decodeJSONArray(valueString)
	if !ok {
		return nil, "array invalid json format error", nil
	}

	elemType := strings.ToLower(strings.TrimSpace(strings.Split(asdu, "[")[0]))

	// Integer elements must be plain in-range integers and are never
	// rounded; see signedArray.
	var x any
	var err error
	switch elemType {
	case "datetime":
		// parity: array elements are ISO-8601 strings here, while a scalar
		// datetime travels as Unix milliseconds.
		a := make([]time.Time, len(raw))
		for i := range raw {
			s, isString := raw[i].(string)
			if !isString {
				return nil, "", fmt.Errorf("element %d of the array is %T, not a timestamp", i, raw[i])
			}
			t, perr := time.Parse(time.RFC3339Nano, s)
			if perr != nil {
				return nil, "", perr
			}
			a[i] = t
		}
		x = a
	case "int16":
		x, err = signedArray[int16](raw, 16, "Int16")
	case "uint16":
		x, err = unsignedArray[uint16](raw, 16, "UInt16")
	case "int32", "integer":
		x, err = signedArray[int32](raw, 32, "Int32")
	case "uint32":
		x, err = unsignedArray[uint32](raw, 32, "UInt32")
	case "int64":
		x, err = signedArray[int64](raw, 64, "Int64")
	case "uint64":
		x, err = unsignedArray[uint64](raw, 64, "UInt64")
	case "float":
		x, err = floatArray[float32](raw, 32, "Float")
	case "double":
		x, err = floatArray[float64](raw, 64, "Double")
	case "boolean":
		a := make([]bool, len(raw))
		for i := range raw {
			b, isBool := raw[i].(bool)
			if !isBool {
				return nil, "", fmt.Errorf("element %d of the array is %T, not a boolean", i, raw[i])
			}
			a[i] = b
		}
		x = a
	case "string", "bytestring", "localizedtext", "qualifiedname",
		"nodeid", "guid", "expandednodeid", "xmlelement":
		a := make([]string, len(raw))
		for i := range raw {
			s, isString := raw[i].(string)
			if !isString {
				return nil, "", fmt.Errorf("element %d of the array is %T, not a string", i, raw[i])
			}
			a[i] = s
		}
		x = a
	default:
		return nil, "", fmt.Errorf("unsupported array type: %s", elemType)
	}
	if err != nil {
		return nil, "", err
	}
	v, err := ua.NewVariant(x)
	return v, "", err
}

// commandExpired reports (and logs) a command that is older than its expiry
// at the moment of dispatch.
func commandExpired(conn *OPCUAConnection, doc bson.M) bool {
	age := time.Since(jsmongo.GetTime(doc, "timeTag"))
	if age <= commandExpiry {
		return false
	}
	jslog.Log(jslog.LevelNoLog, "MongoDB CMD CS - %s - Address %s Command Timeout Expired before dispatch, %v Seconds old",
		conn.Name, jsmongo.GetString(doc, "protocolSourceObjectAddress", ""), age.Seconds())
	conn.secCommandRefused(kindCmdStaleTime, doc,
		fmt.Sprintf("command older than its expiry (%.0f s) at dispatch, not sent", age.Seconds()), "")
	return true
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
