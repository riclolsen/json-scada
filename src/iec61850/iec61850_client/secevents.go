/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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

// Security events (secEvents collection) and identification of the IED.
//
// The library's client side reports no security events yet, so everything
// here is detected by the driver: the IED's identity (MMS Identify, the
// LPHD1.PhyNam and LLN0.NamPlt nameplates), its MAC address and TLS
// certificate on every association, and their changes; refused
// certificates; reads and commands of objects the IED does not have;
// refused, flooding and unnecessary commands; reports nothing enabled and
// report sequence numbers going backwards.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Command flooding: more than commandFloodMax commands to one IED within
// commandFloodWindow.
const (
	commandFloodMax    = 10
	commandFloodWindow = 10 * time.Second
	maxIdentifiedLDs   = 32
)

// secWriter delivers the events of every connection; nil (tests) keeps
// them in no collection.
var (
	secWriter   *jssecevents.Writer
	secNodeName string
)

// connSec is the security event state of one connection.
type connSec struct {
	mu       sync.Mutex
	changes  jssecevents.Tracker // identity, nameplates, MAC, certificate
	flood    jssecevents.RateLimit
	reported map[string]bool // per association: what was already reported once
	identity bson.M          // stats.peerIdentity
	// certReported is set once the refused certificate of the current dial
	// was reported, so the dial error does not report it again.
	certReported bool
	peerCert     *x509.Certificate
	peerTLS      *secevent.TLSPeer
	session      uint64
	peerIP       netip.Addr
	addr         string
}

func (c *Iec61850Connection) secOrigin() jssecevents.Origin {
	return jssecevents.Origin{ProtocolDriver: ProtocolDriverName,
		ProtocolDriverInstanceNumber: c.ProtocolDriverInstanceNumber,
		ProtocolConnectionNumber:     c.ProtocolConnectionNumber,
		ConnectionName:               c.Name, NodeName: secNodeName}
}

// secEmit stores a driver event about this connection's IED.
func (c *Iec61850Connection) secEmit(e secevent.Event) {
	if secWriter == nil {
		jslog.Log(jslog.LevelDetailed, "%s security event %s %s", c.Name, e.Kind, e.Detail)
		return
	}
	s := &c.sec
	s.mu.Lock()
	if !e.Peer.RemoteAddr.IsValid() && s.peerIP.IsValid() {
		e.Peer.RemoteAddr = netip.AddrPortFrom(s.peerIP, portOf(s.addr))
		e.Peer.Network = "tcp"
	}
	if e.Peer.TLS == nil && s.peerTLS != nil {
		t := *s.peerTLS
		e.Peer.TLS = &t
	}
	if e.SessionID == 0 {
		e.SessionID = s.session
	}
	s.mu.Unlock()
	e.Protocol, e.Role = "mms", "client"
	secWriter.Emit(c.secOrigin(), toSecDoc(e, secSourceDriver))
}

// secOnce reports e only the first time key is seen in this association.
func (c *Iec61850Connection) secOnce(key string, e secevent.Event) {
	s := &c.sec
	s.mu.Lock()
	if s.reported == nil {
		s.reported = map[string]bool{}
	}
	if s.reported[key] || len(s.reported) > 10000 {
		s.mu.Unlock()
		return
	}
	s.reported[key] = true
	s.mu.Unlock()
	c.secEmit(e)
}

func portOf(addr string) uint16 {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseUint(p, 10, 16)
	return uint16(n)
}

// secNewAssociation starts the state of a new association with the IED at
// addr ("host:port").
func (c *Iec61850Connection) secNewAssociation(addr string) {
	ip := jssecevents.SplitAddr(addr)
	if !ip.IsValid() {
		host, _, _ := net.SplitHostPort(addr)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host); err == nil && len(ips) > 0 {
			ip = ips[0].Unmap()
		}
		cancel()
	}
	s := &c.sec
	s.mu.Lock()
	s.reported = map[string]bool{}
	s.session = secevent.NextSessionID()
	s.peerIP, s.addr = ip, addr
	s.mu.Unlock()
}

// seedIdentity starts the change detection from the identity stored in the
// connection's statistics, so a change across a driver restart is seen.
func (c *Iec61850Connection) seedIdentity(stored bson.M) {
	if stored == nil {
		return
	}
	c.sec.mu.Lock()
	c.sec.identity = stored
	c.sec.mu.Unlock()
	for _, k := range jsmongo.GetDocArray(stored, "keys") {
		c.sec.changes.Seed(jsmongo.GetString(k, "k", ""), jsmongo.GetString(k, "v", ""))
	}
}

