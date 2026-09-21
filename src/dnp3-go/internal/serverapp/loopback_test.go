package serverapp

import (
	"context"
	"testing"
	"time"

	"dnp3-go/internal/dnp3util"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/multidrop"
	"github.com/dscsystems/go-dnp3/outstation"
)

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// recorder collects what a master decodes, keyed by type and index.
type recorder struct {
	master.NopHandler
	mu       chan struct{}
	binaries map[uint16]dnp3.Binary
	analogs  map[uint16]dnp3.Analog
	counters map[uint16]dnp3.Counter
	frozen   map[uint16]dnp3.FrozenCounter
	doubles  map[uint16]dnp3.DoubleBitBinary
	bos      map[uint16]dnp3.BinaryOutputStatus
	aos      map[uint16]dnp3.AnalogOutputStatus
}

func newRecorder() *recorder {
	r := &recorder{
		mu:       make(chan struct{}, 1),
		binaries: map[uint16]dnp3.Binary{},
		analogs:  map[uint16]dnp3.Analog{},
		counters: map[uint16]dnp3.Counter{},
		frozen:   map[uint16]dnp3.FrozenCounter{},
		doubles:  map[uint16]dnp3.DoubleBitBinary{},
		bos:      map[uint16]dnp3.BinaryOutputStatus{},
		aos:      map[uint16]dnp3.AnalogOutputStatus{},
	}
	r.mu <- struct{}{}
	return r
}

func (r *recorder) lock()   { <-r.mu }
func (r *recorder) unlock() { r.mu <- struct{}{} }

func (r *recorder) HandleBinary(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.Binary]) {
	r.lock()
	defer r.unlock()
	for _, v := range vs {
		r.binaries[v.Index] = v.Value
	}
}
func (r *recorder) HandleAnalog(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.Analog]) {
	r.lock()
	defer r.unlock()
	for _, v := range vs {
		r.analogs[v.Index] = v.Value
	}
}
func (r *recorder) HandleCounter(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.Counter]) {
	r.lock()
	defer r.unlock()
	for _, v := range vs {
		r.counters[v.Index] = v.Value
	}
}
func (r *recorder) HandleFrozenCounter(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.FrozenCounter]) {
	r.lock()
	defer r.unlock()
	for _, v := range vs {
		r.frozen[v.Index] = v.Value
	}
}
func (r *recorder) HandleDoubleBit(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.DoubleBitBinary]) {
	r.lock()
	defer r.unlock()
	for _, v := range vs {
		r.doubles[v.Index] = v.Value
	}
}
func (r *recorder) HandleBinaryOutputStatus(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.BinaryOutputStatus]) {
	r.lock()
	defer r.unlock()
	for _, v := range vs {
		r.bos[v.Index] = v.Value
	}
}
func (r *recorder) HandleAnalogOutputStatus(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.AnalogOutputStatus]) {
	r.lock()
	defer r.unlock()
	for _, v := range vs {
		r.aos[v.Index] = v.Value
	}
}

// tag builds a realtimeData document with one destination.
func tag(id float64, name string, value any, commonAddress, objectAddress, asdu int, extra map[string]any) map[string]any {
	doc := map[string]any{
		"_id":    id,
		"tag":    name,
		"origin": "supervised",
		"value":  value,
		"protocolDestinations": []any{
			map[string]any{
				"protocolDestinationConnectionNumber": 1.0,
				"protocolDestinationCommonAddress":    float64(commonAddress),
				"protocolDestinationObjectAddress":    float64(objectAddress),
				"protocolDestinationASDU":             float64(asdu),
				"protocolDestinationKConv1":           1.0,
				"protocolDestinationKConv2":           0.0,
				"protocolDestinationHoursShift":       0.0,
			},
		},
	}
	for k, v := range extra {
		doc[k] = v
	}
	return doc
}

