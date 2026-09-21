/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - client engine
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Shared engine of the IEC 101/104 client drivers: acquired-data queue and
 * MongoDB bulk writer (port of MongoUpdate.cs), command-ack write-back and
 * commands change stream (port of MongoCommands.cs).
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

package cliapp

import (
	"context"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"

	"github.com/riclolsen/json-scada/src/go-common/jscommands"
	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// DataBufferLimit is the acquisition buffer limit (same as the C# drivers).
const DataBufferLimit = 10000

// Conn is the driver-side state of one protocol connection.
type Conn struct {
	Cfg model.ConnCfg
	// IsConnected reports whether the protocol connection is up (set by main).
	IsConnected func() bool
	// SendCmd sends one command object with cause Activation (set by main).
	SendCmd func(obj *conv.InfoObject, ca int) error
	// RawCmd, when set, fully sends a command from the queue document and
	// bypasses the default 101/104 BuildInfoObj/SBO path. Used by the IEC 103
	// client (FUN/INF general command). Returns an error to cancel.
	RawCmd func(asduNum, objAddr, commonAddr int, value float64, duration int) error

	mu             sync.Mutex
	lastSelected   *conv.InfoObject
	lastSelectedCa int

	inserted   map[[2]int]bool // {CA, IOA} pairs known in realtimeData (autoCreateTags)
	lastNewKey float64
}

// SetLastSelected stores the execute twin of an SBO command.
func (c *Conn) SetLastSelected(obj *conv.InfoObject, ca int) {
	c.mu.Lock()
	c.lastSelected = obj
	c.lastSelectedCa = ca
	c.mu.Unlock()
}

// SelectedFor returns the stored execute twin when the confirmed select
// matches its object address (C# parity: compared by IOA only, not cleared).
func (c *Conn) SelectedFor(ioa int) *conv.InfoObject {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastSelected != nil && c.lastSelected.Ioa == ioa {
		return c.lastSelected
	}
	return nil
}

// Engine is the shared client-driver engine.
type Engine struct {
	Cfg            jsconfig.Config
	DriverName     string
	InstanceNumber int
	Conns          []*Conn
	// Active reports whether this node runs the protocol. Command execution
	// gates on it; acquisition does not, which is this driver family's design.
	Active func() bool

	dataQ chan conv.IecValue
	ackQ  chan conv.IecCmdAck
}

// New creates the engine.
func New(cfg jsconfig.Config, driverName string, instanceNumber int, active func() bool) *Engine {
	return &Engine{
		Cfg:            cfg,
		DriverName:     driverName,
		InstanceNumber: instanceNumber,
		Active:         active,
		dataQ:          make(chan conv.IecValue, DataBufferLimit),
		ackQ:           make(chan conv.IecCmdAck, 1000),
	}
}

// FindConn returns the connection with the given protocolConnectionNumber.
func (e *Engine) FindConn(connNumber int) *Conn {
	for _, c := range e.Conns {
		if int(c.Cfg.ProtocolConnectionNumber) == connNumber {
			return c
		}
	}
	return nil
}

// EnqueueValues enqueues acquired values; when the buffer is full the value
// is discarded (C# parity: queue is trimmed at DataBufferLimit).
func (e *Engine) EnqueueValues(vals []conv.IecValue) {
	for _, v := range vals {
		select {
		case e.dataQ <- v:
		default:
			jslog.Log(jslog.LevelDetailed, "Dequeue Data (buffer full, discarding)")
		}
	}
}

// EnqueueAcks enqueues command confirmations.
func (e *Engine) EnqueueAcks(acks []conv.IecCmdAck) {
	for _, a := range acks {
		select {
		case e.ackQ <- a:
		default:
			jslog.Log(jslog.LevelBasic, "Command ack queue full, discarding")
		}
	}
}

// PreloadInsertedAddresses loads existing {CA, IOA} pairs from realtimeData
// for autoCreateTags connections, so restarts never re-create tags.
func (e *Engine) PreloadInsertedAddresses(db *mongo.Database) {
	colRt := db.Collection(jsmongo.RealtimeDataCollectionName)
	for _, c := range e.Conns {
		c.inserted = map[[2]int]bool{}
		if !c.Cfg.AutoCreateTags {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cur, err := colRt.Find(ctx,
			bson.M{"protocolSourceConnectionNumber": c.Cfg.ProtocolConnectionNumber},
			options.Find().SetProjection(bson.M{
				"protocolSourceCommonAddress": 1,
				"protocolSourceObjectAddress": 1,
			}))
		if err != nil {
			cancel()
			jslog.Log(jslog.LevelBasic, "%s", c.Cfg.Name+" - Error preloading tags: "+err.Error())
			continue
		}
		count := 0
		for cur.Next(ctx) {
			var doc bson.M
			if cur.Decode(&doc) == nil {
				ca := int(jsmongo.GetU32(doc, "protocolSourceCommonAddress", 0))
				ioa := int(jsmongo.GetU32(doc, "protocolSourceObjectAddress", 0))
				c.inserted[[2]int{ca, ioa}] = true
				count++
			}
		}
		cur.Close(ctx)
		cancel()
		jslog.Log(jslog.LevelBasic, "%s - Found %d tags in database.", c.Cfg.Name, count)
	}
}

// InvalidateConnPoints marks all points of a connection invalid
// (called on connection loss, port of the C# ConnectionHandler CLOSED case).
func (e *Engine) InvalidateConnPoints(connNumber int) {
	client, db, err := jsmongo.ConnectAndPing(e.Cfg)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Error connecting to MongoDB to invalidate points: "+err.Error())
		return
	}
	defer client.Disconnect(context.Background())
	jslog.Log(jslog.LevelBasic, "Invalidating points on connection %d", connNumber)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = db.Collection(jsmongo.RealtimeDataCollectionName).UpdateMany(ctx,
		bson.M{"protocolSourceConnectionNumber": connNumber},
		bson.M{"$set": bson.M{"invalid": true, "timeTag": time.Now()}})
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Error invalidating points: "+err.Error())
	}
}

