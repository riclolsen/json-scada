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

// Security events (secEvents collection) and peer identification.
//
// The library reports transport, TLS and association events through its
// secevent dispatcher; the driver adds what only it can see: clients
// outside the allow-list, rejected certificates with the client's address,
// refused, flooding and unnecessary controls, and changes of a client's
// MAC address, ACSE identity or certificate for the same IP address.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Control flooding: more than controlFloodMax controls from one client IP
// within controlFloodWindow.
const (
	controlFloodMax    = 10
	controlFloodWindow = 10 * time.Second
	maxKnownPeers      = 64
)

// secState is the driver's security event state; nil (tests, self-test)
// reports nothing.
type secState struct {
	writer *jssecevents.Writer
	origin jssecevents.Origin
	disp   *secevent.Dispatcher
	ident  bool // peerIdentification

	macs  jssecevents.Tracker // ip -> MAC
	ids   jssecevents.Tracker // ip -> ACSE calling AP title / AE qualifier
	certs jssecevents.Tracker // ip -> certificate fingerprint
	flood jssecevents.RateLimit

	mu    sync.Mutex
	open  map[string]bson.M // open associations by peer address
	known map[string]bson.M // last identity per peer IP

	checks chan cmdCheck
}

// sec is set by main before the gateway is created.
var sec *secState

// newSecState creates the state of a connection and seeds the change
// trackers with the identities stored in its statistics.
func newSecState(conn *ServerConnection, nodeName string) *secState {
	s := &secState{
		writer: jssecevents.NewWriter(0),
		origin: jssecevents.Origin{ProtocolDriver: ProtocolDriverName,
			ProtocolDriverInstanceNumber: conn.ProtocolDriverInstanceNumber,
			ProtocolConnectionNumber:     conn.ProtocolConnectionNumber,
			ConnectionName:               conn.Name, NodeName: nodeName},
		ident:  conn.PeerIdentification,
		flood:  jssecevents.RateLimit{Max: controlFloodMax, Window: controlFloodWindow},
		open:   map[string]bson.M{},
		known:  map[string]bson.M{},
		checks: make(chan cmdCheck, 256),
	}
	for _, k := range conn.KnownPeers {
		ip := jsmongo.GetString(k, "ip", "")
		if ip == "" {
			continue
		}
		s.macs.Seed(ip, jsmongo.GetString(k, "mac", ""))
		s.ids.Seed(ip, jsmongo.GetString(k, "acseIdentity", ""))
		s.certs.Seed(ip, jsmongo.GetString(k, "tlsSha256", ""))
		s.known[ip] = k
	}
	// Library events: the normal ones (connection accepted, association
	// established, closed) are informational and not stored.
	s.disp = secevent.NewDispatcher(secevent.SinkFunc(func(e secevent.Event) {
		if e.Severity == secevent.Info && e.Kind != secevent.KindConnNeverAssociated {
			return
		}
		s.writer.Emit(s.origin, toSecDoc(e, secSourceLibrary))
	}), secevent.Options{})
	return s
}

// run delivers the events and checks the controls until ctx ends.
func (s *secState) run(ctx context.Context, cfg jsconfig.Config) {
	if s == nil {
		return
	}
	go s.writer.Run(ctx, cfg)
	go s.checkLoop(ctx, cfg)
}

// close flushes the library dispatcher once the server has stopped.
func (s *secState) close() {
	if s != nil {
		_ = s.disp.Close()
	}
}

// emit stores a driver event about the client of an association.
func (s *secState) emit(c *mms.ServerConn, e secevent.Event) {
	if s == nil {
		return
	}
	if c != nil {
		if !e.Peer.RemoteAddr.IsValid() {
			e.Peer = c.PeerInfo()
		}
		if e.SessionID == 0 {
			e.SessionID = c.SessionID()
		}
	}
	if e.Protocol == "" {
		e.Protocol = "mms"
	}
	e.Role = "server"
	s.writer.Emit(s.origin, toSecDoc(e, secSourceDriver))
}

// opened records a new association at once; its identification runs in a
// goroutine of its own (the MAC lookup may run a command and the connection
// handler must not wait) and completes the entry only while the same
// session is still open.
func (s *secState) opened(c *mms.ServerConn, addr string) {
	if s == nil || c == nil || !s.ident {
		return
	}
	pi := c.PeerInfo()
	session := c.SessionID()
	if !pi.RemoteAddr.IsValid() {
		return
	}
	s.mu.Lock()
	s.open[addr] = bson.M{"address": addr, "ip": pi.RemoteAddr.Addr().Unmap().String(),
		"sessionId": int64(session), "connectedAt": time.Now()}
	s.mu.Unlock()
	go s.identify(pi, session, addr)
}

