package main

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/riclolsen/tase2/mms"
	"github.com/riclolsen/tase2/secevent"
	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
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
// test after a timeout.
func waitSecEvent(t *testing.T, w *jssecevents.Writer, what string, match func(jssecevents.Event) bool) jssecevents.Event {
	t.Helper()
	var kinds []string
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-w.C():
			if match(e) {
				return e
			}
			kinds = append(kinds, e.Kind+"/"+e.Source)
		case <-deadline:
			t.Fatalf("timed out waiting for %s; got %v", what, kinds)
			return jssecevents.Event{}
		}
	}
}

func TestSecEventsClientLoopback(t *testing.T) {
	w := captureSecEvents(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	dm := tase2.NewDataModel()
	d := dm.AddDomain("SUB_A")
	p := d.AddDataPoint("ICCP_RealQ_01", tase2.ICCPTypeRealQ)
	p.UpdateValue(tase2.NewRealQ(1.5, tase2.QualityGood), tase2.QualityGood)
	dm.BuildDiscoverySnapshots()
	srvEP := tase2.NewEndpoint(tase2.EndpointPassive)
	srvEP.SetLocalAPTitle("1.1.999.1", 12)
	srv := tase2.NewServerWithConfig(dm, srvEP, tase2.ServerConfig{
		Port: port, LocalAPTitle: "1.1.999.1", LocalAEQual: 12,
		SupportedCBBs: []tase2.ConformanceBlock{tase2.CBB1_BasicPeriodicData, tase2.CBB2_ExtendedDataSets},
		VendorName:    "Loopback", ModelName: "Test", Revision: "2.0",
	})
	if err := srvEP.Listen(port); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srvEP.Disconnect()
	go func() {
		if err := srvEP.Accept(); err == nil {
			_ = srv.Start()
		}
	}()

	// The server identified itself differently in an earlier run.
	stored, _ := bson.Marshal(bson.M{"keys": bson.A{bson.M{"k": "identify", "v": "Loopback | Test | 1.0"}}})
	conn := protocolConnection{Name: "SECLOOP", ProtocolConnectionNumber: 9301,
		Stats: storedStats{PeerIdentity: stored}}
	sec := newConnSec(conn)
	defer sec.close()

	cliEP := tase2.NewEndpoint(tase2.EndpointActive)
	cliEP.SetLocalAPTitle("1.1.999.2", 12)
	cliEP.SetRemoteAPTitle("1.1.999.1", 12)
	sec.attach(cliEP)
	time.Sleep(50 * time.Millisecond)
	if err := cliEP.Connect("127.0.0.1", port); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client := tase2.NewClient(cliEP)
	defer client.Close()

	sec.identify(client, cliEP, net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), []string{"SUB_A"})
	id := sec.PeerIdentityDoc()
	if id["vendor"] != "Loopback" || id["revision"] != "2.0" || id["method"] != "identify" {
		t.Errorf("identity = %v, want MMS Identify Loopback/Test/2.0", id)
	}
	if id["tase2Version"] != "2000.8" {
		t.Errorf("tase2Version = %v, want 2000.8", id["tase2Version"])
	}
	if f, _ := id["supportedFeatures"].([]string); len(f) != 2 || f[0] != "CBB1" || f[1] != "CBB2" {
		t.Errorf("supportedFeatures = %v, want [CBB1 CBB2]", id["supportedFeatures"])
	}
	if id["ip"] != "127.0.0.1" || id["apTitle"] != "1.1.999.1" {
		t.Errorf("identity address = %v / %v, want 127.0.0.1 and the called AP title", id["ip"], id["apTitle"])
	}
	e := waitSecEvent(t, w, "peer.identity_changed", func(e jssecevents.Event) bool {
		return e.Kind == string(kindPeerIdentityChanged)
	})
	if e.Value != "Loopback | Test | 2.0" || e.PeerIP != "127.0.0.1" || e.Role != "client" || e.Source != secSourceDriver {
		t.Errorf("peer.identity_changed = %+v", e)
	}

	// A configured point the server does not have: reported once per
	// association.
	rt := newConnRuntime(conn, make(chan dataUpdate, 10))
	rt.sec = sec
	isActive.Store(true)
	defer isActive.Store(false)
	missing := []tagMapping{{tag: rtData{Tag: "MISSING", ProtocolSourceObjectAddress: "SUB_A/NoSuchPoint"},
		ref: tase2.ObjectRef{Domain: "SUB_A", Item: "NoSuchPoint"}}}
	performPoll(client, rt, missing, nil)
	performPoll(client, rt, missing, nil)
	e = waitSecEvent(t, w, "access.unknown_object", func(e jssecevents.Event) bool {
		return e.Kind == string(secevent.KindAccessUnknownObject)
	})
	if e.ObjectRef != "SUB_A/NoSuchPoint" || e.Source != secSourceDriver {
		t.Errorf("access.unknown_object = %+v", e)
	}
	time.Sleep(200 * time.Millisecond)
	for drained := false; !drained; {
		select {
		case e := <-w.C():
			if e.Kind == string(secevent.KindAccessUnknownObject) {
				t.Errorf("missing point reported twice in one association: %+v", e)
			}
		default:
			drained = true
		}
	}
}

