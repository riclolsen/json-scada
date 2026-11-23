/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

import (
	"context"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/mbsec"

	"github.com/otfabric/go-modbus"
	"github.com/otfabric/go-sunspec"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeDB is realtimeData, the sink and the command store in memory.
type fakeDB struct {
	mu    sync.Mutex
	tags  map[float64]bson.M
	next  float64
	sets  map[any]bson.M // last sourceDataUpdate per _id
	acks  map[any]string
	ackOK map[any]bool
}

func newFakeDB() *fakeDB {
	return &fakeDB{tags: map[float64]bson.M{}, next: 1000, sets: map[any]bson.M{}, acks: map[any]string{}, ackOK: map[any]bool{}}
}

func (f *fakeDB) Existing(_ context.Context, conn float64) (map[string]ExistingTag, map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	byKey, names := map[string]ExistingTag{}, map[string]bool{}
	for id, d := range f.tags {
		names[d["tag"].(string)] = true
		byKey[tagKey(int(jsmongo.GetDouble(d, "protocolSourceCommonAddress", 1)), d["protocolSourceObjectAddress"].(string),
			d["origin"] == "command")] = ExistingTag{ID: id}
	}
	return byKey, names, nil
}

func (f *fakeDB) NextKey(context.Context) float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	return f.next
}

func (f *fakeDB) Insert(_ context.Context, doc bson.M) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tags[doc["_id"].(float64)] = doc
	return nil
}

func (f *fakeDB) SetCommandOf(_ context.Context, id any, cmd float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tags[id.(float64)]["commandOfSupervised"] = cmd
	return nil
}

func (f *fakeDB) Apply(_ context.Context, ops []clientapp.RtOp) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, op := range ops {
		if sdu, ok := op.Set["sourceDataUpdate"].(bson.M); ok {
			f.sets[op.ID] = sdu
		}
	}
	return nil
}

func (f *fakeDB) Watch(ctx context.Context, _ float64, _ func(bson.M)) { <-ctx.Done() }

func (f *fakeDB) Ack(_ context.Context, id any, ok bool, desc string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acks[id], f.ackOK[id] = desc, ok
	return nil
}