func (s *secState) closed(addr string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.open, addr)
	s.mu.Unlock()
}

// acseIdentity is the identity a client stated at association.
func acseIdentity(a *secevent.ACSEPeer) string {
	if a == nil || a.CallingAPTitle == "" {
		return ""
	}
	if a.CallingAEQualifier != nil {
		return fmt.Sprintf("%s/%d", a.CallingAPTitle, *a.CallingAEQualifier)
	}
	return a.CallingAPTitle
}

// identify builds the identity of a client, stores it for the statistics
// and reports what changed since the same IP address last connected.
func (s *secState) identify(pi secevent.PeerInfo, session uint64, addr string) {
	if !pi.RemoteAddr.IsValid() {
		return
	}
	ip := pi.RemoteAddr.Addr().Unmap()
	ipS := ip.String()
	mac := jssecevents.LookupMAC(ip)
	acse := acseIdentity(pi.ACSE)
	fp := ""
	id := bson.M{"address": addr, "ip": ipS, "sessionId": int64(session),
		"connectedAt": time.Now()}
	if mac != "" {
		id["mac"] = mac
	}
	if acse != "" {
		id["acseIdentity"] = acse
	}
	if pi.ACSE != nil {
		id["passwordPresented"] = pi.ACSE.PasswordPresented
	}
	if pi.TLS != nil {
		id["tls"] = tlsDoc(pi.TLS)
		fp = pi.TLS.FingerprintSHA256
	}

	base := secevent.Event{Peer: pi, SessionID: session, Protocol: "mms"}
	s.mu.Lock()
	_, seen := s.known[ipS]
	s.mu.Unlock()
	if !seen {
		e := base
		e.Kind, e.Detail = kindPeerIdentified, "first association from this address"
		e.Value = identityText(id)
		s.emit(nil, e)
	}
	if prev, ch := s.macs.Observe(ipS, mac); ch {
		e := base
		e.Kind, e.Value, e.Detail = secevent.KindPeerMACChanged, mac, "MAC address was "+prev
		s.emit(nil, e)
	}
	if prev, ch := s.ids.Observe(ipS, acse); ch {
		e := base
		e.Kind, e.Value, e.Detail = secevent.KindPeerIdentityChanged, acse, "ACSE identity was "+prev
		s.emit(nil, e)
	}
	if prev, ch := s.certs.Observe(ipS, fp); ch {
		e := base
		e.Kind, e.Value, e.Detail = secevent.KindTLSCertChanged, fp, "certificate fingerprint was "+prev
		s.emit(nil, e)
	}

	k := bson.M{"ip": ipS, "lastSeen": time.Now()}
	for _, f := range []string{"mac", "acseIdentity"} {
		if v, ok := id[f]; ok {
			k[f] = v
		}
	}
	if fp != "" {
		k["tlsSha256"] = fp
	}
	s.mu.Lock()
	if cur, ok := s.open[addr]; ok && cur["sessionId"] == int64(session) { // still the same session
		id["connectedAt"] = cur["connectedAt"]
		s.open[addr] = id
	}
	s.known[ipS] = k
	if len(s.known) > maxKnownPeers { // forget the oldest
		oldest, at := "", time.Now()
		for kip, v := range s.known {
			if t, ok := v["lastSeen"].(time.Time); ok && t.Before(at) {
				oldest, at = kip, t
			}
		}
		delete(s.known, oldest)
	}
	s.mu.Unlock()
}

func identityText(id bson.M) string {
	var parts []string
	for _, f := range []string{"mac", "acseIdentity"} {
		if v, ok := id[f].(string); ok && v != "" {
			parts = append(parts, f+"="+v)
		}
	}
	if t, ok := id["tls"].(bson.M); ok {
		if v, ok := t["subject"].(string); ok {
			parts = append(parts, "tls="+v)
		}
	}
	return strings.Join(parts, " ")
}

