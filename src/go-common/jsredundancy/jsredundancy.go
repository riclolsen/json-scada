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

// Package jsredundancy arbitrates which node of an instance runs the protocol.
//
// # The takeover rule
//
// One algorithm, the port of Redundancy.cs: a standby node reads the active
// node's activeNodeKeepAliveTimeTag every TickPeriod and counts how many times
// it comes back UNCHANGED. After more than KeepAliveCountLimit unchanged
// readings it takes over. Nothing is compared against the local clock, so the
// arbitration is immune to clock skew between the two nodes — which is the
// reason this method was chosen over the wall-clock staleness test the C++
// drivers used.
//
// Effective takeover latency is therefore (KeepAliveCountLimit+1) * TickPeriod,
// about 25-30 s, against the ~15 s of the wall-clock method dnp3-go used
// before unification. That is the deliberate trade: slower failover, no
// dependence on the two nodes agreeing about the time.
//
// # Fixed: stalls must be consecutive
//
// Redundancy.cs only ever incremented the counter — it was cleared when a
// node activated or deactivated, but never when the active node's keep-alive
// started advancing again. Separate short stalls therefore accumulated, and a
// live-but-flaky active node was eventually taken over even though it had
// never actually stopped. Every Go driver inherited that.
//
// The counter is now reset whenever the keep-alive advances, so only
// KeepAliveCountLimit+1 CONSECUTIVE unchanged readings trigger a takeover.
// This is strictly more conservative — it makes spurious failovers less
// likely, never more — and it does not change how fast a genuinely dead node
// is replaced, because a dead node's keep-alive never advances.
//
// The behaviour differs from the C# drivers here on purpose. If a mixed pair
// ever runs one of each, the Go node is the one less willing to take over.
//
// # What the active flag gates
//
// Two independent axes, both preserved:
//
//   - Command execution. Every client gates it on Active(). That is universal.
//   - Protocol sessions. A driver that stops its sessions on standby supplies
//     OnActivate and OnDeactivate; one that keeps acquiring on both nodes
//     (the OPC-UA and IEC 60870-5 clients, by design) leaves them nil.
//
// Servers do not arbitrate at all and never construct a Controller.
package jsredundancy

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmodel"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	// TickPeriod is how often the arbitration runs.
	TickPeriod = 5 * time.Second
	// KeepAliveCountLimit is how many consecutive unchanged readings of the
	// active node's keep-alive a standby tolerates before taking over.
	KeepAliveCountLimit = 4
	// RetryPeriod is the pause after a MongoDB failure.
	RetryPeriod = 3 * time.Second
)

// yieldDelay is the random pause a node takes when it gives up being active,
// so that two nodes which lost sight of each other do not flip in lockstep.
func yieldDelay() time.Duration {
	return time.Duration(1000+rand.Intn(4000)) * time.Millisecond
}

// stallCounter is the takeover rule itself: it counts CONSECUTIVE readings
// of the active node's keep-alive that came back unchanged.
//
// It is a type of its own so the loop and its tests share one implementation
// — the rule is the part of this package that must not drift.
type stallCounter struct {
	last  time.Time
	count int
}

// observe records one reading and reports whether the standby should take
// over.
//
// The reset on an advance is the fix to the inherited Redundancy.cs quirk:
// that version only ever incremented, so separate short stalls accumulated
// and a live-but-flaky active node was eventually displaced.
func (s *stallCounter) observe(keepAlive time.Time) (takeOver bool) {
	if s.last.Equal(keepAlive) {
		s.count++
	} else {
		s.count = 0
	}
	s.last = keepAlive
	return s.count > KeepAliveCountLimit
}

// reset forgets the accumulated stall, which happens whenever this node
// changes state.
func (s *stallCounter) reset() { s.count = 0 }

// Controller arbitrates the active node of one driver instance.
type Controller struct {
	Config         jsconfig.Config
	DriverName     string
	InstanceNumber int

	// OnActivate and OnDeactivate are supplied only by a driver that starts
	// and stops its protocol sessions with the active flag. Leaving them nil
	// is the commands-only behaviour: acquisition runs on both nodes.
	// Neither is called for the state the driver is already in.
	OnActivate   func()
	OnDeactivate func()

	// OnTick is called on every cycle while active, after the keep-alive is
	// written, with a live database handle. Drivers use it to publish the
	// per-connection statistics.
	OnTick func(db *mongo.Database)

	// StatusSuffix appends driver-specific text to the "this node is active"
	// line — the OPC-UA client reports its notification and lost-update
	// counters there.
	StatusSuffix func() string

	// MissingInstanceActive decides what happens when the instance document
	// does not exist at all.
	//
	// The C# drivers stay inactive, which is what a redundant pair wants: an
	// instance nobody configured must not start acquiring. The C++ drivers
	// defaulted to active so a single-node system with an incomplete
	// configuration still ran, and dnp3-go inherited that. Set it to keep
	// that bootstrap behaviour; it does not affect the takeover rule, which
	// is the same either way.
	MissingInstanceActive bool

	// OnNodeNotAllowed is called when this node is not listed in nodeNames.
	// Nil terminates the process, which is what every driver did.
	OnNodeNotAllowed func()

	active atomic.Bool
}