func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'G', -1, 64)
}

// sourceDataUpdateDoc builds the $set update for one acquired value
// (field-for-field port of MongoUpdate.cs).
func (e *Engine) sourceDataUpdateDoc(iv conv.IecValue) bson.M {
	var bsontt interface{} = nil
	if iv.HasSourceTimestamp {
		bsontt = iv.SourceTimestamp
	}
	asduStr := iv.AsduStr
	if asduStr == "" {
		asduStr = iv.Asdu.String()
	}
	return bson.M{
		"$set": bson.M{
			"sourceDataUpdate": bson.M{
				"valueAtSource":               iv.Value,
				"valueStringAtSource":         formatValue(iv.Value),
				"asduAtSource":                asduStr,
				"causeOfTransmissionAtSource": strconv.Itoa(iv.Cot),
				"timeTagAtSource":             bsontt,
				"timeTagAtSourceOk":           iv.TimestampOk,
				"timeTag":                     iv.ServerTimestamp,
				"notTopicalAtSource":          iv.Quality.NonTopical,
				"invalidAtSource":             iv.Quality.Invalid,
				"overflowAtSource":            iv.Quality.Overflow,
				"blockedAtSource":             iv.Quality.Blocked,
				"substitutedAtSource":         iv.Quality.Substituted,
				"originator":                  e.DriverName + "|" + strconv.Itoa(iv.ConnNumber),
			},
		},
	}
}

