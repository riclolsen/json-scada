package main

import (
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/riclolsen/tase2/mms"
	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// captureSecEvents makes the driver queue its events in a writer the test
// reads (no MongoDB).
func captureSecEvents(t *testing.T) *jssecevents.Writer {
	t.Helper()
	w := jssecevents.NewWriter(1024)
	secWriter = w
	t.Cleanup(func() { secWriter = nil })
	return w
}

// waitSecEvent returns the first queued event match accepts, failing the
// test after a timeout; the events read before it are returned too.
func waitSecEvent(t *testing.T, w *jssecevents.Writer, what string, match func(jssecevents.Event) bool) (jssecevents.Event, []jssecevents.Event) {
	t.Helper()
	var seen []jssecevents.Event
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-w.C():
			seen = append(seen, e)
			if match(e) {
				return e, seen
			}
		case <-deadline:
			kinds := []string{}
			for _, e := range seen {
				kinds = append(kinds, e.Kind+"/"+e.Source)
			}
			t.Fatalf("timed out waiting for %s; got %v", what, kinds)
			return jssecevents.Event{}, seen
		}
	}
}

func kindIs(kind, source string) func(jssecevents.Event) bool {
	return func(e jssecevents.Event) bool { return e.Kind == kind && (source == "" || e.Source == source) }
}

