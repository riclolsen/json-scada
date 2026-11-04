/*
 * DNP3 Outstation Server Protocol driver for {json:scada}, in Go.
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

// The stats sub-document of protocolConnections. The C++ server writes none;
// this is the same shape the client driver writes (deviation D20).

package serverapp

import (
	"context"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsstats"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// writeKeepAlive stamps the instance document with this node and the current
// time, so health checks (the MCP server's get_system_status, the AdminUI) can
// tell the driver is running. The server has no redundancy election, so this
// is informational only: with several nodes running the instance the last
// writer wins, and nothing reads it to decide who is active (deviation D28).
func (e *Engine) writeKeepAlive(ctx context.Context, db *mongo.Database) {
	updCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName).UpdateOne(updCtx,
		bson.M{
			"protocolDriver":               ProtocolDriverName,
			"protocolDriverInstanceNumber": e.instanceNumber,
		},
		bson.M{"$set": bson.M{
			"activeNodeName":             e.cfg.NodeName,
			"activeNodeKeepAliveTimeTag": jsmongo.Now(),
		}})
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "Failed to write the instance keep-alive: %v", err)
	}
}

// writeStats refreshes the instance keep-alive and the statistics of every
// connection.
func (e *Engine) writeStats(ctx context.Context, db *mongo.Database) {
	e.writeKeepAlive(ctx, db)

	coll := db.Collection(jsmongo.ProtocolConnectionsCollectionName)
	var entries []jsstats.Entry

	for _, conn := range e.conns {
		if conn.Group == nil {
			continue
		}
		counters := conn.Group.Counters.Snapshot()
		busStats := conn.Group.Stats()

		// An outstation has no Connected() of its own — it answers whoever
		// polls it — so the state comes from the channel its session runs on.
		// Deriving it from traffic instead would report a station down
		// whenever the master's poll interval exceeded the window, and the
		// JSON-SCADA default integrity interval is 300 seconds.
		connected := conn.link.Up()

		var (
			confirmTimeouts uint64
			eventsQueued    int
		)
		if s := conn.Station(); s != nil {
			confirmTimeouts = s.Stats().ConfirmTimeouts
			eventsQueued = s.Events().Total()
		}

		stats := bson.M{
			"isConnected":       connected,
			"numBytesRx":        int64(counters.BytesRx),
			"numBytesTx":        int64(counters.BytesTx),
			"numOpen":           int64(counters.Opens),
			"numClose":          int64(counters.Closes),
			"numOpenFail":       int64(counters.OpenFails),
			"numLinkFrameRx":    int64(busStats.FramesDecoded),
			"numLinkFrameTx":    int64(busStats.FramesRouted),
			"numHeaderCrcError": int64(busStats.HeaderCRCErrors),
			"numBodyCrcError":   int64(busStats.BodyCRCErrors),
			"confirmTimeouts":   int64(confirmTimeouts),
			"eventsQueued":      int64(eventsQueued),
		}

		entries = append(entries, jsstats.Entry{
			ConnectionNumber: conn.ProtocolConnectionNumber,
			Stats:            stats,
			Label:            conn.Name,
		})
	}

	jsstats.Writer{
		NodeName: e.cfg.NodeName,
		Timeout:  10 * time.Second,
		OnError: func(en jsstats.Entry, err error) {
			jslog.Log(jslog.LevelDetailed, "%s - Failed to write statistics: %v", en.Label, err)
		},
	}.Write(ctx, coll, entries)
}
