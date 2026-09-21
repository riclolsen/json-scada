/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
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

// Instance keep-alive and connection statistics.
//
// There is no active/standby arbitration here: an MMS server is a passive TCP
// listener, so every node of a redundant pair can listen at the same time and
// let the clients choose which one they connect to. The node is always active
// and simply publishes that it is alive.

package main

import (
	"context"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsstats"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// statsLoop publishes the instance keep-alive and the connection statistics.
func statsLoop(ctx context.Context, cfg jsconfig.Config, g *Gateway) {
	for ctx.Err() == nil {
		cli, _, err := jsmongo.ConnectAndPing(cfg)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		db := cli.Database(cfg.MongoDatabaseName)
		if err := statsCycle(ctx, db, cfg, g); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(3 * time.Second)
		}
		_ = cli.Disconnect(context.Background())
	}
}

func statsCycle(ctx context.Context, db *mongo.Database, cfg jsconfig.Config, g *Gateway) error {
	collInsts := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName)
	collConns := db.Collection(jsmongo.ProtocolConnectionsCollectionName)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := jsmongo.Ping(db, 1*time.Second); err != nil {
			return err
		}

		updCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if _, err := collInsts.UpdateOne(updCtx,
			bson.M{
				"protocolDriver":               ProtocolDriverName,
				"protocolDriverInstanceNumber": instanceNumber,
			},
			bson.M{"$set": bson.M{
				"activeNodeName":             cfg.NodeName,
				"activeNodeKeepAliveTimeTag": bson.NewDateTimeFromTime(time.Now()),
			}}); err != nil {
			jslog.Log(jslog.LevelDetailed, "Stats - %v", err)
		}
		updateConnectionStats(updCtx, collConns, cfg, g)
		cancel()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// updateConnectionStats publishes the server view on the connection
// document, the way the C# driver does.
func updateConnectionStats(ctx context.Context, collConns *mongo.Collection, cfg jsconfig.Config, g *Gateway) {
	if g == nil {
		return
	}
	jsstats.Writer{
		NodeName: cfg.NodeName,
		OnError: func(_ jsstats.Entry, err error) {
			jslog.Log(jslog.LevelDetailed, "Stats - connection stats update: %v", err)
		},
	}.Write(ctx, collConns, []jsstats.Entry{{
		ConnectionNumber: g.conn.ProtocolConnectionNumber,
		Stats: bson.M{
			"isRunning":       g.Serving(),
			"openConnections": g.OpenConnections(),
			"pointsExposed":   len(g.built.ByTag),
		},
	}})
}
