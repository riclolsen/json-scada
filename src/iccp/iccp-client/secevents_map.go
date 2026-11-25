/*
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

// Mapping of tase2 security events (package secevent) onto secEvents
// documents. This file is identical in iccp-client and iccp-server: change
// both together.

package main

import (
	"encoding/hex"
	"strconv"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/riclolsen/tase2/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Event sources, the secEvents source field.
const (
	secSourceLibrary = "library" // detected by the tase2 library
	secSourceDriver  = "driver"  // detected by the driver's own code
)

// Kinds the driver reports that the library catalogue has no entry for.
const (
	kindPeerIdentified      secevent.Kind = "peer.identified"       // first identification of a peer
	kindPeerIdentityChanged secevent.Kind = "peer.identity_changed" // the peer's stated identity changed
	kindCmdUnnecessary      secevent.Kind = "cmd.unnecessary"       // a control that would not change the state
	kindCmdObjectUnknown    secevent.Kind = "cmd.object_unknown"    // a command for an object the peer or model does not have
	kindCmdWrongType        secevent.Kind = "cmd.wrong_type"        // a command value of the wrong type for its object
	kindAccessTagUnknown    secevent.Kind = "access.tag_unknown"    // a command for a tag that is not configured
)

// driverKinds classifies the driver's own kinds.
var driverKinds = map[secevent.Kind]secevent.KindInfo{
	kindPeerIdentified:      {Category: secevent.CatPeer, Severity: secevent.SeverityInfo},
	kindPeerIdentityChanged: {Category: secevent.CatPeer, Severity: secevent.SeverityAlert, Technique: "T0849", AlwaysDeliver: true},
	kindCmdUnnecessary:      {Category: secevent.CatControl, Severity: secevent.SeverityNotice, Technique: "T0855"},
	kindCmdObjectUnknown:    {Category: secevent.CatControl, Severity: secevent.SeverityWarning, Technique: "T0855"},
	kindCmdWrongType:        {Category: secevent.CatControl, Severity: secevent.SeverityWarning, Technique: "T0855"},
	kindAccessTagUnknown:    {Category: secevent.CatAccess, Severity: secevent.SeverityWarning},
}

// kindInfo is the classification of a kind, the driver's own included.
func kindInfo(k secevent.Kind) secevent.KindInfo {
	if info, ok := driverKinds[k]; ok {
		return info
	}
	info, _ := k.Info()
	return info
}

// objectRefOf renders the object an event concerns: "Domain/Item" (the
// item alone at VCC scope), else the transfer set or dataset.
func objectRefOf(e secevent.Event) string {
	name := e.Item
	if name == "" {
		name = e.TransferSet
	}
	if name == "" {
		name = e.DataSet
	}
	if e.Domain != "" && name != "" {
		return e.Domain + "/" + name
	}
	if name != "" {
		return name
	}
	return e.Domain
}

// toSecDoc maps e onto a secEvents document; source is secSourceLibrary or
// secSourceDriver. Fields the kind fixes are filled when e leaves them unset.
func toSecDoc(e secevent.Event, source string) jssecevents.Event {
	info := kindInfo(e.Kind)
	sev := e.Severity
	if sev == secevent.SeverityInfo {
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
		SessionID: int64(e.ConnID),
		ObjectRef: objectRefOf(e),
		Value:     e.Value,
		Detail:    e.Detail,
		Count:     e.Count,
		Technique: e.Technique,
	}
	if d.TimeTag.IsZero() {
		d.TimeTag = time.Now()
	}
	if d.Protocol == "" {
		d.Protocol = "iccp"
	}
	if d.Category == "" {
		d.Category = string(info.Category)
	}
	if d.Technique == "" {
		d.Technique = info.Technique
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
	if peer := peerDoc(p, e.Role); len(peer) > 0 {
		d.Peer = peer
	}

	f := bson.M{}
	for k, v := range map[string]string{"layer": e.Layer, "phase": e.Phase, "reason": e.Reason,
		"service": e.Service, "domain": e.Domain, "item": e.Item, "transferSet": e.TransferSet,
		"dataSet": e.DataSet} {
		if v != "" {
			f[k] = v
		}
	}
	if e.HasInvokeID {
		f["invokeId"] = int64(e.InvokeID)
	}
	if e.ErrorClass != 0 || e.ErrorCode != 0 {
		f["errorClass"], f["errorCode"] = e.ErrorClass, e.ErrorCode
	}
	if len(e.Raw) > 0 {
		f["raw"] = hex.EncodeToString(e.Raw)
	}
	if len(f) > 0 {
		d.Fields = f
	}
	return d
}

// peerDoc is the secEvents peer field: what is known of the peer's
// identity besides its addresses. The ACSE AP title is the calling one the
// peer stated (server side) or the called one this client addressed.
func peerDoc(p secevent.PeerInfo, role string) bson.M {
	peer := bson.M{}
	if p.MACSource != "" {
		peer["macSource"] = string(p.MACSource)
	}
	if p.Interface != "" {
		peer["interface"] = p.Interface
	}
	if t := p.TLS; t != nil {
		peer["tls"] = tlsDoc(t)
	}
	if p.APTitle != "" || p.Authenticated {
		ac := bson.M{"authenticated": p.Authenticated}
		key, qual := "callingApTitle", "callingAeQualifier"
		if role == "client" {
			key, qual = "calledApTitle", "calledAeQualifier"
		}
		if p.APTitle != "" {
			ac[key], ac[qual] = p.APTitle, p.AEQualifier
		}
		peer["acse"] = ac
	}
	if p.BLT != "" {
		peer["blt"] = p.BLT
	}
	return peer
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
	if v := jssecevents.TLSVersionName(t.Version); v != "" {
		m["version"] = v
	}
	return m
}

// acseIdentity is the "apTitle/aeQualifier" text of a peer's ACSE
// identity, "" when it stated none.
func acseIdentity(p secevent.PeerInfo) string {
	if p.APTitle == "" {
		return ""
	}
	return p.APTitle + "/" + strconv.Itoa(p.AEQualifier)
}
