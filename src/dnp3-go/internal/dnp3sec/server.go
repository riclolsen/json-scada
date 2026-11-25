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

package dnp3sec

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/dscsystems/go-dnp3/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	// maxKnownPeers bounds the masters remembered in stats.peerIdentity.known.
	maxKnownPeers = 64
	// RefusalEvery is how often the same refusal of the same object is
	// stored: a master retrying a refused control does so every few seconds.
	RefusalEvery = time.Minute
	// refusalMemory is how long a refusal the driver reported keeps go-dnp3's
	// report of the same control out (DropServerRefusals).
	refusalMemory = 5 * time.Second
)

type objKey struct{ group, index int }

// Server is the security state of one outstation connection: the master
// connected and the masters known (stats.peerIdentity), the controls the
// driver refuses, and the check of the controls that would not change the
// state they command.
type Server struct {
	Rep *Reporter
	// RT returns the realtimeData collection for the command checks.
	RT func() *mongo.Collection
	// LinkAddress is the master's link address (the connection's
	// remoteLinkAddress: the bus delivers frames from it only).
	LinkAddress int

	mu       sync.Mutex
	open     bson.M            // the master connected now
	conn     uint64            // generation of the connection: Opening and Closed advance it
	known    map[string]bson.M // last identity of every master IP
	refused  map[objKey]time.Time
	throttle jssecevents.Throttle
	checks   chan cmdCheck
}

// NewServer starts the state from the masters stored in
// stats.peerIdentity.known; their MAC addresses, link addresses and
// certificates seed go-dnp3's change detection, so a change across a restart
// is reported.
func NewServer(rep *Reporter, stored bson.M, linkAddress int, rt func() *mongo.Collection) *Server {
	s := &Server{Rep: rep, RT: rt, LinkAddress: linkAddress, known: map[string]bson.M{},
		refused: map[objKey]time.Time{}, throttle: jssecevents.Throttle{Every: RefusalEvery},
		checks: make(chan cmdCheck, 256)}
	if !s.ident() {
		return s
	}
	for _, k := range jsmongo.GetDocArray(stored, "known") {
		ip := jsmongo.GetString(k, "ip", "")
		if ip == "" {
			continue
		}
		switch t := k["lastSeen"].(type) { // stored as a BSON date
		case bson.DateTime:
			k["lastSeen"] = t.Time()
		case time.Time:
		default:
			k["lastSeen"] = time.Time{} // unknown: the first to forget
		}
		s.known[ip] = k
		s.Rep.Seed(ip, jsmongo.GetString(k, "mac", ""), jsmongo.GetInt(k, "linkAddress", -1),
			jsmongo.GetString(k, "tlsSha256", ""))
	}
	return s
}

func (s *Server) ident() bool { return s != nil && s.Rep != nil && s.Rep.Ident }

// Opening starts a connection: its generation, for Opened.
func (s *Server) Opening() uint64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn++
	return s.conn
}

// Opened records the master of connection gen, as go-dnp3 describes it
// (address, MAC, TLS session), unless that connection has closed or been
// replaced meanwhile. Its changes (MAC, link address, certificate) are
// go-dnp3's to report; a first connection from an address is
// peer.identified.
func (s *Server) Opened(gen uint64, p secevent.PeerInfo) {
	if !s.ident() {
		return
	}
	id := peerBase(p)
	id["linkAddress"] = s.LinkAddress
	ip, _ := id["ip"].(string)
	k := bson.M{"ip": ip, "linkAddress": s.LinkAddress, "lastSeen": time.Now()}
	if mac, _ := id["mac"].(string); mac != "" {
		k["mac"], k["macSource"] = mac, id["macSource"]
	}
	if p.TLS != nil && p.TLS.FingerprintSHA256 != "" {
		k["tlsSha256"], k["tlsSubject"] = p.TLS.FingerprintSHA256, p.TLS.Subject
	}
	s.mu.Lock()
	if gen != s.conn {
		s.mu.Unlock()
		return
	}
	s.open = id
	_, seen := s.known[ip]
	if ip != "" {
		s.known[ip] = k
		if len(s.known) > maxKnownPeers { // forget the oldest
			oldest, at := "", time.Now()
			for kip, v := range s.known {
				if t, ok := v["lastSeen"].(time.Time); ok && t.Before(at) {
					oldest, at = kip, t
				}
			}
			delete(s.known, oldest)
		}
	}
	s.mu.Unlock()
	if !seen && ip != "" {
		e := jssecevents.Event{Kind: KindPeerIdentified, Value: Summary(id), Detail: "first connection from this address"}
		SetPeer(&e, p)
		s.Rep.Emit(e)
	}
}

// Closed forgets the connected master; a late Opened of the closed
// connection is ignored.
func (s *Server) Closed() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.conn++
	s.open = nil
	s.mu.Unlock()
}

