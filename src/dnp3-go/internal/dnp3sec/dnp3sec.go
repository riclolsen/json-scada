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

// Package dnp3sec maps the DNP3 drivers' security events onto secEvents
// documents (go-common/jssecevents): go-dnp3's secevent events (through the
// Sink of a Reporter, the routine ones left out), the kinds the drivers
// detect themselves, and the peer identity kept in stats.peerIdentity
// (Client, Server). Contract in docs/schema.md (secEvents).
package dnp3sec

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/dscsystems/go-dnp3/netpeer"
	"github.com/dscsystems/go-dnp3/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Event sources, the secEvents source field.
const (
	SourceLibrary = "library" // detected by go-dnp3
	SourceDriver  = "driver"  // detected by the driver's own code
)

// The kinds only the drivers report. The others they raise themselves
// (cmd.rejected, conn.unexpected, flood.commands) are go-dnp3's, so the
// viewer filters library and driver detections alike.
const (
	KindPeerIdentified      = "peer.identified"       // first identification of a peer
	KindPeerIdentityChanged = "peer.identity_changed" // a device attribute (group 0) changed
	KindCmdObjectUnknown    = "cmd.object_unknown"    // a command to an object that does not exist
	KindCmdWrongType        = "cmd.wrong_type"        // a command of the wrong type for its object
	KindCmdUnnecessary      = "cmd.unnecessary"       // a command to the state the point already is in
	KindCmdStaleTime        = "cmd.stale_time"        // a queued command older than its expiry, not sent

	// KindCmdRejected is go-dnp3's cmd.rejected, which the drivers raise too.
	KindCmdRejected = string(secevent.KindCmdRejected)
)

// Info classifies a kind.
type Info struct {
	Category  string
	Severity  string
	Technique string // MITRE ATT&CK for ICS hint
}

var catalogue = map[string]Info{
	KindPeerIdentified:      {"peer", "info", ""},
	KindPeerIdentityChanged: {"peer", "alert", "T0839"},
	KindCmdObjectUnknown:    {"command", "warning", "T0855"},
	KindCmdWrongType:        {"command", "warning", "T0855"},
	KindCmdUnnecessary:      {"command", "notice", "T0855"},
	KindCmdStaleTime:        {"command", "warning", "T0831"},
}

// Describe returns the classification of a kind, the library's included.
func Describe(kind string) (Info, bool) {
	if i, ok := catalogue[kind]; ok {
		return i, true
	}
	k := secevent.Kind(kind)
	if !k.Known() {
		return Info{}, false
	}
	return Info{Category: k.Category().String(), Severity: k.DefaultSeverity().String(), Technique: k.Technique()}, true
}

// routine are the library kinds left out of secEvents besides the
// informational ones: normal operation the drivers log, which would bury the
// events that matter. Every accepted control is cmd.received (the server
// logs it, the client acknowledges it in commandsQueue), and a master sets
// the outstation's clock on every connection.
var routine = map[secevent.Kind]bool{
	secevent.KindCmdReceived:     true,
	secevent.KindCtrlTimeWrite:   true,
	secevent.KindCtrlUnsolEnable: true,
}

// identityKinds are left out when peerIdentification is off.
var identityKinds = map[secevent.Kind]bool{
	secevent.KindPeerMACChanged:           true,
	secevent.KindPeerMACChangedMidsession: true,
	secevent.KindPeerMACShared:            true,
	secevent.KindPeerIPChanged:            true,
	secevent.KindPeerLinkAddrChanged:      true,
	secevent.KindTLSCertChanged:           true,
}

// DialFailedEvery is how often a connection that cannot reach its device
// reports it: the channel retries with a backoff, and every attempt is a
// conn.dial_failed.
const DialFailedEvery = 10 * time.Minute

// Reporter stamps and stores the events of one driver connection. A nil
// Reporter, or one without a Writer (tests), only logs them.
type Reporter struct {
	W      *jssecevents.Writer
	Origin jssecevents.Origin
	Role   string // "client" or "server"
	// Ident is the connection's peerIdentification: off, the identity
	// events are left out.
	Ident bool
	// Drop leaves out a library event the driver reports itself, with more
	// detail (the tag, the reason). Nil keeps them all.
	Drop func(secevent.Event) bool
	// Capture receives the events instead of the Writer (tests).
	Capture func(jssecevents.Event)

	dial jssecevents.Throttle
	// cache is the connection's peer history (MAC, link address and
	// certificate per IP). It is per connection, not go-dnp3's process-wide
	// one: the stations of a multidrop line behind one terminal server share
	// an IP and have different link addresses, which in one history would be
	// a peer.link_addr_changed at every poll.
	cache *netpeer.Cache
}

