/*
 * DNP3 Client Protocol driver for {json:scada}, in Go.
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

// Draining the value queue into realtimeData. Port of processMongo().

package clientapp

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsrtdata"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// The window outside which a timestamp is discarded, in milliseconds since the
// epoch: 2001-09-09 to 2033-05-18.
//
// parity: this is the C++ driver's guard against a device reporting a wild
// time. It also silently zeroes any legitimate timestamp outside that window,
// which will matter in 2033 (quirk Q2).
const (
	minSaneTimeMs = 1000000000000
	maxSaneTimeMs = 2000000000000
)

func sanitizeTime(ms int64) int64 {
	if ms < minSaneTimeMs || ms > maxSaneTimeMs {
		return 0
	}
	return ms
}

// mongoUpdateLoop drains the queue into realtimeData for the life of the
// process, reconnecting on failure.
func (e *Engine) mongoUpdateLoop(ctx context.Context) {
	jslog.Log(jslog.LevelBasic, "processMongo thread started")

	for ctx.Err() == nil {
		cli, err := jsmongo.Connect(e.cfg)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo: %v", err)
			sleepCtx(ctx, 3*time.Second)
			continue
		}
		db := cli.Database(e.cfg.MongoDatabaseName)
		jslog.Log(jslog.LevelDetailed, "processMongo: Connected to MongoDB")

		e.drainLoop(ctx, db)

		_ = cli.Disconnect(context.Background())
		sleepCtx(ctx, 3*time.Second)
	}
}

// drainLoop writes batches until MongoDB fails or the context ends.
func (e *Engine) drainLoop(ctx context.Context, db *mongo.Database) {
	rtd := db.Collection(jsmongo.RealtimeDataCollectionName)

	for ctx.Err() == nil {
		batch, squashed := e.queue.Drain()
		if len(batch) == 0 {
			// Only a closed queue drains empty.
			return
		}
		if squashed > 0 {
			// The queue was over its threshold, so this many event values were
			// folded into the entry already held for their point. The points
			// themselves are all still here; their intermediate values are not.
			jslog.Log(jslog.LevelBasic,
				"processMongo: %d event value(s) coalesced under load; %d values in this batch",
				squashed, len(batch))
		}
		if !jsmongo.IsLive(db) {
			jslog.Log(jslog.LevelDetailed, "processMongo: MongoDB connection lost, reconnecting...")
			return
		}
		if err := e.writeBatch(ctx, rtd, batch); err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo: %v", err)
			return
		}
	}
}

// writeBatch inserts any newly discovered tags and then applies one
// sourceDataUpdate per value.
//
// The inserts go first so that the updates that follow find the documents they
// were created for, which is the ordering the C++ driver relies on too.
func (e *Engine) writeBatch(ctx context.Context, rtd *mongo.Collection, batch []Dnp3Value) error {
	var (
		inserts []any
		writes  []mongo.WriteModel
	)
	// Points already written in this batch, so the bulk write can be ordered
	// only when it has to be.
	seen := make(map[[3]int]bool, len(batch))
	hasDuplicates := false

	for _, iv := range batch {
		if math.IsNaN(iv.Value) || math.IsInf(iv.Value, 0) || iv.Value > 1e100 || iv.Value < -1e100 {
			jslog.Log(jslog.LevelDetailed, "Mongo: Skipping invalid value addr=%d val=%v",
				iv.Address, iv.Value)
			continue
		}

		conn := e.connByNumber(iv.ConnNumber)
		if conn != nil {
			if docs := autoCreateFor(ctx, conn, rtd, iv); len(docs) > 0 {
				inserts = append(inserts, docs...)
			}
		}

		serverTime := sanitizeTime(iv.ServerTimestamp)
		sourceTime := int64(0)
		if iv.HasSourceTimestamp {
			sourceTime = sanitizeTime(iv.SourceTimestamp)
		}

		jslog.Log(jslog.LevelDetailed, "Mongo: Writing conn=%d addr=%d group=%d val=%v",
			iv.ConnNumber, iv.Address, iv.BaseGroup, iv.Value)

		key := [3]int{iv.ConnNumber, iv.BaseGroup, iv.Address}
		if seen[key] {
			hasDuplicates = true
		}
		seen[key] = true

		// This filter is served by the standard realtimeData index on
		// (protocolSourceConnectionNumber, protocolSourceCommonAddress,
		// protocolSourceObjectAddress). Without it every update is a
		// collection scan and a large batch cannot finish; see the timeout
		// hint below.
		filter := bson.M{
			"protocolSourceConnectionNumber": iv.ConnNumber,
			"protocolSourceCommonAddress":    iv.BaseGroup,
			"protocolSourceObjectAddress":    iv.Address,
		}
		update := jsrtdata.SourceDataUpdate{
			ValueAtSource:               iv.Value,
			ValueStringAtSource:         iv.ValueString,
			AsduAtSource:                strconv.Itoa(iv.Group) + " " + strconv.Itoa(iv.Variation),
			CauseOfTransmissionAtSource: strconv.Itoa(iv.COT),
			TimeTagAtSource:             bson.DateTime(sourceTime),
			TimeTagAtSourceOk:           iv.TimeTagOk,
			TimeTag:                     bson.DateTime(serverTime),
			NotTopicalAtSource:          iv.Quality.NotTopical(),
			InvalidAtSource:             iv.Quality.Invalid(),
			OverflowAtSource:            iv.Quality.Overflow(),
			BlockedAtSource:             iv.Quality.Blocked(),
			SubstitutedAtSource:         iv.Quality.Substituted(),
			Extra: bson.M{
				"carryAtSource":     iv.Quality.Carry(),
				"transientAtSource": iv.Quality.Transient,
				"originator":        ProtocolDriverName + "|" + strconv.Itoa(iv.ConnNumber),
			},
		}.SetDoc()

		writes = append(writes, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update))
	}

	if len(inserts) > 0 {
		tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, err := rtd.InsertMany(tctx, inserts, options.InsertMany().SetOrdered(false))
		cancel()
		if err != nil {
			// A duplicate key here means another node created the tag first,
			// which is not fatal.
			jslog.Log(jslog.LevelDetailed, "Mongo: insert_many error (possible duplicate): %v", err)
		}
	}
	if len(writes) > 0 {
		tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		// Ordering is only needed when the batch carries more than one value
		// for a point, which after the queue's coalescing means an event
		// sequence. Unordered execution would then leave which value survives
		// in the document up to the server; ordered execution is sequential
		// and much slower, so it is used only when it buys something.
		_, err := rtd.BulkWrite(tctx, writes, options.BulkWrite().SetOrdered(hasDuplicates))
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) && len(writes) > 500 {
				jslog.Log(jslog.LevelNoLog,
					"processMongo: a bulk write of %d values timed out. Check that realtimeData "+
						"carries the index on (protocolSourceConnectionNumber, "+
						"protocolSourceCommonAddress, protocolSourceObjectAddress); without it "+
						"every update scans the collection.", len(writes))
			}
			return err
		}
	}
	return nil
}

// invalidatePoints marks every point of a connection invalid after its link
// drops. Port of the ChannelListener::OnStateChange handler.
func (e *Engine) invalidatePoints(ctx context.Context, connNumber int, name string) {
	cli, err := jsmongo.Connect(e.cfg)
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "%s - Failed to invalidate points: %v", name, err)
		return
	}
	defer func() { _ = cli.Disconnect(context.Background()) }()

	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = cli.Database(e.cfg.MongoDatabaseName).
		Collection(jsmongo.RealtimeDataCollectionName).
		UpdateMany(tctx,
			bson.M{"protocolSourceConnectionNumber": connNumber},
			bson.M{"$set": bson.M{"invalid": true, "timeTag": jsmongo.Now()}})
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "%s - Failed to invalidate points: %v", name, err)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
