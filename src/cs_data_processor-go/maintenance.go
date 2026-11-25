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

// Periodic housekeeping, identical to the timers of cs_data_processor.js:
// freezing unchanging analogs, invalidating points that stopped being
// updated, and invalidating whole connections whose driver instance died.

package main

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	frozenDetectCycle  = 17317 * time.Millisecond
	invalidDetectCycle = 43000 * time.Millisecond
	// drivers whose keep-alive is supervised
	keepAliveGraceSeconds = 15
)

// supervisedDrivers is the list of client drivers checked for a stalled
// keep-alive, same list as in the Node.js version.
var supervisedDrivers = bson.A{
	"IEC60870-5-104",
	"IEC60870-5-101",
	"IEC60870-5-103",
	"DNP3",
	"MQTT-SPARKPLUG-B",
	"OPC-UA",
	"OPC-DA",
	"TELEGRAF-LISTENER",
	"PLCTAG",
	"PLC4X",
	"MODBUS",
	"SUNSPEC",
	"IEC61850",
	"ICCP",
}

// StartMaintenance launches the housekeeping goroutines.
func StartMaintenance(ctx context.Context, cfg Config) {
	go loopEvery(ctx, frozenDetectCycle, markFrozenAnalogs)
	go loopEvery(ctx, invalidDetectCycle, func(ctx context.Context) {
		markInvalidPoints(ctx)
		invalidateStoppedDrivers(ctx, cfg)
	})
}

func loopEvery(ctx context.Context, d time.Duration, f func(context.Context)) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !mongoIsConnected() {
				continue
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						Log(LogLevelMin, "Error on maintenance task: %v", r)
					}
				}()
				f(ctx)
			}()
		}
	}
}

// markFrozenAnalogs marks as frozen the unchanged analog values greater
// than 1 after their frozenDetectTimeout.
func markFrozenAnalogs(ctx context.Context) {
	rt, _, _, _, _, ok := Mongo.Handles()
	if !ok {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	filter := bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "type", Value: "analog"}},
		bson.D{{Key: "invalid", Value: false}},
		bson.D{{Key: "frozen", Value: false}},
		bson.D{{Key: "frozenDetectTimeout", Value: bson.D{{Key: "$gt", Value: 0.0}}}},
		bson.D{{Key: "timeTag", Value: bson.D{{Key: "$ne", Value: nil}}}},
		bson.D{{Key: "$expr", Value: bson.D{{Key: "$gt", Value: bson.A{
			bson.D{{Key: "$abs", Value: "$value"}}, 1.0}}}}},
		bson.D{{Key: "$expr", Value: bson.D{{Key: "$lt", Value: bson.A{
			"$timeTag",
			bson.D{{Key: "$subtract", Value: bson.A{
				time.Now(),
				bson.D{{Key: "$multiply", Value: bson.A{"$frozenDetectTimeout", 1000.0}}},
			}}},
		}}}}},
	}}}

	if _, err := rt.UpdateMany(opCtx, filter,
		bson.D{{Key: "$set", Value: bson.D{{Key: "frozen", Value: true}}}}); err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Error on Mongodb query! %v", err)
	}
}

// markInvalidPoints invalidates supervised points not updated within their
// invalidDetectTimeout.
func markInvalidPoints(ctx context.Context) {
	rt, _, _, _, _, ok := Mongo.Handles()
	if !ok {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	filter := bson.D{{Key: "$expr", Value: bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "$eq", Value: bson.A{"$origin", "supervised"}}},
		bson.D{{Key: "$ne", Value: bson.A{"$substituted", true}}},
		bson.D{{Key: "$eq", Value: bson.A{"$invalid", false}}},
		bson.D{{Key: "$lt", Value: bson.A{
			"$sourceDataUpdate.timeTag",
			bson.D{{Key: "$subtract", Value: bson.A{
				time.Now(),
				bson.D{{Key: "$multiply", Value: bson.A{1000.0, "$invalidDetectTimeout"}}},
			}}},
		}}},
	}}}}}

	if _, err := rt.UpdateMany(opCtx, filter,
		bson.D{{Key: "$set", Value: bson.D{{Key: "invalid", Value: true}}}}); err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Error on Mongodb query! %v", err)
	}
}

// invalidateStoppedDrivers looks for client driver instances that stopped
// updating their keep-alive and invalidates every point of their connections.
func invalidateStoppedDrivers(ctx context.Context, cfg Config) {
	rt, _, _, _, db, ok := Mongo.Handles()
	if !ok || db == nil {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	filter := bson.D{{Key: "$expr", Value: bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "$in", Value: bson.A{"$protocolDriver", supervisedDrivers}}},
		bson.D{{Key: "$eq", Value: bson.A{"$enabled", true}}},
		bson.D{{Key: "$lt", Value: bson.A{
			"$activeNodeKeepAliveTimeTag",
			bson.D{{Key: "$subtract", Value: bson.A{
				time.Now(),
				bson.D{{Key: "$multiply", Value: bson.A{1000.0, float64(keepAliveGraceSeconds)}}},
			}}},
		}}},
	}}}}}

	cur, err := db.Collection(ProtocolDriverInstancesCollectionName).Find(opCtx, filter)
	if err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Error on Mongodb query! %v", err)
		return
	}
	var instances []bson.Raw
	if err := cur.All(opCtx, &instances); err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Error on Mongodb query! %v", err)
		return
	}

	for _, raw := range instances {
		inst := rawDoc{raw: raw}
		Log(LogLevelMin, "PROTOCOL INSTANCE NOT RUNNING DETECTED!")
		driverName := inst.str("protocolDriver")
		instNum := inst.lookup("protocolDriverInstanceNumber")
		instNumF, _ := rawValueNum(instNum)
		Log(LogLevelMin, "Driver Name: %s Instance Number: %s", driverName,
			jsNumberToString(instNumF))

		connCur, err := db.Collection(ProtocolConnectionsCollectionName).Find(opCtx, bson.D{
			{Key: "protocolDriver", Value: driverName},
			{Key: "protocolDriverInstanceNumber", Value: rawOrNil(instNum)},
		})
		if err != nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Error on Mongodb query! %v", err)
			continue
		}
		var conns []bson.Raw
		if err := connCur.All(opCtx, &conns); err != nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Error on Mongodb query! %v", err)
			continue
		}
		for _, craw := range conns {
			conn := rawDoc{raw: craw}
			connNum := conn.lookup("protocolConnectionNumber")
			connNumF, _ := rawValueNum(connNum)
			Log(LogLevelMin, "Data invalidated for connection: %s", jsNumberToString(connNumF))
			if _, err := rt.UpdateMany(opCtx, bson.D{
				{Key: "origin", Value: "supervised"},
				{Key: "protocolSourceConnectionNumber", Value: rawOrNil(connNum)},
				{Key: "invalid", Value: false},
			}, bson.D{{Key: "$set", Value: bson.D{{Key: "invalid", Value: true}}}}); err != nil {
				M.Inc(CntErrors, 1)
				Log(LogLevelMin, "Error on Mongodb query! %v", err)
			}
		}
	}
}
