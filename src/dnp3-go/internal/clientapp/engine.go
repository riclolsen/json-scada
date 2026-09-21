/*
 * DNP3 Client Protocol driver for {json:scada}, in Go.
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

// Startup and the session lifecycle. Ports of main(), configureMaster() and
// the activation half of processRedundancy().

package clientapp

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Session tuning. opendnp3's defaults, which the C++ driver does not override.
const (
	responseTimeout = 5 * time.Second
	taskRetryPeriod = 5 * time.Second
	keepAlivePeriod = 30 * time.Second
	// linkTimeout and linkRetries apply on serial, where link confirmation is
	// on.
	linkTimeout = time.Second
	linkRetries = 3
	// timeSyncPeriod is how often the clock is written while connected. The
	// stack does not act on the outstation's NEED_TIME indication and the
	// indication cannot be inspected from outside the module, so the sync is
	// scheduled instead (deviation D3).
	timeSyncPeriod = 60 * time.Second
	// statsPeriod is how often connection statistics are refreshed.
	statsPeriod = 5 * time.Second
)

// Engine is the running driver.
type Engine struct {
	cfg            jsconfig.Config
	instanceNumber int

	conns  []*Connection
	byNum  map[int]*Connection
	groups []*dnp3util.Group

	queue      *ValueQueue
	redundancy *jsredundancy.Controller
}

// Run starts the driver and blocks until it is signalled to stop.
func Run(args jsconfig.Args, cfg jsconfig.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := &Engine{
		cfg:            cfg,
		instanceNumber: args.InstanceNumber,
		byNum:          map[int]*Connection{},
		queue:          NewValueQueue(),
	}

	cli, db, err := jsmongo.ConnectAndWait(ctx, cfg)
	if err != nil {
		jslog.Fatal("Error connecting to MongoDB - %v", err)
	}

	if err := loadInstance(ctx, db, args.InstanceNumber, !args.LogLevelFromCLI); err != nil {
		jslog.Fatal("%v", err)
	}
	if args.LogLevelFromCLI {
		jslog.Log(jslog.LevelDetailed,
			"Main: Keeping CLI log level override after loading instance configuration.")
	} else {
		jslog.Log(jslog.LevelDetailed,
			"Main: Effective log level loaded from instance configuration.")
	}

	conns, err := loadConnections(ctx, db, args.InstanceNumber)
	if err != nil {
		jslog.Fatal("%v", err)
	}
	_ = cli.Disconnect(context.Background())

	e.conns = conns
	for _, c := range conns {
		e.byNum[c.ProtocolConnectionNumber] = c
	}

	if err := e.buildChannels(); err != nil {
		jslog.Fatal("%v", err)
	}

	// The sessions are not started here: they start when redundancy makes this
	// node active, which is the equivalent of the C++ driver creating every
	// master in the disabled state.
	jslog.Log(jslog.LevelDetailed,
		"Main: All connections configured; waiting for redundancy activation.")

	e.redundancy = &jsredundancy.Controller{
		Config:         cfg,
		DriverName:     ProtocolDriverName,
		InstanceNumber: args.InstanceNumber,
		OnActivate:     func() { e.startSessions(ctx) },
		OnDeactivate:   e.stopSessions,
		OnTick:         func(db *mongo.Database) { e.writeStats(ctx, db) },
		// parity: the C++ driver defaults to active when there is no instance
		// document, so a single-node system with an incomplete configuration
		// still runs. The takeover rule itself is the shared one.
		MissingInstanceActive: true,
	}

	go e.mongoUpdateLoop(ctx)
	jslog.Log(jslog.LevelBasic, "Main: processMongo thread started.")
	go e.commandsLoop(ctx)
	jslog.Log(jslog.LevelBasic, "Main: processMongoCmd thread started.")
	go e.redundancy.Run(ctx)
	jslog.Log(jslog.LevelBasic, "Main: processRedundancy thread started.")
	go e.linkWatchLoop(ctx)

	jslog.Log(jslog.LevelBasic, "Main: Entering main loop...")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-sigs:
			jslog.Log(jslog.LevelNoLog, "Exiting application!")
			cancel()
			e.stopSessions()
			for _, g := range e.groups {
				g.Close()
			}
			e.queue.Close()
			jslog.Flush()
			return
		case <-ticker.C:
			jslog.Log(jslog.LevelDetailed, "Main: Still running...")
		}
	}
}

// buildChannels groups the connections onto shared buses and hands each one the
// channel its session will run on.
func (e *Engine) buildChannels() error {
	specs := make([]dnp3util.StationSpec, 0, len(e.conns))
	for _, c := range e.conns {
		specs = append(specs, dnp3util.StationSpec{
			ConnectionNumber:  c.ProtocolConnectionNumber,
			Name:              c.Name,
			LocalLinkAddress:  c.LocalLinkAddress,
			RemoteLinkAddress: c.RemoteLinkAddress,
			Channel:           c.ChannelSpec(),
			AllowedRemoteIPs:  nil, // a master does not filter its peers
			ScanHint:          c.shortestScanPeriod(),
		})
	}

	groups, err := dnp3util.BuildGroups(specs, true)
	if err != nil {
		return err
	}
	e.groups = groups

	for _, g := range groups {
		for num, ch := range g.Channels {
			conn := e.byNum[num]
			conn.Chan = ch
			conn.Group = g
			jslog.Log(jslog.LevelDetailed, "Main: Channel created for %s", conn.Name)
			jslog.Log(jslog.LevelBasic, "%s - Connection configured.", conn.Name)
		}
	}
	return nil
}

// startSessions builds and runs a master session per connection. Called when
// this node becomes the active one.
func (e *Engine) startSessions(ctx context.Context) {
	for _, conn := range e.conns {
		if conn.Session() != nil {
			continue
		}
		cfg := master.Config{
			LocalAddr:          uint16(conn.LocalLinkAddress),
			RemoteAddr:         uint16(conn.RemoteLinkAddress),
			ResponseTimeout:    responseTimeout,
			TaskRetryPeriod:    taskRetryPeriod,
			KeepAlive:          keepAlivePeriod,
			IntegrityOnStartup: true,
			Log:                jslog.NewStackLogger(conn.Name),
		}
		if conn.EnableUnsolicited {
			cfg.DisableUnsolOnStartup = false
			cfg.UnsolClassMask = dnp3.Class123
		} else {
			cfg.DisableUnsolOnStartup = true
			cfg.UnsolClassMask = 0
		}
		if conn.ConnectionMode == dnp3util.ModeSerial {
			cfg.UseLinkConfirms = true
			cfg.LinkRetries = linkRetries
			cfg.LinkTimeout = linkTimeout
		}

		session := master.New(cfg, newSOEHandler(conn, e.queue))
		sctx, cancel := context.WithCancel(ctx)
		conn.setSession(session, cancel)

		jslog.Log(jslog.LevelDetailed, "%s - Starting master session...", conn.Name)
		go func(conn *Connection, session *master.Session, sctx context.Context) {
			if err := session.Run(sctx, conn.Chan); err != nil && sctx.Err() == nil {
				jslog.Log(jslog.LevelBasic, "%s - Session stopped: %v", conn.Name, err)
			}
		}(conn, session, sctx)

		e.startScans(sctx, conn, session)
		if conn.TimeSyncMode > 0 {
			go e.timeSyncLoop(sctx, conn, session)
		}
	}
}

// stopSessions cancels every session. Called when this node goes to standby and
// on shutdown.
func (e *Engine) stopSessions() {
	for _, conn := range e.conns {
		conn.stopSession()
	}
}

// connByNumber finds a connection by its protocolConnectionNumber.
func (e *Engine) connByNumber(n int) *Connection {
	return e.byNum[n]
}

// linkWatchLoop notices a connection losing its link and invalidates its
// points, the way the C++ ChannelListener does on a channel close.
func (e *Engine) linkWatchLoop(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		for _, conn := range e.conns {
			session := conn.Session()
			now := session != nil && session.Connected()
			conn.connected.Store(now)
			if conn.wasConnected && !now {
				jslog.Log(jslog.LevelBasic, "%s - Channel state: CLOSED", conn.Name)
				go e.invalidatePoints(ctx, conn.ProtocolConnectionNumber, conn.Name)
			}
			if !conn.wasConnected && now {
				jslog.Log(jslog.LevelBasic, "%s - Channel state: OPEN", conn.Name)
			}
			conn.wasConnected = now
		}
	}
}
