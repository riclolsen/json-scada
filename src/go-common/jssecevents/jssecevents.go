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

// Package jssecevents writes cyber-security events into the secEvents
// collection: the document shape, a non-blocking batched writer and the
// small detectors (change trackers, rate limits, MAC lookup) the drivers
// share. It does not depend on any protocol library: a driver maps its
// library's events onto Event. Contract in docs/schema.md (secEvents).
package jssecevents

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CollectionName is the security events collection.
const CollectionName = "secEvents"

// Severities, in increasing order (RFC 5424 order, as go-iec61850's
// secevent). SeverityLevel stores the index, for filtering and sorting.
var Severities = []string{"info", "notice", "warning", "alert", "critical"}

// SeverityLevel is the index of a severity name, 0 when unknown.
func SeverityLevel(name string) int {
	for i, s := range Severities {
		if s == name {
			return i
		}
	}
	return 0
}

// Event is one secEvents document. Empty fields are left out.
type Event struct {
	// ID is assigned by the writer before the first insert attempt, so a
	// retried batch that was partly inserted stores each event once.
	ID            bson.ObjectID `bson:"_id,omitempty"`
	TimeTag       time.Time     `bson:"timeTag"`
	Kind          string        `bson:"kind"`               // stable identifier, e.g. "conn.unexpected"
	Category      string        `bson:"category,omitempty"` // connection, tls, peer, access, control, ...
	Severity      string        `bson:"severity"`           // info, notice, warning, alert, critical
	SeverityLevel int           `bson:"severityLevel"`      // 0 (info) .. 4 (critical)
	Action        string        `bson:"action,omitempty"`   // observed, rejected, disconnected, ...
	Source        string        `bson:"source"`             // "library" (detected by the protocol library) or "driver"
	Protocol      string        `bson:"protocol,omitempty"`
	Role          string        `bson:"role,omitempty"` // client or server

	ProtocolDriver               string `bson:"protocolDriver"`
	ProtocolDriverInstanceNumber int    `bson:"protocolDriverInstanceNumber"`
	ProtocolConnectionNumber     int    `bson:"protocolConnectionNumber"`
	ConnectionName               string `bson:"connectionName,omitempty"`
	NodeName                     string `bson:"nodeName,omitempty"`

	SessionID    int64  `bson:"sessionId,omitempty"`
	PeerAddress  string `bson:"peerAddress,omitempty"` // ip:port
	PeerIP       string `bson:"peerIp,omitempty"`
	PeerMAC      string `bson:"peerMac,omitempty"`
	LocalAddress string `bson:"localAddress,omitempty"`
	// Peer holds what is known about the peer's identity: TLS certificate,
	// ACSE AP title, MAC source, and so on.
	Peer bson.M `bson:"peer,omitempty"`

	ObjectRef string `bson:"objectRef,omitempty"`
	FC        string `bson:"fc,omitempty"`
	Tag       string `bson:"tag,omitempty"` // json-scada tag, when one is concerned
	Value     string `bson:"value,omitempty"`
	Previous  string `bson:"previous,omitempty"` // the value before a change (identity, MAC, ...)
	Detail    string `bson:"detail,omitempty"`
	Error     string `bson:"error,omitempty"`
	// Protocol fields that only some events carry (AddCause, ctlNum, ...).
	Fields bson.M `bson:"fields,omitempty"`

	Count     int        `bson:"count"` // repeats folded into this event
	FirstSeen *time.Time `bson:"firstSeen,omitempty"`
	Technique string     `bson:"technique,omitempty"` // MITRE ATT&CK for ICS hint
	Std62351  string     `bson:"std62351,omitempty"`  // IEC 62351-14 event identifier
}

// Origin stamps the events of one driver connection.
type Origin struct {
	ProtocolDriver               string
	ProtocolDriverInstanceNumber int
	ProtocolConnectionNumber     int
	ConnectionName               string
	NodeName                     string
}

// Writer queues events and inserts them in batches on its own goroutine.
// Emit never blocks; a full queue drops the event and counts it.
type Writer struct {
	ch      chan Event
	dropped atomic.Uint64
	once    sync.Once
	done    chan struct{}
	stopped atomic.Bool
	macs    MACCache
	// pending are the events taken from the queue and not yet stored; they
	// survive a failed insert and a reconnection. Only the goroutine running
	// RunWith touches it.
	pending []any
}

// Writer limits.
const (
	DefaultQueueSize = 4096
	batchSize        = 200
	flushEvery       = time.Second
	// maxPending bounds the events kept while MongoDB is unreachable.
	maxPending = 4 * DefaultQueueSize
)

// onlyDuplicates reports an insert whose only failures are duplicate keys:
// events of a retried batch that an earlier attempt had already stored.
func onlyDuplicates(err error) bool {
	var bwe mongo.BulkWriteException
	if !errors.As(err, &bwe) || bwe.WriteConcernError != nil || len(bwe.WriteErrors) == 0 {
		return false
	}
	for _, we := range bwe.WriteErrors {
		if we.Code != 11000 {
			return false
		}
	}
	return true
}

