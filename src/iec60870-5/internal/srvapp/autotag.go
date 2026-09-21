/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - auto destinations
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Port of TagsDestinations.cs: pushes a protocolDestinations entry onto
 * every existing realtimeData tag not yet mapped to the connection,
 * allocating sequential IOAs within per-category ranges.
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

package srvapp

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// IOA partition — one shared address space per CA, split into
// non-overlapping ranges (same constants as TagsDestinations.cs).
const (
	ioaBaseDigital = 1
	ioaTopDigital  = 20000 // ASDU  1 (M_SP_NA_1)
	ioaBaseAnalog  = 20001
	ioaTopAnalog   = 40000 // ASDU 13 (M_ME_NC_1)
	ioaBaseDigCmd  = 40001
	ioaTopDigCmd   = 50000 // ASDU 45 (C_SC_NA_1)
	ioaBaseAnaCmd  = 50001
	ioaTopAnaCmd   = 60000 // ASDU 50 (C_SE_NC_1)
)

// DistributeAutoTags runs once at startup per connection with
// autoCreateTags=true.
func (e *Engine) DistributeAutoTags() {
	colRt := e.DB().Collection(jsmongo.RealtimeDataCollectionName)
	ctx := context.Background()

	for _, srv := range e.Conns {
		if !srv.Cfg.AutoCreateTags {
			continue
		}
		if srv.Cfg.SizeOfIOA < 2 {
			jslog.Log(jslog.LevelBasic, "%s", srv.Cfg.Name+" - autoCreateTags not supported with sizeOfIOA=1, skipping.")
			continue
		}
		jslog.Log(jslog.LevelBasic, "%s", srv.Cfg.Name+" - autoCreateTags: distributing protocol destinations...")

		// Detect highest IOA already in use for this connection, bucketed by range.
		lastDigital := ioaBaseDigital - 1
		lastAnalog := ioaBaseAnalog - 1
		lastDigCmd := ioaBaseDigCmd - 1
		lastAnaCmd := ioaBaseAnaCmd - 1

		cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		cur, err := colRt.Find(cctx,
			bson.M{"protocolDestinations": bson.M{"$elemMatch": bson.M{
				"protocolDestinationConnectionNumber": srv.Cfg.ProtocolConnectionNumber}}},
			options.Find().SetProjection(bson.M{"protocolDestinations": 1}))
		if err != nil {
			cancel()
			jslog.Log(jslog.LevelBasic, "%s", srv.Cfg.Name+" - autoCreateTags: query error: "+err.Error())
			continue
		}
		for cur.Next(cctx) {
			var doc struct {
				ProtocolDestinations []bson.M `bson:"protocolDestinations"`
			}
			if cur.Decode(&doc) != nil {
				continue
			}
			for _, d := range doc.ProtocolDestinations {
				if int(jsmongo.GetDouble(d, "protocolDestinationConnectionNumber", 0)) !=
					int(srv.Cfg.ProtocolConnectionNumber) {
					continue
				}
				ioa := int(jsmongo.GetU32(d, "protocolDestinationObjectAddress", 0))
				if ioa >= ioaBaseDigital && ioa <= ioaTopDigital && ioa > lastDigital {
					lastDigital = ioa
				}
				if ioa >= ioaBaseAnalog && ioa <= ioaTopAnalog && ioa > lastAnalog {
					lastAnalog = ioa
				}
				if ioa >= ioaBaseDigCmd && ioa <= ioaTopDigCmd && ioa > lastDigCmd {
					lastDigCmd = ioa
				}
				if ioa >= ioaBaseAnaCmd && ioa <= ioaTopAnaCmd && ioa > lastAnaCmd {
					lastAnaCmd = ioa
				}
			}
		}
		cur.Close(cctx)
		cancel()

		// Process four categories: commands first (when enabled), then the
		// monitored points. Monitored covers every non-command origin
		// (supervised, calculated, manual, tags without origin), so that with
		// an empty topics filter the complete database is distributed.
		categories := []struct {
			label     string
			pointType string
			origin    bson.M
			isCommand bool
			asdu      int
			startIoa  int
			topIoa    int
		}{
			{"digital command", "digital", bson.M{"origin": "command"}, true, 45, lastDigCmd, ioaTopDigCmd},
			{"analog command", "analog", bson.M{"origin": "command"}, true, 50, lastAnaCmd, ioaTopAnaCmd},
			{"digital monitored", "digital", bson.M{"origin": bson.M{"$ne": "command"}}, false, 1, lastDigital, ioaTopDigital},
			{"analog monitored", "analog", bson.M{"origin": bson.M{"$ne": "command"}}, false, 13, lastAnalog, ioaTopAnalog},
		}

		// the server's link-level address is used as the common address
		// of the created destinations
		autoCA := srv.Cfg.LocalLinkAddress
		if autoCA == 0 {
			autoCA = 1 // default when localLinkAddress is missing
		}

		for _, cat := range categories {
			if cat.isCommand && !srv.Cfg.CommandsEnabledVal() {
				continue
			}
			nextIoa := cat.startIoa
			filter := bson.M{"$and": bson.A{
				bson.M{"type": cat.pointType},
				cat.origin,
				bson.M{"protocolDestinations": bson.M{"$not": bson.M{"$elemMatch": bson.M{
					"protocolDestinationConnectionNumber": srv.Cfg.ProtocolConnectionNumber}}}},
			}}
			cctx, cancel := context.WithTimeout(ctx, 300*time.Second)
			cur, err := colRt.Find(cctx, filter,
				options.Find().SetSort(bson.M{"_id": 1}).
					SetProjection(bson.M{"_id": 1, "tag": 1, "group1": 1}))
			if err != nil {
				cancel()
				jslog.Log(jslog.LevelBasic, "%s", srv.Cfg.Name+" - autoCreateTags: query error: "+err.Error())
				continue
			}
			for cur.Next(cctx) {
				var tagDoc bson.M
				if cur.Decode(&tagDoc) != nil {
					continue
				}
				// Optional topics filter: skip tags whose group1 does not
				// contain any topic substring
				if len(srv.Cfg.Topics) > 0 {
					g1, _ := tagDoc["group1"].(string)
					matched := false
					for _, topic := range srv.Cfg.Topics {
						if strings.Contains(g1, topic) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}

				nextIoa++
				if nextIoa > cat.topIoa {
					jslog.Log(jslog.LevelBasic, "%s", srv.Cfg.Name+
						" - autoCreateTags: IOA range exhausted for "+cat.label+", stopping.")
					break
				}

				tagID := tagDoc["_id"]
				tagStr, _ := tagDoc["tag"].(string)

				// Ensure the protocolDestinations array field exists before pushing
				uctx, ucancel := context.WithTimeout(ctx, 10*time.Second)
				_, _ = colRt.UpdateOne(uctx,
					bson.M{"$and": bson.A{
						bson.M{"_id": tagID},
						bson.M{"protocolDestinations": bson.M{"$exists": false}},
					}},
					bson.M{"$set": bson.M{"protocolDestinations": bson.A{}}})
				ucancel()

				destDoc := bson.D{
					{Key: "protocolDestinationConnectionNumber", Value: srv.Cfg.ProtocolConnectionNumber},
					{Key: "protocolDestinationCommonAddress", Value: autoCA},
					{Key: "protocolDestinationObjectAddress", Value: float64(nextIoa)},
					{Key: "protocolDestinationASDU", Value: float64(cat.asdu)},
					{Key: "protocolDestinationCommandDuration", Value: 0.0},
					{Key: "protocolDestinationCommandUseSBO", Value: false},
					{Key: "protocolDestinationKConv1", Value: 1.0},
					{Key: "protocolDestinationKConv2", Value: 0.0},
					{Key: "protocolDestinationGroup", Value: 0.0},
					{Key: "protocolDestinationHoursShift", Value: 0.0},
				}
				uctx, ucancel = context.WithTimeout(ctx, 10*time.Second)
				_, err := colRt.UpdateOne(uctx, bson.M{"_id": tagID},
					bson.M{"$push": bson.M{"protocolDestinations": destDoc}})
				ucancel()
				if err != nil {
					jslog.Log(jslog.LevelBasic, "%s", srv.Cfg.Name+" - autoCreateTags: update error: "+err.Error())
					continue
				}
				jslog.Log(jslog.LevelBasic, "%s - autoCreateTags: Creating destination for tag: %v %s IOA: %d",
					srv.Cfg.Name, tagID, tagStr, nextIoa)
			}
			cur.Close(cctx)
			cancel()
		}

		// Report tags left out because their type has no IEC 60870-5
		// monitor/control representation (only digital and analog are mapped).
		cctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		nLeft, err := colRt.CountDocuments(cctx, bson.M{"$and": bson.A{
			bson.M{"type": bson.M{"$nin": bson.A{"digital", "analog"}}},
			bson.M{"protocolDestinations": bson.M{"$not": bson.M{"$elemMatch": bson.M{
				"protocolDestinationConnectionNumber": srv.Cfg.ProtocolConnectionNumber}}}},
		}})
		cancel()
		if err == nil && nLeft > 0 {
			jslog.Log(jslog.LevelBasic, "%s - autoCreateTags: %d tags not distributed (type not supported by IEC 60870-5).",
				srv.Cfg.Name, nLeft)
		}
		jslog.Log(jslog.LevelBasic, "%s", srv.Cfg.Name+" - autoCreateTags: Distribution complete.")
	}
}
