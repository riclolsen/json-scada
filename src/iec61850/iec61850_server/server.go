/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
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

// MMS server creation and lifecycle. The server is created once over the
// model built at startup and listens for as long as the process runs: a TCP
// listener is passive, so this driver has no active/standby arbitration.

package main

import (
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
)

// Gateway holds the running server and everything it serves.
type Gateway struct {
	conn  *ServerConnection
	built *BuiltModel
	srv   *server.Server

	bindAddr string
	useTLS   bool
	tlsCfg   *tls.Config

	serving   atomic.Bool
	openConns atomic.Int32
	listener  net.Listener

	// lastBindErrLog throttles the "failed to start" message of the retry.
	lastBindErrLog time.Time
}

// parseBindAddress resolves ipAddressLocalBind into "host:port". The
// default port is 102, or 3782 for a secured connection, as in the C#.
func parseBindAddress(conn *ServerConnection) string {
	host := "0.0.0.0"
	port := 102
	if conn.UseSecurity {
		port = 3782
	}
	s := strings.TrimSpace(conn.IPAddressLocalBind)
	if s != "" {
		if idx := strings.LastIndex(s, ":"); idx > 0 && idx < len(s)-1 {
			if p, err := strconv.Atoi(s[idx+1:]); err == nil {
				host = s[:idx]
				port = p
			} else {
				host = s
			}
		} else {
			host = s
		}
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// NewGateway creates the MMS server over an already built model. It is
// called once: server.New materialises the report control blocks into the
// model, so calling it twice would duplicate them.
func NewGateway(conn *ServerConnection, built *BuiltModel) (*Gateway, error) {
	g := &Gateway{conn: conn, built: built, bindAddr: parseBindAddress(conn)}

	maxConns := 1
	if conn.ServerModeMultiActive {
		maxConns = int(conn.MaxClientConnections)
		if maxConns < 1 {
			maxConns = 1
		}
	}

	opts := []server.Option{
		server.WithIdentity(server.Identity{
			Vendor: "JSON-SCADA", Model: ProtocolDriverName, Revision: DriverVersion,
		}),
		server.WithMaxConnections(maxConns),
	}
	if bufDepth := int(conn.MaxQueueSize); bufDepth > 0 {
		opts = append(opts, server.WithReportBufferSize(bufDepth))
	}
	if conn.UseSecurity {
		cfg, err := buildTLS(conn)
		if err != nil {
			return nil, err
		}
		g.useTLS, g.tlsCfg = true, cfg
	}
	if lg := libLogger(); lg != nil {
		opts = append(opts, server.WithLogger(lg))
	}

	// server.New materialises the report control block instances into the
	// model; their report identifiers need correcting before any client
	// reads them.
	g.srv = server.New(built.Model, opts...)
	if n := fixReportIDs(built.Model); n > 0 {
		jslog.Log(jslog.LevelDetailed, "Report identifiers set on %d report control block instance(s).", n)
	}
	g.srv.OnConnection(g.onConnection)

	jslog.Log(jslog.LevelBasic, "IedServer created (max %d client connection(s), buffered report depth %d).",
		maxConns, int(conn.MaxQueueSize))
	jslog.Log(jslog.LevelNoLog, "Bind address: %s%s", g.bindAddr, tlsSuffix(conn.UseSecurity))
	return g, nil
}

// fixReportIDs gives every materialised report control block instance an
// RptID of its own: the object reference of that instance, in MMS notation.
//
// The library composes the default RptID from the *configured* control
// block name, while the instances it materialises carry an index suffix —
// so `brcbMX01` with two instances yields `brcbMX0101` and `brcbMX0102`,
// both reporting `LD/LLN0$BR$brcbMX01`, a control block that does not
// exist. IEC 61850-8-1 has the default RptID be the object reference of
// the report control block, which is what libiec61850 (and therefore the
// C# driver) sends; clients that match an incoming report to the control
// block they enabled — IEDExplorer among them — cannot bind the report to
// their model otherwise, and the values arrive unattached to any data
// object. Two instances sharing one identifier cannot be told apart either.
//
// This walks the model after materialisation and writes the correct value.
// It assumes the driver never configures an RptID of its own, which the
// model builder does not.
func fixReportIDs(m *model.Model) int {
	fixed := 0
	for _, ld := range m.Devices {
		for _, ln := range ld.Nodes {
			for _, do := range ln.Objects {
				rptID := do.Attribute("RptID")
				if rptID == nil || do.Attribute("RptEna") == nil {
					continue // not a report control block
				}
				want := ld.Name + "/" + ln.Name + "$" + rptID.FC.String() + "$" + do.Name
				if rptID.Value != nil && rptID.Value.Text() == want {
					continue
				}
				rptID.Value = mms.NewVisibleString(want)
				fixed++
			}
		}
	}
	return fixed
}

func tlsSuffix(useSecurity bool) string {
	if useSecurity {
		return " (TLS)"
	}
	return ""
}

// onConnection logs client connections and enforces the optional address
// allow-list, closing the association of a client that is not on it — the
// equivalent of the C# driver's con.Abort().
func (g *Gateway) onConnection(ev server.ConnectionEvent) {
	switch ev.State {
	case server.ConnectionOpened:
		g.openConns.Store(int32(ev.Open))
		jslog.Log(jslog.LevelBasic, "IEC61850 client connected: %s", ev.Peer)
		if !g.peerAllowed(ev.Peer) {
			jslog.Log(jslog.LevelBasic, "Client %s not in allow-list, aborting connection.", ev.Peer)
			if ev.Conn != nil {
				_ = ev.Conn.Close()
			}
		}
	case server.ConnectionClosed:
		g.openConns.Store(int32(ev.Open))
		jslog.Log(jslog.LevelBasic, "IEC61850 client disconnected: %s", ev.Peer)
	case server.ConnectionRefused:
		jslog.Log(jslog.LevelBasic, "Client %s refused: %d connection(s) already open.", ev.Peer, ev.Open)
	}
}

// peerAllowed matches a peer against the connection's ipAddresses list,
// comparing addresses without their port. An empty list allows anyone.
func (g *Gateway) peerAllowed(peer string) bool {
	if len(g.conn.IPAddresses) == 0 {
		return true
	}
	ip := hostOnly(peer)
	for _, a := range g.conn.IPAddresses {
		if a == "" {
			continue
		}
		if hostOnly(a) == ip {
			return true
		}
	}
	return false
}

func hostOnly(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return strings.Trim(addr, "[]")
}

// Start binds and serves. It is idempotent: starting a running server does
// nothing.
func (g *Gateway) Start() {
	if g.serving.Load() {
		return
	}
	var ln net.Listener
	var err error
	if g.useTLS {
		ln, err = tls.Listen("tcp", g.bindAddr, g.tlsCfg)
	} else {
		ln, err = net.Listen("tcp", g.bindAddr)
	}
	if err != nil {
		// Start is retried every second; do not repeat the message that often.
		if now := time.Now(); now.Sub(g.lastBindErrLog) >= 30*time.Second {
			g.lastBindErrLog = now
			jslog.Log(jslog.LevelNoLog, "ERROR: failed to start MMS server on %s (port in use or insufficient "+
				"privileges for port < 1024?): %v", g.bindAddr, err)
		}
		return
	}
	g.listener = ln
	g.serving.Store(true)
	go func() {
		_ = g.srv.Serve(ln)
	}()
	jslog.Log(jslog.LevelNoLog, "IEC 61850 MMS server STARTED on %s%s", g.bindAddr, tlsSuffix(g.useTLS))
}

// Stop closes the listener and every open association.
func (g *Gateway) Stop() {
	if !g.serving.Load() {
		return
	}
	_ = g.srv.Close()
	g.serving.Store(false)
	g.openConns.Store(0)
	jslog.Log(jslog.LevelNoLog, "IEC 61850 MMS server STOPPED.")
}

// Serving reports whether the MMS server is accepting clients.
func (g *Gateway) Serving() bool { return g.serving.Load() }

// OpenConnections is the number of client connections currently held.
func (g *Gateway) OpenConnections() int { return g.srv.OpenConnections() }

// Addr is the address the server binds to.
func (g *Gateway) Addr() string {
	if g.listener != nil {
		return g.listener.Addr().String()
	}
	return g.bindAddr
}