func (f *fakeDB) bindings(context.Context) ([]bson.M, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []bson.M
	for _, d := range f.tags {
		if d["origin"] != "command" {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDB) byTag(tag string) bson.M {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.tags {
		if d["tag"] == tag {
			return d
		}
	}
	return nil
}

func (f *fakeDB) value(tag string) (bson.M, bool) {
	d := f.byTag(tag)
	if d == nil {
		return nil, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.sets[d["_id"]]
	return v, ok
}

// device serves a SunSpec device on a loopback port.
func device(t *testing.T, unit uint8) (*sunspec.Server, string) {
	t.Helper()
	srv, err := sunspec.NewServer(&sunspec.ServerOptions{BaseAddress: 40000, UnitID: unit},
		sunspec.ServerModel{ID: 1, Values: (&sunspec.Values{}).
			Set("Mn", "Acme").Set("Md", "PV-5000").Set("Vr", "1.0.0").Set("SN", "A-0001")},
		sunspec.ServerModel{ID: 103, Values: (&sunspec.Values{}).
			Set("W", 4200).Set("W_SF", 0).Set("PhVphA", 230.4).Set("V_SF", -1).Set("St", "MPPT")},
		sunspec.ServerModel{ID: 123, Values: (&sunspec.Values{}).
			Set("WMaxLimPct", 100).Set("WMaxLimPct_SF", 0).Set("Conn", "CONNECT")},
	)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := modbus.NewServer(&modbus.ServerConfig{URL: "tcp://127.0.0.1:0"}, srv)
	if err != nil {
		t.Fatal(err)
	}
	if err := ms.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ms.Stop() })
	return srv, ms.Addr().String()
}

func eventually(t *testing.T, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestLoopbackDiscoveryAutotagPollCommand(t *testing.T) {
	srv, addr := device(t, 2)
	db := newFakeDB()
	cfg := Normalize(bson.M{
		"protocolConnectionNumber": 7.0, "name": "INV", "ipAddresses": bson.A{addr},
		"unitIds": bson.A{2.0}, "autoCreateTags": true, "pollingInterval": 100.0, "timeoutMs": 500.0,
	})
	c := NewConnection(cfg, db)
	c.Commands, c.Tags, c.LoadBindings = db, db, db.bindings
	w := jssecevents.NewWriter(64)
	c.SetSecurity(&mbsec.Reporter{W: w, Role: "client", Origin: jssecevents.Origin{ProtocolDriver: "SUNSPEC"}}, nil)
	c.Start()
	defer c.Stop()

	eventually(t, "the W value", func() bool {
		v, ok := db.value("INV;2;103/W")
		return ok && v["valueAtSource"] == 4200.0 && v["invalidAtSource"] == false
	})
	if v, _ := db.value("INV;2;103/PhVphA"); v["valueAtSource"] != 230.4 {
		t.Errorf("PhVphA = %v, want 230.4 (scale factor applied)", v["valueAtSource"])
	}
	if v, _ := db.value("INV;2;103/St"); v["valueStringAtSource"] != "MPPT" {
		t.Errorf("St string = %v, want the enum symbol", v["valueStringAtSource"])
	}
	w103 := db.byTag("INV;2;103/W")
	if w103["unit"] != "W" || w103["protocolSourceASDU"] != "int16" || w103["protocolSourceCommonAddress"] != 2.0 {
		t.Errorf("W tag = unit %v asdu %v ca %v", w103["unit"], w103["protocolSourceASDU"], w103["protocolSourceCommonAddress"])
	}
	for _, skipped := range []string{"INV;2;103/W_SF", "INV;2;103/ID", "INV;2;103/L", "INV;2;103/A"} {
		if db.byTag(skipped) != nil {
			t.Errorf("tag %s should not be created (scale factor, header or not implemented)", skipped)
		}
	}
	if db.byTag("INV;2;103/W;cmd") != nil {
		t.Error("read-only point got a command tag")
	}
	cmd := db.byTag("INV;2;123/WMaxLimPct;cmd")
	sup := db.byTag("INV;2;123/WMaxLimPct")
	if cmd == nil || sup == nil {
		t.Fatal("WMaxLimPct supervised and command tags not created")
	}
	if cmd["supervisedOfCommand"] != sup["_id"] || sup["commandOfSupervised"] != cmd["_id"] || cmd["origin"] != "command" {
		t.Errorf("command link: cmd.supervisedOfCommand=%v sup._id=%v sup.commandOfSupervised=%v cmd._id=%v",
			cmd["supervisedOfCommand"], sup["_id"], sup["commandOfSupervised"], cmd["_id"])
	}
	if id := c.PeerIdentity(); id == nil || !strings.Contains(identitySummary(id), "Acme/PV-5000/1.0.0/A-0001") {
		t.Errorf("peer identity = %v", id)
	}

	// a command: write WMaxLimPct and see it on the device and in the tag
	c.HandleCommand(context.Background(), bson.M{"_id": "c1", "tag": "INV;2;123/WMaxLimPct;cmd",
		"protocolSourceCommonAddress": 2.0, "protocolSourceObjectAddress": "123/WMaxLimPct", "value": 55.0,
		"timeTag": time.Now()})
	if !db.ackOK["c1"] {
		t.Fatalf("command not acknowledged: %q", db.acks["c1"])
	}
	m, err := srv.Model(2)
	if err != nil || m.Point("WMaxLimPct") == nil || m.Point("WMaxLimPct").RawValue != uint16(55) {
		t.Fatalf("device WMaxLimPct after the command: %v %v", m, err)
	}
	if v, _ := db.value("INV;2;123/WMaxLimPct"); v["valueAtSource"] != 55.0 {
		t.Errorf("supervised WMaxLimPct = %v after the write", v["valueAtSource"])
	}

	// refused commands
	c.HandleCommand(context.Background(), bson.M{"_id": "c2", "protocolSourceCommonAddress": 2.0,
		"protocolSourceObjectAddress": "103/W", "value": 1.0})
	c.HandleCommand(context.Background(), bson.M{"_id": "c3", "protocolSourceCommonAddress": 2.0,
		"protocolSourceObjectAddress": "999/X", "value": 1.0})
	c.HandleCommand(context.Background(), bson.M{"_id": "c4", "protocolSourceCommonAddress": 2.0,
		"protocolSourceObjectAddress": "123/WMaxLimPct", "value": 1.0, "timeTag": time.Now().Add(-time.Hour)})
	for _, id := range []string{"c2", "c3", "c4"} {
		if db.ackOK[id] {
			t.Errorf("command %s accepted", id)
		}
	}

	// a device value change is published
	if err := srv.SetPoint(1, "W", 3100); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the changed W value", func() bool {
		v, _ := db.value("INV;2;103/W")
		return v["valueAtSource"] == 3100.0
	})

	kinds := map[string]bool{}
	for done := false; !done; {
		select {
		case e := <-w.C():
			kinds[e.Kind] = true
			if e.Protocol == "" {
				t.Errorf("event %s without protocol", e.Kind)
			}
		default:
			done = true
		}
	}
	for _, k := range []string{mbsec.KindPeerIdentified, mbsec.KindCmdRejected, mbsec.KindCmdObjectUnknown, mbsec.KindCmdStaleTime} {
		if !kinds[k] {
			t.Errorf("no %s event (got %v)", k, kinds)
		}
	}
}

