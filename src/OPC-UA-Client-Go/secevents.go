/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
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

// Security events (secEvents collection) and identification of the server.
//
// The opcua library reports what it sees on the wire (malformed and
// oversized messages, sequence errors, failed signatures, refused server
// certificates, unknown channels, unsolicited responses, MAC address
// changes on a reconnection); the driver adds what needs its configuration:
// the server's identity (ServerStatus.BuildInfo, the endpoint's application
// description and certificate, the MAC address) on every session and its
// changes, server restarts, configured nodes the server does not have,
// refused, unknown, mistyped, stale, flooding and unnecessary commands, and
// commands that restart the server, change its configuration or transfer
// files (deviations D37 and D38).

package main

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/secevent"
	"github.com/gopcua/opcua/ua"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	secSourceLibrary = "library"
	secSourceDriver  = "driver"

	// Command flooding: more than commandFloodMax commands to one server
	// within commandFloodWindow.
	commandFloodMax    = 10
	commandFloodWindow = 10 * time.Second

	// libEventEvery: the same library event (kind, status, detail) is stored
	// once per period, with the count of the ones left out. A server that
	// keeps sending a bad message would otherwise fill the collection.
	libEventEvery = 10 * time.Second
	// refusalEvery: the same refusal of the same command address.
	refusalEvery = time.Minute
	// unknownNodeEvery: a configured node the server does not have is
	// reported once per period, not on every reconnection.
	unknownNodeEvery = time.Hour
	// maxUnknownNodeEvents: past this many in one subscription setup, the
	// rest are summarized in one event.
	maxUnknownNodeEvents = 20
	// certRejectedEvery: a refused server certificate, retried on every
	// connection attempt.
	certRejectedEvery = 10 * time.Minute
)

// Kinds detected by the driver.
const (
	kindPeerIdentified      = "peer.identified"
	kindPeerIdentityChanged = "peer.identity_changed"
	kindPeerMACChanged      = "peer.mac_changed"
	kindTLSCertChanged      = "tls.cert_changed"
	kindTLSCertRejected     = "tls.cert_rejected"
	kindRemoteRestart       = "remote.restart"
	kindRemoteState         = "remote.state"
	kindAccessUnknownNode   = "access.unknown_node"
	kindCmdRejected         = "cmd.rejected"
	kindCmdObjectUnknown    = "cmd.object_unknown"
	kindCmdWrongType        = "cmd.wrong_type"
	kindCmdStaleTime        = "cmd.stale_time"
	kindCmdUnnecessary      = "cmd.unnecessary"
	kindFloodCommands       = "flood.commands"
	kindCtrlRestart         = "ctrl.restart"
	kindCtrlConfigChange    = "ctrl.config_change"
	kindFileTransfer        = "file.transfer"
	kindMethodCall          = "ctrl.method_call"
)

// driverKinds gives each driver kind its category and severity.
var driverKinds = map[string]struct{ category, severity string }{
	kindPeerIdentified:      {"peer", "info"},
	kindPeerIdentityChanged: {"peer", "alert"},
	kindPeerMACChanged:      {"peer", "alert"},
	kindTLSCertChanged:      {"security", "alert"},
	kindTLSCertRejected:     {"security", "alert"},
	kindRemoteRestart:       {"remote", "warning"},
	kindRemoteState:         {"remote", "warning"},
	kindAccessUnknownNode:   {"access", "warning"},
	kindCmdRejected:         {"command", "warning"},
	kindCmdObjectUnknown:    {"command", "warning"},
	kindCmdWrongType:        {"command", "warning"},
	kindCmdStaleTime:        {"command", "warning"},
	kindCmdUnnecessary:      {"command", "notice"},
	kindFloodCommands:       {"flood", "alert"},
	kindCtrlRestart:         {"control", "notice"},
	kindCtrlConfigChange:    {"control", "notice"},
	kindFileTransfer:        {"file", "notice"},
	kindMethodCall:          {"control", "notice"},
}

// secWriter delivers the events of every connection; nil (tests) keeps
// them in no collection. secCapture, when set (tests), sees every event
// stored.
var (
	secWriter   *jssecevents.Writer
	secNodeName string
	secCapture  func(jssecevents.Event)
)

// connSec is the security event state of one connection. It survives the
// sessions: the identity history, the throttles and the flood detector are
// per connection.
type connSec struct {
	mu         sync.Mutex
	changes    jssecevents.Tracker // identity fields, MAC address, certificate
	flood      jssecevents.RateLimit
	libRepeat  jssecevents.Throttle
	refusals   jssecevents.Throttle
	nodes      jssecevents.Throttle
	certs      jssecevents.Throttle
	identity   bson.M // stats.peerIdentity
	identified bool   // peer.identified reported, or an identity was stored
	endpoint   *ua.EndpointDescription
	peerAddr   string // ip:port of the session's secure channel
	peerIP     netip.Addr
	methods    map[string]methodInfo // classification of the methods called
}

