/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"modbus-go/internal/drv"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"
	"github.com/riclolsen/json-scada/src/go-common/jsstats"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// App identifies the server driver.
var App = drv.AppDefs{
	Name:      "MODBUS_SERVER",
	EnvPrefix: "JS_MODBUSSRV_",
	Version:   "0.2.0",
	Msg:       "{json:scada} - Modbus Server Driver (Go)",
}

// Main runs the driver with the full os.Args; it returns the process exit code.
func Main(argv []string) int {
	s := drv.Start(App, argv)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	jslog.Log(jslog.LevelNoLog, "Mongo - Connecting...")
	cli, db, err := jsmongo.ConnectAndWait(ctx, s.Config)
	if err != nil {
		return 0
	}
	defer func() { _ = cli.Disconnect(context.Background()) }()
	jslog.Log(jslog.LevelNoLog, "Mongo - Connected.")

	if _, err := drv.EnsureInstance(ctx, db, App, s); err != nil {
		jslog.Log(jslog.LevelNoLog, "Redundancy - Error: %v", err)
	}
	if drv.InstanceDisabled(ctx, db, App, s.Instance) {
		jslog.Log(jslog.LevelNoLog, "Instance %d of %s is disabled. Exiting.", s.Instance, App.Name)
		jslog.Flush()
		return 0
	}

	connColl := db.Collection(jsmongo.ProtocolConnectionsCollectionName)
	rtColl := db.Collection(jsmongo.RealtimeDataCollectionName)
	cmdColl := db.Collection(jsmongo.CommandsQueueCollectionName)

	fctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	docs, err := jsmongo.FindAll(fctx, connColl, bson.M{
		"protocolDriver": App.Name, "protocolDriverInstanceNumber": s.Instance, "enabled": true,
	})
	cancel()
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "Error reading connections: %v", err)
		return -1
	}
	if len(docs) == 0 {
		jslog.Log(jslog.LevelNoLog, "No enabled connections found for driver %s instance %d. Exiting.", App.Name, s.Instance)
		jslog.Flush()
		return 0
	}

	var servers []*Server
	seen := map[float64]bool{}
	for _, d := range docs {
		cfg := Normalize(d)
		if seen[cfg.ProtocolConnectionNumber] {
			continue
		}
		seen[cfg.ProtocolConnectionNumber] = true
		srv := NewServer(cfg, MongoCommands{Coll: cmdColl}, nil)
		srv.SetFeeder(&Watcher{Cfg: cfg, Coll: rtColl, PM: srv.PM})
		servers = append(servers, srv)
		jslog.Log(jslog.LevelNoLog, "Loaded server %s %q mode=%s", drv.AnyString(cfg.ProtocolConnectionNumber), cfg.Name, cfg.ConnectionMode)
	}

	statsWriter := jsstats.Writer{
		NodeName: s.Config.NodeName,
		Timeout:  10 * time.Second,
		OnError: func(e jsstats.Entry, err error) {
			jslog.Log(jslog.LevelNoLog, "Stats write error: %v", err)
		},
	}
	ctl := &jsredundancy.Controller{
		Config:                s.Config,
		DriverName:            App.Name,
		InstanceNumber:        s.Instance,
		MissingInstanceActive: true,
		OnActivate: func() {
			jslog.Log(jslog.LevelNoLog, "Driver - Becoming ACTIVE, starting servers.")
			for _, srv := range servers {
				srv.Start()
			}
		},
		OnDeactivate: func() {
			jslog.Log(jslog.LevelNoLog, "Driver - Becoming INACTIVE, stopping servers.")
			for _, srv := range servers {
				if !srv.Cfg.KeepProtocolRunningWhileInactive {
					srv.Stop()
				}
			}
		},
		OnTick: func(db *mongo.Database) {
			entries := make([]jsstats.Entry, 0, len(servers))
			for _, srv := range servers {
				entries = append(entries, jsstats.Entry{
					ConnectionNumber: srv.Cfg.ProtocolConnectionNumber, Stats: srv.Stats().BSON(), Label: srv.Cfg.Name,
				})
			}
			statsWriter.Write(ctx, db.Collection(jsmongo.ProtocolConnectionsCollectionName), entries)
		},
	}
	go ctl.Run(ctx)
	go drv.WatchInstanceEnabled(ctx, db, App, s.Instance, stop)

	<-ctx.Done()
	jslog.Log(jslog.LevelNoLog, "Driver - shutting down...")
	for _, srv := range servers {
		srv.Stop()
	}
	jslog.Flush()
	return 0
}