// Emit completes e (protocol, role, source, classification) and stores it.
func (r *Reporter) Emit(e jssecevents.Event) {
	if e.Protocol == "" {
		e.Protocol = "dnp3"
	}
	if r != nil && r.Role != "" {
		e.Role = r.Role
	}
	if e.Source == "" {
		e.Source = SourceDriver
	}
	if e.TimeTag.IsZero() {
		e.TimeTag = time.Now()
	}
	if info, ok := Describe(e.Kind); ok {
		if e.Category == "" {
			e.Category = info.Category
		}
		if e.Severity == "" {
			e.Severity = info.Severity
		}
		if e.Technique == "" {
			e.Technique = info.Technique
		}
	}
	switch {
	case r != nil && r.Capture != nil:
		r.Capture(e)
	case r == nil || r.W == nil:
		jslog.Log(jslog.LevelDetailed, "Security event %s [%s] %s %s", e.Kind, e.Severity, e.PeerAddress, e.Detail)
	default:
		r.W.Emit(r.Origin, e)
	}
}

// Keep reports whether a library event goes to secEvents.
func (r *Reporter) Keep(e secevent.Event) bool {
	if e.Severity == secevent.Info && !e.Kind.AlwaysDeliver() {
		return false
	}
	if routine[e.Kind] {
		return false
	}
	if r == nil {
		return true
	}
	if !r.Ident && identityKinds[e.Kind] {
		return false
	}
	if e.Kind == secevent.KindConnDialFailed {
		ok, _ := r.dial.Allow("dial", time.Now())
		return ok
	}
	return r.Drop == nil || !r.Drop(e)
}

// Sink receives the library's events for this connection.
func (r *Reporter) Sink() secevent.Sink {
	return secevent.SinkFunc(func(e secevent.Event) {
		if e.Kind == secevent.KindConnConnected || e.Kind == secevent.KindConnAccepted {
			// A working connection (informational, not stored): the next
			// outage is reported at once.
			r.dial.Reset()
			return
		}
		if !r.Keep(e) {
			return
		}
		r.Emit(FromLib(e))
	})
}

// NewReporter returns the reporter of a connection; dial failures are
// throttled to one per DialFailedEvery.
func NewReporter(w *jssecevents.Writer, origin jssecevents.Origin, role string, ident bool) *Reporter {
	return &Reporter{W: w, Origin: origin, Role: role, Ident: ident,
		dial: jssecevents.Throttle{Every: DialFailedEvery}, cache: netpeer.NewCache(0, 0)}
}

// Policy is the go-dnp3 security policy of the connection: report only, with
// the library's thresholds, its events to Sink, the connection's own peer
// history, and no MAC lookups when peerIdentification is off. It takes
// effect on the sessions and buses it is given to.
func (r *Reporter) Policy() *secevent.Policy {
	p := secevent.DefaultPolicy(r.Sink())
	p.DisableMACLookup = !r.Ident
	p.MACCache = r.cache
	return p
}

// ObjectRef names a DNP3 object: "g12v1 #5", "g41 #3", "#7".
func ObjectRef(group, variation, index int) string {
	s := ""
	switch {
	case group >= 0 && variation > 0:
		s = fmt.Sprintf("g%dv%d", group, variation)
	case group >= 0:
		s = fmt.Sprintf("g%d", group)
	}
	if index >= 0 {
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf("#%d", index)
	}
	return s
}

// SetPeer sets the peer fields of e from a go-dnp3 peer description.
func SetPeer(e *jssecevents.Event, p secevent.PeerInfo) {
	if e.PeerAddress == "" {
		switch {
		case p.RemoteAddr.IsValid():
			e.PeerAddress = p.RemoteAddr.String()
		case p.SerialPort != "":
			e.PeerAddress = "serial://" + p.SerialPort
		}
	}
	if e.PeerIP == "" && p.RemoteAddr.IsValid() {
		e.PeerIP = p.RemoteAddr.Addr().Unmap().String()
	}
	if e.PeerMAC == "" && p.MAC != nil && p.MACSource != secevent.MACGateway {
		e.PeerMAC = p.MAC.String()
	}
	if e.LocalAddress == "" && p.LocalAddr.IsValid() {
		e.LocalAddress = p.LocalAddr.String()
	}
	peer := bson.M{}
	if p.MACSource != secevent.MACNone {
		peer["macSource"] = p.MACSource.String()
	}
	if p.Interface != "" {
		peer["interface"] = p.Interface
	}
	if p.TLS != nil {
		peer["tls"] = TLSDoc(p.TLS)
	}
	if p.LinkAddr != 0 {
		peer["linkAddress"] = int(p.LinkAddr)
	}
	if len(peer) > 0 {
		if e.Peer == nil {
			e.Peer = bson.M{}
		}
		for k, v := range peer {
			if _, set := e.Peer[k]; !set {
				e.Peer[k] = v
			}
		}
	}
}

// TLSDoc describes a TLS session or a refused certificate.
func TLSDoc(t *secevent.TLSPeer) bson.M {
	m := bson.M{"verified": t.Verified}
	if v := jssecevents.TLSVersionName(t.Version); v != "" {
		m["version"] = v
	}
	if t.Subject != "" {
		m["subject"], m["issuer"], m["serial"] = t.Subject, t.Issuer, t.SerialHex
	}
	if t.FingerprintSHA256 != "" {
		m["sha256"] = t.FingerprintSHA256
	}
	if !t.NotAfter.IsZero() {
		m["notAfter"] = t.NotAfter
	}
	return m
}