func newConnSec() *connSec {
	return &connSec{
		flood:     jssecevents.RateLimit{Max: commandFloodMax, Window: commandFloodWindow},
		libRepeat: jssecevents.Throttle{Every: libEventEvery},
		refusals:  jssecevents.Throttle{Every: refusalEvery},
		nodes:     jssecevents.Throttle{Every: unknownNodeEvery},
		certs:     jssecevents.Throttle{Every: certRejectedEvery},
		methods:   map[string]methodInfo{},
	}
}

func (c *OPCUAConnection) secOrigin() jssecevents.Origin {
	return jssecevents.Origin{ProtocolDriver: ProtocolDriverName,
		ProtocolDriverInstanceNumber: c.ProtocolDriverInstanceNumber,
		ProtocolConnectionNumber:     c.ProtocolConnectionNumber,
		ConnectionName:               c.Name, NodeName: secNodeName}
}

// secStore completes and stores an event about this connection's server.
func (c *OPCUAConnection) secStore(e jssecevents.Event) {
	e.Protocol, e.Role = "opcua", "client"
	if e.Source == "" {
		e.Source = secSourceDriver
	}
	if k, ok := driverKinds[e.Kind]; ok && e.Source == secSourceDriver {
		if e.Category == "" {
			e.Category = k.category
		}
		if e.Severity == "" {
			e.Severity = k.severity
		}
	}
	if e.PeerAddress == "-" {
		e.PeerAddress = "" // the event's peer is unknown, and not the session's
	} else if e.PeerAddress == "" {
		c.sec.mu.Lock()
		e.PeerAddress = c.sec.peerAddr
		if e.PeerIP == "" && c.sec.peerIP.IsValid() {
			e.PeerIP = c.sec.peerIP.String()
		}
		c.sec.mu.Unlock()
	}
	if !c.PeerIdentification {
		// No MAC address with peerIdentification off: the writer looks one
		// up for an event that has a peer IP and no MAC. peerAddress keeps
		// the IP.
		e.PeerIP, e.PeerMAC = "", ""
	}
	if secCapture != nil {
		secCapture(e)
	}
	if secWriter == nil {
		jslog.Log(jslog.LevelDetailed, "%s - security event %s %s", c.Name, e.Kind, e.Detail)
		return
	}
	secWriter.Emit(c.secOrigin(), e)
}

// --- library events -----------------------------------------------------

// secLibraryEvent is the opcua library's security event handler. It runs on
// the protocol goroutines: it only filters and queues.
func (c *OPCUAConnection) secLibraryEvent(e secevent.Event) {
	if e.Severity == secevent.Info {
		return // routine: connections opening, accepted handshakes
	}
	if e.Kind == secevent.MACChanged {
		if !c.PeerIdentification {
			return
		}
		// The library remembers the MAC address only within one client,
		// and the driver builds a new client on every rebuild: its own
		// history (seeded from the stored identity) decides, so a change is
		// reported once whichever of the two sees it first.
		ip := netip.Addr{}
		if e.Remote != nil {
			ip = jssecevents.SplitAddr(e.Remote.String())
		}
		if _, changed := c.sec.changes.Observe(macKey(ip), e.RemoteMAC.String()); !changed {
			return
		}
	}
	key := fmt.Sprintf("%s|%d|%s", e.Kind, uint32(e.Status), e.Detail)
	ok, n := c.sec.libRepeat.Allow(key, time.Now())
	if !ok {
		return
	}
	d := libSecDoc(e)
	if n > 1 {
		d.Count = n
	}
	c.secStore(d)
}

// libCategory groups the library's kinds.
func libCategory(k secevent.Kind) string {
	switch k {
	case secevent.CertificateRejected, secevent.SecurityCheckFailed:
		return "security"
	case secevent.MACChanged:
		return "peer"
	}
	switch {
	case strings.HasPrefix(string(k), "connection."):
		return "connection"
	case strings.HasPrefix(string(k), "message."):
		return "protocol"
	case strings.HasPrefix(string(k), "service."):
		return "access"
	}
	return "other"
}

