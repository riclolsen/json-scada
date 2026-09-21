/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
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

// Redundancy control. Port of Redundancy.cs: the standby node takes over
// when the active node stops refreshing its keep-alive.
//
// parity: in this driver the active flag gates command execution only.
// Acquisition and the MongoDB writer run on both nodes, exactly as in the
// C# driver — see deviation D9 in README.md.

package main

import (
	"context"
	"fmt"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"
	"github.com/riclolsen/json-scada/src/go-common/jsstats"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// redundancy is the arbitrator. Command execution consults it; acquisition
// does not, which is this driver's deviation D9.
var redundancy = &jsredundancy.Controller{}

// initRedundancy configures the arbitrator. Called from main before any
// goroutine that consults it starts.
//
// parity: no OnActivate/OnDeactivate is supplied, because the active flag
// gates command execution only. Acquisition and the MongoDB writer run on
// both the active and the standby node, exactly as in the C# driver — see
// deviation D9 in README.md.
func initRedundancy(ctx context.Context, cfg jsconfig.Config, conns []*OPCUAConnection) {
	redundancy.Config = cfg
	redundancy.DriverName = ProtocolDriverName
	redundancy.InstanceNumber = instanceNumber
	redundancy.OnTick = func(db *mongo.Database) {
		updateConnectionStats(ctx,
			db.Collection(jsmongo.ProtocolConnectionsCollectionName), cfg, conns)
	}
	redundancy.StatusSuffix = func() string {
		return fmt.Sprintf(" - Notification events: %d - Lost updates: %d",
			CntNotificEvents.Load(), CntLostDataUpdates.Load())
	}
}

// updateConnectionStats publishes a heartbeat on each connection document.
//
// parity: the C# driver writes only nodeName and timeTag here, for every
// connection whose client object exists. Do not add counters without
// changing the C# driver too.
func updateConnectionStats(ctx context.Context, collConns *mongo.Collection, cfg jsconfig.Config, conns []*OPCUAConnection) {
	entries := make([]jsstats.Entry, 0, len(conns))
	for _, conn := range conns {
		entries = append(entries, jsstats.Entry{
			ConnectionNumber: conn.ProtocolConnectionNumber,
		})
	}
	jsstats.Writer{
		NodeName: cfg.NodeName,
		OnError: func(_ jsstats.Entry, err error) {
			jslog.Log(jslog.LevelDetailed, "Redundancy - stats update: %v", err)
		},
	}.Write(ctx, collConns, entries)
}
