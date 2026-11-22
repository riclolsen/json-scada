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

// Tracking whether one session's link is up.
//
// A master has Session.Connected(); an outstation has nothing equivalent,
// because it answers whoever polls it rather than owning a connection. Deriving
// it from traffic gets it wrong on a slow poll cycle — the JSON-SCADA default
// integrity interval is 300 seconds, so an idle-looking station is normal. What
// is actually wanted is the state of the connection the session is using, which
// is visible right here, on the channel handed to it.

package dnp3util

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/secevent"
)

// LinkState reports whether the session on a channel currently has a
// connection.
type LinkState struct {
	up   atomic.Bool
	hook atomic.Pointer[func(up bool)]
}

// SetHook sets the function told when the session's link comes up and when it
// goes down. It runs on the session's goroutine, so it must not block.
func (l *LinkState) SetHook(f func(up bool)) {
	if l != nil {
		l.hook.Store(&f)
	}
}

func (l *LinkState) notify(up bool) {
	if f := l.hook.Load(); f != nil {
		(*f)(up)
	}
}

// Up reports whether the link is currently established.
func (l *LinkState) Up() bool {
	if l == nil {
		return false
	}
	return l.up.Load()
}

type linkStateChannel struct {
	inner channel.Channel
	state *LinkState
}

// WrapLinkState decorates a channel so that the connection state of the session
// running on it can be read.
func WrapLinkState(inner channel.Channel) (channel.Channel, *LinkState) {
	c := &linkStateChannel{inner: inner, state: &LinkState{}}
	return c, c.state
}

func (c *linkStateChannel) Connect(ctx context.Context) (io.ReadWriteCloser, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		c.state.up.Store(false)
		return nil, err
	}
	c.state.up.Store(true)
	c.state.notify(true)
	return &linkStateConn{ch: c, inner: conn}, nil
}

func (c *linkStateChannel) Close() error {
	c.state.up.Store(false)
	return c.inner.Close()
}

func (c *linkStateChannel) String() string { return c.inner.String() }

type linkStateConn struct {
	ch     *linkStateChannel
	inner  io.ReadWriteCloser
	closed atomic.Bool
}

// PeerInfo implements secevent.PeerInfoer for the connection underneath.
func (c *linkStateConn) PeerInfo() secevent.PeerInfo { return secevent.PeerOf(c.inner) }

func (c *linkStateConn) Read(p []byte) (int, error) {
	n, err := c.inner.Read(p)
	if err != nil {
		c.markDown()
	}
	return n, err
}

func (c *linkStateConn) Write(p []byte) (int, error) {
	n, err := c.inner.Write(p)
	if err != nil {
		c.markDown()
	}
	return n, err
}

func (c *linkStateConn) Close() error {
	c.markDown()
	return c.inner.Close()
}

// markDown clears the state once. A session that has already reconnected must
// not be marked down by the late failure of the connection it abandoned, so the
// flag is only cleared by the first failure of this particular connection.
func (c *linkStateConn) markDown() {
	if c.closed.CompareAndSwap(false, true) {
		c.ch.state.up.Store(false)
		c.ch.state.notify(false)
	}
}