// FromLib maps a go-dnp3 event onto a secEvents document.
func FromLib(e secevent.Event) jssecevents.Event {
	d := jssecevents.Event{
		TimeTag:      e.Time,
		Kind:         string(e.Kind),
		Category:     e.Category.String(),
		Severity:     e.Severity.String(),
		Action:       e.Action.String(),
		Source:       SourceLibrary,
		Protocol:     "dnp3",
		SessionID:    int64(e.ConnID),
		LocalAddress: e.Local,
		Value:        e.Value,
		Detail:       e.Detail,
		Count:        e.Count,
		Technique:    e.Technique,
	}
	if e.Category == 0 {
		d.Category = ""
	}
	if e.Err != nil {
		d.Error = e.Err.Error()
	}
	if !e.FirstSeen.IsZero() && e.Count > 1 {
		fs := e.FirstSeen
		d.FirstSeen = &fs
	}
	SetPeer(&d, e.Peer)

	f := bson.M{}
	if e.Transport != "" {
		f["transport"] = e.Transport
	}
	if e.Fields&secevent.HasLinkAddrs != 0 {
		f["linkSource"], f["linkDestination"] = int(e.LinkSrc), int(e.LinkDest)
	}
	if e.Fields&secevent.HasLinkFunc != 0 {
		f["linkFunction"] = int(e.LinkFunc)
	}
	if e.Fields&secevent.HasTransportSeq != 0 {
		f["transportSeq"] = int(e.TransportSeq)
	}
	if e.Fields&secevent.HasAppFunc != 0 {
		d.FC = secevent.FuncName(e.AppFunc)
		f["appFunction"] = int(e.AppFunc)
	}
	if e.Fields&secevent.HasAppSeq != 0 {
		f["appSeq"], f["expectedSeq"] = int(e.AppSeq), int(e.ExpectedSeq)
	}
	if e.Fields&secevent.HasIIN != 0 {
		f["iin"] = int(e.IIN)
	}
	group, variation, index := -1, 0, -1
	if e.Fields&secevent.HasObject != 0 {
		group, variation = int(e.Group), int(e.Variation)
		f["group"], f["variation"], f["qualifier"] = group, variation, int(e.Qualifier)
	}
	if e.Fields&secevent.HasIndex != 0 {
		index = int(e.Index)
		f["index"] = index
	}
	if e.Fields&secevent.HasCommandStatus != 0 {
		f["commandStatus"] = e.CommandStatus.String()
	}
	if e.Fields&secevent.HasControlCode != 0 {
		f["controlCode"] = int(e.ControlCode)
	}
	if e.Fields&secevent.HasUser != 0 {
		f["user"] = int(e.User)
	}
	if e.Fields&secevent.HasFileStatus != 0 {
		f["fileStatus"] = e.FileStatus.String()
	}
	if e.FileName != "" {
		f["fileName"] = e.FileName
		d.ObjectRef = e.FileName
	}
	if len(e.Raw) > 0 {
		f["raw"] = hex.EncodeToString(e.Raw)
	}
	if len(f) > 0 {
		d.Fields = f
	}
	if group >= 0 || index >= 0 {
		d.ObjectRef = ObjectRef(group, variation, index)
	}
	return d
}

// Seed starts go-dnp3's change detection (the connection's peer history)
// from what a stored identity remembers of an IP — its MAC, DNP3 link address
// and certificate — so a change across a driver restart is reported too.
func (r *Reporter) Seed(ip, mac string, linkAddr int, certSHA256 string) {
	a, err := netip.ParseAddr(ip)
	if err != nil || r == nil || r.cache == nil {
		return
	}
	o := netpeer.Observation{IP: a, CertSHA256: certSHA256}
	if hw, err := net.ParseMAC(mac); err == nil {
		o.MAC, o.Source = hw, netpeer.SourceNeighbor
	}
	if linkAddr >= 0 && linkAddr <= 0xFFFF {
		o.LinkAddr, o.HasLinkAddr = uint16(linkAddr), true
	}
	r.cache.Observe(o)
}

// StoredIdentity is stats.peerIdentity of a connection document.
func StoredIdentity(doc bson.M) bson.M {
	return jsmongo.GetDoc(jsmongo.GetDoc(doc, "stats"), "peerIdentity")
}

// peerBase is what a connection's peer description says of it, with the
// keys compared between connections.
func peerBase(p secevent.PeerInfo) bson.M {
	id := bson.M{"connectedAt": time.Now()}
	if !p.ConnectedAt.IsZero() {
		id["connectedAt"] = p.ConnectedAt
	}
	switch {
	case p.RemoteAddr.IsValid():
		id["address"] = p.RemoteAddr.String()
		id["ip"] = p.RemoteAddr.Addr().Unmap().String()
	case p.SerialPort != "":
		id["address"] = "serial://" + p.SerialPort
	}
	if p.MAC != nil && p.MACSource != secevent.MACGateway {
		id["mac"] = p.MAC.String()
	}
	if p.MACSource != secevent.MACNone {
		id["macSource"] = p.MACSource.String()
	}
	if p.TLS != nil {
		id["tls"] = TLSDoc(p.TLS)
	}
	return id
}
