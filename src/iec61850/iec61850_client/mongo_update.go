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

// Acquired-value queue and the MongoDB writer. Port of MongoUpdate.cs.

package main

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsrtdata"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// dataQueue holds acquired values until the writer flushes them.
var dataQueue jsrtdata.Queue[IECValue]

// enqueueValue queues one acquired value. Called from report callbacks, so
// it must not block.
func enqueueValue(iv IECValue) { dataQueue.Enqueue(iv) }

func queueLen() int { return dataQueue.Len() }

// dequeueValue removes the oldest queued value.
func dequeueValue() (IECValue, bool) { return dataQueue.Dequeue() }

// trimQueue discards the oldest values when the database is unreachable and
// the queue outgrows its limit.
func trimQueue(limit int) {
	dataQueue.Trim(limit, func() {
		jslog.Log(jslog.LevelDetailed, "MongoDB - Dequeue Data")
	})
}

// mongoUpdateLoop drains the acquired-value queue into realtimeData,
// inserting tags discovered by autoCreateTags along the way.
func mongoUpdateLoop(ctx context.Context, cfg jsconfig.Config, conns []*Iec61850Connection) {
	for ctx.Err() == nil {
		cli, _, err := jsmongo.ConnectAndPing(cfg)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(1 * time.Second)
			trimQueue(DataBufferLimit)
			continue
		}
		db := cli.Database(cfg.MongoDatabaseName)
		collRTD := db.Collection(jsmongo.RealtimeDataCollectionName)

		jslog.Log(jslog.LevelNoLog, "MongoDB Update Thread Started...")

		err = updateCycle(ctx, db, collRTD, conns)
		if err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(1 * time.Second)
			trimQueue(DataBufferLimit)
		}
		_ = cli.Disconnect(context.Background())
	}
}

func updateCycle(ctx context.Context, db *mongo.Database, collRTD *mongo.Collection, conns []*Iec61850Connection) error {
	var writes []mongo.WriteModel

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := jsmongo.Ping(db, 2500*time.Millisecond); err != nil {
			return err
		}

		start := time.Now()
		writes = writes[:0]

		for {
			iv, ok := dequeueValue()
			if !ok {
				break
			}

			if iv.SelfPublish {
				if insert := maybeInsertTag(ctx, collRTD, conns, iv); insert != nil {
					writes = append(writes, insert)
				}
			}
			writes = append(writes, updateModel(iv))

			if len(writes) >= BulkWriteLimit {
				break
			}
			if time.Since(start) > 400*time.Millisecond {
				break
			}
		}

		if len(writes) > 0 {
			jslog.Log(jslog.LevelBasic, "MongoDB - Bulk writing %d, Total enqueued data %d", len(writes), queueLen())
			res, err := collRTD.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
			if err != nil {
				return err
			}
			jslog.Log(jslog.LevelBasic, "MongoDB - OK:%t - Inserted:%d - Updated:%d",
				res.Acknowledged, res.InsertedCount, res.ModifiedCount)
		}

		// Controllable objects found while browsing get their tag here:
		// they carry no value, so nothing else would create them.
		createCommandTags(ctx, collRTD, conns)

		if queueLen() == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
}

// updateModel builds the sourceDataUpdate write for one acquired value.
//
// parity: the filter carries no functional constraint, so two tags of the
// same connection sharing an object address under different constraints
// update each other — the same limitation the C# driver has.
func updateModel(iv IECValue) mongo.WriteModel {
	var srcTime any
	if iv.HasSourceTimestamp {
		srcTime = bson.NewDateTimeFromTime(iv.SourceTimestamp)
	}

	filter := jsrtdata.SupervisedFilter(float64(iv.ConnNumber), iv.Address)

	update := jsrtdata.SourceDataUpdate{
		ValueAtSource:               iv.Value,
		ValueStringAtSource:         iv.ValueString,
		AsduAtSource:                iv.Asdu,
		CauseOfTransmissionAtSource: strconv.Itoa(iv.Cot),
		TimeTagAtSource:             srcTime,
		TimeTagAtSourceOk:           iv.HasSourceTimestamp,
		TimeTag:                     bson.NewDateTimeFromTime(iv.ServerTimestamp),
		NotTopicalAtSource:          false,
		InvalidAtSource:             !iv.Quality,
		OverflowAtSource:            false,
		BlockedAtSource:             false,
		SubstitutedAtSource:         false,
		Extra: bson.M{
			"valueBsonAtSource": parseValueJSON(iv),
			"transientAtSource": iv.IsTransient,
			"originator":        ProtocolDriverName + "|" + strconv.Itoa(iv.ConnNumber),
		},
	}.SetDoc()

	jslog.Log(jslog.LevelDebug, "MongoDB - ADD %s %v", iv.Address, iv.Value)

	return mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update)
}

// parseValueJSON wraps the rendered value the way the C# driver does: the
// stored document is { a: <value> }, an artefact of how it parsed the JSON
// fragment. Keep it — the UI reads that shape today.
func parseValueJSON(iv IECValue) bson.M {
	var parsed any
	if err := json.Unmarshal([]byte(iv.ValueJSON), &parsed); err != nil {
		jslog.Log(jslog.LevelBasic, "%s - %v", iv.ConnName, err)
		return bson.M{}
	}
	return bson.M{"a": parsed}
}

