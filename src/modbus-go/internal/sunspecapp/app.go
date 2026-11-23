/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

import (
	"context"
	"time"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/drv"
	"modbus-go/internal/mbsec"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"
	"github.com/riclolsen/json-scada/src/go-common/jsstats"
	"github.com/riclolsen/json-scada/src/go-common/jstags"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// App identifies the driver.
var App = drv.AppDefs{
	Name:      "SUNSPEC",
	EnvPrefix: "JS_SUNSPEC_",
	Version:   "0.1.0",
	Msg:       "{json:scada} - SunSpec Client Driver (Go)",
}

// Main runs the driver with the full os.Args; it returns the process exit
// code. The lifecycle (instance, redundancy, statistics, bindings reload,
// security event writer) is the MODBUS client's.
func Main(argv []string) int { return clientapp.Run(argv, App, newManagedConnection) }

func newManagedConnection(ctx context.Context, doc bson.M, rt, cmds *mongo.Collection, writer *jssecevents.Writer,
	origin jssecevents.Origin, active func() bool) clientapp.ManagedConnection {
	cfg := Normalize(doc)
	if err := cfg.Link.Validate(); err != nil {
		jslog.Log(jslog.LevelNoLog, "%s: %v; using %s", cfg.Name(), err, clientapp.ModeTCP)
	}
	c := NewConnection(cfg, clientapp.MongoSink{Coll: rt})
	c.Commands = clientapp.MongoCommands{Coll: cmds, Name: cfg.Name()}
	c.Active = active
	c.Tags = MongoTags{Coll: rt, ConnNumber: int(cfg.ConnNumber()), Keys: new(jstags.KeyAllocator)}
	origin.ConnectionName = cfg.Name()
	c.SetSecurity(&mbsec.Reporter{W: writer, Role: "client", Origin: origin}, rt)
	c.LoadBindings = func(ctx context.Context) ([]bson.M, error) {
		fctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return jsmongo.FindAll(fctx, rt, bson.M{
			"protocolSourceConnectionNumber": cfg.ConnNumber(),
			"origin":                         bson.M{"$ne": "command"},
		})
	}
	if err := c.ReloadBindings(ctx); err != nil {
		jslog.Log(jslog.LevelBasic, "%s: bindings load error: %v", cfg.Name(), err)
	}
	return c
}

// KeepRunning implements clientapp.ManagedConnection.
func (c *Connection) KeepRunning() bool { return c.Cfg.Link.KeepProtocolRunningWhileInactive }

// Reload implements clientapp.ManagedConnection.
func (c *Connection) Reload(ctx context.Context, _ *mongo.Collection) error {
	return c.ReloadBindings(ctx)
}

// StatsEntry implements clientapp.ManagedConnection.
func (c *Connection) StatsEntry() jsstats.Entry {
	st := c.Stats().BSON()
	if id := c.PeerIdentity(); id != nil {
		st["peerIdentity"] = id
	}
	return jsstats.Entry{ConnectionNumber: c.Cfg.ConnNumber(), Label: c.Cfg.Name(), Stats: st,
		IsConnected: jsstats.Bool(c.Connected())}
}
