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

// Sizing and configuring the outstation database, and starting the session.
// Port of the database_by_sizes and AddOutstation block of main().

package serverapp

import (
	"context"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/outstation"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Session tuning. opendnp3's defaults, which the C++ server does not override
// except for the event buffer size.
const (
	confirmTimeout   = 5 * time.Second
	selectTimeout    = 5 * time.Second
	maxTxFragment    = 2048
	unsolHoldTime    = 200 * time.Millisecond
	unsolMaxEvents   = 20
	unsolConfirmWait = 5 * time.Second
	unsolMaxRetries  = 3
	linkTimeout      = time.Second
	linkRetries      = 3
)

// application answers the master's clock and restart requests.
type application struct {
	outstation.NopApplication
	conn *Connection
}

// SupportsWriteTime follows timeSyncMode: zero refuses clock writes, which is
// what the field documents.
func (a application) SupportsWriteTime() bool { return a.conn.TimeSyncMode != 0 }

func (a application) WriteAbsoluteTime(t time.Time) bool {
	if a.conn.TimeSyncMode == 0 {
		return false
	}
	jslog.Log(jslog.LevelDetailed, "%s - Master set the clock to %s",
		a.conn.Name, t.Format(time.RFC3339Nano))
	return true
}

// tagsFor reads every supervised tag distributed on a connection.
func tagsFor(ctx context.Context, db *mongo.Database, connNumber int) ([]bson.M, error) {
	return jsmongo.FindAll(ctx, db.Collection(jsmongo.RealtimeDataCollectionName), bson.M{
		"origin": "supervised",
		"protocolDestinations.protocolDestinationConnectionNumber": connNumber,
	})
}

// destinationsFor returns the destinations of a tag that belong to this
// connection.
func destinationsFor(doc bson.M, connNumber int) []Destination {
	var out []Destination
	for _, d := range DestinationsOf(doc) {
		if d.ConnectionNumber == connNumber {
			out = append(out, d)
		}
	}
	return out
}

// sizeDatabase works out how many points of each family the connection needs,
// which is the highest object address of each plus one.
func sizeDatabase(conn *Connection, tags []bson.M) outstation.DatabaseConfig {
	last := map[family]int{}
	for f := famBinary; f <= famTimeAndInterval; f++ {
		last[f] = -1
	}

	droppedTimeAndInterval := 0
	for _, doc := range tags {
		for _, d := range destinationsFor(doc, conn.ProtocolConnectionNumber) {
			f := familyOf(d.CommonAddress)
			if f == famTimeAndInterval {
				droppedTimeAndInterval++
				continue
			}
			if d.ObjectAddress > last[f] {
				last[f] = d.ObjectAddress
			}
		}
	}
	if droppedTimeAndInterval > 0 {
		jslog.Log(jslog.LevelBasic,
			"%s - %d time-and-interval destination(s) ignored: group 50 is not supported.",
			conn.Name, droppedTimeAndInterval)
	}

	return outstation.DatabaseConfig{
		Binary:             last[famBinary] + 1,
		DoubleBitBinary:    last[famDoubleBit] + 1,
		Counter:            last[famCounter] + 1,
		FrozenCounter:      last[famFrozenCounter] + 1,
		Analog:             last[famAnalog] + 1,
		BinaryOutputStatus: last[famBinaryOutputStatus] + 1,
		AnalogOutputStatus: last[famAnalogOutputStatus] + 1,
		OctetString:        last[famOctetString] + 1,
	}
}

// configurePoints applies the family-wide defaults and then the per-point
// variations of each destination.
//
// Database.Configure replaces the whole PointConfig, so every call is a
// read-modify-write: a zero Class would be ClassNone and would silently stop
// the point ever producing an event.
func configurePoints(db *outstation.Database, conn *Connection, tags []bson.M, cfg outstation.DatabaseConfig) {
	setAll := func(f family, count int) {
		pt, ok := f.pointType()
		if !ok {
			return
		}
		def := defaultVariations[f]
		class := defaultClasses[f]
		for i := 0; i < count; i++ {
			db.Configure(pt, uint16(i), outstation.PointConfig{
				Class:           class,
				StaticVariation: def.static,
				EventVariation:  def.event,
				Deadband:        0,
			})
		}
	}

	setAll(famBinary, cfg.Binary)
	setAll(famDoubleBit, cfg.DoubleBitBinary)
	setAll(famCounter, cfg.Counter)
	setAll(famFrozenCounter, cfg.FrozenCounter)
	setAll(famAnalog, cfg.Analog)
	setAll(famBinaryOutputStatus, cfg.BinaryOutputStatus)
	setAll(famAnalogOutputStatus, cfg.AnalogOutputStatus)
	setAll(famOctetString, cfg.OctetString)

	// Per-point overrides, after the family pass, as in the C++ server's second
	// sweep over the tags.
	for _, doc := range tags {
		for _, d := range destinationsFor(doc, conn.ProtocolConnectionNumber) {
			f := familyOf(d.CommonAddress)
			pt, ok := f.pointType()
			if !ok {
				continue
			}
			vp := variationsFor(f, d.ASDU)
			if pc, found := pointConfig(db, pt, uint16(d.ObjectAddress)); found {
				pc.StaticVariation = vp.static
				pc.EventVariation = vp.event
				db.Configure(pt, uint16(d.ObjectAddress), pc)
			}
		}
	}
}

// pointConfig reads a point's present configuration.
func pointConfig(db *outstation.Database, pt dnp3.PointType, index uint16) (outstation.PointConfig, bool) {
	switch pt {
	case dnp3.TypeBinary:
		_, c, ok := db.Binary(index)
		return c, ok
	case dnp3.TypeDoubleBitBinary:
		_, c, ok := db.DoubleBit(index)
		return c, ok
	case dnp3.TypeCounter:
		_, c, ok := db.Counter(index)
		return c, ok
	case dnp3.TypeFrozenCounter:
		_, c, ok := db.FrozenCounter(index)
		return c, ok
	case dnp3.TypeAnalog:
		_, c, ok := db.Analog(index)
		return c, ok
	case dnp3.TypeBinaryOutputStatus:
		_, c, ok := db.BinaryOutputStatus(index)
		return c, ok
	case dnp3.TypeAnalogOutputStatus:
		_, c, ok := db.AnalogOutputStatus(index)
		return c, ok
	case dnp3.TypeOctetString:
		_, c, ok := db.OctetString(index)
		return c, ok
	default:
		return outstation.PointConfig{}, false
	}
}

// buildOutstation sizes, configures and loads the outstation, then starts it.
func (e *Engine) buildOutstation(ctx context.Context, db *mongo.Database, conn *Connection) error {
	jslog.Log(jslog.LevelBasic, "%s - Finding tags distributed for this connection...", conn.Name)
	tags, err := tagsFor(ctx, db, conn.ProtocolConnectionNumber)
	if err != nil {
		return err
	}

	station := e.newOutstation(conn, tags)
	conn.setStation(station)

	go func() {
		if err := station.Run(ctx, conn.Chan); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelBasic, "%s - Outstation stopped: %v", conn.Name, err)
		}
	}()
	jslog.Log(jslog.LevelBasic, "%s - Outstation enabled.", conn.Name)
	return nil
}

