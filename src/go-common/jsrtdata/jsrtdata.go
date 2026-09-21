/*
 * Shared {json:scada} driver support library, in Go.
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

// Package jsrtdata holds the acquired-value queue and the sourceDataUpdate
// document that every client driver writes into realtimeData.
//
// Scope note: the drain loops themselves are NOT here. They are tuned
// differently on purpose — OPC-UA writes with an unacknowledged write concern
// and a 750 ms batch window, the IEC 61850 client pings every cycle and trims
// its queue at 400 ms, dnp3-go inserts new tags with a separate InsertMany
// before the bulk update, and iec60870-5 folds command acks into the same
// pass. What they genuinely share is the queue and the document, so that is
// what this package owns.
package jsrtdata

import (
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Queue is the FIFO an acquisition callback pushes into and the MongoDB
// writer drains. Promoted from the byte-identical dataQueue of the OPC-UA and
// IEC 61850 clients.
//
// Every method is safe for concurrent use: the acquisition side must never
// block, so it takes the lock only long enough to append.
type Queue[T any] struct {
	mu    sync.Mutex
	items []T
}

// Enqueue appends one acquired value.
func (q *Queue[T]) Enqueue(v T) {
	q.mu.Lock()
	q.items = append(q.items, v)
	q.mu.Unlock()
}

// Len reports how many values are waiting.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Dequeue removes the oldest value, reporting false when the queue is empty.
func (q *Queue[T]) Dequeue() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	v := q.items[0]
	q.items = q.items[1:]
	return v, true
}

// Trim discards the oldest values while the queue is longer than limit,
// calling onDrop for each. Used when the database is unreachable and the
// queue would otherwise grow without bound.
func (q *Queue[T]) Trim(limit int, onDrop func()) {
	q.mu.Lock()
	for len(q.items) > limit {
		q.items = q.items[1:]
		if onDrop != nil {
			onDrop()
		}
	}
	q.mu.Unlock()
}

// SourceDataUpdate is the sourceDataUpdate sub-document of realtimeData.
//
// The twelve fields below are written by every client driver. The protocol-
// specific ones are not modelled as struct fields, because the four drivers
// disagree about which exist: dnp3-go writes carryAtSource and
// transientAtSource, the IEC 61850 client writes valueBsonAtSource and
// transientAtSource, OPC-UA writes valueBsonAtSource and valueJsonAtSource,
// and three of the four write originator. Adding a field a driver never wrote
// would change its stored documents, so those go in Extra and each driver
// keeps writing exactly what it wrote before.
//
// The time fields are typed any on purpose. The drivers pass different Go
// types that marshal to the same BSON date — bson.DateTime in OPC-UA, dnp3-go
// and the IEC 61850 client, time.Time in iec60870-5 — and a nil TimeTagAtSource
// means "no source timestamp". Keeping them as any reproduces each driver's
// encoding exactly.
type SourceDataUpdate struct {
	ValueAtSource       float64
	ValueStringAtSource string
	AsduAtSource        string
	// CauseOfTransmissionAtSource is stored as a string in every driver.
	CauseOfTransmissionAtSource string
	TimeTagAtSource             any
	TimeTagAtSourceOk           bool
	TimeTag                     any
	NotTopicalAtSource          bool
	InvalidAtSource             bool
	OverflowAtSource            bool
	BlockedAtSource             bool
	SubstitutedAtSource         bool

	// Extra carries the protocol-specific fields, merged over the base.
	Extra bson.M
}

// BSON renders the sourceDataUpdate sub-document.
func (u SourceDataUpdate) BSON() bson.M {
	doc := bson.M{
		"valueAtSource":               u.ValueAtSource,
		"valueStringAtSource":         u.ValueStringAtSource,
		"asduAtSource":                u.AsduAtSource,
		"causeOfTransmissionAtSource": u.CauseOfTransmissionAtSource,
		"timeTagAtSource":             u.TimeTagAtSource,
		"timeTagAtSourceOk":           u.TimeTagAtSourceOk,
		"timeTag":                     u.TimeTag,
		"notTopicalAtSource":          u.NotTopicalAtSource,
		"invalidAtSource":             u.InvalidAtSource,
		"overflowAtSource":            u.OverflowAtSource,
		"blockedAtSource":             u.BlockedAtSource,
		"substitutedAtSource":         u.SubstitutedAtSource,
	}
	for k, v := range u.Extra {
		doc[k] = v
	}
	return doc
}

// SetDoc renders the whole update: {"$set": {"sourceDataUpdate": ...}}.
func (u SourceDataUpdate) SetDoc() bson.M {
	return bson.M{"$set": bson.M{"sourceDataUpdate": u.BSON()}}
}

// SupervisedFilter matches the supervised point of a connection by object
// address.
//
// parity: the origin clause keeps a supervised point from being updated by
// the command tag that shares its object address. The OPC-UA and IEC 61850
// clients both filter this way; dnp3-go matches on the common address instead
// and has no origin clause, so it builds its own filter.
func SupervisedFilter(connNumber, objectAddress any) bson.M {
	return bson.M{
		"protocolSourceConnectionNumber": connNumber,
		"protocolSourceObjectAddress":    objectAddress,
		"origin":                         "supervised",
	}
}

// UpdateOne is the write model for one acquired value.
func UpdateOne(filter bson.M, u SourceDataUpdate) mongo.WriteModel {
	return mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(u.SetDoc())
}

// MaxBSONDocumentSize is the server limit a single update must stay under.
const MaxBSONDocumentSize = 16000000

// oversizePretestBytes is the cheap length test that decides whether the real
// encoded-size check is worth running.
const oversizePretestBytes = 1000000

// Oversize reports whether an update is too large to store, and its encoded
// size when it is.
//
// parity: the test is two-stage on purpose. The rendered lengths are only a
// pre-filter; the decision is the actual BSON encoding, so a merely long
// string still gets through. A value too large to store is dropped rather
// than failing the whole bulk write.
func Oversize(update bson.M, renderedLen int) (bool, int) {
	if renderedLen <= oversizePretestBytes {
		return false, 0
	}
	raw, err := bson.Marshal(update)
	if err != nil {
		return false, 0
	}
	if len(raw) > MaxBSONDocumentSize {
		return true, len(raw)
	}
	return false, len(raw)
}
