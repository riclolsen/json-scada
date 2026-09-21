/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - client auto tags
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Port of TagsCreation.cs: creates realtimeData documents on first value
 * received for an unknown {CA, IOA} pair, allocating _id keys within the
 * connection's dedicated partition.
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

package cliapp

import (
	"context"
	"math"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"iec60870-5/internal/conv"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// AutoKeyMultiplier partitions the _id space per connection
// (same value as the C# drivers).
const AutoKeyMultiplier = 1000000

// GetNextAutoKey allocates the next _id in the connection's dedicated
// partition [connNumber*1e6, (connNumber+1)*1e6). First call queries MongoDB
// for the current max; subsequent calls just increment the cached value.
func GetNextAutoKey(srv *Conn, colRt *mongo.Collection) float64 {
	if srv.lastNewKey == 0 {
		baseKey := srv.Cfg.ProtocolConnectionNumber * AutoKeyMultiplier
		topKey := (srv.Cfg.ProtocolConnectionNumber + 1) * AutoKeyMultiplier
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var res bson.M
		err := colRt.FindOne(ctx,
			bson.M{"_id": bson.M{"$gt": baseKey, "$lt": topKey}},
			options.FindOne().SetSort(bson.M{"_id": -1}),
		).Decode(&res)
		if err == nil {
			srv.lastNewKey = jsmongo.GetDouble(res, "_id", 0) + 1
		} else {
			srv.lastNewKey = baseKey
		}
	} else {
		srv.lastNewKey++
	}
	return srv.lastNewKey
}

// NewRealtimeTagDoc builds a complete realtimeData document for a new
// auto-created tag (field-for-field port of TagsCreation.cs).
// IEC clients never create command twins — monitor direction has no
// output-status TypeIDs.
func NewRealtimeTagDoc(iv conv.IecValue, connName string, id float64) bson.D {
	baseType := conv.MapAsduToBaseType(iv.Asdu)
	desc := conv.TypeDescription(baseType)
	tag := connName + ";" + strconv.Itoa(iv.CommonAddress) + ";" + strconv.Itoa(iv.Address)
	isDigital := iv.IsDigital
	strIf := func(cond bool, t, f string) string {
		if cond {
			return t
		}
		return f
	}
	alarmState := -1.0
	if isDigital {
		alarmState = 2.0
	}
	return bson.D{
		{Key: "_id", Value: id},
		{Key: "tag", Value: tag},
		{Key: "type", Value: strIf(isDigital, "digital", "analog")},
		{Key: "origin", Value: "supervised"},
		{Key: "description", Value: connName + "~" + desc + "~" + strconv.Itoa(iv.Address)},
		{Key: "ungroupedDescription", Value: desc + " " + strconv.Itoa(iv.Address)},
		{Key: "group1", Value: connName},
		{Key: "group2", Value: "CA " + strconv.Itoa(iv.CommonAddress)},
		{Key: "group3", Value: ""},
		{Key: "protocolSourceConnectionNumber", Value: float64(iv.ConnNumber)},
		{Key: "protocolSourceCommonAddress", Value: float64(iv.CommonAddress)},
		{Key: "protocolSourceObjectAddress", Value: float64(iv.Address)},
		{Key: "protocolSourceASDU", Value: float64(baseType)},
		{Key: "protocolSourceCommandDuration", Value: 0.0},
		{Key: "protocolSourceCommandUseSBO", Value: false},
		{Key: "protocolSourcePublishingInterval", Value: 0.0},
		{Key: "protocolSourceSamplingInterval", Value: 0.0},
		{Key: "protocolSourceQueueSize", Value: 0.0},
		{Key: "protocolSourceDiscardOldest", Value: false},
		{Key: "protocolSourceAccessLevel", Value: 0.0},
		{Key: "commandOfSupervised", Value: 0.0},
		{Key: "supervisedOfCommand", Value: 0.0},
		{Key: "kconv1", Value: 1.0},
		{Key: "kconv2", Value: 0.0},
		{Key: "alarmState", Value: alarmState},
		{Key: "stateTextFalse", Value: strIf(isDigital, "FALSE", "")},
		{Key: "stateTextTrue", Value: strIf(isDigital, "TRUE", "")},
		{Key: "eventTextFalse", Value: strIf(isDigital, "FALSE", "")},
		{Key: "eventTextTrue", Value: strIf(isDigital, "TRUE", "")},
		{Key: "value", Value: iv.Value},
		{Key: "valueString", Value: formatValue(iv.Value)},
		{Key: "valueJson", Value: nil},
		{Key: "invalid", Value: true},
		{Key: "invalidDetectTimeout", Value: 60000.0},
		{Key: "isEvent", Value: false},
		{Key: "alarmDisabled", Value: false},
		{Key: "alarmRange", Value: 0.0},
		{Key: "alarmed", Value: false},
		{Key: "alerted", Value: false},
		{Key: "alertState", Value: ""},
		{Key: "annotation", Value: ""},
		{Key: "commandBlocked", Value: false},
		{Key: "commissioningRemarks", Value: ""},
		{Key: "formula", Value: 0.0},
		{Key: "frozen", Value: false},
		{Key: "frozenDetectTimeout", Value: 0.0},
		{Key: "hiLimit", Value: math.MaxFloat64},
		{Key: "hihiLimit", Value: math.MaxFloat64},
		{Key: "hihihiLimit", Value: math.MaxFloat64},
		{Key: "loLimit", Value: -math.MaxFloat64},
		{Key: "loloLimit", Value: -math.MaxFloat64},
		{Key: "lololoLimit", Value: -math.MaxFloat64},
		{Key: "historianDeadBand", Value: 0.0},
		{Key: "historianPeriod", Value: 0.0},
		{Key: "historianLastValue", Value: nil},
		{Key: "hysteresis", Value: 0.0},
		{Key: "location", Value: nil},
		{Key: "notes", Value: ""},
		{Key: "overflow", Value: false},
		{Key: "parcels", Value: nil},
		{Key: "priority", Value: 0.0},
		{Key: "protocolDestinations", Value: bson.A{}},
		{Key: "sourceDataUpdate", Value: nil},
		{Key: "substituted", Value: false},
		{Key: "timeTag", Value: nil},
		{Key: "timeTagAlarm", Value: nil},
		{Key: "timeTagAlertState", Value: nil},
		{Key: "timeTagAtSource", Value: nil},
		{Key: "timeTagAtSourceOk", Value: false},
		{Key: "transient", Value: false},
		{Key: "unit", Value: ""},
		{Key: "updatesCnt", Value: 0.0},
		{Key: "valueDefault", Value: 0.0},
		{Key: "zeroDeadband", Value: 0.0},
	}
}
