/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package iecsec maps the IEC 60870-5 drivers' security events onto
// secEvents documents (go-common/jssecevents): go-iecp5's secevent events
// (through the Sink of a Reporter, with the routine ones left out), the
// kinds the drivers detect themselves, the peer identity kept in
// stats.peerIdentity (Client, Server) and the IEC 60870-5 protocol fields.
// Contract in docs/schema.md (secEvents).
package iecsec

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/netpeer"
	"github.com/riclolsen/go-iecp5/secevent"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Event sources, the secEvents source field.
const (
	SourceLibrary = "library" // detected by go-iecp5
	SourceDriver  = "driver"  // detected by the driver's own code
)

// The kinds only the drivers report. The others they raise themselves
// (access.unknown_ioa, cmd.rejected, ctrl.end_of_init, ...) are go-iecp5's,
// so the viewer filters library and driver detections alike.
const (
	KindPeerIdentified      = "peer.identified"       // first identification of a peer
	KindPeerIdentityChanged = "peer.identity_changed" // the device identification (IEC 103 ASDU 5) changed
	KindCmdObjectUnknown    = "cmd.object_unknown"    // a command to an object that does not exist
	KindCmdUnnecessary      = "cmd.unnecessary"       // a command to the state the point already is in
	KindCmdStaleTime        = "cmd.stale_time"        // a queued command older than its expiry, not sent
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
	KindCmdObjectUnknown:    {"control", "warning", "T0855"},
	KindCmdUnnecessary:      {"control", "notice", "T0855"},
	KindCmdStaleTime:        {"control", "warning", "T0831"},
}

// Describe returns the classification of a kind, the library's included.
func Describe(kind string) (Info, bool) {
	if i, ok := catalogue[kind]; ok {
		return i, true
	}
	if li, ok := secevent.Info(secevent.Kind(kind)); ok {
		return Info{Category: string(li.Category), Severity: li.Severity.String(), Technique: li.Technique}, true
	}
	return Info{}, false
}

// routine are the library kinds left out of secEvents: normal operation
// (sessions opening and closing, the master's periodic interrogations,
// clock synchronisations, tests and link resets, every accepted command),
// which the drivers log and which would bury the events that matter.
var routine = map[secevent.Kind]bool{
	secevent.KindConnAccepted:       true,
	secevent.KindConnClosed:         true,
	secevent.KindConnDialFailed:     true,
	secevent.KindConnT1Timeout:      true,
	secevent.KindCtrlStartDT:        true,
	secevent.KindCtrlStopDT:         true,
	secevent.KindCtrlInterrogation:  true,
	secevent.KindCtrlClockSync:      true,
	secevent.KindCtrlTestCommand:    true,
	secevent.KindCtrlDelayAcq:       true,
	secevent.KindCtrlLinkReset:      true,
	secevent.KindCmdReceived:        true,
	secevent.KindPeerIPChanged:      true,
	secevent.KindPeerMACUnavailable: true,
	secevent.KindFileDirectoryCall:  true,
}

// identityKinds are left out when peerIdentification is off.
var identityKinds = map[secevent.Kind]bool{
	secevent.KindPeerMACChanged:     true,
	secevent.KindPeerMACChangedLive: true,
	secevent.KindPeerMACShared:      true,
	secevent.KindTLSCertChanged:     true,
}

// Reporter stamps and stores the events of one driver connection. A nil
// Reporter, or one without a Writer (tests), only logs them.
type Reporter struct {
	W        *jssecevents.Writer
	Origin   jssecevents.Origin
	Role     string // "client" or "server"
	Protocol string // "iec104", "iec101", "iec103"
	// Ident is the connection's peerIdentification: off, the identity
	// events are left out.
	Ident bool
	// Drop leaves out a library event the driver reports itself, with more
	// detail (the tag, the reason). Nil keeps them all.
	Drop func(secevent.Event) bool
	// capture receives the events instead of the Writer (tests)
	capture func(jssecevents.Event)
}

// DropServerRefusals is the Drop of a server: the driver reports every
// command it refuses, with the reason.
func DropServerRefusals(e secevent.Event) bool { return e.Kind == secevent.KindCmdRejected }

// DropClientRefusals is the Drop of a 104 client: the driver reports the
// device refusing a command or a parameter, with the command's tag.
func DropClientRefusals(e secevent.Event) bool {
	return e.Kind == secevent.KindRemoteRejected && e.Fields&secevent.FieldType != 0 &&
		(asdu.IsProcessCommand(e.TypeID) || asdu.IsParameter(e.TypeID))
}