// PeerIdentityDoc is stats.peerIdentity, nil when never identified.
func (c *Iec61850Connection) PeerIdentityDoc() bson.M {
	c.sec.mu.Lock()
	defer c.sec.mu.Unlock()
	return c.sec.identity
}

// --- TLS ----------------------------------------------------------------

// withCertEvents records the IED's certificate and reports a refused one,
// whether the driver's verification callback refused it or Go's own chain
// and host name checks did (secDialFailed reports those).
func (c *Iec61850Connection) withCertEvents(cfg *tls.Config) *tls.Config {
	c.sec.mu.Lock()
	c.sec.certReported = false
	c.sec.mu.Unlock()
	if verify := cfg.VerifyPeerCertificate; verify != nil {
		cfg.VerifyPeerCertificate = func(raw [][]byte, chains [][]*x509.Certificate) error {
			err := verify(raw, chains)
			if err != nil {
				c.secCertRejected(raw, err)
			}
			return err
		}
	}
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		t := &secevent.TLSPeer{Version: cs.Version, CipherSuite: cs.CipherSuite, ServerName: cs.ServerName}
		var leaf *x509.Certificate
		if len(cs.PeerCertificates) > 0 {
			leaf = cs.PeerCertificates[0]
			t.CertFields(leaf)
		}
		t.Verified = len(cs.VerifiedChains) > 0 || cfg.VerifyPeerCertificate != nil
		c.sec.mu.Lock()
		c.sec.peerCert, c.sec.peerTLS = leaf, t
		c.sec.mu.Unlock()
		return nil
	}
	return cfg
}

// secCertRejected reports the IED's refused certificate, once per dial.
func (c *Iec61850Connection) secCertRejected(raw [][]byte, err error) {
	c.sec.mu.Lock()
	done := c.sec.certReported
	c.sec.certReported = true
	c.sec.mu.Unlock()
	if done {
		return
	}
	t := &secevent.TLSPeer{}
	if len(raw) > 0 {
		if cert, perr := x509.ParseCertificate(raw[0]); perr == nil {
			t.CertFields(cert)
		} else {
			t.FingerprintSHA256 = secevent.Fingerprint(raw[0])
		}
	}
	c.secEmit(secevent.Event{Kind: secevent.KindTLSCertRejected, Protocol: "tls",
		Action: secevent.Disconnected, Peer: secevent.PeerInfo{TLS: t}, Err: err,
		Detail: "IED certificate refused"})
}

// secDialFailed reports a dial that failed on the IED's certificate in Go's
// own verification (unknown authority, expired, wrong host name), which
// does not pass through VerifyPeerCertificate.
func (c *Iec61850Connection) secDialFailed(err error) {
	if !jssecevents.IsCertError(err) {
		return
	}
	var raw [][]byte
	var ve *tls.CertificateVerificationError
	if errors.As(err, &ve) {
		for _, cert := range ve.UnverifiedCertificates {
			raw = append(raw, cert.Raw)
		}
	}
	c.secCertRejected(raw, err)
}

// --- identification -----------------------------------------------------

