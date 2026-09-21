/*
 * IEC 60870-5-104 Server Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Go reimplementation of src/lib60870.netcore/iec104server using the
 * go-iecp5 library. Drop-in replacement: same protocol driver name,
 * MongoDB semantics and CLI contract.
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

package main

import (
	"context"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs104"
	"go.mongodb.org/mongo-driver/v2/bson"

	"iec60870-5/internal/jscfg"
	"iec60870-5/internal/model"
	"iec60870-5/internal/mongoutil"
	"iec60870-5/internal/srvapp"
	"iec60870-5/internal/tlsutil"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

const (
	protocolDriverName = "IEC60870-5-104_SERVER"
	driverVersion      = "0.3.0"
)

// srv104 holds the runtime state of one served IEC 104 connection.
type srv104 struct {
	cfg     model.ConnCfg
	engConn *srvapp.Conn
	server  *cs104.Server
	engine  *srvapp.Engine

	sessMu   sync.Mutex
	sessions map[asdu.Connect]string // session -> remote address
}

func (s *srv104) sessionCount() int {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	return len(s.sessions)
}

func (s *srv104) originatorIPs() string {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	out := ""
	for _, a := range s.sessions {
		out += a + " "
	}
	return out
}

// ipAllowed implements the client IP whitelist (C# ConnectionRequestHandler):
// empty list or "*" accepts any client, else the IP must be listed.
func (s *srv104) ipAllowed(remote string) bool {
	if len(s.cfg.IPAddresses) == 0 {
		return true
	}
	if len(s.cfg.IPAddresses) >= 1 && s.cfg.IPAddresses[0] == "*" {
		return true
	}
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	for _, a := range s.cfg.IPAddresses {
		if a == host {
			return true
		}
	}
	return false
}

// updateConnStats updates the protocolConnections stats subdocument
// (port of the C# ConnectionEventHandler).
func (s *srv104) updateConnStats(remote, state string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	endpointKey := strings.ReplaceAll(remote, ".", "_") // avoid dotted field paths
	_, err := s.engine.DB().Collection(jsmongo.ProtocolConnectionsCollectionName).UpdateOne(ctx,
		bson.M{"protocolConnectionNumber": s.cfg.ProtocolConnectionNumber},
		bson.M{"$set": bson.M{"stats": bson.M{
			"nodeName":          s.engine.Cfg.NodeName,
			"timeTag":           time.Now(),
			"openConnections":   s.server.GetSessionsLen(),
			"clientConnections": s.sessionCount(),
			"endpoint": bson.M{endpointKey: bson.M{
				"timeTag":         time.Now(),
				"connectionState": state,
			}},
		}}})
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "%s", s.cfg.Name+" - Error updating stats: "+err.Error())
	}
}

// srvHandler adapts go-iecp5 server callbacks to the shared engine.
type srvHandler struct {
	s *srv104
}

func (h *srvHandler) InterrogationHandler(c asdu.Connect, pack *asdu.ASDU, qoi asdu.QualifierOfInterrogation) error {
	h.s.engine.Interrogation(h.s.engConn, c, pack, qoi)
	return nil
}
func (h *srvHandler) CounterInterrogationHandler(c asdu.Connect, pack *asdu.ASDU, qcc asdu.QualifierCountCall) error {
	h.s.engine.HandleCounterInterrogation(h.s.engConn, c, pack, qcc)
	return nil
}
func (h *srvHandler) ReadHandler(c asdu.Connect, pack *asdu.ASDU, ioa asdu.InfoObjAddr) error {
	h.s.engine.HandleRead(h.s.engConn, c, pack, ioa)
	return nil
}
func (h *srvHandler) ClockSyncHandler(c asdu.Connect, pack *asdu.ASDU, t time.Time) error {
	h.s.engine.HandleClockSync(h.s.engConn, c, pack, t)
	return nil
}
func (h *srvHandler) ResetProcessHandler(c asdu.Connect, pack *asdu.ASDU, qrp asdu.QualifierOfResetProcessCmd) error {
	h.s.engine.HandleResetProcess(h.s.engConn, c, pack, qrp)
	return nil
}
func (h *srvHandler) DelayAcquisitionHandler(c asdu.Connect, pack *asdu.ASDU, _ uint16) error {
	_ = pack.SendReplyMirror(c, asdu.ActivationCon)
	return nil
}
func (h *srvHandler) ASDUHandler(c asdu.Connect, pack *asdu.ASDU) error {
	jslog.Log(jslog.LevelDetailed, "%s - %s", h.s.cfg.Name, pack.String())
	if !h.s.engine.HandleCommandASDU(h.s.engConn, c, pack) {
		jslog.Log(jslog.LevelBasic, "%s -   Not implemented type of ASDU received: %d", h.s.cfg.Name, pack.Type)
		pack.Coa.IsNegative = true
		_ = pack.SendReplyMirror(c, asdu.ActivationCon)
	}
	return nil
}
func (h *srvHandler) ASDUHandlerAll(asdu.Connect, *asdu.ASDU, int) error { return nil }

func defInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func main() {
	jslog.Log(jslog.LevelBasic, "{json:scada} IEC60870-5-104 Server Driver - Copyright 2020-2026 RLO")
	jslog.Log(jslog.LevelBasic, "Driver version "+driverVersion+" (Go/go-iecp5)")

	cfg, instanceNumber, err := jscfg.Read()
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", err.Error())
		os.Exit(-1)
	}
	jslog.Log(jslog.LevelBasic, "%s", "MongoDB database name: "+cfg.MongoDatabaseName)
	jslog.Log(jslog.LevelBasic, "%s", "Node name: "+cfg.NodeName)

	engine, err := srvapp.New(cfg, protocolDriverName)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Error connecting to MongoDB: "+err.Error())
		os.Exit(-1)
	}

	if _, err := mongoutil.LoadInstance(engine.DB(), protocolDriverName, instanceNumber, cfg.NodeName); err != nil {
		jslog.Log(jslog.LevelBasic, "%s", err.Error())
		os.Exit(-1)
	}
	jslog.Log(jslog.LevelBasic, "Instance: %d", instanceNumber)

	connCfgs, err := mongoutil.LoadConns(engine.DB(), protocolDriverName, instanceNumber)
	if err != nil || len(connCfgs) == 0 {
		jslog.Log(jslog.LevelBasic, "No connections found!")
		os.Exit(-1)
	}

	var servers []*srv104
	for _, cc := range connCfgs {
		s := &srv104{cfg: cc, engine: engine, sessions: map[asdu.Connect]string{}}
		engConn := &srvapp.Conn{Cfg: cc}
		engConn.OriginatorIP = s.originatorIPs
		s.engConn = engConn
		engine.Conns = append(engine.Conns, engConn)
		servers = append(servers, s)
		jslog.Log(jslog.LevelBasic, "%s", cc.Name)
	}

	engine.DistributeAutoTags()

	jslog.Log(jslog.LevelBasic, "Setting up IEC Connections & ASDU handlers...")
	for _, s := range servers {
		cc := s.cfg
		server := cs104.NewServer(&srvHandler{s: s})
		s.server = server
		s.engConn.Endpoint = server
		s.engConn.HasSession = func() bool { return server.GetSessionsLen() > 0 }

		params := asdu.Params{
			CauseSize:       defInt(int(cc.SizeOfCOT), 2),
			CommonAddrSize:  defInt(int(cc.SizeOfCA), 2),
			InfoObjAddrSize: defInt(int(cc.SizeOfIOA), 3),
			OrigAddress:     asdu.OriginAddr(cc.LocalLinkAddress),
			InfoObjTimeZone: time.Local,
		}
		server.SetParams(&params)
		server.SetConfig(cs104.Config{
			ConnectTimeout0:   time.Duration(defInt(int(cc.T0), 10)) * time.Second,
			SendUnAckLimitK:   uint16(defInt(int(cc.K), 12)),
			SendUnAckTimeout1: time.Duration(defInt(int(cc.T1), 15)) * time.Second,
			RecvUnAckLimitW:   uint16(defInt(int(cc.W), 8)),
			RecvUnAckTimeout2: time.Duration(defInt(int(cc.T2), 10)) * time.Second,
			IdleTimeout3:      time.Duration(defInt(int(cc.T3), 20)) * time.Second,
		})
		server.SetInfoObjTimeZone(time.Local)
		if jslog.Level() >= jslog.LevelDebug {
			server.LogMode(true)
		}

		tlsCfg, err := tlsutil.BuildTLSConfig(&s.cfg, true)
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - Error configuring TLS certificates.")
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - "+err.Error())
			os.Exit(1)
		}
		if tlsCfg != nil {
			server.SetTLSConfig(tlsCfg)
		}

		maxClients := defInt(int(cc.MaxClientConnections), 2)
		sref := s
		server.SetOnConnectionHandler(func(c asdu.Connect) {
			remote := ""
			if uc := c.UnderlyingConn(); uc != nil {
				remote = uc.RemoteAddr().String()
			}
			jslog.Log(jslog.LevelBasic, "%s - New connection request from IP %s", sref.cfg.Name, remote)
			if !sref.ipAllowed(remote) || sref.sessionCount() >= maxClients {
				jslog.Log(jslog.LevelBasic, "%s - Connection rejected from %s", sref.cfg.Name, remote)
				if uc := c.UnderlyingConn(); uc != nil {
					_ = uc.Close()
				}
				return
			}
			sref.sessMu.Lock()
			sref.sessions[c] = remote
			sref.sessMu.Unlock()
			jslog.Log(jslog.LevelBasic, "%s - Connection event %s - OPENED", sref.cfg.Name, remote)
			go sref.updateConnStats(remote, "OPENED")
		})
		server.SetConnectionLostHandler(func(c asdu.Connect) {
			sref.sessMu.Lock()
			remote := sref.sessions[c]
			delete(sref.sessions, c)
			sref.sessMu.Unlock()
			jslog.Log(jslog.LevelBasic, "%s - Connection event %s - CLOSED", sref.cfg.Name, remote)
			go sref.updateConnStats(remote, "CLOSED")
		})

		// bind address
		localBind := cc.IPAddressLocalBind
		if localBind == "" {
			localBind = "0.0.0.0:2404"
		}
		if !strings.Contains(localBind, ":") {
			localBind += ":2404"
		}
		go func(bind string, name string) {
			jslog.Log(jslog.LevelBasic, "%s - New server listening on %s", name, bind)
			server.ListenAndServer(bind) // blocks while serving
			jslog.Log(jslog.LevelBasic, "%s - Server stopped", name)
			os.Exit(1)
		}(localBind, cc.Name)
	}

	go engine.RunDequeueLoop()
	go engine.RunRealtimeStream()

	// main loop: keep MongoDB connection alive (blocks forever)
	engine.MaintainMongo()
}