// newOutstation sizes, configures and loads an outstation from the tags
// distributed on a connection. It does not start it, so the whole construction
// can be exercised without a database.
func (e *Engine) newOutstation(conn *Connection, tags []bson.M) *outstation.Session {
	cfg := sizeDatabase(conn, tags)
	conn.counts = cfg
	jslog.Log(jslog.LevelBasic,
		"%s - Outstation created with %d binary inputs, %d double binary inputs, %d analog inputs, "+
			"%d counters, %d frozen counters, %d binary output statuses, %d analog output statuses, "+
			"%d octet strings",
		conn.Name, cfg.Binary, cfg.DoubleBitBinary, cfg.Analog, cfg.Counter, cfg.FrozenCounter,
		cfg.BinaryOutputStatus, cfg.AnalogOutputStatus, cfg.OctetString)

	ocfg := outstation.Config{
		LocalAddr:      uint16(conn.LocalLinkAddress),
		RemoteAddr:     uint16(conn.RemoteLinkAddress),
		Database:       cfg,
		Events:         outstation.EventBufferConfig{MaxEvents: conn.ServerQueueSize},
		MaxTxFragment:  maxTxFragment,
		ConfirmTimeout: confirmTimeout,
		SelectTimeout:  selectTimeout,
		Unsolicited: outstation.UnsolicitedConfig{
			Enabled:        conn.EnableUnsolicited,
			HoldTime:       unsolHoldTime,
			MaxEvents:      unsolMaxEvents,
			ConfirmTimeout: unsolConfirmWait,
			MaxRetries:     unsolMaxRetries,
		},
		// Group 0 identity. The library derives the point counts and the
		// fragment sizes from this same config, so they are not repeated here.
		Attributes: deviceAttributes(conn),
		Log:        jslog.NewStackLogger(conn.Name),
	}
	if conn.ConnectionMode == "SERIAL" {
		ocfg.UseLinkConfirms = true
		ocfg.LinkRetries = linkRetries
		ocfg.LinkTimeout = linkTimeout
	}

	// A connection with commands disabled gets no command handler, which makes
	// the library install its RejectingCommandHandler and answer NOT_SUPPORTED
	// to every control — the same result the C++ server produces.
	var handler outstation.CommandHandler
	if conn.CommandsEnabled {
		handler = &commandHandler{engine: e, conn: conn}
	}

	station := outstation.New(ocfg, application{conn: conn}, handler)

	// Point configuration and the initial load happen before Run, so nothing is
	// on the wire yet; the event buffer is then cleared, which is what
	// EventMode::Suppress achieves in the C++ server.
	configurePoints(station.Database(), conn, tags, cfg)
	for _, doc := range tags {
		for _, d := range destinationsFor(doc, conn.ProtocolConnectionNumber) {
			ApplyValue(station.Database(), doc, d, conn)
		}
	}
	station.Events().Reset()

	return station
}

// reloadIntegrity re-reads every distributed tag and applies it, which is what
// the C++ server does after a change-stream reconnection so that changes missed
// during the outage are not lost.
func (e *Engine) reloadIntegrity(ctx context.Context, db *mongo.Database, conn *Connection) {
	tags, err := tagsFor(ctx, db, conn.ProtocolConnectionNumber)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s - Cannot reload integrity data: %v", conn.Name, err)
		return
	}
	jslog.Log(jslog.LevelBasic, "%s - Store integrity data.", conn.Name)

	conn.Update(func(d *outstation.Database) {
		for _, doc := range tags {
			for _, dest := range destinationsFor(doc, conn.ProtocolConnectionNumber) {
				ApplyValue(d, doc, dest, conn)
			}
		}
	})
}
