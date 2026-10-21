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

// Acquired-value queue and the MongoDB writer. Port of MongoUpdate.cs.
//
// Only sourceDataUpdate is written for data; the tag value, alarms and
// history are all derived by cs_data_processor.

package main

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsrtdata"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

// dataQueue holds acquired values until the writer flushes them. It stands
// in for the C# ConcurrentQueue<OPC_Value>.
var dataQueue jsrtdata.Queue[OPCValue]

// enqueueValue queues one acquired value. Called from the notification
// pump, so it must never block.
func enqueueValue(ov OPCValue) { dataQueue.Enqueue(ov) }

func queueLen() int { return dataQueue.Len() }

func dequeueValue() (OPCValue, bool) { return dataQueue.Dequeue() }

// mongoUpdateLoop drains the acquired-value queue into realtimeData,
// inserting tags discovered by autoCreateTags along the way.
func mongoUpdateLoop(ctx context.Context, cfg jsconfig.Config, conns []*OPCUAConnection) {
	for ctx.Err() == nil {
		cli, _, err := jsmongo.ConnectAndPing(cfg)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		db := cli.Database(cfg.MongoDatabaseName)

		// parity: the C# driver writes with an unacknowledged write
		// concern, trading durability for throughput on a stream that is
		// refreshed continuously anyway.
		collRTD := db.Collection(jsmongo.RealtimeDataCollectionName,
			options.Collection().SetWriteConcern(writeconcern.Unacknowledged()))

		jslog.Log(jslog.LevelNoLog, "MongoDB Update Thread Started...")

		err = updateCycle(ctx, collRTD, conns)
		if err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(1 * time.Second)
		}
		_ = cli.Disconnect(context.Background())
	}
}

func updateCycle(ctx context.Context, collRTD *mongo.Collection, conns []*OPCUAConnection) error {
	var writes []mongo.WriteModel

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		start := time.Now()
		writes = writes[:0]

		for {
			ov, ok := dequeueValue()
			if !ok {
				break
			}

			m := updateModel(ov)
			if ov.SelfPublish {
				if m == nil {
					// Too large to store: a new tag is still created, but
					// without the value, or the insert would fail too.
					ov.ValueString, ov.ValueJSON = "", ""
				}
				writes = append(writes, maybeInsertTag(ctx, collRTD, conns, &ov)...)
			}
			if m != nil {
				writes = append(writes, m)
			}

			if len(writes) >= BulkWriteLimit {
				break
			}
			if time.Since(start) > 750*time.Millisecond {
				jslog.Log(jslog.LevelBasic, "break ms %d", time.Since(start).Milliseconds())
				break
			}
		}

		if len(writes) > 0 {
			flushStart := time.Now()
			jslog.Log(jslog.LevelBasic, "MongoDB - Bulk writing %d, Total enqueued data %d", len(writes), queueLen())
			_, err := collRTD.BulkWrite(ctx, writes,
				options.BulkWrite().SetOrdered(false).SetBypassDocumentValidation(true))
			if err != nil {
				jslog.Log(jslog.LevelNoLog, "MongoDB - Bulk write error - %v", err)
			} else {
				ms := time.Since(flushStart).Milliseconds()
				ups := 0
				if ms > 0 {
					ups = int(float64(len(writes)) / (float64(ms) / 1000))
				}
				jslog.Log(jslog.LevelBasic, "MongoDB - Bulk written %d documents in %d ms, updates per second: %d",
					len(writes), ms, ups)
			}
		}

		if queueLen() == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
}

// maxUpdateBytes is the largest sourceDataUpdate this driver writes.
//
// deviation D26: the .NET driver only refused an update over MongoDB's 16 MB
// document limit. That is not enough: the data processor copies the value
// into the same realtimeData document (value, valueString, valueJson), so a
// tag whose update is far below 16 MB still ends up over it, and the write
// fails with BSONObjectTooLarge. 3 MB leaves room for those copies.
const maxUpdateBytes = 3 * 1024 * 1024

// updateTooLarge reports whether an update exceeds maxUpdateBytes once
// encoded, and its encoded size when it does.
//
// The rendered length is only a cheap pre-filter: encoded, a numeric array
// grows to many times its JSON text (an element is a type byte, its index as
// a key and 8 bytes of value, against "1," in JSON), so the pre-filter is set
// well below the limit and the decision is the real encoding.
func updateTooLarge(update bson.M, renderedLen int) (bool, int) {
	if renderedLen <= maxUpdateBytes/16 {
		return false, 0
	}
	raw, err := bson.Marshal(update)
	if err != nil {
		return false, 0
	}
	return len(raw) > maxUpdateBytes, len(raw)
}

