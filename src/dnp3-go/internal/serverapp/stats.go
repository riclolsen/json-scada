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

// writeStats refreshes the statistics of every connection.
func (e *Engine) writeStats(ctx context.Context, db *mongo.Database) {
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
