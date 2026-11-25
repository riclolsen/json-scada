/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

// Security events (secEvents collection) and the identification of masters.
//
// go-modbus reports what its server sees of a connection (malformed MBAP
// headers and PDUs, stalled writes); since it listens behind the front-end,
// the driver drops its connection events, which name the loopback, and
// reports the masters' own: refused by the allow-list or the client limit,
// refused certificates and handshakes, a new master and changes of the MAC
// address or certificate of an IP. The handler adds what only it knows:
// reads of addresses that do not exist (and scans of them), units not
// served, refused, flooding and unnecessary commands, writes stored in
// supervised points, and, through the PDU tap, diagnostics (restart,
// listen-only), file record transfers, device identification and function
// codes the server does not serve.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbnet"
	"modbus-go/internal/mbsec"

	"github.com/otfabric/go-modbus"
	"github.com/otfabric/go-modbus/secevent"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Detection limits.
const (
	// commandFloodMax commands relayed from one master IP within
	// commandFloodWindow are a flood.
	commandFloodMax    = 10
	commandFloodWindow = 10 * time.Second
	// scanMax reads of nonexistent addresses from one master IP within
	// scanWindow are a scan.
	scanMax    = 20
	scanWindow = time.Minute
	// repeatEvery is how often a repeating condition of one key (peer,
	// unit, address) is reported; the repeats in between are counted.
	repeatEvery   = 10 * time.Minute
	maxKnownPeers = 64
)

// FC8 diagnostics sub-functions that change the device's state.
var restartSubFunctions = map[uint16]string{
	0x0001: "restart communications option",
	0x0003: "change ASCII input delimiter",
	0x0004: "force listen only mode",
	0x000A: "clear counters and diagnostic register",
	0x0014: "clear overrun counter and flag",
}

// serverSec is the security event state of one MODBUS_SERVER connection.
type serverSec struct {
	cfg   Config
	addrs *mbnet.AddrMap

	mu  sync.Mutex
	rep *mbsec.Reporter
	rt  *mongo.Collection

	macs, certs jssecevents.Tracker // ip -> MAC, certificate fingerprint
	flood       jssecevents.RateLimit
	scan        jssecevents.RateLimit // distinct nonexistent addresses per IP
	scanSeen    jssecevents.Throttle  // ip|addresses already counted in the window
	repeat      jssecevents.Throttle

	open  map[string]bson.M // sessions by master address
	token map[string]uint64 // session of each open address, so a late identification of a closed one is dropped
	seq   uint64
	known map[string]bson.M // last identity per master IP

	checks chan cmdCheck
}

func newServerSec(cfg Config, addrs *mbnet.AddrMap) *serverSec {
	s := &serverSec{cfg: cfg, addrs: addrs,
		flood:    jssecevents.RateLimit{Max: commandFloodMax, Window: commandFloodWindow},
		scan:     jssecevents.RateLimit{Max: scanMax, Window: scanWindow},
		scanSeen: jssecevents.Throttle{Every: scanWindow},
		repeat:   jssecevents.Throttle{Every: repeatEvery},
		open:     map[string]bson.M{}, token: map[string]uint64{}, known: map[string]bson.M{},
		checks: make(chan cmdCheck, 256),
	}
	for _, k := range cfg.KnownPeers {
		ip := jsmongo.GetString(k, "ip", "")
		if ip == "" {
			continue
		}
		s.macs.Seed(ip, jsmongo.GetString(k, "mac", ""))
		s.certs.Seed(ip, jsmongo.GetString(k, "tlsSha256", ""))
		s.known[ip] = k
	}
	return s
}

func (s *serverSec) set(rep *mbsec.Reporter, rt *mongo.Collection) {
	s.mu.Lock()
	s.rep, s.rt = rep, rt
	s.mu.Unlock()
}

func (s *serverSec) reporter() (*mbsec.Reporter, *mongo.Collection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rep, s.rt
}

