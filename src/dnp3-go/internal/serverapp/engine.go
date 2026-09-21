/*
 * DNP3 Outstation Server Protocol driver for {json:scada}, in Go.
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

// Startup and the realtimeData change stream. Port of main().

package serverapp

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"github.com/dscsystems/go-dnp3/outstation"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// statsPeriod is how often connection statistics are refreshed.
const statsPeriod = 5 * time.Second

// Engine is the running driver.
type Engine struct {
	cfg            jsconfig.Config
	instanceNumber int

	conns  []*Connection
	byNum  map[int]*Connection
	groups []*dnp3util.Group

	// cmdMu guards cmdDB, the database handle the command handlers use. It is
	// replaced whenever the change-stream client reconnects, which is how the
	// C++ server re-points its handlers at a fresh mongocxx::client.
	cmdMu sync.RWMutex
	cmdDB *mongo.Database
}

// commandDB returns the database handle for a command lookup, or nil while the
// driver is between connections.
func (e *Engine) commandDB() *mongo.Database {
	e.cmdMu.RLock()
	defer e.cmdMu.RUnlock()
	return e.cmdDB
}

func (e *Engine) setCommandDB(db *mongo.Database) {
	e.cmdMu.Lock()
	e.cmdDB = db
	e.cmdMu.Unlock()
}

// Run starts the driver and blocks until it is signalled to stop.
func Run(args jsconfig.Args, cfg jsconfig.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := &Engine{
		cfg:            cfg,
		instanceNumber: args.InstanceNumber,
		byNum:          map[int]*Connection{},
	}

	cli, db, err := jsmongo.ConnectAndWait(ctx, cfg)
	if err != nil {
		jslog.Fatal("Error connecting to MongoDB - %v", err)
	}

	if err := loadInstance(ctx, db, args.InstanceNumber, cfg.NodeName, !args.LogLevelFromCLI); err != nil {
		jslog.Fatal("%v", err)
	}
	conns, err := loadConnections(ctx, db, args.InstanceNumber)
	if err != nil {
		jslog.Fatal("%v", err)
	}
	e.conns = conns
	for _, c := range conns {
		e.byNum[c.ProtocolConnectionNumber] = c
	}

	// Destinations are created before the outstations are sized, because the
	// sizing reads what this pass has just written.
	for _, conn := range conns {
		if err := e.autoCreateAll(ctx, db, conn); err != nil {
			jslog.Fatal("%s - Error creating destinations: %v", conn.Name, err)
		}
	}

	if err := e.buildChannels(); err != nil {
		jslog.Fatal("%v", err)
	}
	for _, conn := range conns {
		if conn.Chan == nil {
			jslog.Log(jslog.LevelBasic, "%s - Error allocating channel!", conn.Name)
			continue
		}
		if err := e.buildOutstation(ctx, db, conn); err != nil {
			jslog.Fatal("%s - Error building the outstation: %v", conn.Name, err)
		}
	}
	_ = cli.Disconnect(context.Background())

	go e.changeStreamLoop(ctx)
	go e.statsLoop(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs

	jslog.Log(jslog.LevelNoLog, "Exiting application!")
	cancel()
	for _, g := range e.groups {
		g.Close()
	}
	jslog.Flush()
}

// buildChannels groups the connections onto shared buses.
func (e *Engine) buildChannels() error {
	specs := make([]dnp3util.StationSpec, 0, len(e.conns))
	for _, c := range e.conns {
		specs = append(specs, dnp3util.StationSpec{
			ConnectionNumber:  c.ProtocolConnectionNumber,
			Name:              c.Name,
			LocalLinkAddress:  c.LocalLinkAddress,
			RemoteLinkAddress: c.RemoteLinkAddress,
			Channel:           c.ChannelSpec(),
			// For a passive connection the ipAddresses list is the set of
			// clients allowed to connect.
			AllowedRemoteIPs: c.IPAddresses,
		})
	}

	groups, err := dnp3util.BuildGroups(specs, false)
	if err != nil {
		return err
	}
	e.groups = groups

	for _, g := range groups {
		for num, ch := range g.Channels {
			conn := e.byNum[num]
			// The link state is read per station, not per bus: on a multi-drop
			// line each session connects and reconnects on its own.
			conn.Chan, conn.link = dnp3util.WrapLinkState(ch)
			conn.Group = g
			jslog.Log(jslog.LevelBasic, "%s - Created %s channel.", conn.Name, conn.ConnectionMode)
		}
	}
	return nil
}

// changeStreamLoop watches realtimeData and distributes every change to the
// connections it is destined for.
func (e *Engine) changeStreamLoop(ctx context.Context) {
	var resumeToken bson.Raw

	for ctx.Err() == nil {
		cli, db, err := jsmongo.ConnectAndWait(ctx, e.cfg)
		if err != nil {
			return
		}
		e.setCommandDB(db)

		stream, err := db.Collection(jsmongo.RealtimeDataCollectionName).
			Watch(ctx, e.changeStreamPipeline(), e.changeStreamOptions(resumeToken))
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Mongo change stream - Exception: %v", err)
			e.setCommandDB(nil)
			_ = cli.Disconnect(context.Background())
			resumeToken = nil
			sleepCtx(ctx, 5*time.Second)
			continue
		}
		jslog.Log(jslog.LevelBasic, "Watching for changes on collection: realtimeData...")

		// After a reconnection the picture may be stale, so every distributed
		// tag is re-read and applied.
		for _, conn := range e.conns {
			e.reloadIntegrity(ctx, db, conn)
		}

		for stream.Next(ctx) {
			resumeToken = stream.ResumeToken()
			var change struct {
				FullDocument bson.M `bson:"fullDocument"`
			}
			if err := stream.Decode(&change); err != nil {
				jslog.Log(jslog.LevelDetailed, "Mongo change stream - cannot decode change: %v", err)
				continue
			}
			if change.FullDocument != nil {
				e.distribute(change.FullDocument)
			}
		}
		if err := stream.Err(); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelNoLog, "Mongo change stream - Exception: %v", err)
		}

		_ = stream.Close(context.Background())
		e.setCommandDB(nil)
		_ = cli.Disconnect(context.Background())
		sleepCtx(ctx, 5*time.Second)
	}
}

// changeStreamPipeline is the C++ server's filter: a supervised tag update that
// is not itself a sourceDataUpdate write, or any replacement.
func (e *Engine) changeStreamPipeline() mongo.Pipeline {
	numbers := make(bson.A, 0, len(e.conns))
	for _, c := range e.conns {
		numbers = append(numbers, c.ProtocolConnectionNumber)
	}
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"$or": bson.A{
			bson.M{"$and": bson.A{
				bson.M{"fullDocument.protocolDestinations": bson.M{"$ne": nil}},
				bson.M{"fullDocument.protocolDestinations.protocolDestinationConnectionNumber": bson.M{"$in": numbers}},
				bson.M{"updateDescription.updatedFields.sourceDataUpdate": bson.M{"$exists": false}},
				bson.M{"operationType": "update"},
			}},
			bson.M{"operationType": "replace"},
		}}}},
	}
}

func (e *Engine) changeStreamOptions(resumeToken bson.Raw) *options.ChangeStreamOptionsBuilder {
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	if len(resumeToken) > 0 {
		opts.SetResumeAfter(resumeToken)
	}
	return opts
}

// distribute applies one changed tag to every connection it is destined for.
//
// All destinations of one connection are applied inside a single Update, so a
// master polling mid-change cannot see a torn set.
func (e *Engine) distribute(doc bson.M) {
	byConn := map[int][]Destination{}
	for _, d := range DestinationsOf(doc) {
		if _, ok := e.byNum[d.ConnectionNumber]; ok {
			byConn[d.ConnectionNumber] = append(byConn[d.ConnectionNumber], d)
		}
	}
	for num, dests := range byConn {
		conn := e.byNum[num]
		if conn.Station() == nil {
			continue
		}
		conn.Update(func(db *outstation.Database) {
			for _, d := range dests {
				ApplyValue(db, doc, d, conn)
			}
		})
	}
}

// statsLoop refreshes the connection statistics.
func (e *Engine) statsLoop(ctx context.Context) {
	t := time.NewTicker(statsPeriod)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		db := e.commandDB()
		if db == nil {
			continue
		}
		e.writeStats(ctx, db)
	}
}

// trimFloat renders a value for valueString without a trailing exponent.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
