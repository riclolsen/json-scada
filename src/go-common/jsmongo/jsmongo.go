/*
 * Shared {json:scada} driver support library, in Go.
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

// Package jsmongo holds the MongoDB connection helpers and the collection
// names. Merge of dnp3-go/internal/mongoutil and
// iec60870-5/internal/mongoutil.
package jsmongo

import (
	"context"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection names.
const (
	ProtocolConnectionsCollectionName     = "protocolConnections"
	ProtocolDriverInstancesCollectionName = "protocolDriverInstances"
	RealtimeDataCollectionName            = "realtimeData"
	CommandsQueueCollectionName           = "commandsQueue"
)

// URI returns the connection string with the TLS options of json-scada.json
// appended as URI parameters.
//
// parity: OPC-UA-Client-Go appended these with a bare "&" and no test for an
// existing query string, which produces a malformed URI when the connection
// string carries none. This uses the iec60870-5 form, which opens the query
// with "/?" when needed. Drivers whose connection string already contains "?"
// — the normal case — get a byte-identical URI.
func URI(cfg jsconfig.Config) string {
	uri := cfg.MongoConnectionString
	appendOpt := func(opt string) {
		if strings.Contains(uri, "?") {
			uri += "&" + opt
		} else {
			uri += "/?" + opt
		}
	}
	if cfg.TLSCaPemFile != "" || cfg.TLSClientPemFile != "" {
		appendOpt("tls=true")
	}
	if cfg.TLSCaPemFile != "" {
		appendOpt("tlsCAFile=" + cfg.TLSCaPemFile)
	}
	if cfg.TLSClientPemFile != "" {
		appendOpt("tlsCertificateKeyFile=" + cfg.TLSClientPemFile)
	}
	if cfg.TLSClientKeyPassword != "" {
		appendOpt("tlsCertificateKeyFilePassword=" + cfg.TLSClientKeyPassword)
	}
	if cfg.TLSInsecure || cfg.TLSAllowChainErrors {
		appendOpt("tlsInsecure=true")
	}
	if cfg.TLSAllowInvalidHostnames {
		appendOpt("tlsAllowInvalidHostnames=true")
	}
	return uri
}

// Connect opens a client without pinging it.
//
// Every long-running goroutine of a driver owns its own client, the way the
// C++ drivers open a mongocxx::client per thread: a change-stream cursor
// blocks its client, so sharing one would stall the others.
//
// No client-level Timeout is set: it would apply to every operation, and a
// change stream's Next is one, so a watcher would be torn down and rebuilt on
// each expiry. Calls that need a bound pass their own context deadline.
func Connect(cfg jsconfig.Config) (*mongo.Client, error) {
	return mongo.Connect(options.Client().ApplyURI(URI(cfg)))
}

// ConnectAndPing opens a client and verifies it answers, which is what the
// drivers that fail fast on a dead database want.
func ConnectAndPing(cfg jsconfig.Config) (*mongo.Client, *mongo.Database, error) {
	cli, err := Connect(cfg)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := cli.Ping(ctx, nil); err != nil {
		_ = cli.Disconnect(context.Background())
		return nil, nil, err
	}
	return cli, cli.Database(cfg.MongoDatabaseName), nil
}

// Ping checks the database is answering, within the given budget.
func Ping(db *mongo.Database, budget time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	return db.RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
}

// IsLive is Ping with the 5 s budget of isConnected()/isMongoLive().
func IsLive(db *mongo.Database) bool {
	return Ping(db, 5*time.Second) == nil
}

// PingOK is the short-budget liveness test of iec60870-5.
func PingOK(cli *mongo.Client) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return cli.Ping(ctx, nil) == nil
}

// ConnectAndWait retries until the database answers a ping, logging each
// attempt the way the C++ server does.
func ConnectAndWait(ctx context.Context, cfg jsconfig.Config) (*mongo.Client, *mongo.Database, error) {
	for {
		jslog.Log(jslog.LevelBasic, "Connecting to MongoDB")
		cli, err := Connect(cfg)
		if err == nil {
			db := cli.Database(cfg.MongoDatabaseName)
			if IsLive(db) {
				jslog.Log(jslog.LevelBasic, "Connected to MongoDB")
				return cli, db, nil
			}
			_ = cli.Disconnect(context.Background())
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// FindAll runs a query and decodes every document into a bson.M slice.
func FindAll(ctx context.Context, coll *mongo.Collection, filter any,
	opts ...options.Lister[options.FindOptions]) ([]bson.M, error) {
	cur, err := coll.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// Now is the timestamp written into documents the drivers create.
func Now() bson.DateTime {
	return bson.NewDateTimeFromTime(time.Now())
}
