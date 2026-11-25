/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package iecsec

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/secevent"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Server detection limits.
const (
	// ScanMax distinct nonexistent objects asked by one master within
	// ScanWindow are a scan.
	ScanMax    = 20
	ScanWindow = time.Minute
	// maxKnownPeers bounds the masters remembered in stats.peerIdentity.known.
	maxKnownPeers = 64
)

// Server is the security state of one served (outstation) connection: the
// masters connected and known (stats.peerIdentity), requests for objects
// that do not exist, refused commands and the check of the commands that
// would not change the state they command.
type Server struct {
	Rep *Reporter
	// RT returns the realtimeData collection for the command checks.
	RT func() *mongo.Collection

	mu       sync.Mutex
	open     map[string]bson.M // sessions by master address
	known    map[string]bson.M // last identity of every master IP
	scan     jssecevents.RateLimit
	scanSeen jssecevents.Throttle
	checks   chan cmdCheck
}

// NewServer starts the state from the masters stored in
// stats.peerIdentity.known; their MAC addresses and certificates seed
// go-iecp5's change detection, so a change across a restart is reported.
func NewServer(rep *Reporter, stored bson.M, rt func() *mongo.Collection) *Server {
	s := &Server{Rep: rep, RT: rt, open: map[string]bson.M{}, known: map[string]bson.M{},
		scan:     jssecevents.RateLimit{Max: ScanMax, Window: ScanWindow},
		scanSeen: jssecevents.Throttle{Every: ScanWindow},
		checks:   make(chan cmdCheck, 256)}
	if rep == nil || !rep.Ident {
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
		SeedLibCache(ip, jsmongo.GetString(k, "mac", ""), jsmongo.GetString(k, "tlsSha256", ""))
	}
	return s
}

// Opened records a master's session (104). The MAC address and certificate
// come from go-iecp5's admission, which reports their changes itself.
func (s *Server) Opened(remote string, p secevent.PeerInfo) {
	if s == nil || s.Rep == nil || !s.Rep.Ident {
		return
	}
	id := bson.M{"address": remote, "connectedAt": time.Now()}
	ip := ""
	if p.RemoteAddr.IsValid() {
		ip = p.RemoteAddr.Addr().Unmap().String()
		id["ip"] = ip
	}
	k := bson.M{"ip": ip, "lastSeen": time.Now()}
	if p.MAC != nil && p.MACSource != secevent.MACGateway {
		id["mac"], k["mac"] = p.MAC.String(), p.MAC.String()
	}
	if p.TLS != nil {
		t := TLSDoc(p.TLS)
		id["tls"] = t
		if p.TLS.FingerprintSHA256 != "" {
			k["tlsSha256"], k["tlsSubject"] = p.TLS.FingerprintSHA256, p.TLS.Subject
		}
	}
	s.mu.Lock()
	s.open[remote] = id
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

// Closed forgets a master's session.
func (s *Server) Closed(remote string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.open, remote)
	s.mu.Unlock()
}