func TestNotSunSpecUnitIsReported(t *testing.T) {
	_, addr := device(t, 1)
	db := newFakeDB()
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0, "ipAddresses": bson.A{addr},
		"unitIds": bson.A{1.0, 9.0}, "timeoutMs": 300.0, "maxRetries": 0.0})
	c := NewConnection(cfg, db)
	w := jssecevents.NewWriter(64)
	c.SetSecurity(&mbsec.Reporter{W: w, Role: "client"}, nil)
	c.secNewLink(addr)
	cl, br, _, err := c.open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	if br != nil {
		defer br.Close()
	}
	found, _ := c.discoverUnits(context.Background(), cl, cfg.UnitIDs)
	if found != 1 {
		t.Fatalf("found %d units, want 1", found)
	}
	select {
	case e := <-w.C():
		if e.Kind != mbsec.KindAccessUnitUnknown || e.Protocol != Protocol {
			t.Errorf("event %s/%s, want %s", e.Protocol, e.Kind, mbsec.KindAccessUnitUnknown)
		}
	case <-time.After(time.Second):
		t.Error("no event for the unit without a device")
	}
}

func TestAddressRoundTrip(t *testing.T) {
	for _, s := range []string{"103/W", "160/module[2]/DCW", "160#2/module[1]/DCA", "705/curve[1]/pt[3]/V"} {
		a, err := ParseAddress(s)
		if err != nil || a.String() != s {
			t.Errorf("%s -> %v %v", s, a.String(), err)
		}
	}
	for _, s := range []string{"", "103", "x/W", "103#0/W", "160/module/DCW", "160/module[0]/DCW", "103/"} {
		if _, err := ParseAddress(s); err == nil {
			t.Errorf("%q parsed", s)
		}
	}
}

func TestPointValue(t *testing.T) {
	sf := 123.4
	cases := []struct {
		p    sunspec.DecodedPoint
		want Value
	}{
		{sunspec.DecodedPoint{Type: "int16", RawValue: int16(1234), ScaledValue: &sf, SFName: "W_SF", Implemented: true}, Value{Num: 123.4, Str: "123.4"}},
		{sunspec.DecodedPoint{Type: "uint16", RawValue: uint16(7), Implemented: true}, Value{Num: 7, Str: "7"}},
		{sunspec.DecodedPoint{Type: "int16", RawValue: int16(-32768), SFName: "W_SF"}, Value{Num: -32768, Str: "-32768", Invalid: true}},
		{sunspec.DecodedPoint{Type: "uint16", RawValue: uint16(5), SFName: "A_SF", Implemented: true}, Value{Num: 5, Str: "5", Invalid: true}},
		{sunspec.DecodedPoint{Type: "enum16", RawValue: uint16(4), Symbols: []string{"MPPT"}, Implemented: true}, Value{Num: 4, Str: "MPPT"}},
		{sunspec.DecodedPoint{Type: "string", RawValue: "Acme", Implemented: true}, Value{Str: "Acme"}},
	}
	for i, c := range cases {
		if got := PointValue(&c.p); got != c.want {
			t.Errorf("case %d: %+v, want %+v", i, got, c.want)
		}
	}
}

// TestReadmeDocumentsEveryConfigKey fails when config.go reads a key that the
// driver README does not name.
func TestReadmeDocumentsEveryConfigKey(t *testing.T) {
	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile("../../cmd/sunspecclient/README.md")
	if err != nil {
		t.Fatal(err)
	}
	keys := regexp.MustCompile(`\(doc, "([A-Za-z0-9]+)"`).FindAllStringSubmatch(string(src), -1)
	if len(keys) < 7 {
		t.Fatalf("only %d keys found in config.go", len(keys))
	}
	shared := []string{"connectionMode", "ipAddresses", "ipAddressLocalBind", "portName", "baudRate", "parity",
		"stopBits", "timeoutMs", "maxRetries", "pollingInterval", "giInterval", "interRequestDelayMs",
		"autoCreateTags", "commandsEnabled", "peerIdentification", "keepProtocolRunningWhileInactive",
		"localCertFilePath", "privateKeyFilePath", "peerCertFilePath", "chainValidation"}
	for _, k := range keys {
		shared = append(shared, k[1])
	}
	for _, k := range shared {
		if !strings.Contains(string(readme), "`"+k+"`") {
			t.Errorf("config key %s is not documented in the README", k)
		}
	}
}