func TestIdentityDecoders(t *testing.T) {
	v := tase2.NewOrderedStructureValue([]tase2.NamedDataValue{
		{Name: "MajorVersionNumber", Value: tase2.NewIntegerValue(2000)},
		{Name: "MinorVersionNumber", Value: tase2.NewIntegerValue(8)},
	})
	if got := tase2VersionText(v); got != "2000.8" {
		t.Errorf("tase2VersionText = %q", got)
	}
	if tase2VersionText(tase2.NewIntegerValue(1)) != "" {
		t.Error("tase2VersionText of a scalar, want empty")
	}
	f, ok := supportedFeatures(tase2.NewBitStringValue([]byte{0xC8, 0x00}))
	if !ok || len(f) != 3 || f[0] != "CBB1" || f[1] != "CBB2" || f[2] != "CBB5" {
		t.Errorf("supportedFeatures(C8 00) = %v, want [CBB1 CBB2 CBB5]", f)
	}
}

func TestClassifyCommandError(t *testing.T) {
	cases := []struct {
		err  error
		kind secevent.Kind
		ok   bool
	}{
		{&mms.DataAccessError{ErrorCode: mms.DataAccessObjectNonExistent}, kindCmdObjectUnknown, true},
		{&mms.DataAccessError{ErrorCode: mms.DataAccessTypeInconsistent}, kindCmdWrongType, true},
		{&mms.DataAccessError{ErrorCode: mms.DataAccessObjectAccessDenied}, secevent.KindCmdRejected, true},
		{&mms.ServiceError{Class: mms.SvcErrClassAccess, Code: mms.SvcErrAccessObjectNonExistent}, kindCmdObjectUnknown, true},
		{errors.New("server denied the write (no handler or access violation)"), secevent.KindCmdRejected, true},
		{errors.New("i/o timeout"), "", false},
	}
	for i, c := range cases {
		kind, _, ok := classifyCommandError(c.err)
		if kind != c.kind || ok != c.ok {
			t.Errorf("case %d: %v -> %q %v, want %q %v", i, c.err, kind, ok, c.kind, c.ok)
		}
	}
}

func TestDialFailedThrottled(t *testing.T) {
	w := captureSecEvents(t)
	s := newConnSec(protocolConnection{Name: "DOWN"})
	defer s.close()
	for i := 0; i < 5; i++ {
		s.libraryEvent(secevent.Event{Kind: secevent.KindConnDialFailed, Reason: "tcp", Severity: secevent.SeverityNotice})
	}
	if n := len(w.C()); n != 1 {
		t.Fatalf("queued %d dial failures, want 1 per period", n)
	}
	// A routine informational event is not stored.
	s.libraryEvent(secevent.Event{Kind: secevent.KindConnClosed})
	if n := len(w.C()); n != 1 {
		t.Errorf("conn.closed stored, want informational events dropped")
	}
	s.connected(true) // a working association: the next outage reports at once
	s.libraryEvent(secevent.Event{Kind: secevent.KindConnDialFailed, Reason: "tcp", Severity: secevent.SeverityNotice})
	if n := len(w.C()); n != 2 {
		t.Errorf("queued %d after a reconnect, want 2", n)
	}
}

