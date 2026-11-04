/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbnet

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"modbus-go/internal/mbcore"

	"github.com/otfabric/go-serial"
)

// Logger receives the front-end's log lines; the driver maps the three
// methods to log levels 3, 2 and 1.
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Error(msg string)
	DebugEnabled() bool
}

// AddrMap remembers which master stands behind each loopback connection to the
// go-modbus server, so handler log lines and relayed commands name the real
// client instead of 127.0.0.1.
type AddrMap struct{ m sync.Map }

// Put records the real remote of a loopback connection's local address.
func (a *AddrMap) Put(local, remote string) { a.m.Store(local, remote) }

// Delete forgets a loopback connection.
func (a *AddrMap) Delete(local string) { a.m.Delete(local) }

// Resolve maps a go-modbus ClientAddr to the real remote.
func (a *AddrMap) Resolve(clientAddr string) string {
	if v, ok := a.m.Load(clientAddr); ok {
		return v.(string)
	}
	return clientAddr
}

// Framing is the master-side framing of a server connection.
type Framing int

// The two framings.
const (
	FramingMBAP Framing = iota
	FramingRTU
)

// FrontConfig configures a server front-end.
type FrontConfig struct {
	Bind        string
	DefaultPort int
	TLS         *tls.Config // nil: plain TCP
	Framing     Framing
	MaxClients  int
	IdleTimeout time.Duration
	AllowList   []string
	Upstream    string // the go-modbus server, 127.0.0.1:port
	Addrs       *AddrMap
	IdleGap     time.Duration // RTU inter-frame gap
	AcceptUnit  func(unit byte) bool
	Log         Logger
	OnClient    func(remote string, connected bool, count int)
	OnError     func(err error)
}

// Front accepts masters and forwards their requests to the go-modbus server.
type Front struct {
	cfg FrontConfig

	mu      sync.Mutex
	ln      net.Listener
	clients map[net.Conn]struct{}
	wg      sync.WaitGroup
}

// NewFront prepares a front-end; Listen binds it.
func NewFront(cfg FrontConfig) *Front {
	return &Front{cfg: cfg, clients: map[net.Conn]struct{}{}}
}

// Listen binds the listening socket and starts accepting.
func (f *Front) Listen() error {
	host, port := ParseHostPort(f.cfg.Bind, f.cfg.DefaultPort)
	ln, err := net.Listen("tcp", JoinHostPort(host, port))
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.ln = ln
	f.mu.Unlock()
	f.wg.Add(1)
	go f.acceptLoop(ln)
	return nil
}

// Addr is the bound address, once listening.
func (f *Front) Addr() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ln == nil {
		return ""
	}
	return f.ln.Addr().String()
}

// Close stops accepting and drops every client.
func (f *Front) Close() {
	f.mu.Lock()
	if f.ln != nil {
		_ = f.ln.Close()
		f.ln = nil
	}
	for c := range f.clients {
		_ = c.Close()
	}
	f.mu.Unlock()
	f.wg.Wait()
}

func (f *Front) acceptLoop(ln net.Listener) {
	defer f.wg.Done()
	for {
		c, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if f.cfg.OnError != nil {
				f.cfg.OnError(err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		remote := c.RemoteAddr().String()
		if !f.allowed(c) {
			f.cfg.Log.Info(fmt.Sprintf("client %s rejected: not in the ipAddresses allow-list", remote))
			_ = c.Close()
			continue
		}
		f.mu.Lock()
		if f.cfg.MaxClients > 0 && len(f.clients) >= f.cfg.MaxClients {
			f.mu.Unlock()
			f.cfg.Log.Info(fmt.Sprintf("client %s rejected: maxClientConnections (%d) reached", remote, f.cfg.MaxClients))
			_ = c.Close()
			continue
		}
		f.clients[c] = struct{}{}
		f.mu.Unlock()
		if tc, ok := c.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
		}
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			f.serve(c, remote)
		}()
	}
}

func (f *Front) allowed(c net.Conn) bool {
	if len(f.cfg.AllowList) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		host = c.RemoteAddr().String()
	}
	host = strings.TrimPrefix(host, "::ffff:")
	for _, a := range f.cfg.AllowList {
		a = strings.Trim(strings.TrimPrefix(strings.TrimSpace(a), "::ffff:"), "[]")
		if a == host {
			return true
		}
	}
	return false
}

func (f *Front) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.clients)
}