// TestServerLoopback builds an outstation the way the driver does, from a set
// of tags, and reads it back with a master: what the driver distributes has to
// arrive intact, with the variations the destinations asked for.
func TestServerLoopback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &Connection{
		ProtocolConnectionNumber: 1,
		Name:                     "SRV",
		LocalLinkAddress:         10,
		RemoteLinkAddress:        1,
		EnableUnsolicited:        false,
		ServerQueueSize:          100,
		CommandsEnabled:          false,
	}

	tags := []map[string]any{
		tag(1, "BIN0", true, 1, 0, 2, nil),
		tag(2, "BIN1", false, 1, 1, 2, nil),
		// A fractional analog on the default single precision variation: an
		// integer variation would deliver 123 instead.
		tag(3, "ANA0", 123.5, 30, 0, 6, nil),
		tag(4, "ANA1", -7.25, 30, 1, 5, nil),
		tag(5, "CNT0", 4242.0, 20, 0, 1, nil),
		tag(6, "FRZ0", 99.0, 21, 0, 1, nil),
		tag(7, "DBL0", true, 3, 0, 2, nil),
		tag(8, "BOS0", true, 10, 0, 2, nil),
		tag(9, "AOS0", 55.5, 40, 0, 3, nil),
		// An invalid tag has to reach the master as comm-lost.
		tag(10, "ANA2", 1.0, 30, 2, 5, map[string]any{"invalid": true}),
	}

	e := &Engine{byNum: map[int]*Connection{1: conn}}
	station := e.newOutstation(conn, toBsonSlice(tags))
	conn.setStation(station)

	mch, och := channel.Pipe()
	go func() { _ = station.Run(ctx, och) }()

	rec := newRecorder()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 3 * time.Second,
	}, rec)
	go func() { _ = m.Run(ctx, mch) }()

	waitFor(t, "the master to connect", m.Connected)
	if err := m.IntegrityPoll(ctx); err != nil {
		t.Fatalf("IntegrityPoll: %v", err)
	}

	rec.lock()
	defer rec.unlock()

	if v := rec.binaries[0]; !v.Value {
		t.Error("binary 0 should be true")
	}
	if v := rec.binaries[1]; v.Value {
		t.Error("binary 1 should be false")
	}
	if v := rec.analogs[0]; v.Value != 123.5 {
		t.Errorf("analog 0 = %v, want 123.5 (the fraction must survive the variation)", v.Value)
	}
	if v := rec.analogs[1]; v.Value != -7.25 {
		t.Errorf("analog 1 = %v, want -7.25", v.Value)
	}
	if v := rec.analogs[2]; !v.Flags.Has(dnp3.CommLost) {
		t.Errorf("analog 2 is an invalid tag and must arrive comm-lost, got %s", v.Flags)
	}
	if v := rec.counters[0]; v.Value != 4242 {
		t.Errorf("counter 0 = %d, want 4242", v.Value)
	}
	if v := rec.frozen[0]; v.Value != 99 {
		t.Errorf("frozen counter 0 = %d, want 99", v.Value)
	}
	if v := rec.doubles[0]; v.Value != dnp3.DoubleBitDeterminedOn {
		t.Errorf("double-bit 0 = %v, want DeterminedOn", v.Value)
	}
	if v := rec.bos[0]; !v.Value {
		t.Error("binary output status 0 should be true")
	}
	if v := rec.aos[0]; v.Value != 55.5 {
		t.Errorf("analog output status 0 = %v, want 55.5", v.Value)
	}
}

// TestServerDistributesChange checks the change-stream path: a new value
// applied through Session.Update reaches a polling master as an event.
func TestServerDistributesChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &Connection{
		ProtocolConnectionNumber: 1,
		Name:                     "SRV",
		LocalLinkAddress:         10,
		RemoteLinkAddress:        1,
		ServerQueueSize:          100,
	}
	tags := []map[string]any{tag(1, "ANA0", 1.0, 30, 0, 5, nil)}

	e := &Engine{byNum: map[int]*Connection{1: conn}}
	station := e.newOutstation(conn, toBsonSlice(tags))
	conn.setStation(station)

	mch, och := channel.Pipe()
	go func() { _ = station.Run(ctx, och) }()

	rec := newRecorder()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 3 * time.Second,
	}, rec)
	go func() { _ = m.Run(ctx, mch) }()

	waitFor(t, "the master to connect", m.Connected)
	if err := m.IntegrityPoll(ctx); err != nil {
		t.Fatalf("IntegrityPoll: %v", err)
	}

	// The same path the change stream takes.
	e.distribute(toBson(tag(1, "ANA0", 42.5, 30, 0, 5, nil)))

	if err := m.ScanClasses(ctx, dnp3.Class123); err != nil {
		t.Fatalf("ScanClasses: %v", err)
	}

	rec.lock()
	got := rec.analogs[0].Value
	rec.unlock()
	if got != 42.5 {
		t.Errorf("analog 0 = %v after the change, want 42.5", got)
	}
}

