/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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

// Per-IED connection state machine: the port of Process(Iec61850Connection)
// from the C# driver.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// active tells whether this node is the active one of the redundant pair.

// connectionLoop keeps one association to an IED alive, reconnecting for as
// long as the driver runs.
func connectionLoop(ctx context.Context, conn *Iec61850Connection) {
	for {
		if ctx.Err() != nil {
			return
		}
		if !redundancy.Active() {
			time.Sleep(1 * time.Second)
			continue
		}

		err := runConnection(ctx, conn)
		jslog.Log(jslog.LevelBasic, "%sException", conn.Name)
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%v", err)
		}
		closeConnection(conn)

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// requestTimeout is the per-request budget, from the connection's timeoutMs.
func (c *Iec61850Connection) requestTimeout() time.Duration {
	if c.TimeoutMs <= 0 {
		return 20 * time.Second
	}
	return time.Duration(c.TimeoutMs) * time.Millisecond
}

// splitHostPort resolves the endpoint of a connection. Only the first entry
// of ipAddresses is used, as in the C# driver; the default port is 102.
func splitHostPort(addr string) string {
	addr = strings.TrimSpace(addr)
	if host, port, err := net.SplitHostPort(addr); err == nil {
		if _, err := strconv.Atoi(port); err == nil {
			return net.JoinHostPort(host, port)
		}
	}
	return net.JoinHostPort(strings.Trim(addr, "[]"), "102")
}

// runConnection establishes the association, discovers the model, activates
// reports and then polls the entries that no report covers. It returns when
// the association drops or this node goes inactive.
func runConnection(ctx context.Context, conn *Iec61850Connection) error {
	opts := []client.Option{client.WithTimeout(conn.requestTimeout())}
	if conn.UseSecurity {
		jslog.Log(jslog.LevelBasic, "%s Using TLS", conn.Name)
		tlsCfg, err := buildTLS(conn)
		if err != nil {
			return fmt.Errorf("%s TLS configuration error: %w", conn.Name, err)
		}
		opts = append(opts, client.WithTLS(tlsCfg))
	}
	if conn.Password != "" {
		opts = append(opts, client.WithPassword(conn.Password))
	}
	if lg := libLogger(); lg != nil {
		opts = append(opts, client.WithLogger(lg))
	}

	addr := splitHostPort(conn.IPAddresses[0])
	jslog.Log(jslog.LevelBasic, "%s Connecting to %s", conn.Name, addr)

	dialCtx, cancelDial := context.WithTimeout(ctx, conn.requestTimeout())
	cli, err := client.Dial(dialCtx, addr, opts...)
	cancelDial()
	if err != nil {
		return fmt.Errorf("%s Connection error: %w", conn.Name, err)
	}
	conn.SetClient(cli)

	idCtx, cancelID := context.WithTimeout(ctx, conn.requestTimeout())
	vendor, modelName, revision, err := cli.MMS().Identify(idCtx)
	cancelID()
	if err == nil {
		jslog.Log(jslog.LevelBasic, "Vendor:   %s", vendor)
		jslog.Log(jslog.LevelBasic, "Model:    %s", modelName)
		jslog.Log(jslog.LevelBasic, "Revision: %s", revision)
	} else {
		jslog.Log(jslog.LevelDetailed, "%s Identify not supported: %v", conn.Name, err)
	}

	conn.Datasets = nil
	conn.Brcb = nil
	conn.Urcb = nil
	conn.TakeSubscriptions()
	conn.mu.Lock()
	conn.RcbByRptID = map[string]*rcbState{}
	conn.RcbByDataSet = map[string]*rcbState{}
	conn.mu.Unlock()

	removeDiag := installReportDiagnostics(conn)
	defer removeDiag()

	if err := discoverServer(ctx, conn); err != nil {
		return err
	}

	return pollLoop(ctx, conn)
}

// pollLoop reads the entries not covered by a report, then waits giInterval
// seconds while watching for the association dropping or this node going
// inactive — the port of the C# read/wait loop.
func pollLoop(ctx context.Context, conn *Iec61850Connection) error {
	cli := conn.Client()
	for {
		if err := pollSweep(ctx, conn); err != nil {
			return err
		}

		wait := time.Duration(conn.GiInterval * float64(time.Second))
		if wait <= 0 {
			wait = time.Second
		}
		deadline := time.After(wait)
		tick := time.NewTicker(100 * time.Millisecond)
		for waiting := true; waiting; {
			select {
			case <-ctx.Done():
				tick.Stop()
				return ctx.Err()
			case <-cli.Done():
				tick.Stop()
				return fmt.Errorf("%s Connection error detected! %v", conn.Name, cli.Err())
			case <-deadline:
				waiting = false
			case <-tick.C:
				if !redundancy.Active() {
					tick.Stop()
					return fmt.Errorf("%s Node inactive! Disconnecting ...", conn.Name)
				}
				if cli.State() != mms.StateConnected {
					tick.Stop()
					return fmt.Errorf("%s Connection error detected!", conn.Name)
				}
			}
		}
		tick.Stop()
	}
}

// pollSweep issues one read per entry that no report covers. The reads are
// issued together and collected afterwards: the library holds the number
// actually outstanding to what the association negotiated, which is the
// back-pressure the C# driver was getting from its retry-on-error-6 loop.
func pollSweep(ctx context.Context, conn *Iec61850Connection) error {
	cli := conn.Client()

	conn.mu.Lock()
	order := make([]string, len(conn.EntryOrder))
	copy(order, conn.EntryOrder)
	conn.mu.Unlock()

	// The reads run concurrently up to the association's outstanding limit,
	// so the sweep needs the per-request budget once per batch, not once
	// for the whole sweep.
	outstanding := cli.MMS().MaxServOutstanding()
	if outstanding < 1 {
		outstanding = 1
	}
	batches := (len(order) + outstanding - 1) / outstanding
	if batches < 1 {
		batches = 1
	}
	readCtx, cancel := context.WithTimeout(ctx, conn.requestTimeout()*time.Duration(batches))
	defer cancel()

	type pending struct {
		entry *Iec61850Entry
		req   *client.ReadRequest
	}
	var reqs []pending

	for i, key := range order {
		entry := conn.Entry(key)
		if entry == nil || conn.EntryHasReport(entry) { // in a report: not polled
			continue
		}
		if entry.FC == model.CO {
			// A control object is an output: reading it returns the operate
			// structure, not a measurement. Its state is the supervised
			// twin, which is polled or reported on its own.
			continue
		}
		tag := entry.JsTag
		if tag == "" {
			tag = entry.Path
		}
		jslog.Log(jslog.LevelBasic, "%s Async Reading %s %s ind:%d", conn.Name, entry.Path, entry.FC, i+1)
		reqs = append(reqs, pending{
			entry: entry,
			req:   cli.ReadAsync(readCtx, model.ObjectReference(entry.Path), entry.FC),
		})
	}

	for _, p := range reqs {
		value, err := p.req.Result()
		tag := p.entry.JsTag
		if tag == "" {
			tag = p.entry.Path
		}
		if err != nil {
			if isAssociationDown(cli, err) {
				return fmt.Errorf("%s Connection error detected! %v", conn.Name, err)
			}
			jslog.Log(jslog.LevelBasic, "%s READED  %s %s\n    Read error: %v", conn.Name, p.entry.Path, tag, err)
			continue
		}
		var log strings.Builder
		if jslog.Level() > jslog.LevelNoLog {
			fmt.Fprintf(&log, "%s READED  %s %s", conn.Name, p.entry.Path, tag)
		}
		// A point the driver discovered itself publishes its tag; a point
		// configured in realtimeData already has one.
		selfPublish := conn.AutoCreateTags && p.entry.AutoPublish
		iv, _ := buildIECValue(conn, p.entry, value, selfPublish, false, &log)
		enqueueValue(iv)
		jslog.Log(jslog.LevelBasic, "%s", log.String())
	}
	return nil
}

// isAssociationDown separates a dead link from a rejected point. A
// data-access error or a service error means the point is bad; anything the
// transport reports, or a client whose state already dropped, means the
// association is gone.
func isAssociationDown(cli *client.Client, err error) bool {
	if err == nil {
		return false
	}
	var dae mms.DataAccessError
	if errors.As(err, &dae) {
		return false
	}
	var se *mms.ServiceError
	if errors.As(err, &se) {
		return false
	}
	return cli.State() != mms.StateConnected
}

// closeConnection releases report subscriptions and the association.
func closeConnection(conn *Iec61850Connection) {
	cli := conn.Client()
	if cli == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	for _, sub := range conn.TakeSubscriptions() {
		_ = sub.Disable(ctx)
	}
	cancel()
	_ = cli.Close()
	conn.SetClient(nil)
}
