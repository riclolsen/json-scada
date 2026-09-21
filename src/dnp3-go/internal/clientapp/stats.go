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

// The stats sub-document of protocolConnections. Port of the statistics block
// of processRedundancy(), reading opendnp3's ChannelStatistics.

package clientapp

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
//
// The byte and open/close counters come from the channel wrapper; the frame and
// CRC counters come from the multi-drop bus, which is the only thing that
// decodes the stream. On a shared bus those four are per bus, so every
// connection on it reports the same numbers (deviation D4).
func (e *Engine) writeStats(ctx context.Context, db *mongo.Database) {
	coll := db.Collection(jsmongo.ProtocolConnectionsCollectionName)
	var entries []jsstats.Entry

	for _, conn := range e.conns {
		if conn.Group == nil {
			continue
		}
		counters := conn.Group.Counters.Snapshot()
		busStats := conn.Group.Stats()

		// numLinkFrameTx has no source: nothing counts frames on the way out.
		// The master's task count is the closest honest proxy — requests
		// issued — and the README says so rather than implying a frame count.
		var tasksRun uint64
		if session := conn.Session(); session != nil {
			tasksRun = session.Stats().TasksRun
		}

		stats := bson.M{
			"isConnected":       conn.Connected(),
			"numBytesRx":        int64(counters.BytesRx),
			"numBytesTx":        int64(counters.BytesTx),
			"numOpen":           int64(counters.Opens),
			"numClose":          int64(counters.Closes),
			"numOpenFail":       int64(counters.OpenFails),
			"numLinkFrameRx":    int64(busStats.FramesDecoded),
			"numLinkFrameTx":    int64(tasksRun),
			"numHeaderCrcError": int64(busStats.HeaderCRCErrors),
			"numBodyCrcError":   int64(busStats.BodyCRCErrors),
		}

		entries = append(entries, jsstats.Entry{
			ConnectionNumber: conn.ProtocolConnectionNumber,
			Stats:            stats,
			Label:            conn.Name,
		})

		// A frame arriving for nobody on the bus is normal on a line shared
		// with other equipment, but a steady climb with nothing reaching a
		// session means a link address is wrong.
		if busStats.FramesUnrouted > 0 || busStats.FramesDropped > 0 {
			jslog.Log(jslog.LevelDetailed,
				"%s - Bus frames: routed=%d unrouted=%d dropped=%d",
				conn.Name, busStats.FramesRouted, busStats.FramesUnrouted, busStats.FramesDropped)
		}
	}

	jsstats.Writer{
		NodeName: e.cfg.NodeName,
		Timeout:  10 * time.Second,
		OnError: func(en jsstats.Entry, err error) {
			jslog.Log(jslog.LevelDetailed, "%s - Failed to write statistics: %v", en.Label, err)
		},
	}.Write(ctx, coll, entries)
}
