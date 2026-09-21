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

// Class scans, range scans and time synchronisation. Ports of the AddClassScan
// and AddRangeScan block of configureMaster(), and of the timeSyncMode setting
// opendnp3 handles inside its master.

package clientapp

import (
	"context"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
)

// startScans starts the goroutines that keep a connection's polls running: one
// that registers the class scans on every connection, and one per range scan.
func (e *Engine) startScans(ctx context.Context, conn *Connection, session *master.Session) {
	go classScanLoop(ctx, conn, session)

	for _, rs := range conn.RangeScans {
		if rs.Period <= 0 {
			continue
		}
		go rangeScanLoop(ctx, conn, session, rs)
	}
}

// classScanLoop registers the periodic class polls each time the session
// connects.
//
// They cannot be registered once: the stack runs its startup sequence on every
// connection, and that begins by clearing the task scheduler — so a scan
// registered beforehand is dropped, and after the first reconnection the
// session would poll nothing but its own startup integrity read. Registering on
// each rising edge of Connected() is what keeps the configured intervals in
// force for the life of the driver.
func classScanLoop(ctx context.Context, conn *Connection, session *master.Session) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()

	wasConnected := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		connected := session.Connected()
		if connected && !wasConnected {
			registerClassScans(ctx, conn, session)
		}
		wasConnected = connected
	}
}

// registerClassScans queues the configured class polls on the session.
func registerClassScans(ctx context.Context, conn *Connection, session *master.Session) {
	addClass := func(seconds int, mask dnp3.Class, what string) {
		if seconds <= 0 {
			return
		}
		if err := session.AddPeriodicScan(ctx, time.Duration(seconds)*time.Second, mask); err != nil {
			jslog.Log(jslog.LevelBasic, "%s - Cannot schedule the %s scan: %v", conn.Name, what, err)
			return
		}
		jslog.Log(jslog.LevelDetailed, "%s - %s scan every %d s", conn.Name, what, seconds)
	}

	addClass(conn.GIInterval, dnp3.ClassAll, "integrity")
	addClass(conn.Class0ScanInterval, dnp3.Class0, "class 0")
	addClass(conn.Class1ScanInterval, dnp3.Class1, "class 1")
	addClass(conn.Class2ScanInterval, dnp3.Class2, "class 2")
	addClass(conn.Class3ScanInterval, dnp3.Class3, "class 3")
}

// rangeScanLoop polls one configured range.
//
// The library's AddPeriodicScan only takes class masks, so a periodic range
// scan is ours to schedule. It reads once before waiting, matching opendnp3's
// AddRangeScan, which runs its first scan immediately.
func rangeScanLoop(ctx context.Context, conn *Connection, session *master.Session, rs RangeScan) {
	t := time.NewTicker(time.Duration(rs.Period) * time.Second)
	defer t.Stop()

	for {
		if err := session.ScanRange(ctx, uint8(rs.Group), uint8(rs.Variation),
			uint16(rs.StartAddress), uint16(rs.StopAddress)); err != nil {
			if ctx.Err() != nil {
				return
			}
			jslog.Log(jslog.LevelDetailed, "%s - Range scan g%dv%d %d..%d failed: %v",
				conn.Name, rs.Group, rs.Variation, rs.StartAddress, rs.StopAddress, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// timeSyncLoop writes the outstation's clock on every reconnection and then
// periodically.
//
// opendnp3 syncs when the outstation raises NEED_TIME. go-dnp3 tracks that
// indication internally but never acts on it, and app.IIN cannot be named from
// outside the module to test it, so the sync is scheduled instead: on connect,
// then every timeSyncPeriod while the link is up (deviation D3).
func (e *Engine) timeSyncLoop(ctx context.Context, conn *Connection, session *master.Session) {
	lan := conn.TimeSyncMode >= 2

	t := time.NewTicker(time.Second)
	defer t.Stop()

	wasConnected := false
	lastSync := time.Time{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		connected := session.Connected()
		if !connected {
			wasConnected = false
			continue
		}

		justConnected := !wasConnected
		wasConnected = true
		if justConnected {
			// Let the startup sequence finish first; a sync landing in the
			// middle of it would be queued behind the integrity poll anyway.
			lastSync = time.Now().Add(-timeSyncPeriod).Add(2 * time.Second)
			continue
		}
		if time.Since(lastSync) < timeSyncPeriod {
			continue
		}
		lastSync = time.Now()

		sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		var err error
		if lan {
			err = session.SyncTime(sctx)
		} else {
			err = session.SyncTimeWithDelay(sctx)
		}
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			jslog.Log(jslog.LevelDetailed, "%s - Time sync failed: %v", conn.Name, err)
			continue
		}
		mode := "non-LAN"
		if lan {
			mode = "LAN"
		}
		jslog.Log(jslog.LevelDetailed, "%s - Time synchronised (%s).", conn.Name, mode)
	}
}
