/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * A drop-in alternative to the C# driver of src/OPC-UA-Client: same
 * protocol driver name, same configuration documents, same MongoDB
 * semantics, with no .NET runtime dependency.
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

// Startup and configuration loading. Port of Program.cs.

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
		go connectionLoop(ctx, cfg, conn)
	}

	// The C# driver polls the console for Esc; a driver under supervisord
	// or NSSM is stopped with a signal instead (deviation D10).
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-sigs:
			jslog.Log(jslog.LevelNoLog, "Exiting application!")
			cancel()
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
		"enabled":                      true,
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

	// parity: the C# driver breaks out of the loop after the first
	// document, so only the first match is ever considered.
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
func loadConnections(db *mongo.Database) []*OPCUAConnection {
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
	conns := make([]*OPCUAConnection, 0, len(docs))

	for _, doc := range docs {
		conn := connectionFromDoc(doc)
		if err := preloadTags(ctx, collRTD, conn); err != nil {
			jslog.Fatal("Error reading realtime data - %v", err)
		}
		conn.TagKeys.Reset()
		if len(conn.EndpointURLs) < 1 {
			jslog.Fatal("Missing remote endpoint URLs list!")
		}
		conns = append(conns, conn)
		jslog.Log(jslog.LevelNoLog, "%s - New Connection", conn.Name)
	}

	if len(conns) == 0 {
		jslog.Fatal("No connections found!")
	}
	return conns
}

// preloadTags loads the tags already configured for a connection. Supervised
// points become monitored items, grouped by publishing interval; every tag,
// commands included, marks its address as present so autoCreateTags does not
// insert it a second time.
func preloadTags(ctx context.Context, collRTD *mongo.Collection, conn *OPCUAConnection) error {
	cur, err := collRTD.Find(ctx, bson.M{
		"protocolSourceConnectionNumber": conn.ProtocolConnectionNumber,
	})
	if err != nil {
		return err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return err
	}

	jslog.Log(jslog.LevelNoLog, "%s - Found %d tags in database.", conn.Name, len(docs))

	var listMon []*monItem
	var order []float64
	subs := map[float64][]*monItem{}
	addrs := map[string]bool{}

	for _, doc := range docs {
		addr := jsmongo.GetString(doc, "protocolSourceObjectAddress", "")

		if jsmongo.GetString(doc, "origin", "") == "supervised" {
			pub := jsmongo.GetDouble(doc, "protocolSourcePublishingInterval", 0)
			if _, seen := subs[pub]; !seen {
				jslog.Log(jslog.LevelNoLog, "%s - Found publishing interval of %v seconds.", conn.Name, pub)
				subs[pub] = nil
				order = append(order, pub)
			}
			it := &monItem{
				NodeID:      addr,
				DisplayName: jsmongo.GetString(doc, "ungroupedDescription", ""),
				SamplingMs:  jsmongo.GetDouble(doc, "protocolSourceSamplingInterval", 0) * 1000,
				QueueSize:   uint32(jsmongo.GetDouble(doc, "protocolSourceQueueSize", 0)),
			}
			subs[pub] = append(subs[pub], it)

			// parity: with autoCreateTags on, the C# driver puts the
			// preconfigured items in the same single subscription as the
			// discovered ones, each keeping its own sampling interval and
			// queue size.
			listMon = append(listMon, it)
		}

		// parity: every tag of the connection registers its address,
		// including command tags and tags with an empty address.
		addrs[addr] = true
	}

	conn.CommitPreloadedTags(listMon, subs, order, addrs)

	jslog.Log(jslog.LevelDetailed, "%s - %d monitored items preconfigured in %d subscription(s)",
		conn.Name, len(listMon), len(order))
	return nil
}

// reloadConnectionTags puts a connection back into its startup state after a
// discovery that did not finish: the tags created by the partial pass are in
// realtimeData, so they must be read back before browsing again or they
// would be skipped as "already inserted" and never monitored.
func reloadConnectionTags(ctx context.Context, cfg jsconfig.Config, conn *OPCUAConnection) error {
	cli, _, err := jsmongo.ConnectAndPing(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = cli.Disconnect(context.Background()) }()

	conn.ResetDiscovery()

	loadCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return preloadTags(loadCtx, cli.Database(cfg.MongoDatabaseName).Collection(jsmongo.RealtimeDataCollectionName), conn)
}