// libSecDoc maps a library event onto a secEvents document.
func libSecDoc(e secevent.Event) jssecevents.Event {
	d := jssecevents.Event{TimeTag: e.Time, Kind: string(e.Kind), Category: libCategory(e.Kind),
		Severity: e.Severity.String(), Source: secSourceLibrary, Detail: e.Detail, FC: e.Service}
	if e.Err != nil {
		d.Error = e.Err.Error()
	}
	if e.Remote != nil {
		d.PeerAddress = e.Remote.String()
		if ip := jssecevents.SplitAddr(e.Remote.String()); ip.IsValid() {
			d.PeerIP = ip.String()
		}
	}
	if e.Local != nil {
		d.LocalAddress = e.Local.String()
	}
	if len(e.RemoteMAC) > 0 {
		d.PeerMAC = e.RemoteMAC.String()
	}
	if len(e.PreviousMAC) > 0 {
		d.Previous, d.Value = e.PreviousMAC.String(), d.PeerMAC
	}
	switch {
	case e.NodeID != nil:
		d.ObjectRef = e.NodeID.String()
	case e.ObjectID != nil:
		d.ObjectRef = e.ObjectID.String()
	}
	switch e.Kind {
	case secevent.CertificateRejected, secevent.SecurityCheckFailed:
		d.Action = "rejected"
	}
	f := bson.M{}
	if e.ConnID != 0 {
		f["connId"] = int64(e.ConnID)
	}
	if e.SecureChannelID != 0 {
		f["secureChannelId"] = int64(e.SecureChannelID)
	}
	if e.RequestID != 0 {
		f["requestId"] = int64(e.RequestID)
	}
	if e.ObjectID != nil && e.NodeID != nil {
		f["objectId"] = e.ObjectID.String()
	}
	if e.AttributeID != 0 {
		f["attributeId"] = int64(e.AttributeID)
	}
	if e.Status != 0 {
		f["status"] = statusCodeName(e.Status)
	}
	if len(f) > 0 {
		d.Fields = f
	}
	return d
}

// --- the session's peer -------------------------------------------------

// secSetEndpoint records the endpoint a session is built on: the server's
// application description and certificate come from it.
func (c *OPCUAConnection) secSetEndpoint(ep *ua.EndpointDescription) {
	c.sec.mu.Lock()
	c.sec.endpoint = ep
	c.sec.mu.Unlock()
}

// secSetPeer records the address of the session's secure channel.
func (c *OPCUAConnection) secSetPeer(addr net.Addr) {
	if addr == nil {
		return
	}
	c.sec.mu.Lock()
	c.sec.peerAddr = addr.String()
	c.sec.peerIP = jssecevents.SplitAddr(addr.String())
	c.sec.mu.Unlock()
}

// secClearPeer forgets the address of the previous session, so an event of
// a new connection attempt is never given it.
func (c *OPCUAConnection) secClearPeer() {
	c.sec.mu.Lock()
	c.sec.peerAddr, c.sec.peerIP = "", netip.Addr{}
	c.sec.mu.Unlock()
}

// identifySignal is one client's request to identify its server: every
// (re)connection advances gen, and ch wakes identifyLoop. It belongs to one
// opcua.Client, so a signal of a replaced client never reaches its
// successor.
type identifySignal struct {
	ch  chan struct{}
	gen atomic.Uint64
}

func newIdentifySignal() *identifySignal { return &identifySignal{ch: make(chan struct{}, 1)} }

// notify never blocks (it runs in StateChangedFunc). Connections that come
// faster than they are identified collapse into one wake-up, and an
// identification overtaken by one is discarded (see identifyLoop).
func (s *identifySignal) notify() {
	s.gen.Add(1)
	select {
	case s.ch <- struct{}{}:
	default:
	}
}

// identifyLoop identifies the server of a client every time it
// (re)connects, until ctx (the session's) ends. An identification is
// published only while its connection is still the current one.
func identifyLoop(ctx context.Context, conn *OPCUAConnection, cli *opcua.Client, sig *identifySignal) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-sig.ch:
		}
		if cli.State() != opcua.Connected {
			continue
		}
		gen := sig.gen.Load()
		identifyServer(ctx, conn, cli, func() bool {
			return ctx.Err() == nil && sig.gen.Load() == gen && cli.State() == opcua.Connected
		})
	}
}

// --- identification -----------------------------------------------------

// buildInfoNodes are the ServerStatus variables read on every session:
// the standard identification of an OPC UA server (OPC 10000-5 §6.3.2).
var buildInfoNodes = []struct {
	id    uint32
	field string
	label string
}{
	{id.Server_ServerStatus_BuildInfo_ManufacturerName, "vendor", "manufacturer name"},
	{id.Server_ServerStatus_BuildInfo_ProductName, "model", "product name"},
	{id.Server_ServerStatus_BuildInfo_ProductURI, "productUri", "product URI"},
	{id.Server_ServerStatus_BuildInfo_SoftwareVersion, "softwareVersion", "software version"},
	{id.Server_ServerStatus_BuildInfo_BuildNumber, "buildNumber", "build number"},
	{id.Server_ServerStatus_BuildInfo_BuildDate, "buildDate", "build date"},
	{id.Server_ServerStatus_StartTime, "startTime", ""},
	{id.Server_ServerStatus_State, "state", ""},
}

