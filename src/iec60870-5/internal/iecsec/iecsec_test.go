package iecsec

import (
	"sync"
	"testing"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"
)

// recorder collects the events a Reporter emits.
type recorder struct {
	mu  sync.Mutex
	evs []jssecevents.Event
}

func (r *recorder) add(e jssecevents.Event) { r.mu.Lock(); r.evs = append(r.evs, e); r.mu.Unlock() }

func (r *recorder) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var k []string
	for _, e := range r.evs {
		k = append(k, e.Kind)
	}
	return k
}

func (r *recorder) last() jssecevents.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.evs[len(r.evs)-1]
}

func newRep(ident bool) (*Reporter, *recorder) {
	rec := &recorder{}
	return &Reporter{Role: "client", Protocol: "iec104", Ident: ident, capture: rec.add}, rec
}

func TestKeepFiltersRoutineIdentityAndDropped(t *testing.T) {
	r := &Reporter{Ident: false, Drop: DropServerRefusals}
	cases := []struct {
		kind secevent.Kind
		keep bool
	}{
		{secevent.KindConnAccepted, false},
		{secevent.KindCmdReceived, false},
		{secevent.KindPeerMACChanged, false}, // identity, Ident off
		{secevent.KindCmdRejected, false},    // the driver reports it
		{secevent.KindConnUnexpected, true},
		{secevent.KindSeqNsGap, true},
	}
	for _, c := range cases {
		if got := r.Keep(secevent.Event{Kind: c.kind}); got != c.keep {
			t.Errorf("Keep(%s) = %v, want %v", c.kind, got, c.keep)
		}
	}
	r.Ident = true
	if !r.Keep(secevent.Event{Kind: secevent.KindPeerMACChanged}) {
		t.Error("peer.mac_changed dropped with peerIdentification on")
	}
}

func TestDropClientRefusalsOnlyCommands(t *testing.T) {
	cmd := secevent.Event{Kind: secevent.KindRemoteRejected, Fields: secevent.FieldType, TypeID: asdu.C_SC_NA_1}
	gi := secevent.Event{Kind: secevent.KindRemoteRejected, Fields: secevent.FieldType, TypeID: asdu.C_IC_NA_1}
	if !DropClientRefusals(cmd) {
		t.Error("a refused process command is the driver's to report")
	}
	if DropClientRefusals(gi) {
		t.Error("a refused interrogation must come from the library")
	}
}

func TestFromLibFields(t *testing.T) {
	e := FromLib(secevent.Event{Kind: secevent.KindSeqNsGap, Protocol: "iec104",
		Fields: secevent.FieldType | secevent.FieldCause | secevent.FieldCommonAddr | secevent.FieldIOA | secevent.FieldSeq,
		TypeID: asdu.C_SC_NA_1, Cause: asdu.Activation, Negative: true, CommonAddr: 3, IOA: 1001,
		SendSN: 5, RecvSN: 2, ExpectedSN: 4})
	if e.Source != SourceLibrary || e.ObjectRef != "CA 3 IOA 1001" {
		t.Fatalf("source %q objectRef %q", e.Source, e.ObjectRef)
	}
	for k, want := range map[string]any{"typeId": int(asdu.C_SC_NA_1), "cause": int(asdu.Activation),
		"negative": true, "commonAddress": 3, "ioa": 1001, "expectedSeq": 4} {
		if got := e.Fields[k]; got != want {
			t.Errorf("fields[%s] = %v (%T), want %v", k, got, got, want)
		}
	}
}

func TestClientIdentityChangesAcrossRestart(t *testing.T) {
	rep, rec := newRep(true)
	c := NewClient(rep, nil, false)
	c.Begin(bson.M{"address": "10.0.0.5:2404"}, []Key{{K: "mac", V: "aa:aa", Label: "MAC address"}})
	if k := rec.kinds(); len(k) != 1 || k[0] != KindPeerIdentified {
		t.Fatalf("first connection: %v", k)
	}
	c.Begin(bson.M{"address": "10.0.0.5:2404"}, []Key{{K: "mac", V: "aa:aa"}})
	if len(rec.kinds()) != 1 {
		t.Fatalf("same identity reported: %v", rec.kinds())
	}

	// a new process starts from the stored identity and sees the change
	rep2, rec2 := newRep(true)
	c2 := NewClient(rep2, c.PeerIdentity(), false)
	c2.Begin(bson.M{"address": "10.0.0.5:2404"}, []Key{{K: "mac", V: "bb:bb", Label: "MAC address"}})
	if k := rec2.kinds(); len(k) != 1 || k[0] != string(secevent.KindPeerMACChanged) {
		t.Fatalf("after restart: %v", k)
	}
	if e := rec2.last(); e.Previous != "aa:aa" || e.Value != "bb:bb" {
		t.Errorf("previous %q value %q", e.Previous, e.Value)
	}
}

func TestClientLibPeerLeavesMACToLibrary(t *testing.T) {
	rep, rec := newRep(true)
	c := NewClient(rep, nil, true)
	c.Begin(bson.M{}, []Key{{K: "mac", V: "aa:aa"}})
	c.Begin(bson.M{}, []Key{{K: "mac", V: "bb:bb"}})
	if k := rec.kinds(); len(k) != 1 || k[0] != KindPeerIdentified {
		t.Fatalf("events: %v", k)
	}
}

