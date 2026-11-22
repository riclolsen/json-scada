/*
 * ICCP/TASE.2 Server Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-present - Ricardo L. Olsen
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

// Security events (secEvents collection) and identification of the
// clients.
//
// The library reports through the listener's security policy: connections
// from addresses outside ipAddresses, TLS failures (rejected client
// certificates), ACSE identity and authentication problems, malformed
// PDUs, unknown domains and objects, scans, controls and configuration
// changes, sequence errors, floods, MAC and certificate changes. The driver
// adds the clients' identity (address, MAC, ACSE AP title, certificate) in
// the statistics and its changes for the same address, and what only it
// knows about controls: the reason one was refused, commands of unknown
// objects or of the wrong type, floods and unnecessary commands.

package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/riclolsen/tase2/netpeer"
	"github.com/riclolsen/tase2/secevent"
	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	// Control flooding: more than controlFloodMax controls from one client
	// IP within controlFloodWindow.
	controlFloodMax    = 10
	controlFloodWindow = 10 * time.Second
	// maxKnownPeers bounds the client identities kept in the statistics.
	maxKnownPeers = 64
)

// secWriter delivers the events of every connection; nil (tests) keeps
// them in no collection.
var (
	secWriter   *jssecevents.Writer
	secNodeName string
)

// srvSec is the security event state of one listening connection; nil
// reports nothing.
type srvSec struct {
	name   string
	origin jssecevents.Origin
	ident  bool // peerIdentification

	ids   jssecevents.Tracker // ip -> ACSE identity
	flood jssecevents.RateLimit

	mu    sync.Mutex
	open  map[*tase2.Server]bson.M // open associations
	known map[string]bson.M        // last identity per client IP

	rt     *mongo.Collection // realtimeData, for the unnecessary-command check
	checks chan cmdCheck
}

// newSrvSec creates the state of a connection and seeds the change
// detection with the client identities stored in its statistics.
func newSrvSec(conn protocolConnection, rt *mongo.Collection) *srvSec {
	s := &srvSec{
		name: conn.Name,
		origin: jssecevents.Origin{ProtocolDriver: DriverName,
			ProtocolDriverInstanceNumber: conn.ProtocolDriverInstanceNumber,
			ProtocolConnectionNumber:     conn.ProtocolConnectionNumber,
			ConnectionName:               conn.Name, NodeName: secNodeName},
		ident:  conn.peerIdentificationOn(),
		flood:  jssecevents.RateLimit{Max: controlFloodMax, Window: controlFloodWindow},
		open:   map[*tase2.Server]bson.M{},
		known:  map[string]bson.M{},
		rt:     rt,
		checks: make(chan cmdCheck, 256),
	}
	s.seed(conn.Stats.PeerIdentity)
	if rt != nil {
		go s.checkLoop()
	}
	return s
}

// storedKnown is one stats.peerIdentity.known entry read back at startup.
type storedKnown struct {
	IP           string    `bson:"ip"`
	MAC          string    `bson:"mac"`
	MACSource    string    `bson:"macSource"`
	ACSEIdentity string    `bson:"acseIdentity"`
	TLSSHA256    string    `bson:"tlsSha256"`
	LastSeen     time.Time `bson:"lastSeen"`
}

// seed starts the change detection from the stored identities, so a change
// across a driver restart is seen: the ACSE identity per address here, the
// MAC and the certificate in the library's peer history (which then
// reports them).
func (s *srvSec) seed(raw bson.Raw) {
	if len(raw) == 0 {
		return
	}
	var st struct {
		Known []storedKnown `bson:"known"`
	}
	if err := bson.Unmarshal(raw, &st); err != nil {
		return
	}
	for _, k := range st.Known {
		ip, err := netip.ParseAddr(k.IP)
		if err != nil {
			continue
		}
		at := k.LastSeen
		if at.IsZero() || at.After(time.Now()) {
			at = time.Now()
		}
		s.ids.Seed(k.IP, k.ACSEIdentity)
		entry := bson.M{"ip": k.IP, "lastSeen": at}
		for f, v := range map[string]string{"mac": k.MAC, "macSource": k.MACSource,
			"acseIdentity": k.ACSEIdentity, "tlsSha256": k.TLSSHA256} {
			if v != "" {
				entry[f] = v
			}
		}
		s.known[k.IP] = entry
		if mac, err := net.ParseMAC(k.MAC); err == nil && k.MACSource == string(secevent.MACNeighbor) {
			netpeer.DefaultCache().Observe(netpeer.Observation{IP: ip, MAC: mac, Source: netpeer.SourceNeighbor, Time: at})
		}
		if k.TLSSHA256 != "" {
			// The server observes a client's certificate with the calling AP
			// title it stated (by IP without one).
			ap, _, _ := strings.Cut(k.ACSEIdentity, "/")
			netpeer.DefaultCache().Observe(netpeer.Observation{IP: ip, CertFingerprint: k.TLSSHA256, APTitle: ap, Time: at})
		}
	}
	s.trimKnown()
}

// policy is the library's report-only security policy of the listener:
// ipAddresses, when set, are the expected client addresses (others are
// reported, not refused).
func (s *srvSec) policy(ipAddresses []string) tase2.SecurityPolicy {
	p := tase2.DefaultSecurityPolicy()
	p.Sink = secevent.SinkFunc(s.libraryEvent)
	for _, a := range ipAddresses {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if pr, err := netip.ParsePrefix(a); err == nil {
			p.AllowedPeers = append(p.AllowedPeers, pr.Masked())
		} else if ip, err := netip.ParseAddr(a); err == nil {
			ip = ip.Unmap()
			p.AllowedPeers = append(p.AllowedPeers, netip.PrefixFrom(ip, ip.BitLen()))
		} else {
			LogMsg(LogLevelMin, "ICCP - Connection %q: invalid ipAddresses entry %q ignored", s.name, a)
		}
	}
	return p
}

// libraryEvent stores an event of the library. The routine informational
// ones (connection accepted, association established, closed) are not
// stored, and the controls refused by the driver's handlers are reported by
// the driver, with the reason.
func (s *srvSec) libraryEvent(e secevent.Event) {
	if secWriter == nil {
		LogMsg(LogLevelDetailed, "ICCP %s - Security event %s", s.name, e.String())
		return
	}
	if e.Severity == secevent.SeverityInfo && kindInfo(e.Kind).Severity == secevent.SeverityInfo {
		return
	}
	if e.Kind == secevent.KindCmdRejected && e.Reason == "handler" {
		return
	}
	if e.Role == "" {
		e.Role = "server"
	}
	secWriter.Emit(s.origin, toSecDoc(e, secSourceLibrary))
}

// emit stores a driver event about a client; pi identifies it.
func (s *srvSec) emit(pi secevent.PeerInfo, e secevent.Event) {
	if s == nil {
		return
	}
	if !e.Peer.RemoteAddr.IsValid() {
		e.Peer = pi
	}
	e.Protocol, e.Role = "iccp", "server"
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if secWriter == nil {
		LogMsg(LogLevelDetailed, "ICCP %s - Security event %s", s.name, e.String())
		return
	}
	secWriter.Emit(s.origin, toSecDoc(e, secSourceDriver))
}

// --- identification -----------------------------------------------------

// opened records the identity of a new association (its ACSE identity is
// known once the association is accepted) and reports what changed since
// the same address last associated.
func (s *srvSec) opened(srv *tase2.Server, pi secevent.PeerInfo) {
	if s == nil || !s.ident || !pi.RemoteAddr.IsValid() {
		return
	}
	ip := pi.RemoteAddr.Addr().Unmap().String()
	acse := acseIdentity(pi)
	now := time.Now()
	id := bson.M{"address": pi.RemoteAddr.String(), "ip": ip, "connectedAt": now,
		"authenticated": pi.Authenticated}
	if len(pi.MAC) > 0 {
		id["mac"], id["macSource"] = pi.MAC.String(), string(pi.MACSource)
	}
	if acse != "" {
		id["acseIdentity"] = acse
	}
	if pi.BLT != "" {
		id["blt"] = pi.BLT
	}
	if pi.TLS != nil {
		id["tls"] = tlsDoc(pi.TLS)
	}

	s.mu.Lock()
	_, seen := s.known[ip]
	s.mu.Unlock()
	if !seen {
		s.emit(pi, secevent.Event{Kind: kindPeerIdentified, Value: identityText(id),
			Detail: "first association from this address"})
	}
	if prev, changed := s.ids.Observe(ip, acse); changed {
		s.emit(pi, secevent.Event{Kind: kindPeerIdentityChanged, Value: acse,
			Detail: "ACSE identity (AP title/AE qualifier) of this address was " + prev})
	}

	k := bson.M{"ip": ip, "lastSeen": now}
	for _, f := range []string{"mac", "macSource", "acseIdentity"} {
		if v, ok := id[f]; ok {
			k[f] = v
		}
	}
	if pi.TLS != nil && pi.TLS.FingerprintSHA256 != "" {
		k["tlsSha256"] = pi.TLS.FingerprintSHA256
	}
	s.mu.Lock()
	s.open[srv] = id
	s.known[ip] = k
	s.mu.Unlock()
	s.trimKnown()
}

// trimKnown forgets the oldest identities beyond maxKnownPeers.
func (s *srvSec) trimKnown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.known) > maxKnownPeers {
		oldest, at := "", time.Time{}
		for ip, v := range s.known {
			t, _ := v["lastSeen"].(time.Time)
			if oldest == "" || t.Before(at) {
				oldest, at = ip, t
			}
		}
		delete(s.known, oldest)
	}
}

// closed forgets an ended association.
func (s *srvSec) closed(srv *tase2.Server) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.open, srv)
	s.mu.Unlock()
}

func identityText(id bson.M) string {
	var parts []string
	for _, f := range []string{"acseIdentity", "mac"} {
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

// statsDoc is stats.peerIdentity: the open associations, oldest first, and
// the last identity of every address seen; nil when identification is off.
func (s *srvSec) statsDoc() bson.M {
	if s == nil || !s.ident {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	open := make([]bson.M, 0, len(s.open))
	for _, id := range s.open {
		open = append(open, id)
	}
	sort.Slice(open, func(i, j int) bool {
		a, _ := open[i]["connectedAt"].(time.Time)
		b, _ := open[j]["connectedAt"].(time.Time)
		return a.Before(b)
	})
	clients := bson.A{}
	for _, id := range open {
		clients = append(clients, id)
	}
	ips := make([]string, 0, len(s.known))
	for ip := range s.known {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	known := bson.A{}
	for _, ip := range ips {
		known = append(known, s.known[ip])
	}
	return bson.M{"clients": clients, "known": known}
}

// --- controls -----------------------------------------------------------

// controlRefused reports a control the driver refused: kind is
// kindCmdObjectUnknown, kindCmdWrongType or secevent.KindCmdRejected.
func (s *srvSec) controlRefused(pi secevent.PeerInfo, kind secevent.Kind, service, domain, item, tag, value, why string) {
	if s == nil {
		return
	}
	detail := why
	if tag != "" {
		detail += " (tag " + tag + ")"
	}
	s.emit(pi, secevent.Event{Kind: kind, Action: secevent.ActionRejected, Service: service,
		Domain: domain, Item: item, Value: value, Reason: "driver", Detail: detail})
}

// controlReceived counts a control for the flood detector and queues the
// unnecessary-command check of a digital one.
func (s *srvSec) controlReceived(pi secevent.PeerInfo, rtDoc rtData, domain, item string, value float64, at time.Time) {
	if s == nil {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	key := pi.RemoteAddr.Addr().Unmap().String()
	if over, n := s.flood.Hit(key, at); over {
		s.emit(pi, secevent.Event{Kind: secevent.KindFloodControls, Domain: domain, Item: item, Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d controls within %s", controlFloodMax, controlFloodWindow)})
	}
	if rtDoc.Type != "digital" || s.rt == nil {
		return
	}
	select {
	case s.checks <- cmdCheck{pointKey: rtDoc.ID, tag: rtDoc.Tag, domain: domain, item: item,
		value: value, peer: pi, at: at}:
	default: // checks are best effort
	}
}

// cmdCheck is a digital control to compare with the state it commands.
type cmdCheck struct {
	pointKey     float64
	tag          string
	domain, item string
	value        float64
	peer         secevent.PeerInfo
	at           time.Time // received: the state must predate it
}

// checkLoop reports controls that would not change the state they command
// (closing a closed breaker): the supervised point of the command tag
// already holds the commanded value.
func (s *srvSec) checkLoop() {
	for chk := range s.checks {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		state, ok, err := jssecevents.SupervisedState(ctx, s.rt, chk.pointKey, chk.at)
		cancel()
		if err != nil {
			LogMsg(LogLevelDetailed, "ICCP %s - Command check: %v", s.name, err)
			continue
		}
		if ok && (state != 0) == (chk.value != 0) {
			s.emit(chk.peer, secevent.Event{Kind: kindCmdUnnecessary, Domain: chk.domain, Item: chk.item,
				Value:  fmt.Sprint(chk.value),
				Detail: "the supervised point already was in the commanded state (tag " + chk.tag + ")"})
		}
	}
}