// identKey is a compared part of the identity: k is the stored key, field
// (what objectRef shows) its name, label the text of its change.
type identKey struct{ k, field, v, label string }

// The keys are per server: the identity fields, the certificate and the
// start time by configured endpoint URL (a connection may fail over between
// several servers), the MAC address by IP.
func endpointKey(field, endpoint string) string { return field + "|" + endpoint }
func macKey(ip netip.Addr) string {
	if !ip.IsValid() {
		return "mac|"
	}
	return "mac|" + ip.String()
}

// identifyServer reads the server's identity, publishes it in the
// statistics and reports what changed since the previous session, across
// driver restarts too (the history is seeded from the stored identity).
//
// current, when not nil, says whether the connection identified is still the
// client's current one; the result is discarded when it is not.
func identifyServer(ctx context.Context, conn *OPCUAConnection, cli *opcua.Client, current func() bool) {
	if sc := cli.SecureChannel(); sc != nil {
		conn.secSetPeer(sc.RemoteAddr())
	}
	if !conn.PeerIdentification {
		return
	}
	now := time.Now()
	id := bson.M{"collectedAt": now}
	var keys []identKey

	conn.sec.mu.Lock()
	ep, addr, ip := conn.sec.endpoint, conn.sec.peerAddr, conn.sec.peerIP
	conn.sec.mu.Unlock()
	server := ""
	if ep != nil {
		server = ep.EndpointURL
	}
	add := func(field, v, label string) {
		if v = strings.TrimSpace(v); v != "" {
			k := endpointKey(field, server)
			if field == "mac" {
				k = macKey(ip)
			}
			keys = append(keys, identKey{k, field, v, label})
		}
	}
	if addr != "" {
		id["address"] = addr
	}
	if ip.IsValid() {
		id["ip"] = ip.String()
	}
	if mac, err := cli.RemoteMAC(); err == nil && len(mac) > 0 {
		id["mac"], id["macSource"] = mac.String(), "neighbor"
		add("mac", mac.String(), "MAC address")
	}
	if ep != nil {
		id["endpointUrl"] = ep.EndpointURL
		id["securityPolicy"] = policyShortName(ep.SecurityPolicyURI)
		id["securityMode"] = ep.SecurityMode.String()
		if app := ep.Server; app != nil {
			a := bson.M{"applicationUri": app.ApplicationURI, "productUri": app.ProductURI,
				"applicationType": app.ApplicationType.String()}
			if app.ApplicationName != nil {
				a["applicationName"] = app.ApplicationName.Text
			}
			id["application"] = a
			add("applicationUri", app.ApplicationURI, "application URI")
		}
		if len(ep.ServerCertificate) > 0 {
			if cert, err := x509.ParseCertificate(ep.ServerCertificate); err == nil {
				id["certificate"] = jssecevents.CertDoc(cert)
			} else {
				id["certificate"] = bson.M{"sha256": jssecevents.Fingerprint(ep.ServerCertificate)}
			}
			add("cert", jssecevents.Fingerprint(ep.ServerCertificate), "certificate")
		}
	}

	var start time.Time
	state := ua.ServerStateRunning
	stateKnown := false
	req := &ua.ReadRequest{TimestampsToReturn: ua.TimestampsToReturnNeither}
	for _, n := range buildInfoNodes {
		req.NodesToRead = append(req.NodesToRead, &ua.ReadValueID{NodeID: ua.NewNumericNodeID(0, n.id),
			AttributeID: ua.AttributeIDValue})
	}
	rctx, cancel := context.WithTimeout(ctx, time.Duration(conn.TimeoutMs)*time.Millisecond)
	resp, err := cli.Read(rctx, req)
	cancel()
	switch {
	case err != nil:
		id["method"], id["error"] = "error", err.Error()
	case len(resp.Results) != len(buildInfoNodes):
		id["method"], id["error"] = "error", "the server answered a different number of values"
	default:
		id["method"] = "buildInfo"
		for i, n := range buildInfoNodes {
			dv := resp.Results[i]
			if dv == nil || !statusIsGood(dv.Status) || dv.Value == nil {
				continue
			}
			switch v := dv.Value.Value().(type) {
			case string:
				id[n.field] = v
				add(n.field, v, n.label)
			case time.Time:
				if v.IsZero() {
					continue
				}
				id[n.field] = v
				if n.field == "startTime" {
					start = v
				} else {
					add(n.field, v.UTC().Format(time.RFC3339), n.label)
				}
			case int32:
				if n.field == "state" {
					state, stateKnown = ua.ServerState(v), true
					id["state"] = state.String()
				}
			case uint32:
				if n.field == "state" {
					state, stateKnown = ua.ServerState(v), true
					id["state"] = state.String()
				}
			}
		}
	}
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "%s - Server identity not read: %v", conn.Name, err)
	}
	if current != nil && !current() {
		return // the connection read was replaced or closed meanwhile
	}

	conn.sec.mu.Lock()
	previous := conn.sec.identity
	conn.sec.mu.Unlock()

	for _, k := range keys {
		prev, changed := conn.sec.changes.Observe(k.k, k.v)
		if !changed {
			continue
		}
		kind := kindPeerIdentityChanged
		switch k.field {
		case "mac":
			kind = kindPeerMACChanged
		case "cert":
			kind = kindTLSCertChanged
		}
		conn.secStore(jssecevents.Event{Kind: kind, ObjectRef: k.field, Value: k.v, Previous: prev,
			Detail: "the server's " + k.label + " changed", Fields: bson.M{"endpointUrl": server}})
	}

	// A server that restarted has a new StartTime, compared with the last
	// one seen of the same server (stored, or of a previous session), at the
	// millisecond as BSON stores it.
	if !start.IsZero() {
		v := start.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano)
		sk := endpointKey("startTime", server)
		keys = append(keys, identKey{sk, "startTime", v, ""})
		if prev, changed := conn.sec.changes.Observe(sk, v); changed {
			conn.secStore(jssecevents.Event{Kind: kindRemoteRestart, ObjectRef: "Server.ServerStatus.StartTime",
				Value: v, Previous: prev, Detail: "the server restarted",
				Fields: bson.M{"endpointUrl": server}})
		}
	}
	if stateKnown && state != ua.ServerStateRunning {
		conn.secStore(jssecevents.Event{Kind: kindRemoteState, ObjectRef: "Server.ServerStatus.State",
			Value: state.String(), Detail: "the server is not running normally"})
	}

	id["keys"] = mergeKeys(previous, keys)
	conn.sec.mu.Lock()
	first := !conn.sec.identified
	conn.sec.identified = true
	conn.sec.identity = id
	conn.sec.mu.Unlock()
	if first {
		conn.secStore(jssecevents.Event{Kind: kindPeerIdentified, Value: identitySummary(id),
			Detail: "first identification of the server"})
	}
	jslog.Log(jslog.LevelBasic, "%s - Server identity: %s", conn.Name, identitySummary(id))
}