// RunMongoWriter dequeues acquired data and command acks, updating MongoDB
// (port of MongoUpdate.cs ProcessMongo). Runs forever; call as goroutine.
func (e *Engine) RunMongoWriter() {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					jslog.Log(jslog.LevelBasic, "Mongo writer - panic recovered: %v", r)
				}
			}()
			client, db, err := jsmongo.ConnectAndPing(e.Cfg)
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s", "Exception Mongo: "+err.Error())
				time.Sleep(3 * time.Second)
				return
			}
			defer client.Disconnect(context.Background())
			collection := db.Collection(jsmongo.RealtimeDataCollectionName)
			collectionCmd := db.Collection(jsmongo.CommandsQueueCollectionName)

			for {
				if !jsmongo.PingOK(client) {
					jslog.Log(jslog.LevelBasic, "Error on MongoDB connection")
					time.Sleep(3 * time.Second)
					return // reconnect
				}

				// command acks
			ackLoop:
				for {
					select {
					case ia := <-e.ackQ:
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						_ = collectionCmd.FindOneAndUpdate(ctx,
							bson.M{
								"protocolSourceConnectionNumber": ia.ConnNumber,
								"protocolSourceObjectAddress":    jsmongo.AddrMatch(ia.ObjectAddress),
							},
							bson.M{"$set": bson.M{"ack": ia.Ack, "ackTimeTag": ia.AckTimeTag}},
							options.FindOneAndUpdate().SetSort(bson.M{"$natural": -1}),
						)
						cancel()
					default:
						break ackLoop
					}
				}

				// acquired values
				var writes []mongo.WriteModel
				var insertDocs []interface{}
			dataLoop:
				for len(writes) < 5000 {
					select {
					case iv := <-e.dataQ:
						// Auto-create a new tag document on first value for a new {CA, IOA} pair
						srv := e.FindConn(iv.ConnNumber)
						if srv != nil && srv.Cfg.AutoCreateTags {
							key := [2]int{iv.CommonAddress, iv.Address}
							if !srv.inserted[key] {
								srv.inserted[key] = true
								newID := GetNextAutoKey(srv, collection)
								insertDocs = append(insertDocs, NewRealtimeTagDoc(iv, srv.Cfg.Name, newID))
								jslog.Log(jslog.LevelBasic, "%s - INSERT NEW TAG: %s;%d;%d",
									srv.Cfg.Name, srv.Cfg.Name, iv.CommonAddress, iv.Address)
							}
						}
						// addresses are matched as number or string (they may be
						// configured with either representation)
						filter := bson.M{
							"protocolSourceConnectionNumber": iv.ConnNumber,
							"protocolSourceCommonAddress":    jsmongo.AddrMatch(iv.CommonAddress),
							"protocolSourceObjectAddress":    jsmongo.AddrMatch(iv.Address),
						}
						jslog.Log(jslog.LevelDetailed, "MongoDB - ADD %d %s", iv.Address, formatValue(iv.Value))
						writes = append(writes, mongo.NewUpdateOneModel().
							SetFilter(filter).SetUpdate(e.sourceDataUpdateDoc(iv)))
					default:
						break dataLoop
					}
				}

				// Flush inserts before updates so the update filter finds the new docs
				if len(insertDocs) > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					_, err := collection.InsertMany(ctx, insertDocs, options.InsertMany().SetOrdered(false))
					cancel()
					if err != nil {
						// Duplicate key on redundancy failover is tolerated
						jslog.Log(jslog.LevelDetailed, "%s", "Mongo - InsertMany error (possible duplicate): "+err.Error())
					}
				}

				if len(writes) > 0 {
					jslog.Log(jslog.LevelBasic, "MongoDB - Bulk write %d", len(writes))
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					_, err := collection.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
					cancel()
					if err != nil {
						jslog.Log(jslog.LevelBasic, "%s", "Exception Mongo: "+err.Error())
						time.Sleep(3 * time.Second)
						return // reconnect
					}
				} else {
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	}
}

// RunCommandsStream watches commandsQueue inserts via change stream and
// forwards valid commands to the RTU (port of MongoCommands.cs).
func (e *Engine) RunCommandsStream() {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - panic recovered: %v", r)
				}
			}()
			client, db, err := jsmongo.ConnectAndPing(e.Cfg)
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s", "Exception MongoCmd: "+err.Error())
				time.Sleep(3 * time.Second)
				return
			}
			defer client.Disconnect(context.Background())
			collection := db.Collection(jsmongo.CommandsQueueCollectionName)

			jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - Start listening for commands via changestream...")
			pipeline := jscommands.InsertOnlyPipeline()
			ctx := context.Background()
			stream, err := collection.Watch(ctx, pipeline)
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s", "Exception MongoCmd: "+err.Error())
				time.Sleep(3 * time.Second)
				return
			}
			defer stream.Close(ctx)

			for stream.Next(ctx) {
				var change struct {
					OperationType string `bson:"operationType"`
					FullDocument  bson.M `bson:"fullDocument"`
				}
				if err := stream.Decode(&change); err != nil || change.OperationType != "insert" {
					continue
				}
				if !e.Active() {
					continue
				}
				e.processCommand(collection, change.FullDocument)
			}
			if err := stream.Err(); err != nil {
				jslog.Log(jslog.LevelBasic, "%s", "Exception MongoCmd: "+err.Error())
			}
			time.Sleep(3 * time.Second)
		}()
	}
}

