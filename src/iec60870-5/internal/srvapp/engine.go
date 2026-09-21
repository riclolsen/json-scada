/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - server engine
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Shared engine of the IEC 101/104 server drivers: realtimeData change
 * stream and spontaneous-data batcher (port of MongoChangeStream.cs).
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

package srvapp

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// TimeToExpireCommandsWithTime is the validity window in seconds for
// time-tagged commands received by the servers (C# parity).
const TimeToExpireCommandsWithTime = 20 * time.Second

type queuedInfo struct {
	obj *conv.InfoObject
	ca  int
}

// infoQueue is a bounded FIFO with type/CA-aware batch popping
// (port of the C# infoCAQueue + DequeueIecInfo assembly logic).
type infoQueue struct {
	mu    sync.Mutex
	items []queuedInfo
	limit int
}

func (q *infoQueue) push(it queuedInfo) {
	q.mu.Lock()
	if q.limit > 0 && len(q.items) >= q.limit {
		q.items = q.items[1:] // drop oldest
	}
	q.items = append(q.items, it)
	q.mu.Unlock()
}

// popBatch pops the head plus following entries with the same TypeID and
// common address, up to max entries (C# parity: max 30 objects per ASDU).
func (q *infoQueue) popBatch(max int) []queuedInfo {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	head := q.items[0]
	cnt := 1
	for cnt < len(q.items) && cnt < max &&
		q.items[cnt].obj.TypeID == head.obj.TypeID && q.items[cnt].ca == head.ca {
		cnt++
	}
	batch := q.items[:cnt]
	q.items = q.items[cnt:]
	return batch
}

// Conn is the driver-side state of one served protocol connection.
type Conn struct {
	Cfg model.ConnCfg
	// Endpoint is the protocol server object; Send broadcasts/buffers.
	Endpoint asdu.Connect
	// HasSession reports whether at least one master is connected/activated.
	HasSession func() bool
	// OriginatorIP returns the remote endpoint(s) string for command records.
	OriginatorIP func() string

	outQ infoQueue
}

// EnqueueInfo queues one spontaneous information object for sending.
func (c *Conn) EnqueueInfo(obj *conv.InfoObject, ca int) {
	c.outQ.push(queuedInfo{obj: obj, ca: ca})
}

// Engine is the shared server-driver engine.
type Engine struct {
	Cfg        jsconfig.Config
	DriverName string
	Conns      []*Conn

	client      *mongo.Client
	db          *mongo.Database
	dbMu        sync.Mutex
	IsMongoLive atomic.Bool
}

// New creates the engine and opens its MongoDB handle.
func New(cfg jsconfig.Config, driverName string) (*Engine, error) {
	e := &Engine{Cfg: cfg, DriverName: driverName}
	client, db, err := jsmongo.ConnectAndPing(cfg)
	if err != nil {
		return nil, err
	}
	e.client, e.db = client, db
	e.IsMongoLive.Store(true)
	return e, nil
}

// DB returns the engine database handle.
func (e *Engine) DB() *mongo.Database {
	e.dbMu.Lock()
	defer e.dbMu.Unlock()
	return e.db
}

// MaintainMongo pings MongoDB every second, reconnecting on failure
// (port of the C# server main loop). Runs forever; call as goroutine.
func (e *Engine) MaintainMongo() {
	for {
		if e.client == nil || !jsmongo.PingOK(e.client) {
			e.IsMongoLive.Store(false)
			jslog.Log(jslog.LevelBasic, "Exception Mongo - Error on MongoDB connection")
			if e.client != nil {
				_ = e.client.Disconnect(context.Background())
			}
			client, db, err := jsmongo.ConnectAndPing(e.Cfg)
			if err == nil {
				e.dbMu.Lock()
				e.client, e.db = client, db
				e.dbMu.Unlock()
				e.IsMongoLive.Store(true)
			} else {
				e.client = nil
				time.Sleep(3 * time.Second)
				continue
			}
		} else {
			e.IsMongoLive.Store(true)
		}
		time.Sleep(1 * time.Second)
	}
}