// mergeKeys is the stored keys: this session's, plus those a previous
// session read and this one did not (BuildInfo not readable this time), so
// their change is still seen later.
func mergeKeys(previous bson.M, keys []identKey) bson.A {
	now := map[string]string{}
	for _, k := range keys {
		now[k.k] = k.v
	}
	for _, k := range jsmongo.GetDocArray(previous, "keys") {
		if kk := jsmongo.GetString(k, "k", ""); kk != "" {
			if _, ok := now[kk]; !ok {
				now[kk] = jsmongo.GetString(k, "v", "")
			}
		}
	}
	names := make([]string, 0, len(now))
	for n := range now {
		names = append(names, n)
	}
	sort.Strings(names)
	out := bson.A{}
	for _, n := range names {
		out = append(out, bson.M{"k": n, "v": now[n]})
	}
	return out
}

func identitySummary(id bson.M) string {
	var parts []string
	for _, f := range []string{"vendor", "model", "softwareVersion", "address", "mac"} {
		if v, ok := id[f].(string); ok && v != "" {
			parts = append(parts, f+"="+v)
		}
	}
	if a, ok := id["application"].(bson.M); ok {
		if v, _ := a["applicationUri"].(string); v != "" {
			parts = append(parts, "applicationUri="+v)
		}
	}
	return strings.Join(parts, " ")
}

// seedIdentity starts the change detection from the identity stored in the
// connection's statistics, so a change across a driver restart is seen.
func (c *OPCUAConnection) seedIdentity(stored bson.M) {
	if stored == nil || !c.PeerIdentification {
		return
	}
	c.sec.mu.Lock()
	c.sec.identity, c.sec.identified = stored, true
	c.sec.mu.Unlock()
	for _, k := range jsmongo.GetDocArray(stored, "keys") {
		c.sec.changes.Seed(jsmongo.GetString(k, "k", ""), jsmongo.GetString(k, "v", ""))
	}
}

// PeerIdentityDoc is stats.peerIdentity, nil when never identified or with
// peerIdentification off.
func (c *OPCUAConnection) PeerIdentityDoc() bson.M {
	if !c.PeerIdentification {
		return nil
	}
	c.sec.mu.Lock()
	defer c.sec.mu.Unlock()
	return c.sec.identity
}

// --- certificates ---------------------------------------------------------