func (f *Front) serve(c net.Conn, remote string) {
	defer func() {
		_ = c.Close()
		f.mu.Lock()
		delete(f.clients, c)
		n := len(f.clients)
		f.mu.Unlock()
		if f.cfg.OnClient != nil {
			f.cfg.OnClient(remote, false, n)
		}
	}()
	conn := c
	if f.cfg.TLS != nil {
		tc := tls.Server(c, f.cfg.TLS)
		_ = tc.SetDeadline(time.Now().Add(30 * time.Second))
		if err := tc.Handshake(); err != nil {
			if f.cfg.OnError != nil {
				f.cfg.OnError(fmt.Errorf("TLS handshake with %s failed: %w", remote, err))
			}
			return
		}
		_ = tc.SetDeadline(time.Time{})
		conn = tc
		f.mu.Lock()
		delete(f.clients, c)
		f.clients[tc] = struct{}{}
		f.mu.Unlock()
		c = tc
	}
	if f.cfg.OnClient != nil {
		f.cfg.OnClient(remote, true, f.count())
	}
	if f.cfg.Framing == FramingRTU {
		up := NewUpstream(f.cfg.Upstream, remote, f.cfg.Addrs)
		defer up.Close()
		ServeRTU(&netFrameReader{c: conn, idle: f.cfg.IdleTimeout, gap: f.cfg.IdleGap}, conn, remote, up, f.cfg.AcceptUnit, f.cfg.Log)
		return
	}
	up, err := net.DialTimeout("tcp", f.cfg.Upstream, 5*time.Second)
	if err != nil {
		f.cfg.Log.Error(fmt.Sprintf("[%s] cannot reach the protocol server: %v", remote, err))
		return
	}
	local := up.LocalAddr().String()
	f.cfg.Addrs.Put(local, remote)
	defer f.cfg.Addrs.Delete(local)
	if f.cfg.Log.DebugEnabled() {
		Pipe(&traceConn{Conn: conn, remote: remote, log: f.cfg.Log}, up, f.cfg.IdleTimeout)
		return
	}
	Pipe(conn, up, f.cfg.IdleTimeout)
}

// traceConn hex-dumps what a master sends, at log level 3.
type traceConn struct {
	net.Conn
	remote string
	log    Logger
}

func (t *traceConn) Read(p []byte) (int, error) {
	n, err := t.Conn.Read(p)
	if n > 0 {
		t.log.Debug(fmt.Sprintf("[%s] link RX %d bytes: %s", t.remote, n, mbcore.Hex(p[:n], 0)))
	}
	return n, err
}

func (t *traceConn) Write(p []byte) (int, error) {
	t.log.Debug(fmt.Sprintf("[%s] link TX %d bytes: %s", t.remote, len(p), mbcore.Hex(p, 0)))
	return t.Conn.Write(p)
}

// frameReader is a byte source for RTU framing that can report an idle gap.
type frameReader interface {
	Read(p []byte) (int, error)
	// Arm prepares the next Read: with bytes buffered, a gap ends the frame.
	Arm(buffered bool)
	// IdleEndsSession reports whether a timeout with nothing buffered ends the
	// session (a TCP client idle timeout) or is just a quiet line (serial).
	IdleEndsSession() bool
}

type netFrameReader struct {
	c    net.Conn
	idle time.Duration
	gap  time.Duration
}

func (r *netFrameReader) Read(p []byte) (int, error) { return r.c.Read(p) }
func (r *netFrameReader) Arm(buffered bool) {
	switch {
	case buffered:
		_ = r.c.SetReadDeadline(time.Now().Add(r.gap))
	case r.idle > 0:
		_ = r.c.SetReadDeadline(time.Now().Add(r.idle))
	default:
		_ = r.c.SetReadDeadline(time.Time{})
	}
}
func (r *netFrameReader) IdleEndsSession() bool { return true }

// SerialFrameReader reads a serial port opened with its read timeout set to
// the inter-frame gap.
type SerialFrameReader struct {
	Port io.Reader
	Ctx  context.Context
}

func (r *SerialFrameReader) Read(p []byte) (int, error) {
	if r.Ctx != nil && r.Ctx.Err() != nil {
		return 0, r.Ctx.Err()
	}
	return r.Port.Read(p)
}
func (r *SerialFrameReader) Arm(bool)              {}
func (r *SerialFrameReader) IdleEndsSession() bool { return false }

