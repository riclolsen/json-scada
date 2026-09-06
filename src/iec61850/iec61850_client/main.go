/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * A drop-in alternative to the C# driver of src/iec61850_client: same
 * protocol driver name, same configuration documents, same MongoDB
 * semantics, with no native library dependency.
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
	"strings"
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
	cfg, instNum := readConfigFile()
	instanceNumber = instNum

	cli, _, err := jsmongo.ConnectAndPing(cfg)
	if err != nil {
		jslog.Fatal("Error connecting to MongoDB - %v", err)
	}
	db := cli.Database(cfg.MongoDatabaseName)

	inst := loadInstance(db, cfg)
	jslog.Log(jslog.LevelNoLog, "Instance: %d", inst.ProtocolDriverInstanceNumber)

	conns := loadConnections(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initRedundancy(ctx, cfg, conns)
	go redundancy.Run(ctx)
	go mongoUpdateLoop(ctx, cfg, conns)
	go commandsLoop(ctx, cfg, conns)

	for _, conn := range conns {
		jslog.Log(jslog.LevelNoLog, "%s - New Connection", conn.Name)
		go connectionLoop(ctx, conn)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-sigs:
			jslog.Log(jslog.LevelNoLog, "Exiting application!")
			cancel()
			for _, conn := range conns {
				closeConnection(conn)
			}
			jslog.Flush()
			os.Exit(0)
		case <-ticker.C:
		}
	}
}

// loadInstance reads the driver instance document and validates it can run
// on this node, with the same checks and messages as the C# driver.
func loadInstance(db *mongo.Database, cfg jsconfig.Config) *jsmodel.DriverInstance {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	coll := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName)
	cur, err := coll.Find(ctx, bson.M{
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

// loadConnections reads the enabled connections of this instance and
// preloads the tags configured for each of them.
func loadConnections(db *mongo.Database) []*Iec61850Connection {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	coll := db.Collection(jsmongo.ProtocolConnectionsCollectionName)
	cur, err := coll.Find(ctx, bson.M{
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

	collRTD := db.Collection(jsmongo.RealtimeDataCollectionName)
	conns := make([]*Iec61850Connection, 0, len(docs))

	for _, doc := range docs {
		conn := connectionFromDoc(doc)
		preloadEntries(ctx, collRTD, conn)
		conn.TagKeys.Reset()
		if len(conn.IPAddresses) < 1 {
			jslog.Fatal("Missing remote endpoint URLs list!")
		}
		conns = append(conns, conn)
	}

	if len(conns) == 0 {
		jslog.Fatal("No connections found!")
	}
	return conns
}

// preloadEntries loads the tags of a connection: both supervised points and
// command points, since commands are dispatched through the same map.
func preloadEntries(ctx context.Context, collRTD *mongo.Collection, conn *Iec61850Connection) {
	cur, err := collRTD.Find(ctx, bson.M{
		"protocolSourceConnectionNumber": conn.ProtocolConnectionNumber,
	})
	if err != nil {
		jslog.Fatal("Error reading realtime data - %v", err)
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		jslog.Fatal("Error reading realtime data - %v", err)
	}

	for _, doc := range docs {
		tag := jsmongo.GetString(doc, "tag", "")
		objAddr := strings.TrimSpace(jsmongo.GetString(doc, "protocolSourceObjectAddress", ""))
		commonAddr := strings.ToUpper(strings.TrimSpace(jsmongo.GetString(doc, "protocolSourceCommonAddress", "")))
		if conn.AutoCreateTags {
			conn.InsertedTags[tag] = true
		}
		if objAddr == "" {
			continue
		}
		entry := &Iec61850Entry{
			Path:  objAddr,
			FC:    parseFCOrST(commonAddr),
			JsTag: tag,
		}
		key := objAddr + commonAddr
		if _, dup := conn.Entries[key]; !dup {
			conn.Entries[key] = entry
			conn.EntryOrder = append(conn.EntryOrder, key)
		}
	}
	jslog.Log(jslog.LevelDetailed, "%s - %d tags configured", conn.Name, len(conn.Entries))
}