// secCertRejected reports a server certificate the driver refused
// (autoAcceptUntrustedCertificates false and no trusted chain), at most once
// per certRejectedEvery for the same certificate.
func (c *OPCUAConnection) secCertRejected(endpointURL string, der []byte, err error) {
	fp := ""
	var doc bson.M
	if len(der) > 0 {
		fp = jssecevents.Fingerprint(der)
		if cert, perr := x509.ParseCertificate(der); perr == nil {
			doc = jssecevents.CertDoc(cert)
		} else {
			doc = bson.M{"sha256": fp}
		}
	}
	ok, n := c.sec.certs.Allow(fp, time.Now())
	if !ok {
		return
	}
	e := jssecevents.Event{Kind: kindTLSCertRejected, Action: "rejected", ObjectRef: endpointURL,
		Value: fp, Detail: "server certificate not trusted", Fields: bson.M{"endpointUrl": endpointURL}}
	// The server of this attempt, never the previous session's.
	e.PeerAddress = "-"
	if u, perr := url.Parse(endpointURL); perr == nil && u.Host != "" {
		e.PeerAddress = u.Host
		if u.Port() == "" {
			e.PeerAddress = net.JoinHostPort(u.Hostname(), "4840")
		}
		if ip := jssecevents.SplitAddr(e.PeerAddress); ip.IsValid() {
			e.PeerIP = ip.String()
		}
	}
	if doc != nil {
		e.Peer = bson.M{"certificate": doc}
	}
	if err != nil {
		e.Error = err.Error()
	}
	if n > 1 {
		e.Count = n
	}
	c.secStore(e)
}

// --- subscriptions --------------------------------------------------------

// unknownNodeStatus are the monitored item results saying the server has no
// such node or attribute.
var unknownNodeStatus = map[ua.StatusCode]bool{
	ua.StatusBadNodeIDUnknown:      true,
	ua.StatusBadNodeIDInvalid:      true,
	ua.StatusBadAttributeIDInvalid: true,
	ua.StatusBadNotFound:           true,
}

type unknownNode struct {
	nodeID string
	status ua.StatusCode
}

// secUnknownNodes reports the configured nodes the server refused to monitor
// because it does not have them: each one at most once per
// unknownNodeEvery, the first maxUnknownNodeEvents of a setup on their own,
// the rest in one summary.
func (c *OPCUAConnection) secUnknownNodes(nodes []unknownNode) {
	now := time.Now()
	var rest []string
	sent := 0
	for _, n := range nodes {
		if ok, _ := c.sec.nodes.Allow(n.nodeID, now); !ok {
			continue
		}
		if sent >= maxUnknownNodeEvents {
			rest = append(rest, n.nodeID)
			continue
		}
		sent++
		c.secStore(jssecevents.Event{Kind: kindAccessUnknownNode, ObjectRef: n.nodeID, FC: "CreateMonitoredItems",
			Value: statusCodeName(n.status), Detail: "a configured node the server does not have",
			Fields: bson.M{"status": statusCodeName(n.status)}})
	}
	if len(rest) > 0 {
		shown := rest
		if len(shown) > 50 {
			shown = shown[:50]
		}
		c.secStore(jssecevents.Event{Kind: kindAccessUnknownNode, FC: "CreateMonitoredItems", Count: len(rest),
			Value: fmt.Sprint(len(rest)), Detail: "more configured nodes the server does not have",
			Fields: bson.M{"nodes": shown}})
	}
}

// --- commands ---------------------------------------------------------------

// cmdEvent describes a command. valueString is left out: it carries a
// method's input arguments (file contents, certificates and keys for some)
// and string setpoints, which an audit collection must not keep.
func cmdEvent(kind string, doc bson.M, detail string) jssecevents.Event {
	f := bson.M{}
	for _, k := range []string{"protocolSourceASDU", "value"} {
		if v, ok := doc[k]; ok {
			f[k] = v
		}
	}
	return jssecevents.Event{Kind: kind, Tag: jsmongo.GetString(doc, "tag", ""),
		ObjectRef: jsmongo.GetString(doc, "protocolSourceObjectAddress", ""), Detail: detail, Fields: f}
}

// secCommandRefused reports a command not executed (kind cmd.rejected,
// cmd.object_unknown, cmd.wrong_type or cmd.stale_time): refused by the
// driver, or by the server (status, its status name). The same refusal of
// the same address is stored once per refusalEvery, with the count.
func (c *OPCUAConnection) secCommandRefused(kind string, doc bson.M, why, status string) {
	addr := jsmongo.GetString(doc, "protocolSourceObjectAddress", "")
	ok, n := c.sec.refusals.Allow(kind+"|"+addr+"|"+why+"|"+status, time.Now())
	if !ok {
		return
	}
	e := cmdEvent(kind, doc, why)
	e.Action = "rejected"
	if status != "" {
		e.Value = status
		e.Fields["status"] = status
	} else {
		e.Value = fmt.Sprint(doc["value"])
	}
	if n > 1 {
		e.Count = n
	}
	c.secStore(e)
}