// identifyPeer reads the IED's identity, publishes it in the statistics and
// reports what changed since the previous association.
func identifyPeer(ctx context.Context, conn *Iec61850Connection, cli *client.Client, vendor, modelName, revision string, idErr error) {
	budget := conn.requestTimeout()
	id := bson.M{"collectedAt": time.Now(), "address": conn.sec.addr}
	var keys []struct{ k, v, label string }
	addKey := func(k, v, label string) {
		if v != "" {
			keys = append(keys, struct{ k, v, label string }{k, v, label})
		}
	}

	if idErr == nil {
		id["vendor"], id["model"], id["revision"] = vendor, modelName, revision
		addKey("identify", vendor+" | "+modelName+" | "+revision, "MMS identity")
	} else {
		id["identifyError"] = idErr.Error()
	}

	lctx, cancel := context.WithTimeout(ctx, budget)
	lds, err := cli.LogicalDevices(lctx)
	cancel()
	if err == nil {
		sort.Strings(lds)
		if len(lds) > maxIdentifiedLDs {
			lds = lds[:maxIdentifiedLDs]
		}
		devices := bson.A{}
		for _, ld := range lds {
			dev := bson.M{"ld": ld}
			if phy := readNameplate(ctx, cli, budget, ld, "LPHD1", "PhyNam"); len(phy) > 0 {
				dev["phyNam"] = phy
				addKey("phyNam:"+ld, joinFields(phy, "vendor", "model", "hwRev", "swRev", "serNum"),
					ld+" physical device nameplate")
			}
			if np := readNameplate(ctx, cli, budget, ld, "LLN0", "NamPlt"); len(np) > 0 {
				dev["namPlt"] = np
				addKey("namPlt:"+ld, joinFields(np, "vendor", "swRev", "configRev"), ld+" logical device nameplate")
			}
			devices = append(devices, dev)
		}
		id["devices"] = devices
	}

	conn.sec.mu.Lock()
	ip, peerTLS := conn.sec.peerIP, conn.sec.peerTLS
	conn.sec.mu.Unlock()
	if ip.IsValid() {
		id["ip"] = ip.String()
		if mac := jssecevents.LookupMAC(ip); mac != "" {
			id["mac"] = mac
			addKey("mac", mac, "MAC address")
		}
	}
	if peerTLS != nil {
		id["tls"] = tlsDoc(peerTLS)
		addKey("cert", peerTLS.FingerprintSHA256, "certificate")
	}

	previous := conn.PeerIdentityDoc()
	first := previous == nil
	stored := bson.A{}
	now := map[string]bool{}
	for _, k := range keys {
		now[k.k] = true
	}
	// Keep what this association could not read (Identify or a nameplate
	// that failed), so a later change of it is still seen across restarts.
	for _, k := range jsmongo.GetDocArray(previous, "keys") {
		if kk := jsmongo.GetString(k, "k", ""); kk != "" && !now[kk] {
			stored = append(stored, bson.M{"k": kk, "v": jsmongo.GetString(k, "v", "")})
		}
	}
	for _, k := range keys {
		stored = append(stored, bson.M{"k": k.k, "v": k.v})
		prev, changed := conn.sec.changes.Observe(k.k, k.v)
		if !changed {
			continue
		}
		kind := secevent.KindPeerIdentityChanged
		switch k.k {
		case "mac":
			kind = secevent.KindPeerMACChanged
		case "cert":
			kind = secevent.KindTLSCertChanged
		}
		conn.secEmit(secevent.Event{Kind: kind, ObjectRef: strings.TrimPrefix(strings.TrimPrefix(
			strings.TrimPrefix(k.k, "phyNam:"), "namPlt:"), "identify"),
			Value: k.v, Detail: k.label + " changed, was " + prev})
	}
	id["keys"] = stored
	if first {
		conn.secEmit(secevent.Event{Kind: kindPeerIdentified, Value: identitySummary(id),
			Detail: "first identification of the IED"})
	}
	conn.sec.mu.Lock()
	conn.sec.identity = id
	conn.sec.mu.Unlock()
	jslog.Log(jslog.LevelBasic, "%s Peer identity: %s", conn.Name, identitySummary(id))
}

func identitySummary(id bson.M) string {
	var parts []string
	for _, f := range []string{"vendor", "model", "revision", "mac"} {
		if v, ok := id[f].(string); ok && v != "" {
			parts = append(parts, f+"="+v)
		}
	}
	return strings.Join(parts, " ")
}

func joinFields(m bson.M, names ...string) string {
	var out []string
	set := false
	for _, n := range names {
		v, _ := m[n].(string)
		set = set || v != ""
		out = append(out, v)
	}
	if !set {
		return ""
	}
	return strings.Join(out, " | ")
}

// readNameplate reads a nameplate data object (FC DC) into its named
// attributes; nil when the IED does not have it.
func readNameplate(ctx context.Context, cli *client.Client, budget time.Duration, ld, ln, do string) bson.M {
	rctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	spec, err := cli.MMS().GetVariableAccessAttributes(rctx, ld, ln+"$DC$"+do)
	if err != nil || spec == nil || spec.Kind != mms.TypeStructure {
		return nil
	}
	v, err := cli.Read(rctx, model.ObjectReference(ld+"/"+ln+"."+do), model.DC)
	if err != nil || v == nil || v.Type() != mms.TypeStructure {
		return nil
	}
	out := bson.M{}
	for i, comp := range spec.Components {
		if i >= v.Len() {
			break
		}
		if s, ok := nameplateText(v.Index(i)); ok {
			out[comp.Name] = s
		}
	}
	return out
}

func nameplateText(v *mms.Value) (string, bool) {
	if v == nil {
		return "", false
	}
	switch v.Type() {
	case mms.TypeVisibleString, mms.TypeMMSString:
		return strings.TrimSpace(v.Text()), true
	case mms.TypeInteger, mms.TypeUnsigned:
		return fmt.Sprint(v.Int64()), true
	}
	return "", false
}

