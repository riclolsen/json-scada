package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/secevent"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/ua"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// captured collects the events stored while a test runs.
type captured struct {
	mu     sync.Mutex
	events []jssecevents.Event
}

func captureSecEvents(t *testing.T) *captured {
	t.Helper()
	c := &captured{}
	secCapture = func(e jssecevents.Event) {
		c.mu.Lock()
		c.events = append(c.events, e)
		c.mu.Unlock()
	}
	t.Cleanup(func() { secCapture = nil })
	return c
}

func (c *captured) all() []jssecevents.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]jssecevents.Event(nil), c.events...)
}

func (c *captured) kinds() map[string]jssecevents.Event {
	m := map[string]jssecevents.Event{}
	for _, e := range c.all() {
		m[e.Kind] = e
	}
	return m
}

// TestLibraryEventsFiltered: routine events are left out, repeats are
// folded, and the fields reach the secEvents document.
func TestLibraryEventsFiltered(t *testing.T) {
	got := captureSecEvents(t)
	conn := testConn()
	remote := &net.TCPAddr{IP: net.ParseIP("10.1.2.3"), Port: 4840}
	conn.secLibraryEvent(secevent.Event{Kind: secevent.ConnectionAccepted, Severity: secevent.Info, Remote: remote})
	seqErr := secevent.Event{Kind: secevent.SequenceError, Severity: secevent.Alert, Remote: remote,
		SecureChannelID: 7, RequestID: 9, Status: ua.StatusBadSequenceNumberInvalid, Detail: "sequence number 5, want 4",
		Err: errors.New("replayed")}
	for range 3 {
		conn.secLibraryEvent(seqErr)
	}
	evs := got.all()
	if len(evs) != 1 {
		t.Fatalf("stored %d events, want 1 (info left out, repeats folded): %v", len(evs), evs)
	}
	e := evs[0]
	if e.Kind != "message.sequence_error" || e.Source != secSourceLibrary || e.Severity != "alert" ||
		e.Category != "protocol" || e.PeerIP != "10.1.2.3" || e.PeerAddress != "10.1.2.3:4840" ||
		e.Protocol != "opcua" || e.Role != "client" || e.Error != "replayed" {
		t.Errorf("library event = %+v", e)
	}
	if e.Fields["secureChannelId"] != int64(7) || e.Fields["status"] != "BadSequenceNumberInvalid" {
		t.Errorf("fields = %v", e.Fields)
	}
}

// TestMACChangeReportedOnce: the library's change and the driver's history
// agree, so a change is stored once, and not at all with identification off.
func TestMACChangeReportedOnce(t *testing.T) {
	got := captureSecEvents(t)
	conn := testConn()
	conn.seedIdentity(bson.M{"keys": bson.A{bson.M{"k": "mac|10.1.2.3", "v": "00:11:22:33:44:55"},
		bson.M{"k": "mac|10.1.2.4", "v": "00:11:22:33:44:77"}}})
	mac, _ := net.ParseMAC("00:11:22:33:44:66")
	prev, _ := net.ParseMAC("00:11:22:33:44:55")
	remote := &net.TCPAddr{IP: net.ParseIP("10.1.2.3"), Port: 4840}
	ev := secevent.Event{Kind: secevent.MACChanged, Severity: secevent.Alert, RemoteMAC: mac, PreviousMAC: prev,
		Remote: remote}
	conn.secLibraryEvent(ev)
	conn.sec.libRepeat.Reset()
	conn.secLibraryEvent(ev) // the same change again: not news
	if n := len(got.all()); n != 1 {
		t.Fatalf("stored %d MAC changes, want 1: %v", n, got.all())
	}
	if e := got.all()[0]; e.Value != "00:11:22:33:44:66" || e.Previous != "00:11:22:33:44:55" {
		t.Errorf("mac change = %+v", e)
	}
	// Another server of the connection (failover) has its own MAC history.
	other, _ := net.ParseMAC("00:11:22:33:44:77")
	conn.sec.libRepeat.Reset()
	conn.secLibraryEvent(secevent.Event{Kind: secevent.MACChanged, Severity: secevent.Alert, RemoteMAC: other,
		Remote: &net.TCPAddr{IP: net.ParseIP("10.1.2.4"), Port: 4840}})
	if n := len(got.all()); n != 1 {
		t.Errorf("the MAC of another server reported as a change: %v", got.all())
	}

	off := testConn()
	off.PeerIdentification = false
	off.secLibraryEvent(ev)
	if n := len(got.all()); n != 1 {
		t.Errorf("a MAC change stored with peerIdentification off")
	}
}