// emit stores an event about the master at remote ("" for none).
func (s *serverSec) emit(remote string, e jssecevents.Event) {
	mbsec.Peer(&e, remote)
	e.Role = "server"
	rep, _ := s.reporter()
	rep.Emit(e)
}

// emitRepeat stores e at most once per repeatEvery for key, with the count
// of the occurrences held back since the last one.
func (s *serverSec) emitRepeat(key, remote string, e jssecevents.Event) {
	ok, n := s.repeat.Allow(key, time.Now())
	if !ok {
		return
	}
	e.Count = n
	s.emit(remote, e)
}

func ipOf(remote string) string {
	if ip := jssecevents.SplitAddr(remote); ip.IsValid() {
		return ip.String()
	}
	return remote
}

// --- connections and TLS ----------------------------------------------------

func (s *serverSec) rejected(remote, reason string) {
	e := jssecevents.Event{Kind: mbsec.KindConnUnexpected, Action: "rejected",
		Detail: "master not in the ipAddresses allow-list"}
	if reason == mbnet.RejectLimit {
		e.Kind = mbsec.KindConnRejectedLimit
		e.Detail = fmt.Sprintf("maxClientConnections (%s) reached", mbcore.JSNumberString(s.cfg.MaxClientConnections))
	}
	s.emitRepeat(e.Kind+"|"+ipOf(remote), remote, e)
}

// handshake reports a failed TLS handshake: a refused client certificate,
// with its details when known, or another failure.
func (s *serverSec) handshake(remote string, _ *tls.ConnectionState, raw [][]byte, err error) {
	if err == nil {
		return
	}
	e := jssecevents.Event{Kind: mbsec.KindTLSHandshakeFailed, Protocol: "tls", Action: "disconnected",
		Error: err.Error(), Detail: "TLS handshake with the master failed"}
	if jssecevents.IsCertError(err) || len(raw) > 0 {
		e.Kind, e.Detail = mbsec.KindTLSCertRejected, "client certificate refused"
		cert := jssecevents.RawCertDoc(raw)
		var ve *tls.CertificateVerificationError
		if cert == nil && errors.As(err, &ve) && len(ve.UnverifiedCertificates) > 0 {
			cert = jssecevents.CertDoc(ve.UnverifiedCertificates[0])
		}
		if cert != nil {
			e.Peer = bson.M{"tls": cert}
		}
	}
	s.emitRepeat(e.Kind+"|"+ipOf(remote)+"|"+e.Error, remote, e)
}

// opened records a master's session at once; the identification runs on
// its own goroutine, as the MAC lookup may run a command, and completes the
// session only while it is still open.
func (s *serverSec) opened(remote string, cs *tls.ConnectionState) {
	if !s.cfg.PeerIdentification {
		return
	}
	ip := jssecevents.SplitAddr(remote)
	if !ip.IsValid() {
		return
	}
	id := bson.M{"address": remote, "ip": ip.String(), "connectedAt": time.Now()}
	s.mu.Lock()
	s.seq++
	tok := s.seq
	s.open[remote], s.token[remote] = id, tok
	s.mu.Unlock()
	go s.identify(remote, tok, id, cs)
}

func (s *serverSec) closed(remote string) {
	s.mu.Lock()
	delete(s.open, remote)
	delete(s.token, remote)
	s.mu.Unlock()
}