func TestClientDeviceIdentityCarriedForward(t *testing.T) {
	rep, rec := newRep(true)
	c := NewClient(rep, nil, false)
	c.Begin(bson.M{"address": "serial:///dev/ttyS0"}, nil)
	c.AddDevice("CA 1", bson.M{"identification": "ACME RELAY"}, []Key{{K: "ident:1", V: "2|ACME RELAY|01"}})

	// reconnect: the device has not identified itself yet, its key stays
	c.Begin(bson.M{"address": "serial:///dev/ttyS0"}, nil)
	found := false
	for _, k := range jsmongo.GetDocArray(c.PeerIdentity(), "keys") {
		if k["k"] == "ident:1" && k["v"] == "2|ACME RELAY|01" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ident:1 not carried forward: %v", c.PeerIdentity())
	}
	c.AddDevice("CA 1", bson.M{"identification": "OTHER"}, []Key{{K: "ident:1", V: "2|OTHER|01", Label: "device 1 identification"}})
	if e := rec.last(); e.Kind != KindPeerIdentityChanged || e.Previous != "2|ACME RELAY|01" {
		t.Fatalf("last event %+v", e)
	}
	devs := jsmongo.GetDocArray(c.PeerIdentity(), "devices")
	if len(devs) != 1 || devs[0]["identification"] != "OTHER" {
		t.Errorf("devices %v", devs)
	}
}

func TestClientIdentOff(t *testing.T) {
	rep, rec := newRep(false)
	c := NewClient(rep, bson.M{"keys": bson.A{bson.M{"k": "mac", "v": "aa"}}}, false)
	c.Begin(bson.M{"address": "x"}, []Key{{K: "mac", V: "bb"}})
	if c.PeerIdentity() != nil || len(rec.kinds()) != 0 {
		t.Fatalf("identity %v events %v", c.PeerIdentity(), rec.kinds())
	}
}

func cmdASDU(cause asdu.Cause, negative bool) *asdu.ASDU {
	a := asdu.NewASDU(asdu.ParamsWide, asdu.Identifier{Type: asdu.C_SC_NA_1,
		Variable: asdu.VariableStruct{Number: 1},
		Coa:      asdu.CauseOfTransmission{Cause: cause, IsNegative: negative}, CommonAddr: 7})
	a.InfoObj = append(a.InfoObj, 0xE9, 0x03, 0x00, 0x01) // IOA 1001, SCO on
	return a
}

func TestClientAnswer(t *testing.T) {
	rep, rec := newRep(true)
	c := NewClient(rep, nil, false)
	c.Answer(cmdASDU(asdu.ActivationCon, false))
	if len(rec.kinds()) != 0 {
		t.Fatalf("positive confirmation reported: %v", rec.kinds())
	}
	c.Answer(cmdASDU(asdu.ActivationCon, true))
	if e := rec.last(); e.Kind != string(secevent.KindCmdRejected) || e.ObjectRef != "CA 7 IOA 1001" {
		t.Fatalf("negative confirmation: %+v", e)
	}
	c.Answer(cmdASDU(asdu.UnknownIOA, true))
	if e := rec.last(); e.Kind != KindCmdObjectUnknown {
		t.Fatalf("unknown IOA: %+v", e)
	}
}

// A device identification that arrives right after the connection is not
// erased, and a lookup of a connection that ended adds nothing to the next.
func TestClientSessionGeneration(t *testing.T) {
	rep, _ := newRep(true)
	c := NewClient(rep, nil, false)
	g1 := c.begin(bson.M{"address": "a"}, nil)
	c.AddDevice("CA 1", bson.M{"identification": "X"}, []Key{{K: "ident:1", V: "1|X|"}})
	if devs := jsmongo.GetDocArray(c.PeerIdentity(), "devices"); len(devs) != 1 {
		t.Fatalf("device lost: %v", c.PeerIdentity())
	}
	g2 := c.begin(bson.M{"address": "b"}, nil)
	c.addToSession(g1, "mac", "aa:aa", Key{K: "mac", V: "aa:aa"}) // the first connection's late lookup
	if _, ok := c.PeerIdentity()["mac"]; ok {
		t.Fatal("a late lookup of an old connection reached the new identity")
	}
	c.addToSession(g2, "mac", "bb:bb", Key{K: "mac", V: "bb:bb"})
	if c.PeerIdentity()["mac"] != "bb:bb" {
		t.Fatalf("current lookup not added: %v", c.PeerIdentity())
	}
}

func TestClientPeerWithoutIdentification(t *testing.T) {
	rep, rec := newRep(false)
	c := NewClient(rep, nil, false)
	c.Connected("/dev/ttyS1", 3, nil)
	c.Refused103(1, 0x10100)
	if e := rec.last(); e.PeerAddress == "" {
		t.Fatalf("event without the peer: %+v", e)
	}
	if c.PeerIdentity() != nil {
		t.Fatal("identity recorded with peerIdentification off")
	}
}
