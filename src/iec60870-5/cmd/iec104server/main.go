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
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs104"
	"github.com/riclolsen/go-iecp5/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"iec60870-5/internal/iecsec"
	"iec60870-5/internal/jscfg"
	"iec60870-5/internal/model"
	"iec60870-5/internal/mongoutil"
	"iec60870-5/internal/srvapp"
	"iec60870-5/internal/tlsutil"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"
)

const (
	protocolDriverName = "IEC60870-5-104_SERVER"
	driverVersion      = "0.5.0"
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

// sessionList returns the accepted sessions (spontaneous data targets).
func (s *srv104) sessionList() []asdu.Connect {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	out := make([]asdu.Connect, 0, len(s.sessions))
	for c := range s.sessions {
		out = append(out, c)
	}
	return out
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

// allowedPeers is ipAddresses as networks for go-iecp5's admission, which
// then refuses (and reports) an unexpected master before the TLS handshake
// or any byte is read. Nil when any master is accepted, or when an entry is
// not an IP address (the driver's own check in the connection handler then
// applies alone).
func (s *srv104) allowedPeers() []netip.Prefix {
	if len(s.cfg.IPAddresses) == 0 || s.cfg.IPAddresses[0] == "*" {
		return nil
	}
	var out []netip.Prefix
	for _, a := range s.cfg.IPAddresses {
		ip, err := netip.ParseAddr(strings.TrimSpace(a))
		if err != nil {
			return nil
		}
		ip = ip.Unmap()
		out = append(out, netip.PrefixFrom(ip, ip.BitLen()))
	}
	return out
}

// updateConnStats updates the protocolConnections stats subdocument
// (port of the C# ConnectionEventHandler).
func (s *srv104) updateConnStats(remote, state string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	endpointKey := strings.ReplaceAll(remote, ".", "_") // avoid dotted field paths
	_, err := s.engine.DB().Collection(jsmongo.ProtocolConnectionsCollectionName).UpdateOne(ctx,
		bson.M{"protocolConnectionNumber": s.cfg.ProtocolConnectionNumber},
		bson.M{"$set": bson.M{"stats": s.statsDoc(endpointKey, state)}})
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "%s", s.cfg.Name+" - Error updating stats: "+err.Error())
	}
}