// TestServerMultidrop puts two outstations behind one bus and checks that each
// master reaches only its own, which is the multi-drop case JSON-SCADA
// configures by repeating an endpoint and varying the link addresses.
func TestServerMultidrop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mch, och := channel.Pipe()

	// Two outstations on one line, at link addresses 10 and 11.
	outBus := multidrop.New(och, multidrop.Config{})
	defer outBus.Close()

	stations := map[int]*outstation.Session{}
	for i, addr := range []int{10, 11} {
		// Both stations use protocol connection number 1: each has its own
		// engine and its own outstation, and the tag fixtures are written for
		// connection 1. What distinguishes them on the line is the link
		// address, which is the point of the test.
		conn := &Connection{
			ProtocolConnectionNumber: 1,
			Name:                     "SRV",
			LocalLinkAddress:         addr,
			RemoteLinkAddress:        1,
			ServerQueueSize:          100,
		}
		// Each outstation carries one analog whose value identifies it.
		tags := []map[string]any{tag(float64(i+1), "ANA0", float64(addr)*100, 30, 0, 5, nil)}
		e := &Engine{byNum: map[int]*Connection{conn.ProtocolConnectionNumber: conn}}
		station := e.newOutstation(conn, toBsonSlice(tags))
		conn.setStation(station)

		sub, err := outBus.Add(multidrop.Station{
			LocalAddr: uint16(addr), RemoteAddr: 1, Master: false,
		})
		if err != nil {
			t.Fatalf("bus.Add outstation %d: %v", addr, err)
		}
		go func() { _ = station.Run(ctx, sub) }()
		stations[addr] = station
	}

	// Two masters on the other end of the same line.
	mBus := multidrop.New(mch, multidrop.Config{})
	defer mBus.Close()

	recorders := map[int]*recorder{}
	masters := map[int]*master.Session{}
	for _, addr := range []int{10, 11} {
		sub, err := mBus.Add(multidrop.Station{
			LocalAddr: 1, RemoteAddr: uint16(addr), Master: true,
		})
		if err != nil {
			t.Fatalf("bus.Add master for %d: %v", addr, err)
		}
		rec := newRecorder()
		m := master.New(master.Config{
			LocalAddr: 1, RemoteAddr: uint16(addr),
			ResponseTimeout: 5 * time.Second,
		}, rec)
		go func() { _ = m.Run(ctx, sub) }()
		recorders[addr] = rec
		masters[addr] = m
	}

	for _, addr := range []int{10, 11} {
		waitFor(t, "master for station to connect", masters[addr].Connected)
	}

	for _, addr := range []int{10, 11} {
		if err := masters[addr].IntegrityPoll(ctx); err != nil {
			t.Fatalf("IntegrityPoll for station %d: %v", addr, err)
		}
	}

	for _, addr := range []int{10, 11} {
		rec := recorders[addr]
		rec.lock()
		got := rec.analogs[0].Value
		count := len(rec.analogs)
		rec.unlock()

		want := float64(addr) * 100
		if got != want {
			t.Errorf("master for station %d read %v, want %v — the bus routed the wrong outstation's data",
				addr, got, want)
		}
		if count != 1 {
			t.Errorf("master for station %d saw %d analogs, want 1", addr, count)
		}
	}

	if s := mBus.Stats(); s.FramesUnrouted > 0 {
		t.Errorf("%d frames were routed to nobody; the link addresses do not line up",
			s.FramesUnrouted)
	}
}

// TestAllowedRemoteIPsSkippedForActive checks that the allowed-address list is
// only applied to passive connections, where it means "clients that may
// connect"; on an active one it is the list of servers to dial.
func TestAllowedRemoteIPsSkippedForActive(t *testing.T) {
	active := dnp3util.ChannelSpec{
		Name: "A", Mode: dnp3util.ModeTCPActive,
		IPAddresses: []string{"127.0.0.1:20995"},
	}
	if active.IsPassive() {
		t.Error("an active connection must not be treated as passive")
	}
	passive := dnp3util.ChannelSpec{
		Name: "P", Mode: dnp3util.ModeTCPPassive,
		IPAddressLocalBind: "127.0.0.1:20994",
	}
	if !passive.IsPassive() {
		t.Error("a passive connection must be treated as passive")
	}
}