// commandStatusKind classifies a server's refusal of a write or a call.
func commandStatusKind(st ua.StatusCode) string {
	switch st {
	case ua.StatusBadNodeIDUnknown, ua.StatusBadNodeIDInvalid, ua.StatusBadAttributeIDInvalid,
		ua.StatusBadMethodInvalid, ua.StatusBadNotFound:
		return kindCmdObjectUnknown
	case ua.StatusBadTypeMismatch, ua.StatusBadArgumentsMissing, ua.StatusBadTooManyArguments,
		ua.StatusBadInvalidArgument, ua.StatusBadOutOfRange, ua.StatusBadIndexRangeInvalid:
		return kindCmdWrongType
	}
	return kindCmdRejected
}

// secCommandAnswer reports the server refusing a command: st is its status.
func (c *OPCUAConnection) secCommandAnswer(doc bson.M, service string, st ua.StatusCode) {
	if statusIsGood(st) {
		return
	}
	kind := commandStatusKind(st)
	detail := "the server refused the command"
	switch kind {
	case kindCmdObjectUnknown:
		detail = "the server has no such node or method"
	case kindCmdWrongType:
		detail = "the server refused the command's type or arguments"
	}
	c.secCommandRefused(kind, doc, detail+" ("+service+")", statusCodeName(st))
}

// secCommand counts a command sent for the flood detector and checks, in
// the background, a Boolean one against the state its supervised point was
// in before it was sent (a later state may be its own result).
func (c *OPCUAConnection) secCommand(rt *mongo.Collection, doc bson.M, asdu string, value float64) {
	if over, n := c.sec.flood.Hit("cmd", time.Now()); over {
		e := cmdEvent(kindFloodCommands, doc,
			fmt.Sprintf("more than %d commands to the server within %s", commandFloodMax, commandFloodWindow))
		e.Value = fmt.Sprint(n)
		c.secStore(e)
	}
	if rt == nil || !strings.EqualFold(asdu, "boolean") || (value != 0 && value != 1) {
		return
	}
	key, before := jsmongo.GetDouble(doc, "pointKey", 0), time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		state, ok, err := jssecevents.SupervisedState(ctx, rt, key, before)
		if err != nil || !ok || (state != 0) != (value != 0) {
			return
		}
		e := cmdEvent(kindCmdUnnecessary, doc, "the supervised point already was in the commanded state")
		e.Value = fmt.Sprint(value)
		c.secStore(e)
	}()
}

// --- server answers ---------------------------------------------------------

// transportStatus are the statuses of a request that did not get an answer
// from the server's application: the connection, channel or session failed.
// They are not a refusal of the command.
var transportStatus = map[ua.StatusCode]bool{
	ua.StatusBadTimeout: true, ua.StatusBadRequestTimeout: true, ua.StatusBadCommunicationError: true,
	ua.StatusBadConnectionClosed: true, ua.StatusBadServerNotConnected: true, ua.StatusBadShutdown: true,
	ua.StatusBadSecureChannelClosed: true, ua.StatusBadSecureChannelIDInvalid: true,
	ua.StatusBadSessionClosed: true, ua.StatusBadSessionIDInvalid: true, ua.StatusBadSessionNotActivated: true,
	ua.StatusBadTooManyOperations: true, ua.StatusBadRequestTooLarge: true, ua.StatusBadResponseTooLarge: true,
	ua.StatusBadDecodingError: true, ua.StatusBadEncodingError: true, ua.StatusBadTCPMessageTooLarge: true,
}

// serviceStatus is the OPC UA status a request failed with, when the server
// answered it (a bad ServiceResult reaches the driver as the error).
func serviceStatus(err error) (ua.StatusCode, bool) {
	var st ua.StatusCode
	if errors.As(err, &st) && st != ua.StatusOK && !transportStatus[st] {
		return st, true
	}
	return 0, false
}

// secCommandFailed reports a write, call or browse of a command that failed
// with err: the server refusing it (a bad service result) is reported as
// its status says; a failure of the connection is not a refusal.
func (c *OPCUAConnection) secCommandFailed(doc bson.M, service string, err error) {
	if st, ok := serviceStatus(err); ok {
		c.secCommandAnswer(doc, service, st)
	}
}

// --- method calls -------------------------------------------------------------

// methodInfo is what a method call is reported as: kind ("" for none) and the
// names it is known by.
type methodInfo struct {
	kind, name, objectType string
}