func (s *srv104) statsDoc(endpointKey, state string) bson.M {
	st := bson.M{
		"nodeName":          s.engine.Cfg.NodeName,
		"timeTag":           time.Now(),
		"openConnections":   s.server.GetSessionsLen(),
		"clientConnections": s.sessionCount(),
		"isConnected":       s.sessionCount() > 0,
		"endpoint": bson.M{endpointKey: bson.M{
			"timeTag":         time.Now(),
			"connectionState": state,
		}},
	}
	if id := s.engConn.Sec.StatsDoc(); id != nil {
		st["peerIdentity"] = id
	}
	return st
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
		h.s.engConn.Sec.NotImplemented(c, pack)
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
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Error loading connections: "+err.Error())
		os.Exit(-1)
	}
	if len(connCfgs) == 0 {
		jslog.Log(jslog.LevelBasic, "No connections found!")
		os.Exit(-1)
	}

	// security events: one writer for the process, one reporter per connection
	secWriter := jssecevents.NewWriter(0)
	go secWriter.Run(context.Background(), cfg)
	rtColl := func() *mongo.Collection {
		if db := engine.DB(); db != nil {
			return db.Collection(jsmongo.RealtimeDataCollectionName)
		}
		return nil
	}

	var servers []*srv104
	for _, cc := range connCfgs {
		s := &srv104{cfg: cc, engine: engine, sessions: map[asdu.Connect]string{}}
		engConn := srvapp.NewConn(cc)
		engConn.OriginatorIP = s.originatorIPs
		engConn.Sessions = s.sessionList
		rep := &iecsec.Reporter{W: secWriter, Role: "server", Protocol: "iec104",
			Ident: cc.PeerIdentificationVal(), Drop: iecsec.DropServerRefusals,
			Origin: jssecevents.Origin{ProtocolDriver: protocolDriverName, ProtocolDriverInstanceNumber: instanceNumber,
				ProtocolConnectionNumber: int(cc.ProtocolConnectionNumber), ConnectionName: cc.Name, NodeName: cfg.NodeName}}
		engConn.Sec = iecsec.NewServer(rep, iecsec.StoredIdentity(cc.Stats), rtColl)
		go engConn.Sec.Run(context.Background())
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
		srvCfg := cs104.Config{
			ConnectTimeout0:   time.Duration(defInt(int(cc.T0), 10)) * time.Second,
			SendUnAckLimitK:   uint16(defInt(int(cc.K), 12)),
			SendUnAckTimeout1: time.Duration(defInt(int(cc.T1), 15)) * time.Second,
			RecvUnAckLimitW:   uint16(defInt(int(cc.W), 8)),
			RecvUnAckTimeout2: time.Duration(defInt(int(cc.T2), 10)) * time.Second,
			IdleTimeout3:      time.Duration(defInt(int(cc.T3), 20)) * time.Second,
		}
		// go-iecp5 falls back to its default config when the parameters are
		// invalid (out of range, or t2 >= t1), logging it only at debug level
		chk := srvCfg
		if err := chk.Valid(); err != nil {
			jslog.Log(jslog.LevelBasic, "%s - Invalid k/w/t0..t3 parameters (%s): library defaults will be used instead.",
				cc.Name, err.Error())
		} else if 3*int(chk.RecvUnAckLimitW) > 2*int(chk.SendUnAckLimitK) {
			// allowed, but the standard recommends w <= 2/3 k
			jslog.Log(jslog.LevelBasic, "%s - Warning: w (%d) exceeds two thirds of k (%d), throughput may drop to one window per t2.",
				cc.Name, chk.RecvUnAckLimitW, chk.SendUnAckLimitK)
		}
		server.SetConfig(srvCfg)
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
		// go-iecp5 admits the masters (ipAddresses, maxClientConnections)
		// and reports what it refuses; it watches the sessions for protocol
		// violations, sequence errors, floods and control functions
		pol := iecsec.Policy(cc.PeerIdentificationVal())
		pol.Sink = s.engConn.Sec.Rep.Sink()
		if pol.AllowedPeers = s.allowedPeers(); pol.AllowedPeers != nil {
			pol.RejectUnlisted = true
		}
		pol.MaxSessions = maxClients
		server.SetSecurityPolicy(pol)

		sref := s
		server.SetOnConnectionHandler(func(c asdu.Connect) {
			remote := ""
			if uc := c.UnderlyingConn(); uc != nil {
				remote = uc.RemoteAddr().String()
			}
			jslog.Log(jslog.LevelBasic, "%s - New connection request from IP %s", sref.cfg.Name, remote)
			if allowed := sref.ipAllowed(remote); !allowed || sref.sessionCount() >= maxClients {
				jslog.Log(jslog.LevelBasic, "%s - Connection rejected from %s", sref.cfg.Name, remote)
				e := jssecevents.Event{Kind: string(secevent.KindConnUnexpected), Action: "rejected",
					Detail: "connection from an address not in ipAddresses"}
				if allowed {
					e.Kind, e.Detail = string(secevent.KindConnRejectedLimit),
						fmt.Sprintf("maxClientConnections (%d) reached", maxClients)
				}
				iecsec.SetPeer(&e, iecsec.PeerOf(c))
				sref.engConn.Sec.Rep.Emit(e)
				if uc := c.UnderlyingConn(); uc != nil {
					_ = uc.Close()
				}
				return
			}
			sref.sessMu.Lock()
			sref.sessions[c] = remote
			sref.sessMu.Unlock()
			sref.engConn.Sec.Opened(remote, iecsec.PeerOf(c))
			jslog.Log(jslog.LevelBasic, "%s - Connection event %s - OPENED", sref.cfg.Name, remote)
			go sref.updateConnStats(remote, "OPENED")
		})
		server.SetConnectionLostHandler(func(c asdu.Connect) {
			sref.sessMu.Lock()
			remote, ok := sref.sessions[c]
			delete(sref.sessions, c)
			sref.sessMu.Unlock()
			if !ok {
				return // refused in the connection handler
			}
			sref.engConn.Sec.Closed(remote)
			sref.engine.ForgetLink(sref.engConn, c)
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
	go engine.RunStatusLoop(instanceNumber, driverVersion)
	go engine.RunRealtimeStream()

	// main loop: keep MongoDB connection alive (blocks forever)
	engine.MaintainMongo()
}
