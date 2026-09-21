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

// Package jsstats writes the per-connection statistics into
// protocolConnections.
//
// Scope: the drivers publish very different counters — the DNP3 pair writes
// twelve, the OPC-UA and IEC 61850 clients write only the heartbeat the C#
// drivers write. What they share is the update shape and the nodeName and
// timeTag stamping, so that is what lives here; the counters stay with the
// driver that knows how to produce them.
package jsstats

import (
	"context"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Entry is one connection's statistics update.
type Entry struct {
	// ConnectionNumber matches protocolConnectionNumber. Kept as any so a
	// driver can pass the float64 it decoded or a plain int without a cast
	// that could change how the document matches.
	ConnectionNumber any
	// Stats are the driver's own counters. nodeName and timeTag are added.
	Stats bson.M
	// Extra are additional top-level $set fields, merged alongside "stats".
	// The IEC 61850 client uses this for lastReportIds.
	Extra bson.M
	// Label names the connection in an error message. The DNP3 drivers
	// report failures per connection name.
	Label string
}

// Writer publishes statistics into protocolConnections.
type Writer struct {
	NodeName string
	// Timeout bounds each update. Zero means no bound beyond the context,
	// which is what the OPC-UA and IEC 61850 drivers relied on; the DNP3
	// pair sets 10 s.
	Timeout time.Duration
	// OnError reports a failed update. Each driver logs this with its own
	// wording and level, so the handler is supplied rather than assumed;
	// leaving it nil logs at the detailed level.
	OnError func(e Entry, err error)
}

// Write publishes the statistics of every entry.
//
// parity: a failed update is reported and the loop continues, which is what
// every driver did. One unreachable connection document must not stop the
// heartbeat of the others.
func (w Writer) Write(ctx context.Context, coll *mongo.Collection, entries []Entry) {
	now := jsmongo.Now()
	for _, e := range entries {
		stats := bson.M{"nodeName": w.NodeName, "timeTag": now}
		for k, v := range e.Stats {
			stats[k] = v
		}
		set := bson.M{"stats": stats}
		for k, v := range e.Extra {
			set[k] = v
		}

		uctx := ctx
		cancel := context.CancelFunc(func() {})
		if w.Timeout > 0 {
			uctx, cancel = context.WithTimeout(ctx, w.Timeout)
		}
		_, err := coll.UpdateOne(uctx,
			bson.M{"protocolConnectionNumber": e.ConnectionNumber},
			bson.M{"$set": set})
		cancel()
		if err != nil {
			if w.OnError != nil {
				w.OnError(e, err)
			} else {
				jslog.Log(jslog.LevelDetailed, "Stats update: %v", err)
			}
		}
	}
}