func isTimeout(err error) bool {
	if errors.Is(err, serial.ErrTimeout) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// ServeRTU runs one RTU session: it cuts request frames, forwards each to the
// go-modbus server and writes the RTU response back (none for broadcasts). It
// returns when the link fails or, on a socket, when the client stays idle.
func ServeRTU(rd frameReader, w io.Writer, remote string, up *Upstream, accept func(byte) bool, log Logger) {
	var dec mbcore.RTURequestDecoder
	buf := make([]byte, 512)
	handle := func(frames []mbcore.RTUFrame) bool {
		for _, fr := range frames {
			if log.DebugEnabled() {
				log.Debug(fmt.Sprintf("[%s] RX unit=%d %s | PDU: %s", remote, fr.Unit, mbcore.DescribeRequest(fr.PDU), mbcore.Hex(fr.PDU, 0)))
			}
			if fr.Unit != 0 && accept != nil && !accept(fr.Unit) {
				log.Info(fmt.Sprintf("[%s] request ignored: unit id %d not served (strictUnitId is enabled)", remote, fr.Unit))
				continue
			}
			resp, err := up.Exchange(fr.Unit, fr.PDU)
			if err != nil {
				log.Error(fmt.Sprintf("[%s] request error: unit=%d %s -> internal failure: %v", remote, fr.Unit, mbcore.DescribeRequest(fr.PDU), err))
				if len(fr.PDU) == 0 {
					continue
				}
				resp = []byte{fr.PDU[0] | 0x80, mbcore.ExServerDeviceFailure}
			}
			if fr.Unit == 0 {
				log.Debug(fmt.Sprintf("[%s] broadcast executed, no response sent", remote))
				continue
			}
			adu := mbcore.EncodeRTU(fr.Unit, resp)
			if log.DebugEnabled() {
				log.Debug(fmt.Sprintf("[%s] TX unit=%d %s | ADU: %s", remote, fr.Unit, mbcore.DescribeResponse(resp), mbcore.Hex(adu, 0)))
			}
			if _, err := w.Write(adu); err != nil {
				return false
			}
		}
		return true
	}
	for {
		rd.Arm(dec.Buffered() > 0)
		n, err := rd.Read(buf)
		if n > 0 {
			if log.DebugEnabled() {
				log.Debug(fmt.Sprintf("[%s] link RX %d bytes: %s", remote, n, mbcore.Hex(buf[:n], 0)))
			}
			if !handle(dec.Push(buf[:n])) {
				return
			}
		}
		if err != nil {
			if isTimeout(err) {
				if dec.Buffered() > 0 {
					if !handle(dec.OnIdle()) {
						return
					}
					continue
				}
				if rd.IdleEndsSession() {
					return
				}
				continue
			}
			return
		}
		if n == 0 && dec.Buffered() > 0 {
			// a serial read that returned nothing: the gap elapsed
			if !handle(dec.OnIdle()) {
				return
			}
		}
	}
}

// Upstream is one MBAP connection to the go-modbus server, used by an RTU
// session to have its requests served.
type Upstream struct {
	addr, remote string
	addrs        *AddrMap
	conn         net.Conn
	txn          uint16
	timeout      time.Duration
}

// NewUpstream prepares a lazily dialled connection to the go-modbus server.
func NewUpstream(addr, remote string, addrs *AddrMap) *Upstream {
	return &Upstream{addr: addr, remote: remote, addrs: addrs, timeout: 5 * time.Second}
}

// Close drops the connection.
func (u *Upstream) Close() {
	if u.conn != nil {
		u.addrs.Delete(u.conn.LocalAddr().String())
		_ = u.conn.Close()
		u.conn = nil
	}
}

// Exchange sends one request PDU and returns the response PDU. A connection
// the server closed while idle is replaced once; a request the server leaves
// unanswered (a unit it ignores) times out.
func (u *Upstream) Exchange(unit byte, pdu []byte) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		fresh := false
		if u.conn == nil {
			c, err := net.DialTimeout("tcp", u.addr, u.timeout)
			if err != nil {
				return nil, err
			}
			u.conn = c
			u.addrs.Put(c.LocalAddr().String(), u.remote)
			fresh = true
		}
		resp, err := u.once(unit, pdu)
		if err == nil {
			return resp, nil
		}
		u.Close()
		if fresh || attempt > 0 || isTimeout(err) {
			return nil, err
		}
	}
}

func (u *Upstream) once(unit byte, pdu []byte) ([]byte, error) {
	u.txn++
	req := make([]byte, 7+len(pdu))
	binary.BigEndian.PutUint16(req[0:], u.txn)
	binary.BigEndian.PutUint16(req[4:], uint16(len(pdu)+1))
	req[6] = unit
	copy(req[7:], pdu)
	_ = u.conn.SetDeadline(time.Now().Add(u.timeout))
	if _, err := u.conn.Write(req); err != nil {
		return nil, err
	}
	hdr := make([]byte, 7)
	for {
		if _, err := io.ReadFull(u.conn, hdr); err != nil {
			return nil, err
		}
		n := int(binary.BigEndian.Uint16(hdr[4:]))
		if n < 2 || n > 254 {
			return nil, fmt.Errorf("bad MBAP length %d from protocol server", n)
		}
		body := make([]byte, n-1)
		if _, err := io.ReadFull(u.conn, body); err != nil {
			return nil, err
		}
		if binary.BigEndian.Uint16(hdr[0:]) == u.txn {
			return body, nil
		}
		// a late answer to an earlier request: skip it
	}
}

// ServeSerial runs the RTU server on a serial port until ctx ends, reopening
// the port 5 s after it fails.
func ServeSerial(ctx context.Context, open func() (io.ReadWriteCloser, error), upstream string, addrs *AddrMap,
	accept func(byte) bool, log Logger, onState func(open bool, err error)) {
	for ctx.Err() == nil {
		port, err := open()
		if err != nil {
			onState(false, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		onState(true, nil)
		// The port's read timeout is the inter-frame gap, so the session sees ctx
		// end within one gap; closing the port under a blocked Read is not safe.
		up := NewUpstream(upstream, "serial", addrs)
		ServeRTU(&SerialFrameReader{Port: port, Ctx: ctx}, port, "serial", up, accept, log)
		up.Close()
		_ = port.Close()
		if ctx.Err() != nil {
			return
		}
		onState(false, errors.New("serial port closed"))
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
