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
	"errors"
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

// insertedTag is a tag insert in a bulk write: the point is marked as having
// a tag before the write, so a failed insert must clear the mark again.
type insertedTag struct {
	conn *Iec61850Connection
	key  string
}

func updateCycle(ctx context.Context, db *mongo.Database, collRTD *mongo.Collection, conns []*Iec61850Connection) error {
	var (
		writes  []mongo.WriteModel
		batch   []IECValue          // the values behind writes, to requeue
		inserts map[int]insertedTag // write index -> tag insert
	)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := jsmongo.Ping(db, 2500*time.Millisecond); err != nil {
			return err
		}

		start := time.Now()
		writes = writes[:0]
		batch = batch[:0]
		inserts = map[int]insertedTag{}

		for {
			iv, ok := dequeueValue()
			if !ok {
				break
			}

			if iv.SelfPublish {
				if insert, conn, key := maybeInsertTag(ctx, collRTD, conns, iv); insert != nil {
					inserts[len(writes)] = insertedTag{conn, key}
					writes = append(writes, insert)
				}
			}
			writes = append(writes, updateModel(iv))
			batch = append(batch, iv)

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
			var bwe mongo.BulkWriteException
			switch {
			case err == nil:
				jslog.Log(jslog.LevelBasic, "MongoDB - OK:%t - Inserted:%d - Updated:%d",
					res.Acknowledged, res.InsertedCount, res.ModifiedCount)
			case errors.As(err, &bwe) && bwe.WriteConcernError == nil && len(bwe.WriteErrors) > 0:
				// Individual writes were refused; the write is unordered, so
				// the others were applied. An insert refused as a duplicate
				// means the tag exists, which is what the mark says.
				for _, we := range bwe.WriteErrors {
					jslog.Log(jslog.LevelBasic, "MongoDB - write %d refused: %s", we.Index, we.Message)
					if ins, ok := inserts[we.Index]; ok && !mongo.IsDuplicateKeyError(we) {
						delete(ins.conn.InsertedTags, ins.key)
					}
				}
			default:
				// Nothing is known to have been written: keep the batch for
				// the next connection, and let its tag inserts run again (a
				// tag that did get in is refused as a duplicate by the unique
				// tag index).
				for _, ins := range inserts {
					delete(ins.conn.InsertedTags, ins.key)
				}
				dataQueue.Requeue(batch)
				return err
			}
		}

		// Controllable objects found while browsing get their tag here:
		// they carry no value, so nothing else would create them.
		createCommandTags(ctx, collRTD, conns)
		linkCommands(ctx, collRTD)

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

// commandInsertAttempts is how many writer cycles a failed command tag
// insert is retried before it is given up for this session.
const commandInsertAttempts = 20

// createCommandTags creates the tags of the controllable objects found
// while browsing. The command carries supervisedOfCommand and the supervised
// point commandOfSupervised; when the supervised point does not exist yet
// (it is created by the value path, which may only run after a long
// discovery), the command is created now and linked by linkCommands once it
// does. A device that exposes no status for a controllable object yields a
// command that stays unlinked: it works, but gives the operator no feedback.
func createCommandTags(ctx context.Context, collRTD *mongo.Collection, conns []*Iec61850Connection) {
	pending := takeCommandTags()
	for _, ct := range pending {
		conn := connByNumber(conns, ct.ConnNumber)
		if conn == nil {
			continue
		}
		tag := ct.Tag()
		key := pointKey(ct.Ref, "CO")
		if conn.InsertedTags[key] {
			continue
		}

		supervisedID := findSupervised(ctx, collRTD, ct.ConnNumber, ct.Ref)
		id := conn.TagKeys.Next(ctx, collRTD, conn.ProtocolConnectionNumber)
		insCtx, cancelIns := context.WithTimeout(ctx, 10*time.Second)
		_, err := collRTD.InsertOne(insCtx, newCommandDoc(ct, id, supervisedID))
		cancelIns()
		if mongo.IsDuplicateKeyError(err) {
			// The tag exists already (an earlier run): link it if it is not.
			conn.InsertedTags[key] = true
			var existing bson.M
			findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if collRTD.FindOne(findCtx, bson.M{"tag": tag}).Decode(&existing) == nil &&
				jsmongo.GetDouble(existing, "supervisedOfCommand", 0) == 0 {
				queueCommandLink(conn, ct.Ref, jsmongo.GetDouble(existing, "_id", 0))
			}
			cancel()
			continue
		}
		if err != nil {
			ct.Attempts++
			if ct.Attempts < commandInsertAttempts {
				enqueueCommandTag(ct) // retried next cycle
			} else {
				jslog.Log(jslog.LevelBasic, "%s - command tag insert failed for %s, giving up: %v", ct.ConnName, tag, err)
			}
			continue
		}
		conn.InsertedTags[key] = true

		if supervisedID == 0 {
			jslog.Log(jslog.LevelBasic, "%s - INSERT NEW COMMAND TAG: %s - Addr:%s - supervised point not found yet",
				ct.ConnName, tag, ct.Ref)
			queueCommandLink(conn, ct.Ref, id)
			continue
		}
		linkSupervised(ctx, collRTD, ct.ConnName, tag, supervisedID, id)
		jslog.Log(jslog.LevelBasic, "%s - INSERT NEW COMMAND TAG: %s - Addr:%s - supervised _id:%v",
			ct.ConnName, tag, ct.Ref, supervisedID)
	}
}

// findSupervised returns the _id of the supervised point of a controllable
// object, or 0. It is the same data object seen under its status or
// measurand constraint, so it is found by object address whatever its tag
// is called.
func findSupervised(ctx context.Context, collRTD *mongo.Collection, connNumber int, ref string) float64 {
	findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var sup bson.M
	if err := collRTD.FindOne(findCtx, bson.M{
		"protocolSourceConnectionNumber": float64(connNumber),
		"protocolSourceObjectAddress":    ref,
		"origin":                         "supervised",
	}).Decode(&sup); err != nil {
		return 0
	}
	return jsmongo.GetDouble(sup, "_id", 0)
}

// linkSupervised points a supervised tag at its command.
func linkSupervised(ctx context.Context, collRTD *mongo.Collection, connName, tag string, supervisedID, commandID float64) bool {
	updCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := collRTD.UpdateOne(updCtx,
		bson.M{"_id": supervisedID},
		bson.M{"$set": bson.M{"commandOfSupervised": commandID}}); err != nil {
		jslog.Log(jslog.LevelBasic, "%s - cannot link %s to its supervised point: %v", connName, tag, err)
		return false
	}
	return true
}

// commandLink is a command tag still waiting for its supervised point.
type commandLink struct {
	conn      *Iec61850Connection
	ref       string
	commandID float64
}

// commandLinks are the command tags created before their supervised point.
// Fed at startup (preloadEntries) and by createCommandTags; consumed by the
// writer goroutine.
var commandLinks struct {
	mu        sync.Mutex
	items     []commandLink
	lastCheck time.Time
}

func queueCommandLink(conn *Iec61850Connection, ref string, commandID float64) {
	if commandID == 0 {
		return
	}
	commandLinks.mu.Lock()
	commandLinks.items = append(commandLinks.items, commandLink{conn, ref, commandID})
	commandLinks.mu.Unlock()
}

// commandLinkInterval is how often unlinked commands look for their
// supervised point: one query per command, so not every writer cycle.
const commandLinkInterval = 5 * time.Second

// linkCommands links the command tags whose supervised point now exists.
// They wait for as long as the driver runs: a large IED may take minutes to
// be discovered and polled.
func linkCommands(ctx context.Context, collRTD *mongo.Collection) {
	commandLinks.mu.Lock()
	if len(commandLinks.items) == 0 || time.Since(commandLinks.lastCheck) < commandLinkInterval {
		commandLinks.mu.Unlock()
		return
	}
	commandLinks.lastCheck = time.Now()
	items := commandLinks.items
	commandLinks.items = nil
	commandLinks.mu.Unlock()

	var waiting []commandLink
	for _, l := range items {
		supervisedID := findSupervised(ctx, collRTD, l.conn.ProtocolConnectionNumber, l.ref)
		if supervisedID == 0 {
			waiting = append(waiting, l)
			continue
		}
		tag := (CommandTag{ConnName: l.conn.Name, Ref: l.ref}).Tag()
		updCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := collRTD.UpdateOne(updCtx,
			bson.M{"_id": l.commandID},
			bson.M{"$set": bson.M{"supervisedOfCommand": supervisedID}})
		cancel()
		if err != nil || !linkSupervised(ctx, collRTD, l.conn.Name, tag, supervisedID, l.commandID) {
			waiting = append(waiting, l)
			continue
		}
		jslog.Log(jslog.LevelBasic, "%s - LINKED COMMAND TAG: %s - supervised _id:%v", l.conn.Name, tag, supervisedID)
	}

	commandLinks.mu.Lock()
	commandLinks.items = append(waiting, commandLinks.items...)
	commandLinks.mu.Unlock()
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
// has autoCreateTags and the tag is not known yet. It returns the insert and
// the mark it set, which the caller clears if the insert fails.
func maybeInsertTag(ctx context.Context, collRTD *mongo.Collection, conns []*Iec61850Connection, iv IECValue) (mongo.WriteModel, *Iec61850Connection, string) {
	var conn *Iec61850Connection
	for _, c := range conns {
		if c.ProtocolConnectionNumber == iv.ConnNumber {
			conn = c
			break
		}
	}
	if conn == nil {
		return nil, nil, ""
	}

	key := pointKey(iv.Address, iv.CommonAddress)
	if conn.InsertedTags[key] {
		return nil, nil, ""
	}
	conn.InsertedTags[key] = true
	tag := TagFromParameters(iv)

	jslog.Log(jslog.LevelBasic, "%s - INSERT NEW TAG: %s - Addr:%s", iv.ConnName, tag, iv.Address)

	return mongo.NewInsertOneModel().SetDocument(newRealtimeDoc(iv, conn.TagKeys.Next(ctx, collRTD, conn.ProtocolConnectionNumber))), conn, key
}
