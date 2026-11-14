/*
 * A process that watches for raw data updates from protocols using a MongoDB
 * change stream. Converts raw protocol values into analogs/statuses then
 * updates realtime, soe and historical data.
 *
 * Go implementation of src/cs_data_processor (Node.js).
 *
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

package main

import (
	"bytes"
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	cfg := LoadConfig()

	Log(LogLevelMin, "Connecting to MongoDB server...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sig
		Log(LogLevelMin, "Signal %v received, terminating...", s)
		cancel()
		// give the writers a moment to drain their current batch
		time.Sleep(1500 * time.Millisecond)
		os.Exit(0)
	}()

	proc := NewProcessor(cfg)
	M.SetGaugeProvider(proc.QueueDepths)
	StartMetricsReporter(cfg)
	proc.StartWorkers()
	proc.StartWriters(ctx)
	StartRedundancy(ctx, cfg, 5*time.Second, proc)
	StartMaintenance(ctx, cfg)
	StartCommStatus(ctx, cfg)

	runChangeStream(ctx, cfg, proc)
}

// changeStreamPipeline builds the same aggregation pipeline the Node.js
// version uses to filter the change stream server side.
func changeStreamPipeline(cfg Config) mongo.Pipeline {
	and := bson.A{
		bson.D{{Key: "fullDocument.value", Value: bson.D{{Key: "$exists", Value: true}}}},
	}
	if len(cfg.DivideProcessingExpression) > 0 {
		and = append(and, cfg.DivideProcessingExpression)
	}
	and = append(and,
		bson.D{{Key: "updateDescription.updatedFields.sourceDataUpdate",
			Value: bson.D{{Key: "$exists", Value: true}}}},
		bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "operationType", Value: "update"}},
			bson.D{{Key: "operationType", Value: "insert"}},
		}}},
	)
	return mongo.Pipeline{
		bson.D{{Key: "$project", Value: bson.D{{Key: "documentKey", Value: false}}}},
		bson.D{{Key: "$match", Value: bson.D{{Key: "$and", Value: and}}}},
	}
}

// runChangeStream owns the connection lifecycle: connect, watch, read until
// an error, then reconnect resuming from the last token when possible.
func runChangeStream(ctx context.Context, cfg Config, proc *Processor) {
	var resumeToken bson.Raw
	var prevResumeToken bson.Raw
	pipeline := changeStreamPipeline(cfg)

	for ctx.Err() == nil {
		client, err := mongo.Connect(cfg.mongoClientOptions())
		if err != nil {
			Log(LogLevelMin, "%v", err)
			sleepCtx(ctx, 5*time.Second)
			continue
		}
		if !pingMongo(client, 10*time.Second) {
			Log(LogLevelMin, "Disconnected Mongodb!")
			client.Disconnect(context.Background())
			sleepCtx(ctx, 5*time.Second)
			continue
		}

		db := client.Database(cfg.MongoDatabaseName)
		Mongo.Set(client, db)
		Log(LogLevelMin, "Connected correctly to MongoDB server")
		if resumeToken != nil {
			Log(LogLevelMin, "resumeToken: %s", resumeToken.String())
		}

		createSpecialTags(ctx)

		connCtx, connCancel := context.WithCancel(ctx)

		// connection watchdog, equivalent to the 5 s checkConnectedMongo
		// loop of the Node.js version
		go func() {
			t := time.NewTicker(5 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-connCtx.Done():
					return
				case <-t.C:
					if !pingMongo(client, 10*time.Second) {
						Log(LogLevelMin, "Disconnected Mongodb!")
						connCancel()
						return
					}
				}
			}
		}()

		csOpts := options.ChangeStream().
			SetFullDocument(options.UpdateLookup).
			SetBatchSize(cfg.CSBatchSize)
		if cfg.CSMaxAwaitTime > 0 {
			csOpts = csOpts.SetMaxAwaitTime(cfg.CSMaxAwaitTime)
		}
		if resumeToken != nil {
			csOpts = csOpts.SetResumeAfter(resumeToken)
		}

		coll := db.Collection(RealtimeDataCollectionName)
		cs, err := coll.Watch(connCtx, pipeline, csOpts)
		if err != nil {
			Log(LogLevelMin, "Error on ChangeStream! %v", err)
			M.Inc(CntChangeStreamRetry, 1)
			resumeToken, prevResumeToken = invalidateToken(resumeToken, prevResumeToken)
			connCancel()
			Mongo.Clear()
			client.Disconnect(context.Background())
			sleepCtx(ctx, 5*time.Second)
			continue
		}

		// ---- read loop: the only thing done here is copying the event and
		// handing it to the workers, so the cursor is never the bottleneck
		for cs.Next(connCtx) {
			recvAt, recvHr := time.Now(), hrNow()
			M.Inc(CntChangesReceived, 1)
			raw := bson.Raw(bytes.Clone(cs.Current))
			if tok := cs.ResumeToken(); tok != nil {
				resumeToken = bson.Raw(bytes.Clone(tok))
			}
			proc.Submit(changeEvent{raw: raw, recvAt: recvAt, recvHr: recvHr})
		}

		if err := cs.Err(); err != nil && ctx.Err() == nil {
			Log(LogLevelMin, "Error on ChangeStream! %v", err)
			resumeToken, prevResumeToken = invalidateToken(resumeToken, prevResumeToken)
		} else {
			Log(LogLevelMin, "Closed ChangeStream!")
		}
		M.Inc(CntChangeStreamRetry, 1)

		cs.Close(context.Background())
		connCancel()
		Mongo.Clear()
		client.Disconnect(context.Background())
		sleepCtx(ctx, 5*time.Second)
	}
}

// invalidateToken drops a resume token that already failed once, so that a
// permanently invalid token does not block reconnection forever.
func invalidateToken(resume, prev bson.Raw) (bson.Raw, bson.Raw) {
	if resume != nil && prev != nil && bytes.Equal(resume, prev) {
		return nil, nil
	}
	return resume, resume
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