func TestWriteResult(t *testing.T) {
	ok := tase2.NewBooleanValue(true)
	cases := []struct {
		resp          *mms.WriteResponse
		success       bool
		indeterminate bool
	}{
		{nil, false, true},
		{&mms.WriteResponse{}, false, true},
		{&mms.WriteResponse{Results: []any{(*mms.DataValue)(ok), (*mms.DataValue)(ok)}}, false, true},
		{&mms.WriteResponse{Results: []any{nil}}, false, false},
		{&mms.WriteResponse{Results: []any{&mms.DataAccessError{ErrorCode: mms.DataAccessObjectAccessDenied}}}, false, false},
		{&mms.WriteResponse{Results: []any{"odd"}}, false, true},
		{&mms.WriteResponse{Results: []any{(*mms.DataValue)(ok)}}, true, false},
	}
	for i, c := range cases {
		err := writeResult(c.resp)
		if (err == nil) != c.success || errors.Is(err, errWriteIndeterminate) != c.indeterminate {
			t.Errorf("case %d: err = %v, want success %v indeterminate %v", i, err, c.success, c.indeterminate)
		}
	}
	var dae *mms.DataAccessError
	if !errors.As(writeResult(&mms.WriteResponse{Results: []any{&mms.DataAccessError{}}}), &dae) {
		t.Error("a data access error is not wrapped")
	}
}

// A select-before-operate command selects on the association, then
// operates with the value (a real setpoint keeps its fraction).
func TestSendCommandSBO(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	dm := tase2.NewDataModel()
	d := dm.AddDomain("SUB_A")
	d.AddControlPoint("SP1", tase2.ControlTypeSetpoint, tase2.DeviceClassSBO).CurrentValue = tase2.NewFloat32Value(0)
	d.AddControlPoint("SP2", tase2.ControlTypeSetpoint, tase2.DeviceClassSBO).CurrentValue = tase2.NewFloat32Value(0)
	dm.BuildDiscoverySnapshots()
	srvEP := tase2.NewEndpoint(tase2.EndpointPassive)
	srvEP.SetLocalAPTitle("1.1.999.1", 12)
	srv := tase2.NewServerWithConfig(dm, srvEP, tase2.ServerConfig{Port: port, LocalAPTitle: "1.1.999.1", LocalAEQual: 12})
	var mu sync.Mutex
	var calls []string
	record := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, s)
	}
	srv.SetSelectHandler(func(domain, item string, _ *tase2.DataValue) tase2.HandlerResult {
		record("select " + item)
		if item == "SP2" {
			return tase2.ResultFailure
		}
		return tase2.ResultSuccess
	})
	srv.SetOperateHandler(func(domain, item string, v *tase2.DataValue) tase2.HandlerResult {
		record("operate " + item + " " + v.String())
		return tase2.ResultSuccess
	})
	if err := srvEP.Listen(port); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srvEP.Disconnect()
	go func() {
		if err := srvEP.Accept(); err == nil {
			_ = srv.Start()
		}
	}()

	cliEP := tase2.NewEndpoint(tase2.EndpointActive)
	cliEP.SetLocalAPTitle("1.1.999.2", 12)
	cliEP.SetRemoteAPTitle("1.1.999.1", 12)
	time.Sleep(50 * time.Millisecond)
	if err := cliEP.Connect("127.0.0.1", port); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client := tase2.NewClient(cliEP)
	defer client.Close()

	tag := rtData{Tag: "SP", Type: "analog", ProtocolSourceCommandUseSBO: true}
	cmd := commandQueueEntry{Tag: "SP", Value: 12.5, ProtocolSourceASDU: "float", TimeTag: time.Now()}
	if _, err := sendCommand(client, nil, nil, cmd, tag, "SUB_A", "SP1"); err != nil {
		t.Fatalf("SBO command: %v", err)
	}
	// A refused select is not followed by the operate.
	if _, err := sendCommand(client, nil, nil, cmd, tag, "SUB_A", "SP2"); err == nil {
		t.Error("command with a refused select succeeded")
	}
	// A command that expired by the time it was selected is not operated.
	cmd.TimeTag = time.Now().Add(-commandExpiry - time.Second)
	if _, err := sendCommand(client, nil, nil, cmd, tag, "SUB_A", "SP1"); err == nil {
		t.Error("expired command succeeded")
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"select SP1", "operate SP1 12.500000", "select SP2", "select SP1"}
	if len(calls) != len(want) {
		t.Fatalf("server saw %q, want %q", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("server saw %q, want %q", calls, want)
			break
		}
	}
}