// Standard object types (namespace 0) whose methods are reported, and as
// what. TrustListType is a FileType: it comes first.
var methodTypes = []struct {
	typeID uint32
	prefix string // of the standard method names of the type and its instances
	kind   string
}{
	{id.TrustListType, "TrustList", kindCtrlConfigChange},
	{id.ServerConfigurationType, "ServerConfiguration", kindCtrlConfigChange},
	{id.CertificateGroupType, "CertificateGroup", kindCtrlConfigChange},
	{id.FileDirectoryType, "FileDirectoryType", kindFileTransfer},
	{id.FileType, "FileType", kindFileTransfer},
	{id.ProgramStateMachineType, "ProgramStateMachineType", kindCtrlRestart},
}

// standardMethodKind classifies a method of namespace 0 by its standard name
// (e.g. FileType_Open, ServerConfiguration_UpdateCertificate,
// Server_RequestServerStateChange); "" when it is none of them.
func standardMethodKind(name string) string {
	switch {
	case name == "Server_RequestServerStateChange":
		return kindCtrlRestart
	case strings.Contains(name, "TrustList"), strings.HasPrefix(name, "ServerConfiguration"),
		strings.Contains(name, "CertificateGroup"):
		return kindCtrlConfigChange
	case strings.HasPrefix(name, "FileType_"), strings.HasPrefix(name, "FileDirectoryType_"):
		return kindFileTransfer
	case strings.HasPrefix(name, "ProgramStateMachineType_"):
		return kindCtrlRestart
	}
	return ""
}

// classifyMethod finds what calling methodID on objectID is: a standard
// method of namespace 0 by its id, otherwise a method of an object whose
// type is one of methodTypes (the type itself, not a subtype of it); any
// other method is an ordinary call (kindMethodCall).
func classifyMethod(ctx context.Context, cli *opcua.Client, methodID, objectID *ua.NodeID) methodInfo {
	info := methodInfo{kind: kindMethodCall}
	if methodID.Namespace() == 0 && methodID.Type() == ua.NodeIDTypeNumeric {
		if name := id.Name(methodID.IntID()); name != "" {
			info.name = name
			if k := standardMethodKind(name); k != "" {
				info.kind = k
			}
			return info
		}
	}
	if qn, err := cli.Node(methodID).BrowseName(ctx); err == nil && qn != nil {
		info.name = qn.Name
	}
	resp, err := cli.Browse(ctx, &ua.BrowseRequest{NodesToBrowse: []*ua.BrowseDescription{{
		NodeID: objectID, BrowseDirection: ua.BrowseDirectionForward,
		ReferenceTypeID: ua.NewNumericNodeID(0, id.HasTypeDefinition), IncludeSubtypes: false,
		ResultMask: uint32(ua.BrowseResultMaskAll)}}})
	if err != nil || len(resp.Results) == 0 || len(resp.Results[0].References) == 0 {
		return info
	}
	t := resp.Results[0].References[0].NodeID
	if t == nil || t.NodeID == nil {
		return info
	}
	info.objectType = t.NodeID.String()
	if t.NodeID.Namespace() == 0 && t.NodeID.Type() == ua.NodeIDTypeNumeric {
		for _, mt := range methodTypes {
			if t.NodeID.IntID() == mt.typeID {
				info.kind = mt.kind
				info.objectType = mt.prefix
			}
		}
	}
	return info
}

// methodCacheMax bounds the classifications kept per connection.
const methodCacheMax = 1024

// secMethodCall reports a method call (ctrl.restart, ctrl.config_change,
// file.transfer or ctrl.method_call) after it was dispatched: classifying
// it takes requests to the server, which must never delay the command.
// outcome is the call's status ("Good" or the error); the arguments are
// counted, never stored.
func (c *OPCUAConnection) secMethodCall(cli *opcua.Client, doc bson.M, methodID, objectID *ua.NodeID, nArgs int, outcome string) {
	key := methodID.String() + "|" + objectID.String()
	c.sec.mu.Lock()
	info, known := c.sec.methods[key]
	c.sec.mu.Unlock()
	if !known {
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		info = classifyMethod(ctx, cli, methodID, objectID)
		cancel()
		c.sec.mu.Lock()
		if len(c.sec.methods) >= methodCacheMax {
			c.sec.methods = map[string]methodInfo{}
		}
		c.sec.methods[key] = info
		c.sec.mu.Unlock()
	}
	detail := map[string]string{
		kindFileTransfer:     "file transfer method called",
		kindCtrlConfigChange: "configuration or security change method called",
		kindCtrlRestart:      "server or program state control method called",
		kindMethodCall:       "method called",
	}[info.kind]
	if info.name != "" {
		detail += ": " + info.name
	}
	e := cmdEvent(info.kind, doc, detail)
	e.FC, e.Value = "Call", outcome
	e.Fields["methodId"], e.Fields["objectId"], e.Fields["arguments"] = methodID.String(), objectID.String(), nArgs
	if info.name != "" {
		e.Fields["method"] = info.name
	}
	if info.objectType != "" {
		e.Fields["objectType"] = info.objectType
	}
	c.secStore(e)
}