// commandTagQueue holds controllable objects found while browsing, waiting
// for their tag to be created.
var commandTagQueue struct {
	mu    sync.Mutex
	items []CommandTag
}

func enqueueCommandTag(ct CommandTag) {
	commandTagQueue.mu.Lock()
	commandTagQueue.items = append(commandTagQueue.items, ct)
	commandTagQueue.mu.Unlock()
}

// takeCommandTags removes and returns everything queued.
func takeCommandTags() []CommandTag {
	commandTagQueue.mu.Lock()
	defer commandTagQueue.mu.Unlock()
	items := commandTagQueue.items
	commandTagQueue.items = nil
	return items
}

// commandLinkAttempts is how many writer cycles a command tag waits for its
// supervised twin to appear before it is created without the link. The twin
// is created by the value path, so it normally shows up within a cycle or
// two of the first poll or report.
const commandLinkAttempts = 40

// createCommandTags creates the tags of the controllable objects found
// while browsing, each linked to the point where its effect shows: the
// command carries supervisedOfCommand, the supervised point gets
// commandOfSupervised. A device that exposes no status for a controllable
// object yields an unlinked command, which still works but gives the
// operator no feedback.
func createCommandTags(ctx context.Context, collRTD *mongo.Collection, conns []*Iec61850Connection) {
	pending := takeCommandTags()
	for _, ct := range pending {
		conn := connByNumber(conns, ct.ConnNumber)
		if conn == nil {
			continue
		}
		tag := ct.Tag()
		if conn.InsertedTags[tag] {
			continue
		}

		// The supervised twin is the same data object seen under its
		// status or measurand constraint, so it is found by object
		// address whatever its tag is called.
		supervisedID := 0.0
		findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var sup bson.M
		err := collRTD.FindOne(findCtx, bson.M{
			"protocolSourceConnectionNumber": float64(ct.ConnNumber),
			"protocolSourceObjectAddress":    ct.Ref,
			"origin":                         "supervised",
		}).Decode(&sup)
		cancel()
		if err == nil {
			supervisedID = jsmongo.GetDouble(sup, "_id", 0)
		}
		if supervisedID == 0 && ct.Attempts < commandLinkAttempts {
			// The supervised tag is created by the value path; wait for it
			// rather than committing a blind command.
			ct.Attempts++
			enqueueCommandTag(ct)
			continue
		}

		id := conn.TagKeys.Next(ctx, collRTD, conn.ProtocolConnectionNumber)
		insCtx, cancelIns := context.WithTimeout(ctx, 10*time.Second)
		_, err = collRTD.InsertOne(insCtx, newCommandDoc(ct, id, supervisedID))
		cancelIns()
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%s - command tag insert failed for %s: %v", ct.ConnName, tag, err)
			continue
		}
		conn.InsertedTags[tag] = true

		if supervisedID != 0 {
			updCtx, cancelUpd := context.WithTimeout(ctx, 10*time.Second)
			_, err = collRTD.UpdateOne(updCtx,
				bson.M{"_id": supervisedID},
				bson.M{"$set": bson.M{"commandOfSupervised": id}})
			cancelUpd()
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s - cannot link %s to its supervised point: %v", ct.ConnName, tag, err)
			}
			jslog.Log(jslog.LevelBasic, "%s - INSERT NEW COMMAND TAG: %s - Addr:%s - supervised _id:%v",
				ct.ConnName, tag, ct.Ref, supervisedID)
		} else {
			jslog.Log(jslog.LevelBasic, "%s - INSERT NEW COMMAND TAG: %s - Addr:%s - no supervised point found",
				ct.ConnName, tag, ct.Ref)
		}
	}
}

func connByNumber(conns []*Iec61850Connection, number int) *Iec61850Connection {
	for _, c := range conns {
		if c.ProtocolConnectionNumber == number {
			return c
		}
	}
	return nil
}

// maybeInsertTag inserts a tag discovered in a report when the connection
// has autoCreateTags and the tag is not known yet.
func maybeInsertTag(ctx context.Context, collRTD *mongo.Collection, conns []*Iec61850Connection, iv IECValue) mongo.WriteModel {
	var conn *Iec61850Connection
	for _, c := range conns {
		if c.ProtocolConnectionNumber == iv.ConnNumber {
			conn = c
			break
		}
	}
	if conn == nil {
		return nil
	}

	tag := TagFromParameters(iv)
	if conn.InsertedTags[tag] {
		return nil
	}
	conn.InsertedTags[tag] = true

	jslog.Log(jslog.LevelBasic, "%s - INSERT NEW TAG: %s - Addr:%s", iv.ConnName, tag, iv.Address)

	return mongo.NewInsertOneModel().SetDocument(newRealtimeDoc(iv, conn.TagKeys.Next(ctx, collRTD, conn.ProtocolConnectionNumber)))
}