// StatsDoc is stats.peerIdentity: the open sessions and the last identity of
// every master IP seen; nil when identification is off or nothing was seen
// (the 101 server, which does not identify its masters).
func (s *Server) StatsDoc() bson.M {
	if s == nil || s.Rep == nil || !s.Rep.Ident {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.open) == 0 && len(s.known) == 0 {
		return nil
	}
	addrs := make([]string, 0, len(s.open))
	for a := range s.open {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)
	clients := bson.A{}
	for _, a := range addrs {
		clients = append(clients, s.open[a])
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

// emit stores an event about a request received on link c.
func (s *Server) emit(c asdu.Connect, e jssecevents.Event) {
	if s == nil {
		return
	}
	SetPeer(&e, PeerOf(c))
	s.Rep.Emit(e)
}

func peerKey(c asdu.Connect) string {
	p := PeerOf(c)
	if p.RemoteAddr.IsValid() {
		return p.RemoteAddr.Addr().Unmap().String()
	}
	return "serial"
}

// noteUnknown feeds the scan detector: one master asking for many distinct
// objects that do not exist is enumerating the station.
func (s *Server) noteUnknown(c asdu.Connect, pack *asdu.ASDU, ioa int) {
	who := peerKey(c)
	now := time.Now()
	if fresh, _ := s.scanSeen.Allow(fmt.Sprintf("%s|%d|%d|%d", who, pack.Type, pack.CommonAddr, ioa), now); !fresh {
		return // the same object again: polling, not a scan
	}
	if over, n := s.scan.Hit(who, now); over {
		s.emit(c, jssecevents.Event{Kind: string(secevent.KindAccessScan), Fields: PackFields(pack), Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d distinct nonexistent objects requested within %s: the station is being enumerated",
				ScanMax, ScanWindow)})
	}
}

// UnknownRead reports a read (C_RD_NA_1) of an object the station does not
// have.
func (s *Server) UnknownRead(c asdu.Connect, pack *asdu.ASDU, ioa int) {
	if s == nil {
		return
	}
	s.emit(c, jssecevents.Event{Kind: string(secevent.KindAccessUnknownIOA), Action: "rejected", Fields: PackFields(pack),
		ObjectRef: ObjectRef(int(pack.CommonAddr), ioa), Detail: "read of an object that does not exist"})
	s.noteUnknown(c, pack, ioa)
}

// UnknownCommand reports a command to an object the station does not have.
func (s *Server) UnknownCommand(c asdu.Connect, pack *asdu.ASDU, ioa int) {
	if s == nil {
		return
	}
	e := jssecevents.Event{Kind: KindCmdObjectUnknown, Action: "rejected", Fields: PackFields(pack),
		ObjectRef: ObjectRef(int(pack.CommonAddr), ioa), Detail: "command to an object that does not exist (no tag maps it)"}
	if asdu.IsProcessCommand(pack.Type) {
		e.Value = asdu.DescribeCommand(pack)
	}
	s.emit(c, e)
	s.noteUnknown(c, pack, ioa)
}

// UnknownCA reports a request to a common address the station does not
// serve.
func (s *Server) UnknownCA(c asdu.Connect, pack *asdu.ASDU, what string) {
	if s == nil {
		return
	}
	s.emit(c, jssecevents.Event{Kind: string(secevent.KindAccessUnknownCA), Fields: PackFields(pack),
		ObjectRef: ObjectRef(int(pack.CommonAddr), -1),
		Detail:    what + " to a common address no tag is mapped to"})
	s.noteUnknown(c, pack, -1)
}

// Refused reports a command the station refused, with the reason.
func (s *Server) Refused(c asdu.Connect, pack *asdu.ASDU, ioa int, tag, why string) {
	if s == nil {
		return
	}
	e := jssecevents.Event{Kind: string(secevent.KindCmdRejected), Action: "rejected", Tag: tag, Fields: PackFields(pack),
		ObjectRef: ObjectRef(int(pack.CommonAddr), ioa), Detail: "command refused: " + why}
	if asdu.IsProcessCommand(pack.Type) {
		e.Value = asdu.DescribeCommand(pack)
	}
	s.emit(c, e)
}

// ExecuteWithoutSelect reports the execute of a select-before-operate
// command this master did not select (or selected too long ago).
func (s *Server) ExecuteWithoutSelect(c asdu.Connect, pack *asdu.ASDU, ioa int, tag string) {
	if s == nil {
		return
	}
	e := jssecevents.Event{Kind: string(secevent.KindCmdExecNoSelect), Action: "rejected", Tag: tag,
		Fields: PackFields(pack), ObjectRef: ObjectRef(int(pack.CommonAddr), ioa),
		Detail: "execute of a select-before-operate command without a valid select by this master"}
	if asdu.IsProcessCommand(pack.Type) {
		e.Value = asdu.DescribeCommand(pack)
	}
	s.emit(c, e)
}

// StaleTime reports a time-tagged command outside the accepted window:
// replayed, delayed, or sent with a wrong clock.
func (s *Server) StaleTime(c asdu.Connect, pack *asdu.ASDU, ioa int, tag string, cmdTime time.Time, window time.Duration) {
	if s == nil {
		return
	}
	s.emit(c, jssecevents.Event{Kind: string(secevent.KindCmdStaleTimeTag), Action: "rejected", Tag: tag,
		Fields: PackFields(pack), ObjectRef: ObjectRef(int(pack.CommonAddr), ioa),
		Value: cmdTime.UTC().Format(time.RFC3339Nano),
		Detail: fmt.Sprintf("command time %v from the local clock, outside the %v window: a replayed command or clocks out of step",
			time.Until(cmdTime).Round(time.Millisecond), window)})
}

// NotImplemented reports a request of a type the station does not serve. A
// file transfer request (upload of a file or directory from the station) is
// reported as such.
func (s *Server) NotImplemented(c asdu.Connect, pack *asdu.ASDU) {
	if s == nil {
		return
	}
	t := pack.Type
	if asdu.IsFile(t) && asdu.IsMonitorDirection(t) {
		return // go-iecp5 reports it (file.download_attempt)
	}
	e := jssecevents.Event{Kind: string(secevent.KindAccessUnknownType), Action: "rejected", Fields: PackFields(pack),
		ObjectRef: ObjectRef(int(pack.CommonAddr), -1), Detail: "request of a type the station does not serve: " + t.String()}
	if asdu.IsFile(t) {
		e.Kind = string(secevent.KindFileUpload)
		e.Detail = "file transfer request (" + t.String() + ") refused: the station serves no files"
	}
	s.emit(c, e)
}

// cmdCheck is a digital command to compare with the state it commands.
type cmdCheck struct {
	peer     secevent.PeerInfo
	pointKey float64
	tag, ref string
	value    float64
	fields   bson.M
	at       time.Time // received: the state must predate it
}

// Forwarded queues the check of a digital command, forwarded to the
// commandsQueue, against the state its supervised point was in before it
// arrived (received is when the request came in, before the command was
// queued and confirmed: feedback may arrive meanwhile).
func (s *Server) Forwarded(c asdu.Connect, pack *asdu.ASDU, ioa int, tag string, pointKey, value float64, received time.Time) {
	if s == nil || s.RT == nil || pointKey == 0 || (value != 0 && value != 1) {
		return
	}
	switch pack.Type {
	case asdu.C_SC_NA_1, asdu.C_SC_TA_1, asdu.C_DC_NA_1, asdu.C_DC_TA_1:
	default:
		return
	}
	select {
	case s.checks <- cmdCheck{peer: PeerOf(c), pointKey: pointKey, tag: tag, value: value, at: received,
		ref: ObjectRef(int(pack.CommonAddr), ioa), fields: PackFields(pack)}:
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
		state, ok, err := jssecevents.SupervisedState(ctx, rt, chk.pointKey, chk.at)
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "secEvents - command check: %v", err)
			continue
		}
		if ok && (state != 0) == (chk.value != 0) {
			e := jssecevents.Event{Kind: KindCmdUnnecessary, Tag: chk.tag, ObjectRef: chk.ref, Value: fmt.Sprint(chk.value),
				Fields: chk.fields, Detail: "the supervised point already was in the commanded state"}
			SetPeer(&e, chk.peer)
			s.Rep.Emit(e)
		}
	}
}