// StatsDoc is stats.peerIdentity: the master connected now and the last
// identity of every master IP seen; nil when identification is off or
// nothing was seen.
func (s *Server) StatsDoc() bson.M {
	if !s.ident() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.open == nil && len(s.known) == 0 {
		return nil
	}
	clients := bson.A{}
	if s.open != nil {
		clients = append(clients, s.open)
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

// ConnectionRefused reports a connection the allowed-address list
// (ipAddresses) turned away.
func (s *Server) ConnectionRefused(p secevent.PeerInfo) {
	if s == nil {
		return
	}
	e := jssecevents.Event{Kind: string(secevent.KindConnUnexpected), Action: "rejected",
		Detail: "connection from an address not in ipAddresses"}
	SetPeer(&e, p)
	s.Rep.Emit(e)
}

// Refusal is a control the outstation refused.
type Refusal struct {
	Kind      string // cmd.rejected, cmd.object_unknown or cmd.wrong_type
	Service   string // SELECT or OPERATE
	Group     int    // 12 (CROB) or 41 (analog output)
	Variation int
	Index     int
	Tag       string
	Value     string // the control, as text
	Status    string // the command status answered
	Why       string
}

// Refused reports a control the driver refused, with the reason. The same
// refusal of the same object from the same master address is stored once per
// RefusalEvery, with the count of the ones left out.
func (s *Server) Refused(p secevent.PeerInfo, r Refusal) {
	if s == nil {
		return
	}
	key := objKey{r.Group, r.Index}
	s.mu.Lock()
	s.refused[key] = time.Now()
	if len(s.refused) > 4096 {
		for k, t := range s.refused {
			if time.Since(t) > refusalMemory {
				delete(s.refused, k)
			}
		}
	}
	s.mu.Unlock()
	from := p.String()
	if p.RemoteAddr.IsValid() {
		from = p.RemoteAddr.Addr().String() // the port changes on every connection
	}
	ok, n := s.throttle.Allow(fmt.Sprintf("%s|%s|%d|%d|%s", from, r.Kind, r.Group, r.Index, r.Why), time.Now())
	if !ok {
		return
	}
	e := jssecevents.Event{Kind: r.Kind, Action: "rejected", Tag: r.Tag, FC: r.Service, Value: r.Value,
		ObjectRef: ObjectRef(r.Group, r.Variation, r.Index), Detail: r.Why,
		Fields: bson.M{"group": r.Group, "index": r.Index, "commandStatus": r.Status}}
	if r.Variation > 0 {
		e.Fields["variation"] = r.Variation
	}
	if n > 1 {
		e.Count = n
	}
	SetPeer(&e, p)
	s.Rep.Emit(e)
}

// DropServerRefusals is the Drop of a server: go-dnp3's report of a control
// the driver refused (and reported, with the tag and the reason) is left
// out.
func (s *Server) DropServerRefusals(e secevent.Event) bool {
	if e.Kind != secevent.KindCmdRejected && e.Kind != secevent.KindCmdLocalMode {
		return false
	}
	if e.Fields&secevent.HasObject == 0 || e.Fields&secevent.HasIndex == 0 {
		return false
	}
	s.mu.Lock()
	at, ok := s.refused[objKey{int(e.Group), int(e.Index)}]
	s.mu.Unlock()
	return ok && time.Since(at) < refusalMemory
}

// cmdCheck is a digital control to compare with the state it commands.
type cmdCheck struct {
	peer     secevent.PeerInfo
	pointKey float64
	tag, ref string
	value    float64
	at       time.Time // received: the state must predate it
}

// Forwarded queues the check of a digital control, forwarded to
// commandsQueue, against the state its supervised point was in before it
// arrived (received is when the request came in, before the command was
// queued: feedback may arrive meanwhile).
func (s *Server) Forwarded(p secevent.PeerInfo, tag string, pointKey, value float64, group, index int, received time.Time) {
	if s == nil || s.RT == nil || pointKey == 0 || (value != 0 && value != 1) {
		return
	}
	select {
	case s.checks <- cmdCheck{peer: p, pointKey: pointKey, tag: tag, value: value, at: received,
		ref: ObjectRef(group, 0, index)}:
	default: // the checks are best effort
	}
}

// Run checks the queued commands until ctx ends.
func (s *Server) Run(ctx context.Context) {
	for {
		var chk cmdCheck
		select {
		case <-ctx.Done():
			return
		case chk = <-s.checks:
		}
		rt := s.RT()
		if rt == nil {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		state, ok, err := jssecevents.SupervisedState(cctx, rt, chk.pointKey, chk.at)
		cancel()
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "secEvents - command check: %v", err)
			continue
		}
		if ok && (state != 0) == (chk.value != 0) {
			e := jssecevents.Event{Kind: KindCmdUnnecessary, Tag: chk.tag, ObjectRef: chk.ref, Value: fmt.Sprint(chk.value),
				Detail: "the supervised point already was in the commanded state"}
			SetPeer(&e, chk.peer)
			s.Rep.Emit(e)
		}
	}
}
