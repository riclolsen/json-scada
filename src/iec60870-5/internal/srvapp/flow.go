/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - send flow control
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
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

package srvapp

import (
	"errors"
	"sync"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs101"
	"github.com/riclolsen/go-iecp5/cs104"
)

// sendBufferWait bounds how long one ASDU waits for room in a full send
// buffer. A 104 session drains as the master acknowledges (a dead one is
// closed by t1), a 101 server as the master polls.
const sendBufferWait = 10 * time.Second

// sendRetryDelay is the pause between attempts on a full send buffer.
const sendRetryDelay = 2 * time.Millisecond

// isBufferFull reports the go-iecp5 "no room in the send queue" errors.
func isBufferFull(err error) bool {
	return errors.Is(err, cs104.ErrBufferFulled) || errors.Is(err, cs101.ErrSendQueueFull)
}

// flowControl waits for room in a full send buffer instead of dropping the
// ASDU (go-iecp5 Send never blocks). It waits only on a link that accepted
// an ASDU within the last sendBufferWait: a 104 session that never sent
// STARTDT (a standby connection of a redundancy group) has a buffer that
// never drains, and waiting on it would stall every other master.
type flowControl struct {
	mu     sync.Mutex
	lastOK map[asdu.Connect]time.Time
}

func (f *flowControl) markOK(c asdu.Connect) {
	f.mu.Lock()
	if f.lastOK == nil {
		f.lastOK = map[asdu.Connect]time.Time{}
	}
	f.lastOK[c] = time.Now()
	f.mu.Unlock()
}

func (f *flowControl) draining(c asdu.Connect) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return time.Since(f.lastOK[c]) < sendBufferWait
}

// forget drops the state of a closed link.
func (f *flowControl) forget(c asdu.Connect) {
	f.mu.Lock()
	delete(f.lastOK, c)
	f.mu.Unlock()
}

// send sends one ASDU on c, waiting for buffer room as described above.
func (f *flowControl) send(c asdu.Connect, a *asdu.ASDU) error {
	deadline := time.Now().Add(sendBufferWait)
	for {
		err := c.Send(a)
		if err == nil {
			f.markOK(c)
			return nil
		}
		if !isBufferFull(err) || !f.draining(c) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(sendRetryDelay)
	}
}

// flowConn is an asdu.Connect whose Send goes through the flow control, so
// the asdu builders and SendReplyMirror wait for room too.
type flowConn struct {
	asdu.Connect
	f *flowControl
}

func (w flowConn) Send(a *asdu.ASDU) error { return w.f.send(w.Connect, a) }

// reply wraps a link for sending with flow control.
func (e *Engine) reply(c asdu.Connect) asdu.Connect {
	if _, ok := c.(flowConn); ok {
		return c
	}
	return flowConn{Connect: c, f: &e.flow}
}

// ForgetLink drops the flow-control and select state of a closed link
// (call from the connection-lost handler).
func (e *Engine) ForgetLink(srv *Conn, c asdu.Connect) {
	e.flow.forget(c)
	srv.clearSelect(c)
}
