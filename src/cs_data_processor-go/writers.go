/*
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

// Batching writers.
//
// The Node.js implementation drains its FIFO queues on fixed setTimeout
// cycles (150 ms for realtimeData, 333 ms for the historian), so an update
// waits on average half a cycle before it is even sent. Here each writer is a
// goroutine that blocks on its channel and starts a linger timer only when
// the first item of a batch arrives: a lone update leaves after the linger
// (20 ms by default, and configurable down to 0), while a burst is flushed as
// soon as it reaches the maximum batch size.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// StartWriters launches the three writer goroutines.
func (p *Processor) StartWriters(ctx context.Context) {
	go p.rtDataWriter(ctx)
	go p.histAndSQLWriter(ctx)
	go p.soeWriter(ctx)
}

// ---------------------------------------------------------------------------
// realtimeData writer
// ---------------------------------------------------------------------------

func (p *Processor) rtDataWriter(ctx context.Context) {
	batch := make([]*rtUpdate, 0, p.cfg.RtWriteMaxBatch)
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	timerArmed := false

	flush := func() {
		if timerArmed {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timerArmed = false
		}
		if len(batch) == 0 {
			return
		}
		p.flushRtBatch(ctx, batch)
		for i := range batch {
			batch[i] = nil
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case u := <-p.rtCh:
			batch = append(batch, u)
			if len(batch) >= p.cfg.RtWriteMaxBatch {
				flush()
				continue
			}
			if !timerArmed {
				if p.cfg.RtWriteLinger <= 0 {
					flush()
					continue
				}
				timer.Reset(p.cfg.RtWriteLinger)
				timerArmed = true
			}
		case <-timer.C:
			timerArmed = false
			flush()
		}
	}
}

func (p *Processor) flushRtBatch(ctx context.Context, batch []*rtUpdate) {
	if !Mongo.WaitReady(ctx) {
		return
	}
	_, rtFast, _, _, _, ok := Mongo.Handles()
	if !ok || rtFast == nil {
		return
	}

	flushStart := hrNow()
	models := make([]mongo.WriteModel, 0, len(batch)+8)
	linger := M.Stage(StageWriteLinger)
	for _, u := range batch {
		linger.Observe(hrSub(flushStart, u.enqueuedAt).Microseconds())
		filter := bson.D{{Key: "_id", Value: rawOrNil(u.id)}}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(bson.D{{Key: "$set", Value: u.set}}))
		if len(u.addToSet) > 0 {
			models = append(models, mongo.NewUpdateOneModel().
				SetFilter(filter).
				SetUpdate(bson.D{{Key: "$addToSet", Value: u.addToSet}}))
		}
	}

	writeStart := hrNow()
	_, err := rtFast.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	M.Stage(StageBulkWrite).ObserveHrSince(writeStart)
	done := time.Now()
	if err != nil && ctx.Err() == nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Error on Mongodb query! %v", err)
		return
	}
	M.Inc(CntMongoBulkWrites, 1)
	M.Inc(CntMongoDocsWritten, int64(len(models)))

	e2e := M.Stage(StageEndToEnd)
	for _, u := range batch {
		if u.hasSource {
			e2e.Observe(done.Sub(u.sourceTime).Microseconds())
		}
	}
	Log(LogLevelNormal, "Mongo Updates %d", len(models))
}

// ---------------------------------------------------------------------------
// historian + PostgreSQL SQL files writer
// ---------------------------------------------------------------------------

func (p *Processor) histAndSQLWriter(ctx context.Context) {
	interval := p.cfg.HistWriteLinger
	if interval <= 0 {
		interval = 10 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	hist := make([]histEntry, 0, 256)
	rtSQL := make([]string, 0, 256)

	flush := func() {
		if len(hist) == 0 && len(rtSQL) == 0 {
			return
		}
		start := hrNow()
		p.writeHistBatch(ctx, hist)
		p.writeRtSQLBatch(rtSQL)
		M.Stage(StageHistWrite).ObserveHrSince(start)
		hist = hist[:0]
		rtSQL = rtSQL[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case e := <-p.histCh:
			hist = append(hist, e)
			if len(hist) >= p.cfg.HistWriteMaxBatch {
				flush()
			}
		case s := <-p.sqlRtCh:
			rtSQL = append(rtSQL, s)
			if len(rtSQL) >= p.cfg.HistWriteMaxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (p *Processor) writeHistBatch(ctx context.Context, entries []histEntry) {
	if len(entries) == 0 {
		return
	}
	Log(LogLevelNormal, "PGSQL/Mongo Hist updates %d", len(entries))

	// MongoDB historian
	_, _, histColl, _, _, ok := Mongo.Handles()
	if ok && histColl != nil {
		docs := make([]any, 0, len(entries))
		for _, e := range entries {
			docs = append(docs, e.obj)
		}
		if _, err := histColl.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false)); err != nil && ctx.Err() == nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Error on Mongodb query! %v", err)
		} else {
			M.Inc(CntHistDocsWritten, int64(len(docs)))
		}
	}

	// PostgreSQL file, picked up by src/process_pg_sql
	var b strings.Builder
	b.Grow(len(entries) * 160)
	b.WriteString("START TRANSACTION;\n")
	b.WriteString("INSERT INTO hist (tag, time_tag, value, value_json, time_tag_at_source, flags) VALUES ")
	for _, e := range entries {
		b.WriteString("\n(")
		b.WriteString(e.sql)
		b.WriteString("),")
	}
	sqlText := b.String()
	sqlText = sqlText[:len(sqlText)-1] + " \n" // drop the last comma
	sqlText += "ON CONFLICT (tag, time_tag) DO NOTHING;\n"
	sqlText += "COMMIT;\n"
	p.writeSQLFile("pg_hist_", sqlText)
}

func (p *Processor) writeRtSQLBatch(rows []string) {
	if len(rows) == 0 {
		return
	}
	Log(LogLevelNormal, "PGSQL RT updates %d", len(rows))

	var b strings.Builder
	b.Grow(len(rows) * 512)
	b.WriteString("WITH ordered_values AS (  SELECT DISTINCT ON (tag) tag, time_tag, json_data FROM (VALUES ")
	for _, r := range rows {
		b.WriteString("\n (")
		b.WriteString(r)
		b.WriteString("),")
	}
	sqlText := b.String()
	sqlText = sqlText[:len(sqlText)-1] + " \n" // drop the last comma
	sqlText += `) AS t(tag, time_tag, json_data)
          ORDER BY tag, time_tag DESC
        )
        INSERT INTO realtime_data (tag, time_tag, json_data)
        SELECT tag, time_tag::timestamptz, json_data::jsonb
        FROM ordered_values
        ON CONFLICT (tag) DO UPDATE 
        SET time_tag = EXCLUDED.time_tag,
            json_data = EXCLUDED.json_data;
    `
	p.writeSQLFile("pg_rtdata_", sqlText)
}

var sqlFileSeq atomic.Int64

// writeSQLFile drops a SQL file in the folder scanned by the
// src/process_pg_sql loader, keeping the file name pattern of the Node.js
// version (<prefix><epoch ms>_<instance>.sql).
func (p *Processor) writeSQLFile(prefix, content string) {
	dir := p.cfg.SQLFilesDir
	name := fmt.Sprintf("%s%d_%d.sql", prefix, time.Now().UnixMilli(), p.cfg.Instance)
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		// same millisecond as a previous flush, keep the names unique
		name = fmt.Sprintf("%s%d-%d_%d.sql", prefix, time.Now().UnixMilli(),
			sqlFileSeq.Add(1), p.cfg.Instance)
		path = filepath.Join(dir, name)
	}
	// Write to a temporary name first so the SQL processor never picks up a
	// partially written file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Error writing SQL file! %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Error writing SQL file! %v", err)
		os.Remove(tmp)
		return
	}
	M.Inc(CntSQLFilesWritten, 1)
}

// ---------------------------------------------------------------------------
// soeData writer
// ---------------------------------------------------------------------------

func (p *Processor) soeWriter(ctx context.Context) {
	batch := make([]any, 0, p.cfg.SoeWriteMaxBatch)
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	timerArmed := false

	flush := func() {
		if timerArmed {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timerArmed = false
		}
		if len(batch) == 0 {
			return
		}
		if Mongo.WaitReady(ctx) {
			if _, _, _, soe, _, ok := Mongo.Handles(); ok && soe != nil {
				start := hrNow()
				_, err := soe.InsertMany(ctx, batch, options.InsertMany().SetOrdered(false))
				M.Stage(StageSoeWrite).ObserveHrSince(start)
				if err != nil && ctx.Err() == nil {
					M.Inc(CntErrors, 1)
					Log(LogLevelMin, "Error on Mongodb query! %v", err)
				}
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case d := <-p.soeCh:
			batch = append(batch, d)
			if len(batch) >= p.cfg.SoeWriteMaxBatch {
				flush()
				continue
			}
			if !timerArmed {
				if p.cfg.SoeWriteLinger <= 0 {
					flush()
					continue
				}
				timer.Reset(p.cfg.SoeWriteLinger)
				timerArmed = true
			}
		case <-timer.C:
			timerArmed = false
			flush()
		}
	}
}
