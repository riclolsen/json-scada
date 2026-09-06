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

// The realtimeData change stream: the same event source the OPC server
// drivers use. Updates that only touch sourceDataUpdate are skipped, since
// cs_data_processor follows them with the processed value this driver
// wants.

package main

import (
	"context"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// changeStreamLoop watches realtimeData and enqueues model updates.
func changeStreamLoop(ctx context.Context, cfg jsconfig.Config, g *Gateway) {
	for ctx.Err() == nil {
		cli, _, err := jsmongo.ConnectAndPing(cfg)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception MongoCS")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		db := cli.Database(cfg.MongoDatabaseName)
		if err := watchRealtimeData(ctx, db, g); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelNoLog, "Exception MongoCS")
			jslog.Log(jslog.LevelNoLog, "%v", err)
			time.Sleep(3 * time.Second)
		}
		_ = cli.Disconnect(context.Background())
	}
}

// csPipeline is the C# BuildCsFilter, expressed as an aggregation pipeline.
func csPipeline(topics []string) mongo.Pipeline {
	and := bson.A{
		bson.M{"updateDescription.updatedFields.sourceDataUpdate": bson.M{"$exists": false}},
		bson.M{"fullDocument._id": bson.M{"$gt": 0}},
		bson.M{"operationType": "update"},
	}
	if len(topics) > 0 {
		and = append(bson.A{bson.M{"fullDocument.group1": bson.M{"$in": topics}}}, and...)
	}
	return mongo.Pipeline{bson.D{{Key: "$match", Value: bson.M{
		"$or": bson.A{
			bson.M{"$and": and},
			bson.M{"operationType": "replace"},
		},
	}}}}
}

func watchRealtimeData(ctx context.Context, db *mongo.Database, g *Gateway) error {
	if err := jsmongo.Ping(db, 1*time.Second); err != nil {
		return err
	}
	coll := db.Collection(jsmongo.RealtimeDataCollectionName)

	cs, err := coll.Watch(ctx, csPipeline(g.conn.Topics),
		options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err != nil {
		return err
	}
	defer cs.Close(context.Background())

	jslog.Log(jslog.LevelNoLog, "MongoDB CS - listening for realtime data updates...")

	for cs.Next(ctx) {
		var ev struct {
			OperationType string `bson:"operationType"`
			FullDocument  bson.M `bson:"fullDocument"`
		}
		if err := cs.Decode(&ev); err != nil {
			jslog.Log(jslog.LevelDetailed, "MongoDB CS - decode: %v", err)
			continue
		}
		if ev.OperationType != "update" && ev.OperationType != "replace" {
			continue
		}
		if ev.FullDocument == nil {
			continue
		}
		p := pointFromDoc(ev.FullDocument)
		if p.Tag == "" {
			continue
		}
		mp := g.built.ByTag[p.Tag]
		if mp == nil {
			// A tag that matches the filter but is not in the model was
			// created after startup: the model is static, so it will only
			// be served after a restart.
			jslog.Log(jslog.LevelDetailed, "MongoDB CS - tag %s is not in the model (added after startup?)", p.Tag)
			continue
		}
		if mp.IsCommand {
			continue // command points are outputs, not pushed
		}
		enqueueUpdate(updateFromPoint(mp, p))
	}
	return cs.Err()
}