// updateModel builds the sourceDataUpdate write for one acquired value.
func updateModel(ov OPCValue) mongo.WriteModel {
	var srcTime any
	if ov.HasSourceTimestamp {
		srcTime = bson.NewDateTimeFromTime(ov.SourceTimestamp)
	}

	var valBSON any
	if ov.ValueJSON != "" {
		if err := json.Unmarshal([]byte(ov.ValueJSON), &valBSON); err != nil {
			jslog.Log(jslog.LevelBasic, "%s - %v", ov.ConnName, err)
			valBSON = nil
		}
	}

	update := jsrtdata.SourceDataUpdate{
		ValueAtSource:               ov.Value,
		ValueStringAtSource:         ov.ValueString,
		AsduAtSource:                ov.Asdu,
		CauseOfTransmissionAtSource: strconv.Itoa(ov.Cot),
		TimeTagAtSource:             srcTime,
		TimeTagAtSourceOk:           ov.HasSourceTimestamp,
		TimeTag:                     bson.NewDateTimeFromTime(ov.ServerTimestamp),
		NotTopicalAtSource:          false,
		InvalidAtSource:             !ov.Quality,
		OverflowAtSource:            false,
		BlockedAtSource:             false,
		SubstitutedAtSource:         false,
		Extra: bson.M{
			"valueBsonAtSource": valBSON,
			"valueJsonAtSource": ov.ValueJSON,
		},
	}.SetDoc()

	// deviation D26: an update larger than maxUpdateBytes is logged and
	// dropped rather than failing with BSONObjectTooLarge.
	if over, size := updateTooLarge(update, len(ov.ValueJSON)+len(ov.ValueString)); over {
		jslog.Log(jslog.LevelNoLog,
			"%s - Update too large for %s - %d bytes (limit %d), discarded",
			ov.ConnName, ov.Address, size, maxUpdateBytes)
		return nil
	}

	// The origin filter keeps a supervised point from being updated by the
	// command tag that shares its object address.
	filter := jsrtdata.SupervisedFilter(float64(ov.ConnNumber), ov.Address)

	jslog.Log(jslog.LevelDebug, "MongoDB - ADD %s %v", ov.Address, ov.Value)

	return mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
}

// maybeInsertTag creates the tag of a point the driver discovered itself,
// plus its command twin when the variable is writable. Returns the inserts
// to add to the bulk write, which is empty for a point that already has a
// tag.
func maybeInsertTag(ctx context.Context, collRTD *mongo.Collection, conns []*OPCUAConnection, ov *OPCValue) []mongo.WriteModel {
	conn := connByNumber(conns, ov.ConnNumber)
	if conn == nil {
		// parity: an unknown connection number must not fall back to the
		// first connection, which would create the tag under the wrong one.
		jslog.Log(jslog.LevelDetailed, "%s - Connection number not found for address %s, skipping tag insert",
			ov.ConnName, ov.Address)
		return nil
	}

	if !conn.AddInsertedAddress(ov.Address) {
		return nil // already in realtimeData
	}

	tag := tagFromOPCParameters(*ov)
	jslog.Log(jslog.LevelDetailed, "%s - INSERT NEW TAG: %s - Addr:%s", ov.ConnName, tag, ov.Address)

	key := conn.TagKeys.Next(ctx, collRTD, ov.ConnNumber)

	// A value that arrived through a notification carries no browse path;
	// fill it in from what browsing recorded.
	if details, ok := conn.NodeDetailsFor(ov.Address); ok {
		ov.ParentName = details.ParentName
		ov.Path = details.Path
	} else {
		ov.ParentName = ""
		ov.Path = ""
		jslog.Log(jslog.LevelDetailed, "%s - NodeId not found in NodeIdsDetails: %s", ov.ConnName, ov.Address)
	}

	var writes []mongo.WriteModel

	// The command twin is inserted first so the supervised tag can point at
	// it, and it points back at the supervised key that follows it.
	commandOfSupervised := 0.0
	if conn.CommandsEnabled && ov.CreateCommandForSupervised {
		cmdDoc := newRealtimeDoc(*ov, key, 0)
		cmdDoc["protocolSourcePublishingInterval"] = 0.0
		cmdDoc["protocolSourceSamplingInterval"] = 0.0
		cmdDoc["protocolSourceQueueSize"] = 0.0
		writes = append(writes, mongo.NewInsertOneModel().SetDocument(cmdDoc))

		commandOfSupervised = key
		key = conn.TagKeys.Next(ctx, collRTD, ov.ConnNumber)
	}

	ov.CreateCommandForSupervised = false
	doc := newRealtimeDoc(*ov, key, commandOfSupervised)
	doc["protocolSourcePublishingInterval"] = conn.AutoCreateTagPublishingInterval
	doc["protocolSourceSamplingInterval"] = conn.AutoCreateTagSamplingInterval
	doc["protocolSourceQueueSize"] = conn.AutoCreateTagQueueSize
	writes = append(writes, mongo.NewInsertOneModel().SetDocument(doc))

	return writes
}
