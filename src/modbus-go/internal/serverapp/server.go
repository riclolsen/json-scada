/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

// One MODBUS_SERVER connection. The go-modbus server listens on a private
// loopback port; masters connect to a front-end (mbnet.Front) that enforces
// the allow-list, the client limit and the idle timeout, terminates TLS with
// the connection's own settings and converts RTU framing to MBAP. A serial
// port is served by an RTU gateway the same way. Ported from connection.ts.

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbnet"

	"github.com/otfabric/go-modbus"
	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Feeder keeps the point map current.
type Feeder interface {
	FullReload(ctx context.Context) error
	Run(ctx context.Context)
}

// RetryListen is the delay before a failed listen is retried.
var RetryListen = 5 * time.Second

// Stats are the server statistics written to protocolConnections.stats.
type Stats struct {
	Status     string
	Clients    int
	Reads      int64
	Writes     int64
	Exceptions int64
	LastError  string
}

// BSON is the stats document.
func (s Stats) BSON() bson.M {
	return bson.M{
		"status": s.Status, "clients": int64(s.Clients), "reads": s.Reads,
		"writes": s.Writes, "exceptions": s.Exceptions, "lastError": s.LastError,
	}
}

// Server is one connection.
type Server struct {
	Cfg    Config
	PM     *PointMap
	Feeder Feeder // nil: the map is fed by the caller

	log   drv.ConnLogger
	addrs mbnet.AddrMap
	st    counters
	wh    *WriteHandler

	mu      sync.Mutex
	status  string
	clients int
	lastErr string
	cancel  context.CancelFunc
	done    chan struct{}
	front   *mbnet.Front
}

// NewServer prepares a connection; Start runs it.
func NewServer(cfg Config, sink CommandSink, feeder Feeder) *Server {
	pm := NewPointMap(cfg)
	s := &Server{Cfg: cfg, PM: pm, Feeder: feeder, log: drv.ConnLogger{Prefix: cfg.Name}, status: "stopped"}
	s.wh = NewWriteHandler(cfg, sink, pm)
	return s
}

// SetFeeder sets the feeder before Start.
func (s *Server) SetFeeder(f Feeder) { s.Feeder = f }

// Stats is a snapshot.
func (s *Server) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{Status: s.status, Clients: s.clients, Reads: s.st.reads.Load(), Writes: s.st.writes.Load(),
		Exceptions: s.st.exceptions.Load(), LastError: s.lastErr}
}

// Addr is the address masters connect to, once listening (TCP and TLS modes).
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.front == nil {
		return ""
	}
	return s.front.Addr()
}

func (s *Server) setStatus(st, lastErr string) {
	s.mu.Lock()
	s.status = st
	if lastErr != "" {
		s.lastErr = lastErr
	}
	s.mu.Unlock()
}

// Start runs the connection in the background; it is a no-op when running.
func (s *Server) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	go func() {
		defer close(s.done)
		s.run(ctx)
	}()
}

// Stop ends the connection and waits for it.
func (s *Server) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel = nil
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
	s.setStatus("stopped", "")
}

// Running reports a started connection.
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel != nil
}

func (s *Server) run(ctx context.Context) {
	if s.Feeder != nil {
		if err := s.Feeder.FullReload(ctx); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelBasic, "%s: full reload error: %v", s.Cfg.Name, err)
		}
		go s.Feeder.Run(ctx)
	}
	if err := s.Cfg.Validate(); err != nil {
		jslog.Log(jslog.LevelBasic, "%s: %v; serving %s", s.Cfg.Name, err, ModeTCP)
		s.Cfg.ConnectionMode = ModeTCP
	}
	for ctx.Err() == nil {
		err := s.serve(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			s.setStatus("listen failed", err.Error())
			jslog.Log(jslog.LevelBasic, "%s: listen failed: %v", s.Cfg.Name, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(RetryListen):
		}
	}
}

// startBackend starts the go-modbus server on a free loopback port.
func (s *Server) startBackend() (*modbus.Server, string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	idle := time.Duration(s.Cfg.ClientIdleTimeoutMs) * time.Millisecond
	backendIdle := 24 * time.Hour // the front-end owns the idle timeout
	if idle > 0 {
		backendIdle = idle + 30*time.Second
	}
	maxc := int(s.Cfg.MaxClientConnections)
	if maxc < 1 {
		maxc = 1
	}
	h := &Handler{cfg: s.Cfg, pm: s.PM, wh: s.wh, addrs: &s.addrs, st: &s.st}
	srv, err := modbus.NewServer(&modbus.ServerConfig{
		URL:        "tcp://" + addr,
		Timeout:    backendIdle,
		MaxClients: uint(2*maxc + 4),
		Logger:     drv.LibLogger{Prefix: s.Cfg.Name},
		Metrics:    metrics{name: s.Cfg.Name, st: &s.st},
	}, h)
	if err != nil {
		return nil, "", err
	}
	if err := srv.Start(); err != nil {
		return nil, "", err
	}
	return srv, addr, nil
}

