/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Exposes JSON-SCADA realtimeData points (filtered by group1 via the
 * connection topics list) as an IEC 61850 MMS server, mirroring the C#
 * driver of src/iec61850_server without a native library.
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

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmodel"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// instanceNumber is the driver instance this process runs.
var instanceNumber = 1

func main() {
	if len(os.Args) > 1 && os.Args[1] == "selftest" {
		logBanner()
		runSelfTest(os.Args)
		return
	}

	cfg, instNum := readConfigFile()
	instanceNumber = instNum

	cli, _, err := jsmongo.ConnectAndPing(cfg)
	if err != nil {
		jslog.Fatal("Error connecting to MongoDB - %v", err)
	}
	db := cli.Database(cfg.MongoDatabaseName)

	loadInstance(db, cfg)
	conn := loadConnection(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The points to expose, and the model over them. Both are fixed for
	// the life of the process: the model is static, as in the C# driver.
	loadCtx, loadCancel := context.WithTimeout(ctx, 5*time.Minute)
	points := selectPoints(loadCtx, db.Collection(jsmongo.RealtimeDataCollectionName), conn)
	loadCancel()

	built := BuildModel(points, conn)
	exportManifest(built, conn)

	gw, err := NewGateway(conn, built)
	if err != nil {
		jslog.Fatal("Error creating the MMS server - %v", err)
	}
	installControlHandlers(gw)

	go statsLoop(ctx, cfg, gw)
	go changeStreamLoop(ctx, cfg, gw)
	go updateLoop(ctx, gw)
	go commandInserterLoop(ctx, cfg)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	lastOpen := -1

	for {
		select {
		case <-sigs:
			jslog.Log(jslog.LevelNoLog, "Shutdown requested...")
			cancel()
			gw.Stop()
			jslog.Flush()
			os.Exit(0)

		case <-ticker.C:
			// An MMS server is a passive listener: there is no active/standby
			// arbitration, this node listens for as long as it runs. Start is
			// retried until the bind succeeds (the port may still be held by
			// a previous instance shutting down).
			if !gw.Serving() {
				gw.Start()
				if gw.Serving() {
					applyInitialValues(gw, points)
					lastOpen = -1
				}
				continue
			}

			if open := gw.OpenConnections(); open != lastOpen {
				jslog.Log(jslog.LevelNoLog, "Open MMS connections: %d", open)
				lastOpen = open
			}
		}
	}
}

// loadInstance reads the driver instance document and validates it can run
// on this node, with the same checks and messages as the C# driver.
func loadInstance(db *mongo.Database, cfg jsconfig.Config) *jsmodel.DriverInstance {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cur, err := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName).Find(ctx, bson.M{
		"protocolDriver":               ProtocolDriverName,
		"protocolDriverInstanceNumber": instanceNumber,
	})
	if err != nil {
		jslog.Fatal("Error reading driver instances - %v", err)
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		jslog.Fatal("Error reading driver instances - %v", err)
	}
	if len(docs) == 0 {
		jslog.Fatal("Driver instance [%d] not found in configuration!", instanceNumber)
	}

	// parity: the C# driver only ever looks at the first document.
	inst := jsmodel.InstanceFromDoc(docs[0])
	if !inst.Enabled {
		jslog.Fatal("Driver instance [%d] disabled!", instanceNumber)
	}
	if !jsmodel.NodeAllowed(inst, cfg.NodeName) {
		jslog.Fatal("Node '%s' not found in instances configuration!", cfg.NodeName)
	}
	return inst
}

// loadConnection reads the single enabled connection of this instance.
// Server drivers serve one connection per instance; extra ones are ignored
// with a warning, as in the C# driver.
func loadConnection(db *mongo.Database) *ServerConnection {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cur, err := db.Collection(jsmongo.ProtocolConnectionsCollectionName).Find(ctx, bson.M{
		"protocolDriver":               ProtocolDriverName,
		"protocolDriverInstanceNumber": instanceNumber,
		"enabled":                      true,
	})
	if err != nil {
		jslog.Fatal("Error reading protocol connections - %v", err)
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		jslog.Fatal("Error reading protocol connections - %v", err)
	}
	if len(docs) == 0 {
		jslog.Fatal("No enabled connection found for this instance!")
	}

	conn := connectionFromDoc(docs[0])
	if len(docs) > 1 {
		jslog.Log(jslog.LevelNoLog, "WARNING: more than one connection for this instance, using the first: %s", conn.Name)
	}
	jslog.Log(jslog.LevelNoLog, "Connection: %s [%d]", conn.Name, conn.ProtocolConnectionNumber)
	return conn
}