func startIdentityServer(t *testing.T, product string) (*opcua.Client, string) {
	t.Helper()
	port := freePort(t)
	srv := server.New(
		server.EndPoint("127.0.0.1", port),
		server.EnableSecurity("None", ua.MessageSecurityModeNone),
		server.EnableAuthMode(ua.UserTokenTypeAnonymous),
		server.ManufacturerName("ACME"),
		server.ProductName(product),
	)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("cannot start the test server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	endpoint := fmt.Sprintf("opc.tcp://127.0.0.1:%d", port)
	cli, err := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err != nil {
		t.Fatalf("cannot build a client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := cli.Connect(ctx); err != nil {
		t.Fatalf("cannot connect to the test server: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close(context.Background()) })
	return cli, endpoint
}

// TestIdentifyServer reads the server's BuildInfo, and compares it with the
// identity stored before: a new product name and a new StartTime (a
// restart) are reported.
func TestIdentifyServer(t *testing.T) {
	got := captureSecEvents(t)
	cli, endpoint := startIdentityServer(t, "RTU-OPC 2")
	conn := testConn()
	conn.TimeoutMs = 5000
	conn.secSetEndpoint(&ua.EndpointDescription{EndpointURL: endpoint, SecurityMode: ua.MessageSecurityModeNone,
		SecurityPolicyURI: ua.SecurityPolicyURINone,
		Server:            &ua.ApplicationDescription{ApplicationURI: "urn:acme:rtu"}})
	conn.seedIdentity(bson.M{"keys": bson.A{
		bson.M{"k": "startTime|" + endpoint, "v": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)},
		bson.M{"k": "model|" + endpoint, "v": "RTU-OPC 1"}, bson.M{"k": "vendor|" + endpoint, "v": "ACME"},
		// Another server of the connection: neither a restart nor a change here.
		bson.M{"k": "startTime|opc.tcp://other:4840", "v": "2020-01-01T00:00:00Z"},
		bson.M{"k": "model|opc.tcp://other:4840", "v": "OTHER"}}})

	identifyServer(context.Background(), conn, cli, nil)
	id := conn.PeerIdentityDoc()
	if id["vendor"] != "ACME" || id["model"] != "RTU-OPC 2" || id["method"] != "buildInfo" ||
		id["ip"] != "127.0.0.1" || id["securityMode"] != "MessageSecurityModeNone" {
		t.Errorf("identity = %v", id)
	}
	if _, ok := id["startTime"].(time.Time); !ok {
		t.Errorf("startTime = %v", id["startTime"])
	}
	for _, e := range got.all() {
		if e.Kind == kindPeerIdentityChanged && e.ObjectRef == "model" && e.Previous == "OTHER" {
			t.Errorf("compared with another server of the connection: %+v", e)
		}
	}
	k := got.kinds()
	if e, ok := k[kindPeerIdentityChanged]; !ok || e.ObjectRef != "model" || e.Value != "RTU-OPC 2" ||
		e.Previous != "RTU-OPC 1" || e.PeerIP != "127.0.0.1" || e.Severity != "alert" {
		t.Errorf("peer.identity_changed = %+v (%v)", e, got.all())
	}
	if n := countKind(got.all(), kindRemoteRestart); n != 1 {
		t.Errorf("%d remote.restart, want 1: %v", n, got.all())
	}
	if _, ok := k[kindPeerIdentified]; ok {
		t.Error("peer.identified with an identity stored before")
	}

	// A second session to the same server: nothing changed.
	n := len(got.all())
	identifyServer(context.Background(), conn, cli, nil)
	if len(got.all()) != n {
		t.Errorf("events on an unchanged identity: %v", got.all()[n:])
	}

	// The server refuses a write to a node it does not have.
	resp, err := cli.Write(context.Background(), &ua.WriteRequest{NodesToWrite: []*ua.WriteValue{{
		NodeID: ua.NewStringNodeID(1, "nope"), AttributeID: ua.AttributeIDValue,
		Value: &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(true)}}}})
	if err != nil || len(resp.Results) != 1 {
		t.Fatalf("write: %v %v", resp, err)
	}
	doc := bson.M{"tag": "BRK1", "protocolSourceObjectAddress": "ns=1;s=nope", "value": 1.0}
	conn.secCommandAnswer(doc, "Write", resp.Results[0])
	if e, ok := got.kinds()[kindCmdObjectUnknown]; !ok || e.Tag != "BRK1" || e.ObjectRef != "ns=1;s=nope" ||
		e.Action != "rejected" {
		t.Errorf("cmd.object_unknown = %+v (%v)", e, got.all())
	}
}

// TestIdentificationOff: no identity, no MAC address in the events.
func TestIdentificationOff(t *testing.T) {
	got := captureSecEvents(t)
	cli, _ := startIdentityServer(t, "X")
	conn := testConn()
	conn.PeerIdentification = false
	identifyServer(context.Background(), conn, cli, nil)
	if conn.PeerIdentityDoc() != nil {
		t.Error("an identity published with peerIdentification off")
	}
	conn.secCommandRefused(kindCmdRejected, bson.M{"protocolSourceObjectAddress": "ns=1;i=5"},
		"commands are disabled on the connection", "")
	if e := got.all()[0]; e.PeerIP != "" || e.PeerAddress != "127.0.0.1:"+portOf(e.PeerAddress) {
		t.Errorf("event with identification off = %+v", e)
	}
}

func portOf(addr string) string {
	_, p, _ := net.SplitHostPort(addr)
	return p
}

// TestCommandEvents: refusals are throttled per address, a flood is
// reported once per window, and method names are classified.
func TestCommandEvents(t *testing.T) {
	got := captureSecEvents(t)
	conn := testConn()
	doc := bson.M{"tag": "SP1", "protocolSourceObjectAddress": "ns=2;s=SP1", "value": 1e9,
		"protocolSourceASDU": "Int16"}
	conn.secCommandRefused(kindCmdWrongType, doc, "command not sent: value out of range", "")
	conn.secCommandRefused(kindCmdWrongType, doc, "command not sent: value out of range", "")
	for range commandFloodMax + 3 {
		conn.secCommand(nil, doc, "Int16", 1e9)
	}
	k := got.kinds()
	if n := len(got.all()); n != 2 {
		t.Errorf("stored %d events, want a refusal and a flood: %v", n, got.all())
	}
	if e := k[kindCmdWrongType]; e.Tag != "SP1" || e.Value != "1e+09" || e.Fields["protocolSourceASDU"] != "Int16" {
		t.Errorf("cmd.wrong_type = %+v", e)
	}
	if e, ok := k[kindFloodCommands]; !ok || e.Severity != "alert" {
		t.Errorf("flood.commands = %+v", e)
	}

	for name, want := range map[string]string{"FileType_Open": kindFileTransfer,
		"FileDirectoryType_CreateFile":                                                  kindFileTransfer,
		"ServerConfiguration_UpdateCertificate":                                         kindCtrlConfigChange,
		"ServerConfiguration_CertificateGroups_DefaultApplicationGroup_TrustList_Write": kindCtrlConfigChange,
		"TrustListType_AddCertificate":                                                  kindCtrlConfigChange, "Server_RequestServerStateChange": kindCtrlRestart,
		"ProgramStateMachineType_Halt": kindCtrlRestart, "Server_GetMonitoredItems": "",
		"AcknowledgeableConditionType_Acknowledge": ""} {
		if got := standardMethodKind(name); got != want {
			t.Errorf("standardMethodKind(%q) = %q, want %q", name, got, want)
		}
	}
	// The arguments of a command never reach the event.
	e := cmdEvent(kindFileTransfer, bson.M{"valueString": `["secret file contents"]`, "value": 0.0}, "")
	if _, ok := e.Fields["valueString"]; ok {
		t.Error("valueString stored in a security event")
	}
	for st, want := range map[ua.StatusCode]string{ua.StatusBadNodeIDUnknown: kindCmdObjectUnknown,
		ua.StatusBadMethodInvalid: kindCmdObjectUnknown, ua.StatusBadTypeMismatch: kindCmdWrongType,
		ua.StatusBadUserAccessDenied: kindCmdRejected, ua.StatusBadNotWritable: kindCmdRejected} {
		if got := commandStatusKind(st); got != want {
			t.Errorf("commandStatusKind(%v) = %q, want %q", st, got, want)
		}
	}
}

func countKind(evs []jssecevents.Event, kind string) int {
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// TestStaleIdentificationDiscarded: a result read for a connection that was
// replaced meanwhile is not published.
func TestStaleIdentificationDiscarded(t *testing.T) {
	got := captureSecEvents(t)
	cli, endpoint := startIdentityServer(t, "X")
	conn := testConn()
	conn.TimeoutMs = 5000
	conn.secSetEndpoint(&ua.EndpointDescription{EndpointURL: endpoint})
	identifyServer(context.Background(), conn, cli, func() bool { return false })
	if conn.PeerIdentityDoc() != nil || len(got.all()) != 0 {
		t.Errorf("a superseded identification published: %v %v", conn.PeerIdentityDoc(), got.all())
	}

	// The signal of one client: connections collapse into one wake-up, and
	// each advances the generation an identification is checked against.
	sig := newIdentifySignal()
	sig.notify()
	sig.notify()
	if len(sig.ch) != 1 || sig.gen.Load() != 2 {
		t.Errorf("signal: %d queued, generation %d", len(sig.ch), sig.gen.Load())
	}
}

// TestCertRejectedNamesTheAttemptedServer: the event of a refused
// certificate names the server being connected to, not the previous
// session's.
func TestCertRejectedNamesTheAttemptedServer(t *testing.T) {
	got := captureSecEvents(t)
	conn := testConn()
	conn.secSetPeer(&net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 4840})
	conn.secCertRejected("opc.tcp://10.0.0.2:4841/ua", nil, errors.New("unknown authority"))
	conn.secCertRejected("opc.tcp://plc2/ua", nil, errors.New("unknown authority"))
	evs := got.all()
	if len(evs) != 1 || evs[0].PeerAddress != "10.0.0.2:4841" || evs[0].PeerIP != "10.0.0.2" {
		t.Fatalf("cert rejected = %+v", evs)
	}
	conn.sec.certs.Reset()
	conn.secCertRejected("opc.tcp://plc2/ua", []byte{1}, errors.New("unknown authority"))
	if e := got.all()[1]; e.PeerAddress != "plc2:4840" || e.PeerIP != "" {
		t.Errorf("cert rejected by host name = %+v", e)
	}
}

// TestServiceStatus: a refusal in the service result is reported, a failure
// of the connection is not.
func TestServiceStatus(t *testing.T) {
	got := captureSecEvents(t)
	conn := testConn()
	doc := bson.M{"protocolSourceObjectAddress": "ns=2;s=V1", "value": 1.0}
	conn.secCommandFailed(doc, "Write", fmt.Errorf("write: %w", ua.StatusBadUserAccessDenied))
	conn.secCommandFailed(doc, "Write", ua.StatusBadTimeout)
	conn.secCommandFailed(doc, "Write", errors.New("connection reset"))
	evs := got.all()
	if len(evs) != 1 || evs[0].Kind != kindCmdRejected || evs[0].Value != "BadUserAccessDenied" {
		t.Errorf("service refusals = %v", evs)
	}
}

// TestUnknownNodesSummarized: past maxUnknownNodeEvents in one setup, the
// rest come in one summary, and a node is not reported again soon.
func TestUnknownNodesSummarized(t *testing.T) {
	got := captureSecEvents(t)
	conn := testConn()
	var nodes []unknownNode
	for i := range maxUnknownNodeEvents + 5 {
		nodes = append(nodes, unknownNode{fmt.Sprintf("ns=2;i=%d", i), ua.StatusBadNodeIDUnknown})
	}
	conn.secUnknownNodes(nodes)
	evs := got.all()
	if len(evs) != maxUnknownNodeEvents+1 || evs[len(evs)-1].Count != 5 {
		t.Fatalf("stored %d events (last %+v), want %d and a summary of 5", len(evs), evs[len(evs)-1],
			maxUnknownNodeEvents+1)
	}
	conn.secUnknownNodes(nodes)
	if len(got.all()) != len(evs) {
		t.Error("unknown nodes reported again within the period")
	}
}
