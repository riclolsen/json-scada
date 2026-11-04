/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

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

// App identifies the client driver.
var App = drv.AppDefs{
	Name:      "MODBUS",
	EnvPrefix: "JS_MODBUS_",
	Version:   "0.2.0",
	Msg:       "{json:scada} - Modbus Client Driver (Go)",
}

// BindingsReloadPeriod is how often the tag bindings are re-read while active.
const BindingsReloadPeriod = 60 * time.Second

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

	var ctl *jsredundancy.Controller
	active := func() bool { return ctl != nil && ctl.Active() }

	var conns []*Connection
	seen := map[float64]bool{}
	for _, d := range docs {
		cfg := Normalize(d)
		if seen[cfg.ProtocolConnectionNumber] {
			continue
		}
		seen[cfg.ProtocolConnectionNumber] = true
		if err := cfg.Validate(); err != nil {
			jslog.Log(jslog.LevelNoLog, "%s: %v; using %s", cfg.Name, err, ModeTCP)
		}
		conn := NewConnection(cfg, MongoSink{Coll: rtColl})
		conn.Commands = MongoCommands{Coll: cmdColl, Name: cfg.Name}
		conn.Active = active
		if cfg.AutoCreateTags {
			if err := AutoCreateTags(ctx, cfg, MongoTags{Coll: rtColl}); err != nil {
				jslog.Log(jslog.LevelNoLog, "%s: auto-tag error: %v", cfg.Name, err)
			}
		}
		if err := loadBindings(ctx, conn, rtColl); err != nil {
			jslog.Log(jslog.LevelNoLog, "%s: bindings load error: %v", cfg.Name, err)
		}
		conns = append(conns, conn)
		jslog.Log(jslog.LevelNoLog, "Loaded connection %s %q mode=%s", drv.AnyString(cfg.ProtocolConnectionNumber), cfg.Name, cfg.ConnectionMode)
	}

	statsWriter := jsstats.Writer{
		NodeName: s.Config.NodeName,
		Timeout:  10 * time.Second,
		OnError: func(e jsstats.Entry, err error) {
			jslog.Log(jslog.LevelNoLog, "Stats write error: %v", err)
		},
	}
	ctl = &jsredundancy.Controller{
		Config:                s.Config,
		DriverName:            App.Name,
		InstanceNumber:        s.Instance,
		MissingInstanceActive: true,
		OnActivate: func() {
			jslog.Log(jslog.LevelNoLog, "Driver - Becoming ACTIVE, starting connections.")
			for _, c := range conns {
				c.Start()
			}
		},
		OnDeactivate: func() {
			jslog.Log(jslog.LevelNoLog, "Driver - Becoming INACTIVE, stopping connections.")
			for _, c := range conns {
				if !c.Cfg.KeepProtocolRunningWhileInactive {
					c.Stop()
				}
			}
		},
		OnTick: func(db *mongo.Database) {
			entries := make([]jsstats.Entry, 0, len(conns))
			for _, c := range conns {
				entries = append(entries, jsstats.Entry{
					ConnectionNumber: c.Cfg.ProtocolConnectionNumber, Stats: c.Stats().BSON(), Label: c.Cfg.Name,
				})
			}
			statsWriter.Write(ctx, db.Collection(jsmongo.ProtocolConnectionsCollectionName), entries)
		},
	}
	go ctl.Run(ctx)
	go drv.WatchInstanceEnabled(ctx, db, App, s.Instance, stop)

	t := time.NewTicker(BindingsReloadPeriod)
	defer t.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-t.C:
			if !ctl.Active() {
				continue
			}
			for _, c := range conns {
				if err := loadBindings(ctx, c, rtColl); err != nil {
					jslog.Log(jslog.LevelNoLog, "Bindings reload error: %v", err)
				}
			}
		}
	}
	jslog.Log(jslog.LevelNoLog, "Driver - shutting down...")
	for _, c := range conns {
		c.Stop()
	}
	jslog.Flush()
	return 0
}

func loadBindings(ctx context.Context, c *Connection, rt *mongo.Collection) error {
	fctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	docs, err := jsmongo.FindAll(fctx, rt, bson.M{
		"protocolSourceConnectionNumber": c.Cfg.ProtocolConnectionNumber,
		"origin":                         bson.M{"$ne": "command"},
	})
	if err != nil {
		return err
	}
	c.SetBindings(docs)
	return nil
}