// RunDequeueLoop drains the per-connection out-queues, assembling
// multi-object spontaneous ASDUs (port of DequeueIecInfo).
func (e *Engine) RunDequeueLoop() {
	for {
		sentSomething := false
		for _, srv := range e.Conns {
			// single-redundancy-group emulation: hold events while no
			// master is connected (lib60870 buffers in this mode)
			if !srv.Cfg.MultiActiveVal() && srv.HasSession != nil && !srv.HasSession() {
				continue
			}
			batch := srv.outQ.popBatch(30)
			if len(batch) == 0 {
				continue
			}
			sentSomething = true
			objs := make([]*conv.InfoObject, 0, len(batch))
			for _, it := range batch {
				objs = append(objs, it.obj)
			}
			coa := asdu.CauseOfTransmission{Cause: asdu.Spontaneous}
			err := conv.SendInfoBatch(srv.Endpoint, coa, asdu.CommonAddr(batch[0].ca), batch[0].obj.TypeID, objs)
			if err != nil {
				jslog.Log(jslog.LevelDetailed, "%s - Error sending spontaneous data: %s", srv.Cfg.Name, err.Error())
			} else {
				jslog.Log(jslog.LevelBasic, "%s - Spont ASDU Type: %d with %d objects",
					srv.Cfg.Name, batch[0].obj.TypeID, len(batch))
			}
		}
		if !sentSomething {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// RunRealtimeStream watches realtimeData updates via change stream and
// queues data to the served connections (port of ProcessMongoCS).
func (e *Engine) RunRealtimeStream() {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					jslog.Log(jslog.LevelBasic, "MongoCS - panic recovered: %v", r)
				}
			}()
			client, db, err := jsmongo.ConnectAndPing(e.Cfg)
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s", "Exception MongoCS: "+err.Error())
				time.Sleep(3 * time.Second)
				return
			}
			defer client.Disconnect(context.Background())
			collection := db.Collection(jsmongo.RealtimeDataCollectionName)

			jslog.Log(jslog.LevelBasic, "MongoDB CMD CS - Start listening for realtime data updates via changestream...")
			// observe updates and replaces, avoid updates with sourceDataUpdate
			// field (those are handled by cs_data_processor), require
			// protocolDestinations not null (same filter as the C# driver)
			pipeline := mongo.Pipeline{
				{{Key: "$match", Value: bson.M{"$or": bson.A{
					bson.M{"$and": bson.A{
						bson.M{"fullDocument.protocolDestinations": bson.M{"$ne": nil}},
						bson.M{"updateDescription.updatedFields.sourceDataUpdate": bson.M{"$exists": false}},
						bson.M{"operationType": "update"},
					}},
					bson.M{"operationType": "replace"},
				}}}},
			}
			ctx := context.Background()
			stream, err := collection.Watch(ctx, pipeline,
				options.ChangeStream().SetFullDocument(options.UpdateLookup))
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s", "Exception MongoCS: "+err.Error())
				time.Sleep(3 * time.Second)
				return
			}
			defer stream.Close(ctx)

			for stream.Next(ctx) {
				var change struct {
					OperationType string            `bson:"operationType"`
					FullDocument  model.RtDataPoint `bson:"fullDocument"`
				}
				if err := stream.Decode(&change); err != nil {
					continue
				}
				if change.OperationType != "update" && change.OperationType != "replace" {
					continue
				}
				e.processPointUpdate(&change.FullDocument)
			}
			if err := stream.Err(); err != nil {
				jslog.Log(jslog.LevelBasic, "%s", "Exception MongoCS: "+err.Error())
			}
			time.Sleep(3 * time.Second)
		}()
	}
}

func (e *Engine) processPointUpdate(doc *model.RtDataPoint) {
	if doc.ProtocolDestinations == nil {
		return
	}
	for _, dst := range doc.ProtocolDestinations {
		for _, srv := range e.Conns {
			if int(dst.ConnectionNumber) != int(srv.Cfg.ProtocolConnectionNumber) {
				continue
			}
			quality := conv.Quality{
				Invalid:     doc.Invalid || doc.Overflow || doc.Transient,
				Substituted: doc.Substituted,
			}
			var timeTag *time.Time
			if doc.TimeTagAtSource != nil {
				t := doc.TimeTagAtSource.Add(time.Duration(dst.HoursShift * float64(time.Hour)))
				timeTag = &t
				// note: the C# driver flags the CP56 time invalid when
				// timeTagAtSourceOk is false; go-iecp5 does not expose the
				// IV bit on encoding, the timestamp is sent as-is
			}
			// C# parity: spontaneous distribution does NOT apply the
			// destination kconv1/kconv2 factors (BuildInfoObj is called
			// with default conversion in MongoChangeStream.cs)
			io := conv.BuildInfoObj(int(dst.ASDU), int(dst.ObjectAddress), doc.Value,
				false, 0, quality, timeTag, 1, 0, false)
			if io != nil {
				srv.EnqueueInfo(io, int(dst.CommonAddress))
				jslog.Log(jslog.LevelDetailed, "%s - Spont Tag:%s Value:%s Key:%v TI:%d CA:%d",
					srv.Cfg.Name, doc.Tag, formatValue(doc.Value), doc.ID, int(dst.ASDU), int(dst.CommonAddress))
			}
		}
	}
}