func TestSecEventsServerLoopback(t *testing.T) {
	w := captureSecEvents(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	isActive.Store(true)
	defer isActive.Store(false)

	brk := rtData{ID: 31, Tag: "BRK-CMD", Type: "digital", Origin: "command", Group1: "SUB_A", CommandBlocked: true}
	meas := rtData{ID: 32, Tag: "MEAS", Type: "analog", Value: 1, Group1: "SUB_A"}
	// The client stated another AP title when it last came from this
	// address (stored statistics of an earlier run).
	stored, _ := bson.Marshal(bson.M{"known": bson.A{bson.M{"ip": "127.0.0.1", "acseIdentity": "1.1.999.7/12",
		"lastSeen": time.Now().Add(-time.Hour)}}})
	conn := protocolConnection{
		Name: "SECLOOP", ProtocolConnectionNumber: 9201, CommandsEnabled: true,
		RemoteApTitle: "1.1.999.2", RemoteAeQualifier: 12, Topics: []string{"SUB_A"},
		IPAddresses: []string{"10.9.9.9"},
		Stats:       storedStats{PeerIdentity: stored},
	}
	tags := []rtData{brk, meas}
	model := buildModel(tags, []protocolConnection{conn}, 0)
	populateRtDataCache(tags, model.tagByKey)

	sec := newSrvSec(conn, nil)
	cfg := tase2.ServerConfig{Port: port, LocalAPTitle: "1.1.999.1", LocalAEQual: 12,
		VendorName: "JSON-SCADA", ModelName: "ICCP-Server-Test", Revision: DriverVersion}
	registry := newServerRegistry()
	srvEP := tase2.NewEndpoint(tase2.EndpointPassive)
	srvEP.SetLocalAPTitle("1.1.999.1", 12)
	if err := srvEP.SetSecurityPolicy(sec.policy(conn.IPAddresses)); err != nil {
		t.Fatalf("policy: %v", err)
	}
	defer srvEP.CloseSecurity()
	if err := srvEP.Listen(port); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srvEP.Disconnect()
	go srvEP.ServeClients(serveClientCallbacks(registry, model.dataModel, cfg, conn, model.datasetDefs, nil, sec))

	cliEP := tase2.NewEndpoint(tase2.EndpointActive)
	cliEP.SetLocalAPTitle("1.1.999.2", 12)
	cliEP.SetRemoteAPTitle("1.1.999.1", 12)
	time.Sleep(50 * time.Millisecond)
	if err := cliEP.Connect("127.0.0.1", port); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client := tase2.NewClient(cliEP)
	defer client.Close()

	// A client outside ipAddresses is reported (not refused).
	e, _ := waitSecEvent(t, w, "conn.unexpected", kindIs("conn.unexpected", secSourceLibrary))
	if e.PeerIP != "127.0.0.1" || e.ProtocolConnectionNumber != 9201 {
		t.Errorf("conn.unexpected = %+v, want peer 127.0.0.1 of connection 9201", e)
	}
	// The address stated another AP title last time.
	e, _ = waitSecEvent(t, w, "peer.identity_changed", kindIs(string(kindPeerIdentityChanged), secSourceDriver))
	if e.Value != "1.1.999.2/12" || e.Role != "server" {
		t.Errorf("peer.identity_changed = %+v, want value 1.1.999.2/12", e)
	}

	// The identity is in the statistics.
	id := sec.statsDoc()
	clients, _ := id["clients"].(bson.A)
	if len(clients) != 1 || clients[0].(bson.M)["acseIdentity"] != "1.1.999.2/12" {
		t.Fatalf("peerIdentity.clients = %v, want the open association of 1.1.999.2/12", clients)
	}
	known, _ := id["known"].(bson.A)
	if len(known) != 1 || known[0].(bson.M)["acseIdentity"] != "1.1.999.2/12" {
		t.Errorf("peerIdentity.known = %v, want 127.0.0.1 updated to 1.1.999.2/12", known)
	}

	// A read of an object the model does not have (library).
	if _, err := client.ReadDataValue("SUB_A", "NO_SUCH_POINT"); err == nil {
		t.Error("read of a missing point succeeded")
	}
	waitSecEvent(t, w, "access.unknown_object", kindIs("access.unknown_object", secSourceLibrary))

	// A blocked command is refused by the driver, which reports why; the
	// library's own refusal by the handler is not stored twice.
	if err := client.Operate("SUB_A", sanitizePointName(brk.Tag), tase2.NewIntegerValue(1)); err == nil {
		t.Error("operate of a blocked command succeeded")
	}
	e, seen := waitSecEvent(t, w, "driver cmd.rejected", kindIs("cmd.rejected", secSourceDriver))
	if e.Action != "rejected" || e.Detail != "command blocked (tag BRK-CMD)" || e.PeerIP != "127.0.0.1" {
		t.Errorf("driver cmd.rejected = %+v", e)
	}
	time.Sleep(300 * time.Millisecond)
	for drained := false; !drained; {
		select {
		case e := <-w.C():
			seen = append(seen, e)
		default:
			drained = true
		}
	}
	for _, e := range seen {
		if e.Kind == "cmd.rejected" && e.Source == secSourceLibrary {
			t.Errorf("library cmd.rejected stored besides the driver's: %+v", e)
		}
	}

	client.Close()
	waitRegistryEmpty(t, registry)
	if c, _ := sec.statsDoc()["clients"].(bson.A); len(c) != 0 {
		t.Errorf("peerIdentity.clients after disconnect = %v, want none", c)
	}
}

func TestForwardCommandWrongType(t *testing.T) {
	w := captureSecEvents(t)
	sp := rtData{ID: 41, Tag: "SP-CMD", Type: "analog", Origin: "command", Group1: "SUB_A"}
	conn := protocolConnection{Name: "C", ProtocolConnectionNumber: 61, CommandsEnabled: true}
	m := buildModel([]rtData{sp}, []protocolConnection{conn}, 0)
	populateRtDataCache([]rtData{sp}, m.tagByKey)

	from := commandOrigin{sec: newSrvSec(conn, nil), domain: "SUB_A", item: "SP_CMD"}
	// A text for an analog command is refused before the queue (the nil
	// collection is never reached).
	if r := forwardCommand(nil, conn, sp.Tag, tase2.NewVisibleStringValue("open"), from); r != tase2.ResultFailure {
		t.Fatalf("text command result = %v, want failure", r)
	}
	e, _ := waitSecEvent(t, w, "cmd.wrong_type", kindIs(string(kindCmdWrongType), secSourceDriver))
	if e.Action != "rejected" || e.ObjectRef != "SUB_A/SP_CMD" || e.Value != "open" {
		t.Errorf("cmd.wrong_type = %+v", e)
	}
	if !numericCommand(tase2.NewVisibleStringValue(" 12.5 ")) || numericCommand(nil) {
		t.Error("numericCommand: a numeric string is a number, nil is not")
	}
}

func TestSrvSecKnownBounded(t *testing.T) {
	var entries bson.A
	base := time.Now().Add(-time.Hour)
	for i := 0; i < maxKnownPeers+5; i++ {
		entries = append(entries, bson.M{"ip": fmt.Sprintf("10.0.0.%d", i), "lastSeen": base.Add(time.Duration(i) * time.Second)})
	}
	raw, _ := bson.Marshal(bson.M{"known": entries})
	s := newSrvSec(protocolConnection{Name: "K", Stats: storedStats{PeerIdentity: raw}}, nil)
	if len(s.known) != maxKnownPeers {
		t.Fatalf("known = %d, want %d", len(s.known), maxKnownPeers)
	}
	if _, ok := s.known["10.0.0.0"]; ok {
		t.Error("the oldest stored identity was kept, want it forgotten")
	}
	off := false
	if newSrvSec(protocolConnection{PeerIdentification: &off}, nil).statsDoc() != nil {
		t.Error("statsDoc with peerIdentification off, want nil")
	}
}

func TestCommandNumber(t *testing.T) {
	cases := []struct {
		dv   *tase2.DataValue
		want float64
		ok   bool
	}{
		{tase2.NewVisibleStringValue(" 1 "), 1, true},
		{tase2.NewVisibleStringValue("12.5"), 12.5, true},
		{tase2.NewVisibleStringValue("NaN"), 0, false},
		{tase2.NewVisibleStringValue("+Inf"), 0, false},
		{tase2.NewVisibleStringValue("open"), 0, false},
		{tase2.NewFloat64Value(math.NaN()), 0, false},
		{tase2.NewFloat64Value(math.Inf(-1)), 0, false},
		{tase2.NewFloat32Value(12.5), 12.5, true},
		{tase2.NewIntegerValue(-3), -3, true},
		{tase2.NewBooleanValue(true), 1, true},
		{nil, 0, false},
	}
	for i, c := range cases {
		v, _, ok := commandNumber(c.dv)
		if ok != c.ok || (ok && v != c.want) {
			t.Errorf("case %d: %v -> %v %v, want %v %v", i, dataValueText(c.dv), v, ok, c.want, c.ok)
		}
	}
}

// stubQueue replaces the commandsQueue insert, returning the values queued.
func stubQueue(t *testing.T, fail error) *[]float64 {
	t.Helper()
	var mu sync.Mutex
	got := []float64{}
	queueCommand = func(_ *mongo.Collection, _ protocolConnection, _ string, v float64, _ string, _ rtData, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, v)
		return fail
	}
	t.Cleanup(func() { queueCommand = insertCommand })
	return &got
}

