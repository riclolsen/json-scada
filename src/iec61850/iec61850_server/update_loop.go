/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
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

// The model update path: a single writer drains the queue fed by the
// MongoDB change stream and applies batches atomically, which is what
// drives report generation.

package main

import (
	"context"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
)

// updateBatchLimit is how many point updates are applied under one model
// transaction, as in the C# driver.
const updateBatchLimit = 500

// PointUpdate is a pending value change to push into the model.
type PointUpdate struct {
	Point       *MappedPoint
	Value       float64
	ValueString string
	Invalid     bool
	Substituted bool
	Overflow    bool
	Transient   bool
	Test        bool

	SourceTime    time.Time
	HasSourceTime bool
	SourceTimeOk  bool
}

// updateQueue holds pending updates between the change stream and the
// model writer.
var updateQueue struct {
	mu    sync.Mutex
	items []PointUpdate
}

func enqueueUpdate(u PointUpdate) {
	updateQueue.mu.Lock()
	updateQueue.items = append(updateQueue.items, u)
	updateQueue.mu.Unlock()
}

func dequeueUpdate() (PointUpdate, bool) {
	updateQueue.mu.Lock()
	defer updateQueue.mu.Unlock()
	if len(updateQueue.items) == 0 {
		return PointUpdate{}, false
	}
	u := updateQueue.items[0]
	updateQueue.items = updateQueue.items[1:]
	return u, true
}

func updateQueueLen() int {
	updateQueue.mu.Lock()
	defer updateQueue.mu.Unlock()
	return len(updateQueue.items)
}

// updateFromPoint builds an update from a realtimeData document.
func updateFromPoint(mp *MappedPoint, p *Point) PointUpdate {
	return PointUpdate{
		Point:         mp,
		Value:         p.Value,
		ValueString:   p.ValueString,
		Invalid:       p.Invalid,
		Substituted:   p.Substituted,
		Overflow:      p.Overflow,
		Transient:     p.Transient,
		SourceTime:    p.TimeTagAtSource,
		HasSourceTime: p.HasTimeTagAtSource,
		SourceTimeOk:  p.TimeTagAtSourceOk,
	}
}

// updateLoop is the single model writer.
func updateLoop(ctx context.Context, g *Gateway) {
	for ctx.Err() == nil {
		if !g.Serving() {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		upd, ok := dequeueUpdate()
		if !ok {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		g.srv.Update(func(tx *server.Tx) {
			for n := 0; ; n++ {
				applyUpdate(tx, upd)
				if n+1 >= updateBatchLimit {
					return
				}
				if upd, ok = dequeueUpdate(); !ok {
					return
				}
			}
		})
	}
}

// applyUpdate writes one point into the model: timestamp first, then
// quality, then the value — the value write is the report trigger, so the
// other two are already in place when the report fires.
func applyUpdate(tx *server.Tx, upd PointUpdate) {
	mp := upd.Point
	if mp == nil {
		return
	}

	ts := upd.SourceTime
	tq := mms.TimeAccuracy(10)
	if !upd.HasSourceTime {
		ts = time.Now().UTC()
		tq |= mms.TimeClockNotSynchronized
	} else if !upd.SourceTimeOk {
		tq |= mms.TimeClockNotSynchronized
	}
	tx.Set(mp.TRef, mp.FC, mms.NewUTCTime(ts, tq))
	tx.Set(mp.QRef, mp.FC, mapQuality(upd).Value())

	switch mp.Kind {
	case KindSPS, KindSPC:
		tx.Set(mp.ValueRef, mp.FC, mms.NewBool(upd.Value != 0))
	case KindMV, KindAPC:
		tx.Set(mp.ValueRef, mp.FC, mms.NewFloat32(float32(upd.Value)))
	case KindINS, KindINC:
		tx.Set(mp.ValueRef, mp.FC, mms.NewInt32(int32(upd.Value)))
	case KindVSS:
		tx.Set(mp.ValueRef, mp.FC, mms.NewVisibleString(upd.ValueString))
	}
}

// mapQuality maps the json-scada quality flags onto an IEC 61850-7-3
// Quality, exactly as the C# driver does.
func mapQuality(upd PointUpdate) model.Quality {
	q := model.QualityGood
	switch {
	case upd.Invalid:
		q = q.WithValidity(model.ValidityInvalid)
	case upd.Overflow || upd.Transient:
		q = q.WithValidity(model.ValidityQuestionable)
		if upd.Overflow {
			q |= model.QualityOverflow
		}
		if upd.Transient {
			q |= model.QualityOscillatory
		}
	default:
		q = q.WithValidity(model.ValidityGood)
	}
	if upd.Substituted {
		q |= model.QualitySubstituted
	}
	if upd.Test {
		q |= model.QualityTest
	}
	return q
}

// applyInitialValues pushes the startup snapshot of the database into the
// model, so a client that connects before the first change stream event
// still reads current data.
func applyInitialValues(g *Gateway, points []*Point) {
	n := 0
	g.srv.Update(func(tx *server.Tx) {
		for _, p := range points {
			mp := g.built.ByTag[p.Tag]
			if mp == nil || mp.IsCommand {
				continue
			}
			applyUpdate(tx, updateFromPoint(mp, p))
			n++
		}
	})
	jslog.Log(jslog.LevelBasic, "Initial values loaded into the model (%d points).", n)
}
