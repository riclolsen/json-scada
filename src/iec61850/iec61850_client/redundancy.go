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

// Redundancy control. Port of Redundancy.cs: exactly one node of an
// instance acquires data at a time; the standby takes over when the active
// node stops refreshing its keep-alive.

package main

import (
	"context"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"
	"github.com/riclolsen/json-scada/src/go-common/jsstats"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// redundancy is the arbitrator.
//
// parity: this driver gates BOTH axes. Command execution consults Active(),
// and connectionLoop polls it to keep protocol sessions stopped on standby —
// which is why no OnActivate/OnDeactivate is supplied: the sessions poll the
// flag themselves rather than being driven by callbacks, exactly as before.
var redundancy = &jsredundancy.Controller{}

// initRedundancy configures the arbitrator. Called from main before any
// goroutine that consults it starts.
func initRedundancy(ctx context.Context, cfg jsconfig.Config, conns []*Iec61850Connection) {
	redundancy.Config = cfg
	redundancy.DriverName = ProtocolDriverName
	redundancy.InstanceNumber = instanceNumber
	redundancy.OnTick = func(db *mongo.Database) {
		updateConnectionStats(ctx,
			db.Collection(jsmongo.ProtocolConnectionsCollectionName), cfg, conns)
	}
}

// updateConnectionStats publishes the last seen buffered-report EntryIDs
// and a heartbeat on each connection document. The EntryIDs are read back
// at startup so buffered reports resume where they stopped.
func updateConnectionStats(ctx context.Context, collConns *mongo.Collection, cfg jsconfig.Config, conns []*Iec61850Connection) {
	var entries []jsstats.Entry
	for _, conn := range conns {
		if conn.Client() == nil {
			continue
		}
		ids := bson.M{}
		for k, v := range conn.SnapshotReportIDs() {
			ids[k] = bson.Binary{Subtype: 0, Data: v}
		}
		entries = append(entries, jsstats.Entry{
			ConnectionNumber: conn.ProtocolConnectionNumber,
			Extra:            bson.M{"lastReportIds": ids},
		})
	}
	jsstats.Writer{
		NodeName: cfg.NodeName,
		OnError: func(_ jsstats.Entry, err error) {
			jslog.Log(jslog.LevelDetailed, "Redundancy - stats update: %v", err)
		},
	}.Write(ctx, collConns, entries)
}