// An analog command tag is a real setpoint: a client operates it with its
// fraction, which reaches the queue unchanged.
func TestSetpointRoundTrip(t *testing.T) {
	queued := stubQueue(t, nil)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	isActive.Store(true)
	defer isActive.Store(false)

	sp := rtData{ID: 51, Tag: "SP-CMD", Type: "analog", Origin: "command", Group1: "SUB_A"}
	brk := rtData{ID: 52, Tag: "BRK-CMD", Type: "digital", Origin: "command", Group1: "SUB_A"}
	conn := protocolConnection{Name: "SP", ProtocolConnectionNumber: 9202, CommandsEnabled: true,
		RemoteApTitle: "1.1.999.2", RemoteAeQualifier: 12, Topics: []string{"SUB_A"}}
	tags := []rtData{sp, brk}
	model := buildModel(tags, []protocolConnection{conn}, 0)
	populateRtDataCache(tags, model.tagByKey)

	cfg := tase2.ServerConfig{Port: port, LocalAPTitle: "1.1.999.1", LocalAEQual: 12}
	registry := newServerRegistry()
	srvEP := tase2.NewEndpoint(tase2.EndpointPassive)
	srvEP.SetLocalAPTitle("1.1.999.1", 12)
	if err := srvEP.Listen(port); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srvEP.Disconnect()
	go srvEP.ServeClients(serveClientCallbacks(registry, model.dataModel, cfg, conn, model.datasetDefs, nil, nil))

	cliEP := tase2.NewEndpoint(tase2.EndpointActive)
	cliEP.SetLocalAPTitle("1.1.999.2", 12)
	cliEP.SetRemoteAPTitle("1.1.999.1", 12)
	time.Sleep(50 * time.Millisecond)
	if err := cliEP.Connect("127.0.0.1", port); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client := tase2.NewClient(cliEP)
	defer client.Close()

	spItem, brkItem := sanitizePointName(sp.Tag), sanitizePointName(brk.Tag)
	spec, err := client.GetVariableType(tase2.ObjectRef{Domain: "SUB_A", Item: spItem})
	if err != nil || spec.Type != mms.FloatingPoint {
		t.Fatalf("setpoint type = %+v, %v; want a floating point", spec, err)
	}
	spec, err = client.GetVariableType(tase2.ObjectRef{Domain: "SUB_A", Item: brkItem})
	if err != nil || spec.Type != mms.Integer {
		t.Fatalf("digital command type = %+v, %v; want Integer16", spec, err)
	}
	if err := client.Operate("SUB_A", spItem, tase2.NewFloat32Value(12.5)); err != nil {
		t.Fatalf("operate setpoint: %v", err)
	}
	if len(*queued) != 1 || (*queued)[0] != 12.5 {
		t.Errorf("queued %v, want [12.5]", *queued)
	}
	client.Close()
	waitRegistryEmpty(t, registry)
}

// A command the queue does not take is refused and reported by the driver.
func TestForwardCommandQueueFailure(t *testing.T) {
	w := captureSecEvents(t)
	stubQueue(t, errors.New("no primary"))
	sp := rtData{ID: 42, Tag: "SP2-CMD", Type: "analog", Origin: "command", Group1: "SUB_A"}
	conn := protocolConnection{Name: "C", ProtocolConnectionNumber: 62, CommandsEnabled: true}
	m := buildModel([]rtData{sp}, []protocolConnection{conn}, 0)
	populateRtDataCache([]rtData{sp}, m.tagByKey)
	from := commandOrigin{sec: newSrvSec(conn, nil), domain: "SUB_A", item: "SP2_CMD"}
	if r := forwardCommand(nil, conn, sp.Tag, tase2.NewFloat32Value(3), from); r != tase2.ResultFailure {
		t.Fatalf("result = %v, want failure", r)
	}
	e, _ := waitSecEvent(t, w, "cmd.rejected", kindIs("cmd.rejected", secSourceDriver))
	if e.Action != "rejected" || e.Detail != "command could not be queued: no primary (tag SP2-CMD)" {
		t.Errorf("cmd.rejected = %+v", e)
	}
}