// NewWriter returns a writer; call Run to deliver its events.
func NewWriter(queueSize int) *Writer {
	if queueSize <= 0 {
		queueSize = DefaultQueueSize
	}
	return &Writer{ch: make(chan Event, queueSize), done: make(chan struct{})}
}

// Emit queues e, completing the fields it leaves unset. A nil writer
// discards it.
func (w *Writer) Emit(o Origin, e Event) {
	if w == nil || w.stopped.Load() {
		return
	}
	if e.TimeTag.IsZero() {
		e.TimeTag = time.Now()
	}
	if e.Severity == "" {
		e.Severity = "notice"
	}
	e.SeverityLevel = SeverityLevel(e.Severity)
	if e.Source == "" {
		e.Source = "driver"
	}
	if e.Count == 0 {
		e.Count = 1
	}
	if e.Action == "" {
		e.Action = "observed"
	}
	if e.ProtocolDriver == "" {
		e.ProtocolDriver = o.ProtocolDriver
		e.ProtocolDriverInstanceNumber = o.ProtocolDriverInstanceNumber
		e.ProtocolConnectionNumber = o.ProtocolConnectionNumber
		e.ConnectionName = o.ConnectionName
	}
	if e.NodeName == "" {
		e.NodeName = o.NodeName
	}
	jslog.Log(jslog.LevelDetailed, "Security event %s [%s] %s %s %s", e.Kind, e.Severity, e.PeerAddress, e.ObjectRef, e.Detail)
	select {
	case w.ch <- e:
	default:
		w.dropped.Add(1)
	}
}

// Dropped is the number of events lost to a full queue.
func (w *Writer) Dropped() uint64 { return w.dropped.Load() }

// Run connects to MongoDB and inserts the queued events until ctx ends,
// reconnecting as needed. Events queued while MongoDB is unreachable wait
// in the queue (and are dropped beyond its size).
func (w *Writer) Run(ctx context.Context, cfg jsconfig.Config) {
	defer w.once.Do(func() { close(w.done) })
	for ctx.Err() == nil {
		cli, db, err := jsmongo.ConnectAndPing(cfg)
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "secEvents - MongoDB: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		w.RunWith(ctx, db.Collection(CollectionName))
		_ = cli.Disconnect(context.Background())
	}
}

// RunWith inserts the queued events into coll until ctx ends or an insert
// fails. The events not stored stay pending for the next call: the writer
// loses none to a MongoDB outage, only to a full queue (counted and
// reported as meta.events_dropped).
func (w *Writer) RunWith(ctx context.Context, coll *mongo.Collection) {
	add := func(e Event) {
		if e.ID.IsZero() {
			e.ID = bson.NewObjectID()
		}
		if len(w.pending) >= maxPending { // bounded: the oldest is lost, and counted
			w.pending = w.pending[1:]
			w.dropped.Add(1)
		}
		w.pending = append(w.pending, e)
	}
	flush := func() bool {
		if d := w.dropped.Swap(0); d > 0 {
			add(Event{TimeTag: time.Now(), Kind: "meta.events_dropped", Category: "meta",
				Severity: "warning", SeverityLevel: SeverityLevel("warning"), Action: "observed", Source: "driver",
				Count: int(d), Detail: "security events lost: queue full"})
		}
		for len(w.pending) > 0 {
			n := min(len(w.pending), batchSize)
			ictx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := coll.InsertMany(ictx, w.pending[:n], options.InsertMany().SetOrdered(false))
			cancel()
			if err != nil && !onlyDuplicates(err) {
				jslog.Log(jslog.LevelDetailed, "secEvents - insert: %v", err)
				return false
			}
			w.pending = append(w.pending[:0], w.pending[n:]...)
		}
		return true
	}
	tick := time.NewTicker(flushEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			// deliver what is queued, within a short budget
			for drained := false; !drained; {
				select {
				case e := <-w.ch:
					add(w.enrich(e))
				default:
					drained = true
				}
			}
			flush()
			return
		case e := <-w.ch:
			add(w.enrich(e))
			if len(w.pending) >= batchSize && !flush() {
				return
			}
		case <-tick.C:
			if !flush() {
				return
			}
		}
	}
}

// Done is closed once Run has returned and delivered what it could.
func (w *Writer) Done() <-chan struct{} { return w.done }

// enrich adds the peer's MAC address from the OS neighbour table, off the
// protocol goroutines (the lookup may run a command).
func (w *Writer) enrich(e Event) Event {
	if e.PeerMAC == "" && e.PeerIP != "" {
		e.PeerMAC = w.macs.Lookup(SplitAddr(e.PeerIP))
	}
	return e
}

// C is the queue Run delivers from, for tests and for a driver that
// delivers events itself instead of calling Run.
func (w *Writer) C() <-chan Event { return w.ch }