// Emit completes e (protocol, role, source, classification) and stores it.
func (r *Reporter) Emit(e jssecevents.Event) {
	if r != nil {
		if e.Protocol == "" {
			e.Protocol = r.Protocol
		}
		if e.Role == "" {
			e.Role = r.Role
		}
	}
	if e.Source == "" {
		e.Source = SourceDriver
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
	if r != nil && r.capture != nil {
		r.capture(e)
		return
	}
	if r == nil || r.W == nil {
		jslog.Log(jslog.LevelDetailed, "Security event %s [%s] %s %s", e.Kind, e.Severity, e.PeerAddress, e.Detail)
		return
	}
	r.W.Emit(r.Origin, e)
}

// Enabled reports whether events are stored (not only logged).
func (r *Reporter) Enabled() bool { return r != nil && r.W != nil }

// Keep reports whether a library event goes to secEvents.
func (r *Reporter) Keep(e secevent.Event) bool {
	if routine[e.Kind] {
		return false
	}
	if r != nil && !r.Ident && identityKinds[e.Kind] {
		return false
	}
	return r == nil || r.Drop == nil || !r.Drop(e)
}

// Sink receives the library's events for this connection.
func (r *Reporter) Sink() secevent.Sink {
	return secevent.SinkFunc(func(e secevent.Event) {
		if !r.Keep(e) {
			return
		}
		r.Emit(FromLib(e))
	})
}

// Fields are the IEC 60870-5 fields of an ASDU; a negative value is left out.
func Fields(typeID, cause, ca, ioa int) bson.M {
	m := bson.M{}
	if typeID >= 0 {
		m["typeId"] = typeID
		m["type"] = asdu.TypeID(typeID).String()
	}
	if cause >= 0 {
		m["cause"] = cause
	}
	if ca >= 0 {
		m["commonAddress"] = ca
	}
	if ioa >= 0 {
		m["ioa"] = ioa
	}
	return m
}

// PackFields are the fields of a received ASDU, its first object address
// included.
func PackFields(pack *asdu.ASDU) bson.M {
	ioa := -1
	if a, ok := asdu.FirstIOA(pack); ok {
		ioa = int(a)
	}
	f := Fields(int(pack.Type), int(pack.Coa.Cause), int(pack.CommonAddr), ioa)
	if pack.Coa.IsNegative {
		f["negative"] = true
	}
	if pack.Coa.IsTest {
		f["test"] = true
	}
	f["originatorAddress"] = int(pack.OrigAddr)
	return f
}

// ObjectRef names an information object: "CA 1 IOA 3001".
func ObjectRef(ca, ioa int) string {
	if ioa < 0 {
		return "CA " + strconv.Itoa(ca)
	}
	return "CA " + strconv.Itoa(ca) + " IOA " + strconv.Itoa(ioa)
}

// SetPeer sets the peer fields of e from a go-iecp5 peer description.
func SetPeer(e *jssecevents.Event, p secevent.PeerInfo) {
	if p.RemoteAddr.IsValid() {
		e.PeerAddress = p.RemoteAddr.String()
		e.PeerIP = p.RemoteAddr.Addr().Unmap().String()
	} else if p.SerialPort != "" {
		e.PeerAddress = "serial://" + p.SerialPort
	}
	if p.MAC != nil && p.MACSource != secevent.MACGateway {
		e.PeerMAC = p.MAC.String()
	}
	if e.LocalAddress == "" && p.LocalAddr.IsValid() {
		e.LocalAddress = p.LocalAddr.String()
	}
	if p.TLS != nil {
		if e.Peer == nil {
			e.Peer = bson.M{}
		}
		e.Peer["tls"] = TLSDoc(p.TLS)
	}
}

// PeerOf describes the peer behind a go-iecp5 connection (a 104 session,
// a 101 server), when it can tell.
func PeerOf(c asdu.Connect) secevent.PeerInfo {
	if c == nil {
		return secevent.PeerInfo{}
	}
	p, _ := secevent.PeerOf(c)
	return p
}

// TLSDoc describes a TLS session or a refused certificate.
func TLSDoc(t *secevent.TLSPeer) bson.M {
	m := bson.M{"verified": t.Verified}
	if v := jssecevents.TLSVersionName(t.Version); v != "" {
		m["version"] = v
	}
	if t.CipherSuite != 0 {
		m["cipherSuite"] = tlsCipherName(t.CipherSuite)
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

// FromLib maps a go-iecp5 event onto a secEvents document.
func FromLib(e secevent.Event) jssecevents.Event {
	d := jssecevents.Event{
		TimeTag:      e.Time,
		Kind:         string(e.Kind),
		Category:     string(e.Category),
		Severity:     e.Severity.String(),
		Action:       string(e.Action),
		Source:       SourceLibrary,
		Protocol:     e.Protocol,
		SessionID:    int64(e.SessionID),
		LocalAddress: e.Local,
		Value:        e.Value,
		Detail:       e.Detail,
		Count:        e.Count,
		Technique:    e.Technique,
	}
	if d.TimeTag.IsZero() {
		d.TimeTag = time.Now()
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
	if e.Fields&secevent.FieldLinkAddr != 0 {
		f["linkAddress"] = int(e.LinkAddr)
	}
	if e.Fields&secevent.FieldType != 0 {
		f["typeId"], f["type"] = int(e.TypeID), e.TypeID.String()
	}
	if e.Fields&secevent.FieldCause != 0 {
		f["cause"] = int(e.Cause)
		if e.Negative {
			f["negative"] = true
		}
		if e.Test {
			f["test"] = true
		}
	}
	if e.Fields&secevent.FieldOrigAddr != 0 {
		f["originatorAddress"] = int(e.OrigAddr)
	}
	ca, ioa := -1, -1
	if e.Fields&secevent.FieldCommonAddr != 0 {
		ca = int(e.CommonAddr)
		f["commonAddress"] = ca
	}
	if e.Fields&secevent.FieldIOA != 0 {
		ioa = int(e.IOA)
		f["ioa"] = ioa
	}
	if e.Fields&secevent.FieldSeq != 0 {
		f["sendSeq"], f["recvSeq"], f["expectedSeq"] = int(e.SendSN), int(e.RecvSN), int(e.ExpectedSN)
	}
	if len(e.Raw) > 0 {
		f["raw"] = hex.EncodeToString(e.Raw)
	}
	if len(f) > 0 {
		d.Fields = f
	}
	if ca >= 0 {
		d.ObjectRef = ObjectRef(ca, ioa)
	}
	return d
}

// Policy is the go-iecp5 security policy of a connection: report only, the
// library's thresholds, and no MAC lookups when peerIdentification is off.
func Policy(ident bool) secevent.Policy {
	p := secevent.DefaultPolicy()
	if !ident {
		p.Resolver = netpeer.ResolverFunc(func(context.Context, netip.Addr) (netpeer.Result, error) {
			return netpeer.Result{}, netpeer.ErrNotFound
		})
	}
	return p
}

// SeedLibCache starts go-iecp5's MAC and certificate change detection (the
// process-wide cache) from what a stored identity remembers, so a change
// across a driver restart is reported too.
func SeedLibCache(ip, mac, certSHA256 string) {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return
	}
	if hw, err := net.ParseMAC(mac); err == nil {
		netpeer.DefaultCache().Observe(a, hw)
	}
	if certSHA256 != "" {
		netpeer.DefaultCache().ObserveCert(a, certSHA256)
	}
}

// StoredIdentity is stats.peerIdentity of a connection document.
func StoredIdentity(stats bson.M) bson.M { return jsmongo.GetDoc(stats, "peerIdentity") }

// IsCertError reports a TLS failure on the peer's certificate.
func IsCertError(err error) bool { return jssecevents.IsCertError(err) }

// CertRejected describes a refused certificate from the error of a failed
// handshake: the event to report and whether the error was one.
func CertRejected(err error) (jssecevents.Event, bool) {
	if !IsCertError(err) {
		return jssecevents.Event{}, false
	}
	e := jssecevents.Event{Kind: string(secevent.KindTLSCertRejected), Action: "rejected", Protocol: "tls",
		Error: err.Error(), Detail: "the peer's certificate was refused"}
	if cert := unverifiedLeaf(err); cert != nil {
		e.Peer = bson.M{"tls": jssecevents.CertDoc(cert)}
	}
	return e, true
}

// unverifiedLeaf is the certificate a refused handshake was about.
func unverifiedLeaf(err error) *x509.Certificate {
	var cve *tls.CertificateVerificationError
	if errors.As(err, &cve) && len(cve.UnverifiedCertificates) > 0 {
		return cve.UnverifiedCertificates[0]
	}
	return nil
}

func tlsCipherName(id uint16) string { return tls.CipherSuiteName(id) }
