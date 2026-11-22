/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package mbsec maps the Modbus drivers' security events onto secEvents
// documents (go-common/jssecevents): the kinds the drivers report, their
// classification, the mapping of go-modbus's secevent events and the
// Modbus protocol fields. Contract in docs/schema.md (secEvents).
package mbsec

import (
	"encoding/hex"
	"errors"
	"time"

	"modbus-go/internal/mbcore"

	"github.com/otfabric/go-modbus"
	"github.com/otfabric/go-modbus/secevent"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Event sources, the secEvents source field.
const (
	SourceLibrary = "library" // detected by go-modbus
	SourceDriver  = "driver"  // detected by the driver's own code
)

// The kinds the drivers report. The names follow go-modbus's and
// go-iec61850's catalogues where one fits, so the viewer filters alike.
const (
	KindConnUnexpected    = "conn.unexpected"     // a master outside ipAddresses, or one that bypassed the listener
	KindConnRejectedLimit = "conn.rejected_limit" // maxClientConnections reached

	KindTLSCertRejected    = "tls.cert_rejected"    // the peer's certificate was refused
	KindTLSHandshakeFailed = "tls.handshake_failed" // a TLS handshake failed for another reason
	KindTLSCertChanged     = "tls.cert_changed"     // the peer presents another certificate than before

	KindPeerIdentified      = "peer.identified"       // first identification of a peer
	KindPeerIdentityChanged = "peer.identity_changed" // device identification (vendor, product, revision) changed
	KindPeerMACChanged      = "peer.mac_changed"      // the MAC address of the peer's IP changed

	KindAccessObjectNonexistent   = "access.object_nonexistent"   // a read of an address that does not exist
	KindAccessUnitUnknown         = "access.unit_unknown"         // a request to a unit id that is not served
	KindAccessUnrecognizedService = "access.unrecognized_service" // a function code the peer does not serve
	KindAccessEnumeration         = "access.enumeration"          // device identification, report server id, diagnostics
	KindAccessScan                = "access.scan"                 // many reads of nonexistent addresses from one peer

	KindCmdRejected      = "cmd.rejected"       // a write or command refused
	KindCmdObjectUnknown = "cmd.object_unknown" // a write or command to an address that does not exist
	KindCmdUnnecessary   = "cmd.unnecessary"    // a command to the state the point already is in
	KindCmdStaleTime     = "cmd.stale_time"     // a command older than its expiry (replayed or delayed)
	KindFloodControls    = "flood.controls"     // too many commands within a short time

	KindCtrlRestart     = "ctrl.restart"      // FC8 restart communications, listen-only, clear counters
	KindCtrlSettingEdit = "ctrl.setting_edit" // a master's write stored in a supervised (parameter) point
	KindFileRead        = "file.read"         // FC20 read file record
	KindFileWrite       = "file.write"        // FC21 write file record

	KindProtoBadFrame     = "proto.bad_frame"     // an RTU frame with a bad CRC or that cannot be framed
	KindProtoBadResponse  = "proto.bad_response"  // a malformed response (MBAP, CRC, length, protocol id)
	KindProtoTxnMismatch  = "proto.txn_mismatch"  // a response with an unexpected transaction id
	KindProtoUnitMismatch = "proto.unit_mismatch" // a response from another unit id than asked
)

// Info classifies a kind.
type Info struct {
	Category  string
	Severity  string
	Technique string // MITRE ATT&CK for ICS hint
}

var catalogue = map[string]Info{
	KindConnUnexpected:    {"connection", "warning", "T0846"},
	KindConnRejectedLimit: {"connection", "warning", "T0814"},

	KindTLSCertRejected:    {"tls", "alert", "T0830"},
	KindTLSHandshakeFailed: {"tls", "notice", ""},
	KindTLSCertChanged:     {"tls", "warning", "T0830"},

	KindPeerIdentified:      {"peer", "info", ""},
	KindPeerIdentityChanged: {"peer", "alert", "T0839"},
	KindPeerMACChanged:      {"peer", "warning", "T0830"},

	KindAccessObjectNonexistent:   {"access", "warning", "T0861"},
	KindAccessUnitUnknown:         {"access", "warning", "T0846"},
	KindAccessUnrecognizedService: {"access", "warning", "T0846"},
	KindAccessEnumeration:         {"recon", "notice", "T0888"},
	KindAccessScan:                {"recon", "alert", "T0846"},

	KindCmdRejected:      {"control", "warning", "T0855"},
	KindCmdObjectUnknown: {"control", "warning", "T0855"},
	KindCmdUnnecessary:   {"control", "notice", "T0855"},
	KindCmdStaleTime:     {"control", "warning", "T0831"},
	KindFloodControls:    {"flood", "warning", "T0806"},

	KindCtrlRestart:     {"control", "alert", "T0816"},
	KindCtrlSettingEdit: {"control", "notice", "T0836"},
	KindFileRead:        {"control", "warning", "T0845"},
	KindFileWrite:       {"control", "alert", "T0843"},

	KindProtoBadFrame:     {"protocol", "notice", "T0866"},
	KindProtoBadResponse:  {"protocol", "warning", "T0830"},
	KindProtoTxnMismatch:  {"protocol", "notice", "T0830"},
	KindProtoUnitMismatch: {"protocol", "warning", "T0830"},
}

// Describe returns the classification of a kind, the library's included.
func Describe(kind string) (Info, bool) {
	if i, ok := catalogue[kind]; ok {
		return i, true
	}
	if li, ok := secevent.Describe(secevent.Kind(kind)); ok {
		return Info{Category: li.Category.String(), Severity: li.Severity.String(), Technique: li.Technique}, true
	}
	return Info{}, false
}

// Reporter stamps and stores the events of one driver connection. A nil
// Reporter, or one without a Writer (tests), only logs them.
type Reporter struct {
	W      *jssecevents.Writer
	Origin jssecevents.Origin
	Role   string // "client" or "server"
}

// Emit completes e (protocol, role, source, classification) and stores it.
func (r *Reporter) Emit(e jssecevents.Event) {
	if e.Protocol == "" {
		e.Protocol = "modbus"
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
	if r == nil || r.W == nil {
		jslog.Log(jslog.LevelDetailed, "Security event %s [%s] %s %s", e.Kind, e.Severity, e.PeerAddress, e.Detail)
		return
	}
	if e.Role == "" {
		e.Role = r.Role
	}
	r.W.Emit(r.Origin, e)
}

// Enabled reports whether events are stored (not only logged).
func (r *Reporter) Enabled() bool { return r != nil && r.W != nil }

// Fields are the Modbus fields of a request; a negative value is left out.
func Fields(unit, fc, addr, qty, exception int) bson.M {
	m := bson.M{}
	if unit >= 0 {
		m["unitId"] = unit
	}
	if fc >= 0 {
		m["functionCode"] = fc
		m["function"] = mbcore.FCName(fc)
	}
	if addr >= 0 {
		m["address"] = addr
	}
	if qty >= 0 {
		m["quantity"] = qty
	}
	if exception > 0 {
		m["exception"] = exception
		m["exceptionName"] = mbcore.ExceptionName(exception)
	}
	return m
}

// Exception returns the exception code of an exception response error.
func Exception(err error) (code int, ok bool) {
	var ex *modbus.ExceptionError
	if errors.As(err, &ex) {
		return int(ex.ExceptionCode), true
	}
	return 0, false
}

// Peer sets the peer address fields of e from "host:port".
func Peer(e *jssecevents.Event, addr string) {
	if addr == "" {
		return
	}
	e.PeerAddress = addr
	if ip := jssecevents.SplitAddr(addr); ip.IsValid() {
		e.PeerIP = ip.String()
	}
}

// FromLib maps a go-modbus event onto a secEvents document. resolve names
// the real peer of a loopback address (the server's front-end); nil keeps
// the address the library saw.
func FromLib(e secevent.Event, resolve func(addr string) string) jssecevents.Event {
	d := jssecevents.Event{
		TimeTag:   e.Time,
		Kind:      string(e.Kind),
		Category:  e.Category.String(),
		Severity:  e.Severity.String(),
		Action:    e.Action.String(),
		Source:    SourceLibrary,
		Protocol:  e.Protocol,
		Role:      e.Role,
		SessionID: int64(e.ConnID),
		Detail:    e.Detail,
		Count:     e.Count,
		Technique: e.Technique,
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
	if e.Peer.RemoteAddr.IsValid() {
		addr := e.Peer.RemoteAddr.String()
		if resolve != nil {
			addr = resolve(addr)
		}
		Peer(&d, addr)
	}
	if t := e.Peer.TLS; t != nil {
		tm := bson.M{"verified": t.Verified, "version": jssecevents.TLSVersionName(t.Version)}
		if t.Subject != "" {
			tm["subject"], tm["issuer"], tm["serial"] = t.Subject, t.Issuer, t.SerialHex
		}
		if t.FingerprintSHA256 != "" {
			tm["sha256"] = t.FingerprintSHA256
		}
		d.Peer = bson.M{"tls": tm}
	}
	unit, fc, addr, qty, exc := -1, -1, -1, -1, 0
	if e.Has(secevent.FieldUnitID) {
		unit = int(e.UnitID)
	}
	if e.Has(secevent.FieldFunctionCode) {
		fc = int(e.FunctionCode)
	}
	if e.Has(secevent.FieldAddress) {
		addr = int(e.Address)
	}
	if e.Has(secevent.FieldQuantity) {
		qty = int(e.Quantity)
	}
	if e.Has(secevent.FieldException) {
		exc = int(e.Exception)
	}
	f := Fields(unit, fc, addr, qty, exc)
	if e.Has(secevent.FieldSubFunction) {
		f["subFunction"] = int(e.SubFunction)
	}
	if e.Has(secevent.FieldTransactionID) {
		f["transactionId"] = int(e.TransactionID)
	}
	if e.Has(secevent.FieldExpectedTxnID) {
		f["expectedTransactionId"] = int(e.ExpectedTxnID)
	}
	if len(e.Raw) > 0 {
		f["raw"] = hex.EncodeToString(e.Raw)
	}
	if len(f) > 0 {
		d.Fields = f
	}
	return d
}