func (e *Engine) processCommand(collection *mongo.Collection, doc bson.M) {
	connNumber := int(jsmongo.GetDouble(doc, "protocolSourceConnectionNumber", 0))
	objAddr := int(jsmongo.GetU32(doc, "protocolSourceObjectAddress", 0))
	commonAddr := int(jsmongo.GetU32(doc, "protocolSourceCommonAddress", 0))
	asduNum := int(jsmongo.GetU32(doc, "protocolSourceASDU", 0))
	duration := int(jsmongo.GetDouble(doc, "protocolSourceCommandDuration", 0))
	useSbo := jsmongo.GetBool(doc, "protocolSourceCommandUseSBO", false)
	value := jsmongo.GetDouble(doc, "value", 0)
	timeTag := jsmongo.GetTime(doc, "timeTag")
	id := doc["_id"]

	setResult := func(field string, val interface{}) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{field: val}})
	}

	jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - Looking for connection %d...", connNumber)
	srv := e.FindConn(connNumber)
	if srv == nil {
		jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %d OA %d value %s Connection Not Found",
			connNumber, objAddr, formatValue(value))
		return
	}

	if !(srv.IsConnected != nil && srv.IsConnected() && srv.Cfg.CommandsEnabledVal()) {
		reason := "not connected"
		if !srv.Cfg.CommandsEnabledVal() {
			reason = "commands disabled"
		}
		jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s OA %d value %s %s",
			srv.Cfg.Name, objAddr, formatValue(value), reason)
		setResult("cancelReason", reason)
		return
	}

	// IEC 103 (and any driver that sets RawCmd) builds and sends the command
	// itself; the connected/enabled/expiry checks above and below still apply.
	if srv.RawCmd != nil {
		if time.Since(timeTag) >= jscommands.DefaultExpiry {
			jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s OA %d value %s Expired",
				srv.Cfg.Name, objAddr, formatValue(value))
			setResult("cancelReason", "expired")
			return
		}
		if err := srv.RawCmd(asduNum, objAddr, commonAddr, value, duration); err != nil {
			jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s - Error sending command: %s", srv.Cfg.Name, err.Error())
			setResult("cancelReason", "not connected")
			return
		}
		jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s - TI %d OA %d Delivered", srv.Cfg.Name, asduNum, objAddr)
		setResult("delivered", true)
		return
	}

	sc := conv.BuildInfoObj(asduNum, objAddr, value, useSbo, duration, conv.Quality{}, nil, 1, 0, false)
	if sc == nil {
		jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s OA %d value %s ASDU Not Implemented",
			srv.Cfg.Name, objAddr, formatValue(value))
		setResult("cancelReason", "asdu not implemented")
		return
	}

	// Expiry check. Note: uses total elapsed time; the C# driver used the
	// seconds *component* of the timespan, which could accept stale commands.
	if time.Since(timeTag) >= jscommands.DefaultExpiry {
		jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s OA %d value %s Expired",
			srv.Cfg.Name, objAddr, formatValue(value))
		setResult("cancelReason", "expired")
		return
	}

	if useSbo {
		// store the execute twin, to be sent when the select is confirmed
		exec := conv.BuildInfoObj(asduNum, objAddr, value, false, duration, conv.Quality{}, nil, 1, 0, false)
		srv.SetLastSelected(exec, commonAddr)
	}
	// execute or select
	if err := srv.SendCmd(sc, commonAddr); err != nil {
		jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s - Error sending command: %s", srv.Cfg.Name, err.Error())
		setResult("cancelReason", "not connected")
		return
	}
	jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - %s - TI %d OA %d Delivered", srv.Cfg.Name, asduNum, objAddr)
	setResult("delivered", true)
}