// --- reads --------------------------------------------------------------

// secReadError reports a read the IED refused for a reason that is not a
// transient one: an object it does not have, or one it denies access to.
func (c *Iec61850Connection) secReadError(e *Iec61850Entry, err error) {
	var dae mms.DataAccessError
	if !errors.As(err, &dae) {
		return
	}
	kind := secevent.KindAccessObjectNonexistent
	switch dae {
	case mms.AccessObjectNonExistent:
	case mms.AccessObjectAccessDenied:
		kind = secevent.KindAccessDenied
	default:
		return
	}
	c.secOnce("read:"+entryKey(e.Path, e.FC), secevent.Event{Kind: kind, ObjectRef: e.Path, FC: e.FC.String(),
		Fields: secevent.HasAccessError, AccessError: int(dae), Err: err,
		Detail: "configured point refused by the IED (tag " + e.JsTag + ")"})
}

// --- reports ------------------------------------------------------------

// secUnmatchedReport reports an information report no enabled block sent.
func (c *Iec61850Connection) secUnmatchedReport(rptID string) {
	c.secOnce("rpt:"+rptID, secevent.Event{Kind: kindReportUnexpected, ObjectRef: rptID,
		Detail: "report with an RptID no block enabled by this driver advertises"})
}

// secReportSequence checks a report's sequence number against the last one
// of its block in this association: an equal one (a duplicate) or a smaller
// one that is not a wrap-around means a replayed or reordered report.
func (c *Iec61850Connection) secReportSequence(st *rcbState, rep *client.Report) {
	if rep.SubSeqNum != 0 {
		return // a segment repeats the sequence number of its report
	}
	cur := rep.SeqNum
	c.mu.Lock()
	last, seen := st.lastSeq, st.seqSeen
	st.lastSeq, st.seqSeen = cur, true
	c.mu.Unlock()
	if !seen || cur > last || last-cur >= 64 {
		return
	}
	detail := "sequence number went backwards"
	if cur == last {
		detail = "sequence number repeated"
	}
	c.secEmit(secevent.Event{Kind: kindReportSequence, ObjectRef: st.ref, Fields: secevent.HasSqNum, SqNum: cur,
		Value: fmt.Sprint(cur), Detail: fmt.Sprintf("%s (previous %d)", detail, last)})
}

// --- commands -----------------------------------------------------------

// secCommand counts a command for the flood detector and reports one that
// would not change the state it commands. The check runs in the background,
// against the state before the dispatch: it never delays the command.
func (c *Iec61850Connection) secCommand(ctx context.Context, rt *mongo.Collection, e *Iec61850Entry, doc bson.M, value float64) {
	c.sec.mu.Lock()
	if c.sec.flood.Max == 0 {
		c.sec.flood = jssecevents.RateLimit{Max: commandFloodMax, Window: commandFloodWindow}
	}
	c.sec.mu.Unlock()
	if over, n := c.sec.flood.Hit("cmd", time.Now()); over {
		c.secEmit(secevent.Event{Kind: secevent.KindFloodControls, ObjectRef: e.Path, Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d commands to the IED within %s", commandFloodMax, commandFloodWindow)})
	}
	if rt == nil || e.FC != model.CO || (value != 0 && value != 1) {
		return
	}
	before, key, tag, ref := time.Now(), jsmongo.GetDouble(doc, "pointKey", 0), jsmongo.GetString(doc, "tag", ""), e.Path
	go func() {
		state, ok, err := jssecevents.SupervisedState(context.WithoutCancel(ctx), rt, key, before)
		if err != nil || !ok {
			return
		}
		if (state != 0) == (value != 0) {
			c.secEmit(secevent.Event{Kind: kindCmdUnnecessary, ObjectRef: ref, Value: fmt.Sprint(value),
				Detail: "the supervised point already was in the commanded state (tag " + tag + ")"})
		}
	}()
}

// secCommandRefused reports a command refused by the driver or the IED.
func (c *Iec61850Connection) secCommandRefused(kind secevent.Kind, ref string, err error, detail string) {
	e := secevent.Event{Kind: kind, Action: secevent.Rejected, ObjectRef: ref, Err: err, Detail: detail}
	var ce *client.ControlError
	if errors.As(err, &ce) {
		e.Fields |= secevent.HasAddCause
		e.AddCause = int(ce.AddCause)
		e.Detail = strings.TrimSpace(detail + " at " + ce.Stage + ": " + ce.AddCause.String())
	}
	c.secEmit(e)
}