// Active reports whether this node currently runs the protocol.
func (c *Controller) Active() bool { return c.active.Load() }

// ForceActive sets the flag without arbitrating.
//
// For tests, which need a driver to behave as the active node without a
// MongoDB instance document to arbitrate over. A running Run will overwrite
// it on its next cycle, so it is not a way to pin a node active in
// production.
func (c *Controller) ForceActive(v bool) { c.active.Store(v) }

// Run arbitrates until the context is cancelled.
func (c *Controller) Run(ctx context.Context) {
	for ctx.Err() == nil {
		cli, db, err := jsmongo.ConnectAndPing(c.Config)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Redundancy - Mongo connection error: %v", err)
			sleep(ctx, RetryPeriod)
			continue
		}
		c.loop(ctx, db)
		_ = cli.Disconnect(context.Background())
		sleep(ctx, RetryPeriod)
	}
}

// loop runs the arbitration on one database handle until it fails.
func (c *Controller) loop(ctx context.Context, db *mongo.Database) {
	collInsts := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName)

	var stalls stallCounter

	for ctx.Err() == nil {
		if err := jsmongo.Ping(db, 1*time.Second); err != nil {
			jslog.Log(jslog.LevelNoLog, "Redundancy - Error on MongoDB connection")
			return
		}

		findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		var doc bson.M
		err := collInsts.FindOne(findCtx, jsmodel.InstanceFilter(c.DriverName, c.InstanceNumber)).Decode(&doc)
		cancel()

		if err != nil {
			c.noInstance(ctx, &stalls)
			if !sleep(ctx, TickPeriod) {
				return
			}
			continue
		}

		inst := jsmodel.InstanceFromDoc(doc)
		if !jsmodel.NodeAllowed(inst, c.Config.NodeName) {
			if c.OnNodeNotAllowed != nil {
				c.OnNodeNotAllowed()
				return
			}
			jslog.Fatal("Node '%s' not found in instances configuration!", c.Config.NodeName)
		}

		if inst.ActiveNodeName == c.Config.NodeName {
			c.setActive(true, "Redundancy - ACTIVATING this Node!")
			stalls.reset()
		} else {
			if c.active.Load() {
				// Yielding: pause a random time so two nodes that lost sight
				// of each other do not flip together.
				c.setActive(false, "Redundancy - DEACTIVATING this Node (other node active)!")
				stalls.reset()
				sleep(ctx, yieldDelay())
			}
			// The takeover rule. Never a comparison against the local clock.
			if stalls.observe(inst.ActiveNodeKeepAliveTimeTag) {
				c.setActive(true, "Redundancy - ACTIVATING this Node!")
			}
		}

		if c.active.Load() {
			suffix := ""
			if c.StatusSuffix != nil {
				suffix = c.StatusSuffix()
			}
			jslog.Log(jslog.LevelNoLog, "Redundancy - This node is active.%s", suffix)
			c.writeKeepAlive(ctx, collInsts)
			if c.OnTick != nil {
				c.OnTick(db)
			}
		} else if inst.ActiveNodeName != "" {
			jslog.Log(jslog.LevelNoLog,
				"Redundancy - This node is INACTIVE! Node '%s' is active, wait...", inst.ActiveNodeName)
		} else {
			jslog.Log(jslog.LevelNoLog, "Redundancy - This node is INACTIVE! No node is active, wait...")
		}

		if !sleep(ctx, TickPeriod) {
			return
		}
	}
}

// noInstance handles a missing instance document.
func (c *Controller) noInstance(ctx context.Context, stalls *stallCounter) {
	if c.MissingInstanceActive {
		c.setActive(true, "Redundancy - ACTIVATING this Node (no instance found)!")
		return
	}
	if c.active.Load() {
		c.setActive(false, "Redundancy - DEACTIVATING this Node (no instance found)!")
		stalls.reset()
		sleep(ctx, yieldDelay())
	}
}

// setActive flips the flag, logging and calling the callback only on a real
// transition.
func (c *Controller) setActive(want bool, msg string) {
	if c.active.Swap(want) == want {
		return
	}
	jslog.Log(jslog.LevelNoLog, "%s", msg)
	if want {
		if c.OnActivate != nil {
			c.OnActivate()
		}
		return
	}
	if c.OnDeactivate != nil {
		c.OnDeactivate()
	}
}

// writeKeepAlive claims the instance for this node.
func (c *Controller) writeKeepAlive(ctx context.Context, collInsts *mongo.Collection) {
	updCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := collInsts.UpdateOne(updCtx,
		jsmodel.InstanceFilter(c.DriverName, c.InstanceNumber),
		bson.M{"$set": bson.M{
			"activeNodeName":             c.Config.NodeName,
			"activeNodeKeepAliveTimeTag": jsmongo.Now(),
		}})
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "Redundancy - Error updating keep alive: %v", err)
	}
}

// sleep waits, reporting false when the context ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
