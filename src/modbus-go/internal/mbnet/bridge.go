/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbnet

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ParseHostPort splits "host:port", "[v6]:port", "[v6]" or "host". A missing
// port takes defaultPort.
func ParseHostPort(s string, defaultPort int) (string, int) {
	str := strings.TrimSpace(s)
	if strings.HasPrefix(str, "[") {
		if i := strings.Index(str, "]"); i > 0 {
			host := str[1:i]
			rest := str[i+1:]
			if strings.HasPrefix(rest, ":") {
				if p, err := strconv.Atoi(rest[1:]); err == nil {
					return host, p
				}
			}
			return host, defaultPort
		}
	}
	if i := strings.LastIndex(str, ":"); i >= 0 && strings.Index(str, ":") == i {
		p, err := strconv.Atoi(str[i+1:])
		if err != nil {
			p = defaultPort
		}
		return str[:i], p
	}
	return str, defaultPort
}

// JoinHostPort is net.JoinHostPort with an int port.
func JoinHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// DialFunc opens the real link to a device.
type DialFunc func(ctx context.Context) (net.Conn, error)

// Bridge is a loopback TCP listener that connects each accepted connection to
// a link the driver opens itself. go-modbus dials its TCP-based transports
// with fixed options (no local bind, a fixed TLS configuration that demands a
// client certificate, no RTU framing over TLS); pointing the library at the
// bridge gives the driver back full control of the socket while go-modbus
// still owns the protocol.
type Bridge struct {
	ln   net.Listener
	dial DialFunc

	mu      sync.Mutex
	pending net.Conn
	conns   map[net.Conn]struct{}
	lastErr error
	closed  bool
	wg      sync.WaitGroup
}

// NewBridge listens on 127.0.0.1 on an ephemeral port.
func NewBridge(dial DialFunc) (*Bridge, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &Bridge{ln: ln, dial: dial, conns: map[net.Conn]struct{}{}}
	b.wg.Add(1)
	go b.acceptLoop()
	return b, nil
}

// Addr is the loopback address go-modbus should dial.
func (b *Bridge) Addr() string { return b.ln.Addr().String() }

// Prime opens the real link now, so a failure is reported at connect time with
// its own error instead of surfacing later as an end-of-file on the first
// request. The link is handed to the next connection the bridge accepts.
func (b *Bridge) Prime(ctx context.Context) error {
	up, err := b.dial(ctx)
	if err != nil {
		b.setErr(err)
		return err
	}
	b.mu.Lock()
	if b.pending != nil {
		_ = b.pending.Close()
	}
	b.pending = up
	b.mu.Unlock()
	return nil
}

// LastError is the most recent failure to open the real link.
func (b *Bridge) LastError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastErr
}

func (b *Bridge) setErr(err error) {
	b.mu.Lock()
	b.lastErr = err
	b.mu.Unlock()
}

// Close stops the listener and every bridged connection.
func (b *Bridge) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	if b.pending != nil {
		_ = b.pending.Close()
		b.pending = nil
	}
	for c := range b.conns {
		_ = c.Close()
	}
	b.mu.Unlock()
	_ = b.ln.Close()
	b.wg.Wait()
}

func (b *Bridge) acceptLoop() {
	defer b.wg.Done()
	for {
		c, err := b.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		b.mu.Lock()
		up := b.pending
		b.pending = nil
		closed := b.closed
		b.mu.Unlock()
		if closed {
			_ = c.Close()
			return
		}
		if up == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			up, err = b.dial(ctx)
			cancel()
			if err != nil {
				b.setErr(err)
				_ = c.Close()
				continue
			}
		}
		b.track(c, up)
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			Pipe(c, up, 0)
			b.untrack(c, up)
		}()
	}
}

func (b *Bridge) track(cs ...net.Conn) {
	b.mu.Lock()
	for _, c := range cs {
		b.conns[c] = struct{}{}
	}
	b.mu.Unlock()
}

func (b *Bridge) untrack(cs ...net.Conn) {
	b.mu.Lock()
	for _, c := range cs {
		delete(b.conns, c)
	}
	b.mu.Unlock()
}

// Pipe copies both ways until either side ends, then closes both. A positive
// idle closes the link when nothing arrives from a for that long.
func Pipe(a, b net.Conn, idle time.Duration) {
	done := make(chan struct{}, 2)
	go func() {
		copyIdle(b, a, idle)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}

func copyIdle(dst, src net.Conn, idle time.Duration) {
	buf := make([]byte, 32*1024)
	for {
		if idle > 0 {
			_ = src.SetReadDeadline(time.Now().Add(idle))
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