func (s *Server) acceptUnit(unit byte) bool {
	if unit == 0 || !s.Cfg.StrictUnitID || s.PM.AcceptsUnit(int(unit)) {
		return true
	}
	s.log.Info(fmt.Sprintf("request ignored: unit id %d not served (strictUnitId is enabled)", unit))
	return false
}

func (s *Server) idleGap() time.Duration {
	ms := mbcore.RTUIdleGapMs(s.Cfg.BaudRate, s.Cfg.InterFrameDelayMs)
	return time.Duration(ms * float64(time.Millisecond))
}

// serve runs one listening session; it returns when ctx ends or listening fails.
func (s *Server) serve(ctx context.Context) error {
	backend, upstream, err := s.startBackend()
	if err != nil {
		return fmt.Errorf("protocol server: %w", err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = backend.Shutdown(sctx)
		cancel()
	}()

	if s.Cfg.IsSerial() {
		return s.serveSerial(ctx, upstream)
	}

	var tcfg *tls.Config
	if s.Cfg.IsTLS() {
		if tcfg, err = mbnet.ServerTLSConfig(s.Cfg.TLS); err != nil {
			return err
		}
	}
	framing := mbnet.FramingMBAP
	if s.Cfg.IsRTU() {
		framing = mbnet.FramingRTU
	}
	front := mbnet.NewFront(mbnet.FrontConfig{
		Bind: s.Cfg.IPAddressLocalBind, DefaultPort: s.Cfg.DefaultPort(), TLS: tcfg, Framing: framing,
		MaxClients: int(s.Cfg.MaxClientConnections), IdleTimeout: time.Duration(s.Cfg.ClientIdleTimeoutMs) * time.Millisecond,
		AllowList: s.Cfg.IPAddresses, Upstream: upstream, Addrs: &s.addrs, IdleGap: s.idleGap(),
		AcceptUnit: s.acceptUnit, Log: s.log,
		OnClient: func(remote string, connected bool, n int) {
			s.mu.Lock()
			s.clients = n
			s.mu.Unlock()
			if connected {
				s.log.Info(fmt.Sprintf("client connected %s (%d of max %s)", remote, n, mbcore.JSNumberString(s.Cfg.MaxClientConnections)))
			} else {
				s.log.Info(fmt.Sprintf("client disconnected %s (%d remaining)", remote, n))
			}
		},
		OnError: func(err error) {
			s.setStatus(s.Stats().Status, err.Error())
			jslog.Log(jslog.LevelBasic, "%s: listener error: %v", s.Cfg.Name, err)
		},
	})
	if err := front.Listen(); err != nil {
		return err
	}
	s.mu.Lock()
	s.front = front
	s.mu.Unlock()
	s.setStatus("listening", "")
	jslog.Log(jslog.LevelBasic, "%s: listening on %s (%s)", s.Cfg.Name, front.Addr(), s.Cfg.ConnectionMode)
	<-ctx.Done()
	front.Close()
	s.mu.Lock()
	s.front, s.clients = nil, 0
	s.mu.Unlock()
	return nil
}

func (s *Server) serveSerial(ctx context.Context, upstream string) error {
	if _, warn := mbnet.Parity(s.Cfg.Parity); warn != "" {
		jslog.Log(jslog.LevelBasic, "%s: %s", s.Cfg.Name, warn)
	}
	if w := mbnet.HandshakeWarning(s.Cfg.Handshake); w != "" {
		jslog.Log(jslog.LevelBasic, "%s: %s", s.Cfg.Name, w)
	}
	opts := mbnet.SerialOptions{PortName: s.Cfg.PortName, BaudRate: int(s.Cfg.BaudRate), Parity: s.Cfg.Parity,
		StopBits: s.Cfg.StopBits, Handshake: s.Cfg.Handshake}
	gap := s.idleGap()
	open := func() (io.ReadWriteCloser, error) { return mbnet.OpenSerial(opts, gap) }
	mbnet.ServeSerial(ctx, open, upstream, &s.addrs, s.acceptUnit, s.log, func(isOpen bool, err error) {
		if isOpen {
			s.setStatus("listening", "")
			jslog.Log(jslog.LevelBasic, "%s: listening on %s (%s)", s.Cfg.Name, s.Cfg.PortName, ModeSerial)
			return
		}
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		s.setStatus("listen failed", msg)
		jslog.Log(jslog.LevelBasic, "%s: serial port %s: %s, retrying in 5 s", s.Cfg.Name, s.Cfg.PortName, msg)
	})
	return nil
}