// statsDoc is stats.peerIdentity: the open associations and the last
// identity of every address seen; nil when identification is off.
func (s *secState) statsDoc() bson.M {
	if s == nil || !s.ident {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clients := bson.A{}
	addrs := make([]string, 0, len(s.open))
	for a := range s.open {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)
	for _, a := range addrs {
		clients = append(clients, s.open[a])
	}
	known := bson.A{}
	ips := make([]string, 0, len(s.known))
	for ip := range s.known {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	for _, ip := range ips {
		known = append(known, s.known[ip])
	}
	return bson.M{"clients": clients, "known": known}
}

// --- TLS ----------------------------------------------------------------

// withCertEvents makes the server report every client certificate its
// verifier refuses, with the client's address.
func (s *secState) withCertEvents(cfg *tls.Config) *tls.Config {
	if s == nil || cfg == nil || cfg.VerifyPeerCertificate == nil {
		return cfg
	}
	verify := cfg.VerifyPeerCertificate
	base := cfg.Clone()
	cfg.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		c := base.Clone()
		var pi secevent.PeerInfo
		if hello.Conn != nil {
			pi = secevent.PeerInfo{Network: "tcp", RemoteAddr: secevent.AddrPort(hello.Conn.RemoteAddr()),
				LocalAddr: secevent.AddrPort(hello.Conn.LocalAddr())}
		}
		c.VerifyPeerCertificate = func(raw [][]byte, chains [][]*x509.Certificate) error {
			err := verify(raw, chains)
			if err != nil {
				s.certRejected(pi, raw, err)
			}
			return err
		}
		return c, nil
	}
	return cfg
}

func (s *secState) certRejected(pi secevent.PeerInfo, raw [][]byte, err error) {
	t := &secevent.TLSPeer{}
	if len(raw) > 0 {
		if c, perr := x509.ParseCertificate(raw[0]); perr == nil {
			t.CertFields(c)
		} else {
			t.FingerprintSHA256 = secevent.Fingerprint(raw[0])
		}
	}
	pi.TLS = t
	s.emit(nil, secevent.Event{Kind: secevent.KindTLSCertRejected, Protocol: "tls", Peer: pi,
		Action: secevent.Disconnected, Err: err, Detail: "client certificate refused"})
}

// --- Controls -----------------------------------------------------------

// controlRefused reports a control the handler refused.
func (s *secState) controlRefused(c *mms.ServerConn, mp *MappedPoint, cause model.AddCause, test bool, ctlNum uint8, why string) {
	if s == nil {
		return
	}
	s.emit(c, secevent.Event{Kind: secevent.KindCmdRejected, Action: secevent.Rejected,
		ObjectRef: string(mp.ObjRef), Detail: why + " (tag " + mp.Tag + ")",
		Fields:   secevent.HasAddCause | secevent.HasTest | secevent.HasCtlNum,
		AddCause: int(cause), Test: test, CtlNum: ctlNum})
}

// controlReceived counts a control for the flood detector and queues the
// unnecessary-command check.
func (s *secState) controlReceived(c *mms.ServerConn, peer string, mp *MappedPoint, value float64) {
	if s == nil {
		return
	}
	ip := jssecevents.SplitAddr(peer).String()
	if over, n := s.flood.Hit(ip, time.Now()); over {
		s.emit(c, secevent.Event{Kind: secevent.KindFloodControls, Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d controls within %s", controlFloodMax, controlFloodWindow)})
	}
	if mp.Type != "digital" {
		return
	}
	chk := cmdCheck{pointKey: mp.PointKey, tag: mp.Tag, ref: string(mp.ObjRef), value: value, at: time.Now()}
	if c != nil {
		chk.peer, chk.session = c.PeerInfo(), c.SessionID()
	}
	select {
	case s.checks <- chk:
	default: // checks are best effort
	}
}

// cmdCheck is a digital control to compare with the state it commands.
type cmdCheck struct {
	pointKey float64
	tag, ref string
	value    float64
	peer     secevent.PeerInfo
	session  uint64
	at       time.Time // received: the state must predate it
}

// checkLoop reports controls that would not change the state they command
// (closing a closed breaker): the supervised point of the command tag
// already holds the commanded value.
func (s *secState) checkLoop(ctx context.Context, cfg jsconfig.Config) {
	var cli *mongo.Client
	var rt *mongo.Collection
	defer func() {
		if cli != nil {
			_ = cli.Disconnect(context.Background())
		}
	}()
	for {
		var chk cmdCheck
		select {
		case <-ctx.Done():
			return
		case chk = <-s.checks:
		}
		if rt == nil {
			c, db, err := jsmongo.ConnectAndPing(cfg)
			if err != nil {
				continue
			}
			cli, rt = c, db.Collection(jsmongo.RealtimeDataCollectionName)
		}
		state, ok, err := jssecevents.SupervisedState(ctx, rt, chk.pointKey, chk.at)
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "secEvents - command check: %v", err)
			_ = cli.Disconnect(context.Background())
			cli, rt = nil, nil
			continue
		}
		if ok && (state != 0) == (chk.value != 0) {
			s.emit(nil, secevent.Event{Kind: kindCmdUnnecessary, Peer: chk.peer, SessionID: chk.session,
				ObjectRef: chk.ref, Value: fmt.Sprint(chk.value),
				Detail: "the supervised point already was in the commanded state (tag " + chk.tag + ")"})
		}
	}
}
