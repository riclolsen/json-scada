/*
 * DNP3 Client and Server Protocol drivers for {json:scada}, in Go.
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

// A channel.Channel decorator supplying the byte counters of opendnp3's
// ChannelStatistics, the allowed-client-address list of a passive connection,
// and the serial asyncOpenDelay. Everything frame-shaped comes from
// multidrop.Bus.Stats() instead.

package dnp3util

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-dnp3/channel"
)

// Retry is the backoff between connection attempts.
//
// The library's channels take a retry policy of their own and loop inside
// Connect, which means a caller sees one error at the end of the whole cycle —
// and, because the loop ends on the context, that error is the context's, not
// the port's. An operator asking why COM3 will not open would be told
// "context deadline exceeded". Retrying out here instead makes every attempt
// visible: each failure is counted into numOpenFail, as opendnp3 counted them,
// and logged with the reason the open actually failed.
type Retry struct {
	Min    time.Duration
	Max    time.Duration
	Factor float64
	// Jitter is the fraction of the delay to randomise, from 0 to 1. A
	// substation that loses a switch brings every connection down at the same
	// instant; without jitter they all retry in lockstep.
	Jitter float64
}

// DefaultRetry backs off from half a second to a minute, matching the
// library's own default.
var DefaultRetry = Retry{Min: 500 * time.Millisecond, Max: 60 * time.Second, Factor: 2, Jitter: 0.2}

// delay returns the wait before attempt n, counting from zero.
func (r Retry) delay(n int) time.Duration {
	if r.Min <= 0 {
		return 0
	}
	d := float64(r.Min)
	for range n {
		d *= r.Factor
		if r.Max > 0 && d >= float64(r.Max) {
			d = float64(r.Max)
			break
		}
	}
	if r.Jitter > 0 {
		d *= 1 + r.Jitter*(2*rand.Float64()-1)
	}
	return time.Duration(d)
}

// Counters are the physical-stream statistics of one channel.
type Counters struct {
	bytesRx   atomic.Uint64
	bytesTx   atomic.Uint64
	opens     atomic.Uint64
	closes    atomic.Uint64
	openFails atomic.Uint64
}

// CountersSnapshot is a consistent-enough read of the counters for a statistics
// document; the individual loads are atomic but the set is not.
type CountersSnapshot struct {
	BytesRx   uint64
	BytesTx   uint64
	Opens     uint64
	Closes    uint64
	OpenFails uint64
}

// Snapshot reads the counters.
func (c *Counters) Snapshot() CountersSnapshot {
	if c == nil {
		return CountersSnapshot{}
	}
	return CountersSnapshot{
		BytesRx:   c.bytesRx.Load(),
		BytesTx:   c.bytesTx.Load(),
		Opens:     c.opens.Load(),
		Closes:    c.closes.Load(),
		OpenFails: c.openFails.Load(),
	}
}

// CountOptions parameterises the decorator.
type CountOptions struct {
	// Name is used in log lines.
	Name string
	// AllowedRemoteIPs, when not empty, restricts which peers may connect. It
	// is the ipAddresses list of a passive connection: opendnp3 has no such
	// filter either, but the server driver documents the field as one.
	AllowedRemoteIPs []string
	// OpenDelay is applied after a connection is established, before anything
	// is transmitted. It is the serial asyncOpenDelay, which go-dnp3's
	// SerialConfig does not carry.
	OpenDelay time.Duration
	// Retry is the backoff between attempts. The zero value tries once and
	// gives up, which is what a passive channel wants: its Connect blocks on
	// accept rather than failing.
	Retry Retry
}

type countChannel struct {
	// inners are the alternative endpoints of one connection, in the order
	// ipAddresses gives them. Every mode but an active one has exactly one.
	inners   []channel.Channel
	opts     CountOptions
	counters *Counters
	allowed  map[string]bool

	// idx is the endpoint to try next. It advances on every failed attempt and
	// stays put on a success, so a connection settles on whichever address is
	// answering and only moves when that one stops.
	mu  sync.Mutex
	idx int
}

// next returns the endpoint to try and advances past it.
func (c *countChannel) next() (channel.Channel, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	i := c.idx
	return c.inners[i], i
}

// advance moves to the endpoint after a failed attempt.
func (c *countChannel) advance() {
	c.mu.Lock()
	c.idx = (c.idx + 1) % len(c.inners)
	c.mu.Unlock()
}

// WrapCounting decorates a single channel. Most modes have exactly one
// endpoint.
func WrapCounting(inner channel.Channel, opts CountOptions) (channel.Channel, *Counters) {
	return WrapCountingAll([]channel.Channel{inner}, opts)
}

// WrapCountingAll decorates the alternative endpoints of one connection with
// the counters, the address filter and the open delay.
//
// The endpoints are tried in turn: an attempt that fails moves to the next, and
// the backoff is applied once the whole list has been tried, so a device with a
// second address fails over in the time one dial takes rather than waiting out
// a retry delay first. A successful connection stays where it is.
func WrapCountingAll(inners []channel.Channel, opts CountOptions) (channel.Channel, *Counters) {
	c := &countChannel{inners: inners, opts: opts, counters: &Counters{}}
	if len(opts.AllowedRemoteIPs) > 0 {
		c.allowed = make(map[string]bool, len(opts.AllowedRemoteIPs))
		for _, a := range opts.AllowedRemoteIPs {
			// The field holds "host" or "host:port"; only the host is matched,
			// because an inbound connection's source port is ephemeral.
			if h := HostOf(a, ""); h != "" {
				c.allowed[h] = true
			}
		}
		if len(c.allowed) == 0 {
			c.allowed = nil
		}
	}
	return c, c.counters
}

func (c *countChannel) Connect(ctx context.Context) (io.ReadWriteCloser, error) {
	// failures counts attempts since the last success, which is what decides
	// both the endpoint to try and how long to wait once the list is exhausted.
	failures := 0
	for {
		inner, idx := c.next()
		conn, err := inner.Connect(ctx)
		if err != nil {
			// Neither of these is a connection that would not open: a context
			// error is the driver shutting down or going to standby, and a
			// closed channel is the bus being torn down. Both must propagate
			// rather than be retried — the session reads them as a clean
			// shutdown, and a closed channel will never produce a connection
			// however long this loop waits.
			if ctx.Err() != nil || errors.Is(err, channel.ErrClosed) {
				return nil, err
			}
			c.counters.openFails.Add(1)

			if c.opts.Retry.Min <= 0 {
				return nil, err
			}
			// The first failure of a run is worth an operator's attention; the
			// repeats while a device stays down are not.
			level := jslog.LevelDetailed
			if failures == 0 {
				level = jslog.LevelBasic
			}
			if len(c.inners) > 1 {
				jslog.Log(level, "%s - Connection attempt to %s failed: %v",
					c.opts.Name, inner.String(), err)
			} else {
				jslog.Log(level, "%s - Connection attempt failed: %v", c.opts.Name, err)
			}

			failures++
			c.advance()

			// Only once every endpoint has been tried is there a reason to
			// wait: until then the next address is the faster thing to do.
			if failures%len(c.inners) == 0 {
				cycles := failures/len(c.inners) - 1
				if !c.wait(ctx, c.opts.Retry.delay(cycles)) {
					return nil, ctx.Err()
				}
			}
			continue
		}
		if failures > 0 && len(c.inners) > 1 {
			jslog.Log(jslog.LevelBasic, "%s - Connected to %s", c.opts.Name, inner.String())
		}
		failures = 0
		_ = idx

		if !c.permitted(conn) {
			c.counters.closes.Add(1)
			_ = conn.Close()
			continue
		}
		c.counters.opens.Add(1)
		if c.opts.OpenDelay > 0 {
			select {
			case <-ctx.Done():
				_ = conn.Close()
				return nil, ctx.Err()
			case <-time.After(c.opts.OpenDelay):
			}
		}
		return &countConn{ch: c, inner: conn}, nil
	}
}

// wait sleeps, reporting false when the context ended first.
func (c *countChannel) wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// permitted reports whether a peer is on the allowed list. A connection with no
// network address — a pipe in a test — is always permitted.
func (c *countChannel) permitted(conn io.ReadWriteCloser) bool {
	if c.allowed == nil {
		return true
	}
	nc, ok := conn.(net.Conn)
	if !ok || nc.RemoteAddr() == nil {
		return true
	}
	host, _, err := net.SplitHostPort(nc.RemoteAddr().String())
	if err != nil {
		host = nc.RemoteAddr().String()
	}
	if c.allowed[host] {
		return true
	}
	jslog.Log(jslog.LevelBasic, "%s - Refused connection from %s: not in the allowed address list",
		c.opts.Name, host)
	return false
}

func (c *countChannel) Close() error {
	var firstErr error
	for _, inner := range c.inners {
		if err := inner.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *countChannel) String() string {
	if len(c.inners) == 1 {
		return c.inners[0].String()
	}
	parts := make([]string, 0, len(c.inners))
	for _, inner := range c.inners {
		parts = append(parts, inner.String())
	}
	return strings.Join(parts, " | ")
}

type countConn struct {
	ch     *countChannel
	inner  io.ReadWriteCloser
	closed atomic.Bool
}

func (c *countConn) Read(p []byte) (int, error) {
	n, err := c.inner.Read(p)
	if n > 0 {
		c.ch.counters.bytesRx.Add(uint64(n))
	}
	if err != nil {
		c.markClosed()
	}
	return n, err
}

func (c *countConn) Write(p []byte) (int, error) {
	n, err := c.inner.Write(p)
	if n > 0 {
		c.ch.counters.bytesTx.Add(uint64(n))
	}
	if err != nil {
		c.markClosed()
	}
	return n, err
}

func (c *countConn) Close() error {
	c.markClosed()
	return c.inner.Close()
}

// markClosed counts a disconnection once, whether it was noticed as a read
// error, a write error, or an explicit close.
func (c *countConn) markClosed() {
	if c.closed.CompareAndSwap(false, true) {
		c.ch.counters.closes.Add(1)
	}
}