// identify builds the identity of a master, keeps it for the statistics and
// reports what changed since the same IP last connected. base is the
// session's registered document; it is copied, not changed.
func (s *serverSec) identify(remote string, tok uint64, base bson.M, cs *tls.ConnectionState) {
	ip := jssecevents.SplitAddr(remote)
	ipS := ip.String()
	id := bson.M{}
	for k, v := range base {
		id[k] = v
	}
	mac := jssecevents.LookupMAC(ip)
	if mac != "" {
		id["mac"] = mac
	}
	fp := ""
	if t := jssecevents.TLSStateDoc(cs); t != nil {
		id["tls"] = t
		fp, _ = t["sha256"].(string)
	}
	s.mu.Lock()
	_, seen := s.known[ipS]
	s.mu.Unlock()
	if !seen {
		s.emit(remote, jssecevents.Event{Kind: mbsec.KindPeerIdentified, Value: identityText(id),
			Detail: "first connection from this address"})
	}
	if prev, ch := s.macs.Observe(ipS, mac); ch {
		s.emit(remote, jssecevents.Event{Kind: mbsec.KindPeerMACChanged, Value: mac, Previous: prev,
			Detail: "MAC address of the master changed"})
	}
	if prev, ch := s.certs.Observe(ipS, fp); ch {
		s.emit(remote, jssecevents.Event{Kind: mbsec.KindTLSCertChanged, Value: fp, Previous: prev,
			Detail: "the master presents another certificate"})
	}
	k := bson.M{"ip": ipS, "lastSeen": time.Now()}
	if mac != "" {
		k["mac"] = mac
	}
	if fp != "" {
		k["tlsSha256"] = fp
		if t, ok := id["tls"].(bson.M); ok {
			k["tlsSubject"] = t["subject"]
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token[remote] == tok { // still the same session
		s.open[remote] = id
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
}

func identityText(id bson.M) string {
	var parts []string
	if v, ok := id["mac"].(string); ok {
		parts = append(parts, "mac="+v)
	}
	if t, ok := id["tls"].(bson.M); ok {
		if v, ok := t["subject"].(string); ok && v != "" {
			parts = append(parts, "tls="+v)
		}
	}
	return strings.Join(parts, " ")
}

// statsDoc is stats.peerIdentity: the open sessions and the last identity of
// every master IP seen; nil when identification is off.
func (s *serverSec) statsDoc() bson.M {
	if !s.cfg.PeerIdentification {
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

// badFrame reports RTU frames dropped for a bad CRC.
func (s *serverSec) badFrame(remote string, n int) {
	ok, held := s.repeat.Allow("frame|"+remote, time.Now())
	if !ok {
		return
	}
	e := jssecevents.Event{Kind: mbsec.KindProtoBadFrame, Action: "blocked", Count: held + n - 1,
		Detail: "RTU frame(s) with a bad CRC dropped"}
	if remote == "serial" {
		e.Detail += " on " + s.cfg.PortName
		remote = ""
	}
	s.emit(remote, e)
}

// --- library events ---------------------------------------------------------

// libSink receives go-modbus's events. Its connection events name the
// loopback connection from the front-end and are left out (the front-end
// reports the masters' own); a connection the front-end did not open is
// reported by the handler.
func (s *serverSec) libSink(e secevent.Event) {
	switch e.Kind {
	case secevent.KindConnAccepted, secevent.KindConnClosed, secevent.KindConnNoRequest,
		secevent.KindConnIdleTimeout, secevent.KindConnUnexpected, secevent.KindTLSHandshakeEvicted:
		return
	}
	d := mbsec.FromLib(e, s.addrs.Resolve)
	d.Role = "server"
	rep, _ := s.reporter()
	rep.Emit(d)
}

// --- requests ---------------------------------------------------------------

// bypass reports a connection that reached the protocol server's private
// port without going through the listener.
func (s *serverSec) bypass(clientAddr string) {
	s.emitRepeat("bypass|"+ipOf(clientAddr), clientAddr, jssecevents.Event{Kind: mbsec.KindConnUnexpected,
		Action: "blocked", Detail: "connection to the internal protocol server port, not through the listener"})
}

// unitUnknown reports a request to a unit id the connection does not serve.
// On a serial line the requests to the other slaves of the bus are normal
// and are not reported.
func (s *serverSec) unitUnknown(remote string, unit uint8, fc int, answered bool) {
	if remote == "serial" {
		return
	}
	e := jssecevents.Event{Kind: mbsec.KindAccessUnitUnknown, Fields: mbsec.Fields(int(unit), fc, -1, -1, 0),
		ObjectRef: fmt.Sprintf("unit %d", unit)}
	if answered {
		e.Detail = "request to a unit id not in serverUnitIds and without points"
	} else {
		e.Action, e.Detail = "blocked", "request to a unit id not served, not answered (strictUnitId)"
	}
	s.emitRepeat(fmt.Sprintf("unit|%s|%d", ipOf(remote), unit), remote, e)
}

// readNonexistent reports a read of addresses without points and, beyond
// scanMax distinct ones from one IP within scanWindow, a scan (a master
// polling one misconfigured address is not one).
func (s *serverSec) readNonexistent(remote string, unit uint8, fc int, area mbcore.Area, addr, qty uint16) {
	ref := fmt.Sprintf("unit %d %s:%d x%d", unit, area, addr, qty)
	s.emitRepeat("read|"+ipOf(remote)+"|"+ref, remote, jssecevents.Event{Kind: mbsec.KindAccessObjectNonexistent,
		Action: "rejected", ObjectRef: ref, Fields: mbsec.Fields(int(unit), fc, int(addr), int(qty), mbcore.ExIllegalDataAddress),
		Detail: "read of addresses without points"})
	now := time.Now()
	if first, _ := s.scanSeen.Allow(ipOf(remote)+"|"+ref, now); !first {
		return
	}
	if over, n := s.scan.Hit(ipOf(remote), now); over {
		s.emit(remote, jssecevents.Event{Kind: mbsec.KindAccessScan, Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("reads of more than %d different nonexistent addresses within %s", scanMax, scanWindow)})
	}
}

// writeRefused reports a write the handler refused.
func (s *serverSec) writeRefused(remote string, unit uint8, pdu []byte, err error) {
	fc, addr := -1, -1
	area := mbcore.AreaHolding
	if len(pdu) >= 3 {
		fc, addr = int(pdu[0]), int(pdu[1])<<8|int(pdu[2])
	}
	if fc == mbcore.FCWriteSingleCoil || fc == mbcore.FCWriteMultipleCoils {
		area = mbcore.AreaCoil
	}
	e := jssecevents.Event{Kind: mbsec.KindCmdObjectUnknown, Action: "rejected",
		ObjectRef: fmt.Sprintf("unit %d %s:%d", unit, area, addr),
		Fields:    mbsec.Fields(int(unit), fc, addr, -1, exceptionCode(err)),
		Detail:    "write to addresses without points"}
	var r refusal
	if errors.As(err, &r) {
		e.Kind, e.Tag, e.Detail = mbsec.KindCmdRejected, r.tag, r.why
	}
	s.emitRepeat(fmt.Sprintf("write|%s|%s|%s", ipOf(remote), e.Kind, e.ObjectRef), remote, e)
}

// relayed counts a command relayed from a master for the flood detector and
// queues the check of a command to the state its point already is in.
func (s *serverSec) relayed(remote string, p *Point, value float64) {
	if over, n := s.flood.Hit(ipOf(remote), time.Now()); over {
		s.emit(remote, jssecevents.Event{Kind: mbsec.KindFloodControls, Tag: p.Tag, Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d commands within %s", commandFloodMax, commandFloodWindow)})
	}
	if p.TagType != TagDigital || p.PointKey == 0 {
		return
	}
	select {
	case s.checks <- cmdCheck{remote: remote, pointKey: p.PointKey, tag: p.Tag, value: value, at: time.Now()}:
	default: // the checks are best effort
	}
}

// supervisedWrite reports a master's write stored in a supervised point
// (allowWritesToSupervised): a parameter change seen by the SCADA.
func (s *serverSec) supervisedWrite(remote string, p *Point) {
	s.emitRepeat("param|"+ipOf(remote)+"|"+p.Tag, remote, jssecevents.Event{Kind: mbsec.KindCtrlSettingEdit,
		Tag: p.Tag, ObjectRef: fmt.Sprintf("unit %d %s:%d", p.Unit, p.Area, p.Offset),
		Detail: "write stored in a supervised point (allowWritesToSupervised)"})
}

// cmdCheck is a digital command to compare with the state it commands.
type cmdCheck struct {
	remote   string
	pointKey float64
	tag      string
	value    float64
	at       time.Time // received: the state must predate it
}

// run checks the queued commands until ctx ends.
func (s *serverSec) run(ctx context.Context) {
	for {
		var chk cmdCheck
		select {
		case <-ctx.Done():
			return
		case chk = <-s.checks:
		}
		_, rt := s.reporter()
		state, ok, err := jssecevents.SupervisedState(ctx, rt, chk.pointKey, chk.at)
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "%s: secEvents - command check: %v", s.cfg.Name, err)
			continue
		}
		if ok && (state != 0) == (chk.value != 0) {
			s.emit(chk.remote, jssecevents.Event{Kind: mbsec.KindCmdUnnecessary, Tag: chk.tag, Value: fmt.Sprint(chk.value),
				Detail: "the supervised point already was in the commanded state"})
		}
	}
}

// tap sees every request the protocol server answers: diagnostics that
// restart or silence the device, file record transfers, identification
// reads and function codes the server does not serve.
func (s *serverSec) tap(_ context.Context, ex modbus.ServerPDUExchange) {
	if len(ex.Request) == 0 || !s.addrs.Known(ex.ClientAddr) {
		return
	}
	remote := s.addrs.Resolve(ex.ClientAddr)
	fc := int(ex.Request[0])
	exc := 0
	if len(ex.Response) >= 2 && ex.Response[0]&0x80 != 0 {
		exc = int(ex.Response[1])
	}
	action := "observed"
	if exc != 0 {
		action = "rejected"
	}
	key := fmt.Sprintf("%s|%d", ipOf(remote), fc)
	f := mbsec.Fields(int(ex.UnitID), fc, -1, -1, exc)
	switch fc {
	case 0x08:
		sub := uint16(0)
		if len(ex.Request) >= 3 {
			sub = uint16(ex.Request[1])<<8 | uint16(ex.Request[2])
		}
		f["subFunction"] = int(sub)
		if what, ok := restartSubFunctions[sub]; ok {
			s.emit(remote, jssecevents.Event{Kind: mbsec.KindCtrlRestart, Action: action, Fields: f,
				Detail: "diagnostics: " + what})
			return
		}
		s.emitRepeat(key, remote, jssecevents.Event{Kind: mbsec.KindAccessEnumeration, Action: action, Fields: f,
			Detail: fmt.Sprintf("diagnostics sub-function 0x%04X", sub)})
	case 0x14:
		s.emit(remote, jssecevents.Event{Kind: mbsec.KindFileRead, Action: action, Fields: f, Detail: "read file record"})
	case 0x15:
		s.emit(remote, jssecevents.Event{Kind: mbsec.KindFileWrite, Action: action, Fields: f, Detail: "write file record"})
	case 0x11, 0x2B:
		what := "report server id"
		if fc == 0x2B {
			what = "encapsulated interface transport (device identification)"
		}
		s.emitRepeat(key, remote, jssecevents.Event{Kind: mbsec.KindAccessEnumeration, Action: action, Fields: f, Detail: what})
	case mbcore.FCReadCoils, mbcore.FCReadDiscreteInputs, mbcore.FCReadHoldingRegisters, mbcore.FCReadInputRegisters,
		mbcore.FCWriteSingleCoil, mbcore.FCWriteSingleRegister, mbcore.FCWriteMultipleCoils,
		mbcore.FCWriteMultipleRegisters, mbcore.FCMaskWriteRegister:
		// served by the handler, which reports what it refuses
	default:
		if exc == mbcore.ExIllegalFunction {
			s.emitRepeat(key, remote, jssecevents.Event{Kind: mbsec.KindAccessUnrecognizedService, Action: "rejected",
				Fields: f, Detail: "function code not served"})
		}
	}
}
