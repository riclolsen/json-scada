/*
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

// Mapping of go-iec61850 security events (package secevent) onto secEvents
// documents. This file is identical in iec61850_client and iec61850_server:
// change both together.

package main

import (
	"encoding/hex"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/dscsystems/go-iec61850/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Event sources, the secEvents source field.
const (
	secSourceLibrary = "library" // detected by go-iec61850
	secSourceDriver  = "driver"  // detected by the driver's own code
)

// Kinds the driver reports that the library catalogue has no entry for.
const (
	kindCmdUnnecessary   secevent.Kind = "cmd.unnecessary"     // a control that would not change the state
	kindReportSequence   secevent.Kind = "report.sequence"     // a report sequence number went backwards
	kindReportUnexpected secevent.Kind = "report.unexpected"   // a report no enabled block sent
	kindPeerIdentified   secevent.Kind = "peer.identified"     // first identification of a peer
	kindCmdNonexistent   secevent.Kind = "cmd.object_unknown"  // a command for an object the peer does not have
	kindAccessNonexist   secevent.Kind = "access.tag_unknown"  // a command for a tag that is not configured
	kindPeerNameplate    secevent.Kind = "peer.nameplate_read" // nameplate could not be read
)

// driverKinds classifies the driver's own kinds.
var driverKinds = map[secevent.Kind]secevent.KindInfo{
	kindCmdUnnecessary:   {Category: secevent.CatControl, Severity: secevent.Notice, Technique: "T0855"},
	kindReportSequence:   {Category: secevent.CatProtocol, Severity: secevent.Warning, Technique: "T0831"},
	kindReportUnexpected: {Category: secevent.CatUnsolicited, Severity: secevent.Warning},
	kindPeerIdentified:   {Category: secevent.CatPeer, Severity: secevent.Info},
	kindCmdNonexistent:   {Category: secevent.CatControl, Severity: secevent.Warning},
	kindAccessNonexist:   {Category: secevent.CatAccess, Severity: secevent.Warning},
	kindPeerNameplate:    {Category: secevent.CatPeer, Severity: secevent.Info},
}

// kindInfo is the classification of a kind, the driver's own included.
func kindInfo(k secevent.Kind) secevent.KindInfo {
	if info, ok := driverKinds[k]; ok {
		if info.Std62351 == "" {
			info.Std62351 = "x-json-scada." + string(k)
		}
		return info
	}
	return k.Info()
}

// toSecDoc maps e onto a secEvents document; source is secSourceLibrary or
// secSourceDriver. Fields the kind fixes are filled when e leaves them unset.
func toSecDoc(e secevent.Event, source string) jssecevents.Event {
	info := kindInfo(e.Kind)
	sev := e.Severity
	if sev == secevent.Info {
		sev = info.Severity
	}
	d := jssecevents.Event{
		TimeTag:   e.Time,
		Kind:      string(e.Kind),
		Category:  string(e.Category),
		Severity:  sev.String(),
		Action:    string(e.Action),
		Source:    source,
		Protocol:  e.Protocol,
		Role:      e.Role,
		SessionID: int64(e.SessionID),
		ObjectRef: e.ObjectRef,
		FC:        e.FC,
		Value:     e.Value,
		Detail:    e.Detail,
		Count:     e.Count,
		Technique: e.Technique,
		Std62351:  e.Std62351,
	}
	if d.TimeTag.IsZero() {
		d.TimeTag = time.Now()
	}
	if d.Category == "" {
		d.Category = string(info.Category)
	}
	if d.Technique == "" {
		d.Technique = info.Technique
	}
	if d.Std62351 == "" {
		d.Std62351 = info.Std62351
	}
	if e.Err != nil {
		d.Error = e.Err.Error()
	}
	if !e.FirstSeen.IsZero() && e.Count > 1 {
		fs := e.FirstSeen
		d.FirstSeen = &fs
	}

	p := e.Peer
	if p.RemoteAddr.IsValid() {
		d.PeerAddress = p.RemoteAddr.String()
		d.PeerIP = p.RemoteAddr.Addr().Unmap().String()
	}
	if p.LocalAddr.IsValid() {
		d.LocalAddress = p.LocalAddr.String()
	}
	if len(p.MAC) > 0 {
		d.PeerMAC = p.MAC.String()
	}
	peer := bson.M{}
	if p.MACSource != "" {
		peer["macSource"] = string(p.MACSource)
	}
	if t := p.TLS; t != nil {
		peer["tls"] = tlsDoc(t)
	}
	if a := p.ACSE; a != nil {
		ac := bson.M{"passwordPresented": a.PasswordPresented}
		if a.CallingAPTitle != "" {
			ac["callingApTitle"] = a.CallingAPTitle
		}
		if a.CallingAEQualifier != nil {
			ac["callingAeQualifier"] = *a.CallingAEQualifier
		}
		if a.CalledAPTitle != "" {
			ac["calledApTitle"] = a.CalledAPTitle
		}
		if a.AProfileSubject != "" {
			ac["aProfileSubject"] = a.AProfileSubject
			ac["aProfileSha256"] = a.AProfileSHA256
		}
		if a.E2E {
			ac["e2ePeerAe"] = a.E2EPeerAE
		}
		peer["acse"] = ac
	}
	if len(peer) > 0 {
		d.Peer = peer
	}

	f := bson.M{}
	if e.Fields&secevent.HasService != 0 {
		f["service"] = e.Service
	}
	if e.Fields&secevent.HasInvokeID != 0 {
		f["invokeId"] = int64(e.InvokeID)
	}
	if e.Fields&secevent.HasAccessError != 0 {
		f["accessError"] = e.AccessError
	}
	if e.Fields&secevent.HasAddCause != 0 {
		f["addCause"] = e.AddCause
	}
	if e.Fields&secevent.HasCtlNum != 0 {
		f["ctlNum"] = int(e.CtlNum)
	}
	if e.Fields&secevent.HasOrigin != 0 {
		f["orCat"] = e.OrCat
		f["orIdent"] = e.OrIdent
	}
	if e.Fields&secevent.HasTest != 0 {
		f["test"] = e.Test
	}
	if e.Fields&secevent.HasSqNum != 0 {
		f["sqNum"] = int64(e.SqNum)
	}
	if e.Fields&secevent.HasConfRev != 0 {
		f["confRev"] = int64(e.ConfRev)
	}
	if e.Stage != "" {
		f["stage"] = e.Stage
	}
	if s := e.Session; s != nil {
		f["session"] = bson.M{"reason": s.Reason, "durationMs": s.Duration.Milliseconds(),
			"requests": int64(s.Requests), "octetsIn": int64(s.OctetsIn), "octetsOut": int64(s.OctetsOut)}
	}
	if len(e.Raw) > 0 {
		f["raw"] = hex.EncodeToString(e.Raw)
	}
	if len(f) > 0 {
		d.Fields = f
	}
	return d
}

// tlsDoc describes a peer's TLS state and certificate.
func tlsDoc(t *secevent.TLSPeer) bson.M {
	m := bson.M{"verified": t.Verified}
	if t.Subject != "" {
		m["subject"] = t.Subject
	}
	if t.Issuer != "" {
		m["issuer"] = t.Issuer
	}
	if t.SerialHex != "" {
		m["serial"] = t.SerialHex
	}
	if t.FingerprintSHA256 != "" {
		m["sha256"] = t.FingerprintSHA256
	}
	if !t.NotAfter.IsZero() {
		m["notAfter"] = t.NotAfter
	}
	if t.Version != 0 {
		m["version"] = tlsVersionName(t.Version)
	}
	return m
}

func tlsVersionName(v uint16) string {
	switch v {
	case 0x0301:
		return "TLS1.0"
	case 0x0302:
		return "TLS1.1"
	case 0x0303:
		return "TLS1.2"
	case 0x0304:
		return "TLS1.3"
	}
	return ""
}
