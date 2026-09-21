package clientapp

import (
	"context"
	"testing"
	"time"

	"dnp3-go/internal/dnp3util"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/outstation"
)

// waitFor polls until cond holds or the deadline passes.
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

// seededOutstation builds an outstation carrying one point of every family the
// driver handles, with values chosen so that a wrong variation or a missing
// quality bit shows up as a wrong assertion rather than as a coincidence.
func seededOutstation(t *testing.T) (*outstation.Session, channel.Channel, channel.Channel) {
	t.Helper()

	mch, och := channel.Pipe()
	cfg := outstation.Config{
		LocalAddr:  10,
		RemoteAddr: 1,
		Database: outstation.DatabaseConfig{
			Binary: 2, DoubleBitBinary: 2, Counter: 2, FrozenCounter: 2,
			Analog: 2, BinaryOutputStatus: 2, AnalogOutputStatus: 2,
			DefaultClass: dnp3.Class1,
		},
	}
	out := outstation.New(cfg, outstation.NopApplication{}, nil)

	db := out.Database()
	// Single precision on the analogs, so a fractional value survives; the
	// library's default is a 32-bit integer variation. Analog inputs take
	// g30v5/g32v7; analog output status runs g40v1..v4 only, so its single
	// precision variation is 3 — the same pair the server driver configures.
	for i := uint16(0); i < 2; i++ {
		if _, cfg, ok := db.Analog(i); ok {
			cfg.StaticVariation, cfg.EventVariation = 5, 7
			db.Configure(dnp3.TypeAnalog, i, cfg)
		}
		if _, cfg, ok := db.AnalogOutputStatus(i); ok {
			cfg.StaticVariation, cfg.EventVariation = 3, 7
			db.Configure(dnp3.TypeAnalogOutputStatus, i, cfg)
		}
	}

	stamp := dnp3.Now(time.UnixMilli(1700000000000))

	db.UpdateBinary(0, dnp3.Binary{Value: true, Flags: dnp3.Online, Time: stamp})
	db.UpdateBinary(1, dnp3.Binary{Value: false, Flags: dnp3.Online | dnp3.CommLost, Time: stamp})
	db.UpdateDoubleBit(0, dnp3.DoubleBitBinary{
		Value: dnp3.DoubleBitDeterminedOn, Flags: dnp3.Online, Time: stamp})
	db.UpdateDoubleBit(1, dnp3.DoubleBitBinary{
		Value: dnp3.DoubleBitIntermediate, Flags: dnp3.Online, Time: stamp})
	db.UpdateAnalog(0, dnp3.Analog{Value: 123.5, Flags: dnp3.Online, Time: stamp})
	db.UpdateAnalog(1, dnp3.Analog{Value: -7.25, Flags: dnp3.Online | dnp3.OverRange, Time: stamp})
	db.UpdateCounter(0, dnp3.Counter{Value: 4242, Flags: dnp3.Online, Time: stamp})
	db.UpdateCounter(1, dnp3.Counter{Value: 7, Flags: dnp3.Online | dnp3.Rollover, Time: stamp})
	db.UpdateFrozenCounter(0, dnp3.FrozenCounter{Value: 99, Flags: dnp3.Online, Time: stamp})
	db.UpdateBinaryOutputStatus(0, dnp3.BinaryOutputStatus{
		Value: true, Flags: dnp3.Online | dnp3.LocalForced, Time: stamp})
	db.UpdateAnalogOutputStatus(0, dnp3.AnalogOutputStatus{
		Value: 55.5, Flags: dnp3.Online, Time: stamp})

	out.Events().Reset()
	return out, mch, och
}

// TestLoopbackIntegrityPoll runs the driver's own handler against a real
// outstation over an in-process pipe and checks what lands in the queue: the
// common address every family is filed under, the value, and the quality.
func TestLoopbackIntegrityPoll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, mch, och := seededOutstation(t)
	go func() { _ = out.Run(ctx, och) }()

	conn := &Connection{Name: "TEST", ProtocolConnectionNumber: 7}
	queue := NewValueQueue()

	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 3 * time.Second,
	}, newSOEHandler(conn, queue))
	go func() { _ = m.Run(ctx, mch) }()

	waitFor(t, "the master to connect", m.Connected)

	if err := m.IntegrityPoll(ctx); err != nil {
		t.Fatalf("IntegrityPoll: %v", err)
	}

	// The handler runs on the session goroutine, so the last values of the
	// response can land just after IntegrityPoll returns. Wait for the whole
	// set rather than for a fixed delay, which is racy under -race.
	const wantValues = 11
	got := map[[2]int]Dnp3Value{}
	deadline := time.Now().Add(5 * time.Second)
	for len(got) < wantValues && time.Now().Before(deadline) {
		for _, v := range drainAll(queue) {
			got[[2]int{v.BaseGroup, v.Address}] = v
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(got) < wantValues {
		t.Errorf("received %d values, want %d", len(got), wantValues)
	}

	check := func(baseGroup, addr int, want float64, fn func(Dnp3Value)) {
		t.Helper()
		v, ok := got[[2]int{baseGroup, addr}]
		if !ok {
			t.Errorf("no value for group %d address %d", baseGroup, addr)
			return
		}
		if v.Value != want {
			t.Errorf("group %d address %d: value = %v, want %v", baseGroup, addr, v.Value, want)
		}
		if v.ConnNumber != 7 {
			t.Errorf("group %d address %d: connection = %d, want 7", baseGroup, addr, v.ConnNumber)
		}
		// parity: the C++ driver writes variation 0 and cause 20 for every
		// supervised value (quirk Q4).
		if v.Variation != 0 || v.COT != 20 {
			t.Errorf("group %d address %d: variation/cot = %d/%d, want 0/20",
				baseGroup, addr, v.Variation, v.COT)
		}
		if fn != nil {
			fn(v)
		}
	}

	check(dnp3util.GroupBinaryInput, 0, 1, func(v Dnp3Value) {
		if v.ValueString != "true" {
			t.Errorf("binary 0: valueString = %q, want \"true\"", v.ValueString)
		}
		if v.Quality.Invalid() {
			t.Error("binary 0 should be valid")
		}
	})
	check(dnp3util.GroupBinaryInput, 1, 0, func(v Dnp3Value) {
		if !v.Quality.CommLost || !v.Quality.Invalid() || !v.Quality.NotTopical() {
			t.Errorf("binary 1: comm-lost not carried, got %+v", v.Quality)
		}
	})

	check(dnp3util.GroupDoubleBinaryInput, 0, 1, func(v Dnp3Value) {
		if v.Quality.Transient {
			t.Error("double-bit 0 is determined, not transient")
		}
	})
	check(dnp3util.GroupDoubleBinaryInput, 1, 0, func(v Dnp3Value) {
		if !v.Quality.Transient {
			t.Error("double-bit 1 is intermediate and must be transient")
		}
	})

	// A fractional analog proves the variation carried it: an integer variation
	// would deliver 123 here.
	check(dnp3util.GroupAnalogInput, 0, 123.5, nil)
	check(dnp3util.GroupAnalogInput, 1, -7.25, func(v Dnp3Value) {
		if !v.Quality.Overrange || !v.Quality.Overflow() {
			t.Errorf("analog 1: over-range not carried, got %+v", v.Quality)
		}
	})

	check(dnp3util.GroupCounter, 0, 4242, nil)
	check(dnp3util.GroupCounter, 1, 7, func(v Dnp3Value) {
		if !v.Quality.Rollover || !v.Quality.Carry() {
			t.Errorf("counter 1: rollover not carried, got %+v", v.Quality)
		}
	})

	// parity: frozen counters are filed under common address 23, the event
	// group, not the 21 the README's table names (quirk Q1).
	check(dnp3util.GroupFrozenCounterEvent, 0, 99, nil)
	if _, wrong := got[[2]int{dnp3util.GroupFrozenCounter, 0}]; wrong {
		t.Error("frozen counters must stay on common address 23 (quirk Q1)")
	}

	check(dnp3util.GroupBinaryOutputStatus, 0, 1, func(v Dnp3Value) {
		if !v.Quality.Substituted() {
			t.Error("binary output 0 is locally forced and must read as substituted")
		}
	})
	check(dnp3util.GroupAnalogOutputStatus, 0, 55.5, nil)

	// A static read carries no time: only the event variations have a
	// timestamp field. So an integrity poll leaves timeTagAtSource unset and
	// the server timestamp is all there is, which is what the C++ driver
	// produces too. The event path is covered by TestLoopbackEventTimestamp.
	for key, v := range got {
		if v.HasSourceTimestamp {
			t.Errorf("%v: a static read must not carry a source timestamp", key)
		}
		if v.ServerTimestamp == 0 {
			t.Errorf("%v: no server timestamp", key)
		}
	}
}

// TestLoopbackEventTimestamp checks the event path: a change reported as an
// event carries the outstation's timestamp into timeTagAtSource, and
// timeTagAtSourceOk follows the outstation's clock state.
//
// An outstation whose clock has never been set does not claim a synchronised
// timestamp, however the measurement was stored — the quality is a property of
// the device's clock, not of the reading. So the first phase must come back
// unsynchronised, and only after the master writes the time does the flag turn
// true. That the driver passes both through unchanged is the point.
func TestLoopbackEventTimestamp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, mch, och := seededOutstation(t)
	go func() { _ = out.Run(ctx, och) }()

	conn := &Connection{Name: "TEST", ProtocolConnectionNumber: 7}
	queue := NewValueQueue()

	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 3 * time.Second,
	}, newSOEHandler(conn, queue))
	go func() { _ = m.Run(ctx, mch) }()

	waitFor(t, "the master to connect", m.Connected)
	if err := m.IntegrityPoll(ctx); err != nil {
		t.Fatalf("IntegrityPoll: %v", err)
	}
	drainAll(queue)

	// raise changes a point and returns what the driver queued for it.
	raise := func(ms int64, value bool, analog float64) map[[2]int]Dnp3Value {
		t.Helper()
		out.Update(func(db *outstation.Database) {
			db.UpdateBinary(0, dnp3.Binary{
				Value: value, Flags: dnp3.Online, Time: dnp3.Now(time.UnixMilli(ms))})
			db.UpdateAnalog(0, dnp3.Analog{
				Value: analog, Flags: dnp3.Online, Time: dnp3.Now(time.UnixMilli(ms))})
		})
		if err := m.ScanClasses(ctx, dnp3.Class123); err != nil {
			t.Fatalf("ScanClasses: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
		got := map[[2]int]Dnp3Value{}
		for _, v := range drainAll(queue) {
			got[[2]int{v.BaseGroup, v.Address}] = v
		}
		return got
	}

	const firstMs = 1700000001000
	got := raise(firstMs, false, 321.75)

	bin, ok := got[[2]int{dnp3util.GroupBinaryInput, 0}]
	if !ok {
		t.Fatal("the binary change was not delivered as an event")
	}
	if bin.Value != 0 {
		t.Errorf("binary 0: value = %v, want 0", bin.Value)
	}
	if !bin.HasSourceTimestamp || bin.SourceTimestamp != firstMs {
		t.Errorf("binary 0: source timestamp = %d (present=%v), want %d",
			bin.SourceTimestamp, bin.HasSourceTimestamp, firstMs)
	}
	if bin.TimeTagOk {
		t.Error("binary 0: the outstation clock is unset, so the event must not be marked synchronised")
	}

	ana, ok := got[[2]int{dnp3util.GroupAnalogInput, 0}]
	if !ok {
		t.Fatal("the analog change was not delivered as an event")
	}
	if ana.Value != 321.75 {
		t.Errorf("analog 0: value = %v, want 321.75", ana.Value)
	}
	if !ana.HasSourceTimestamp || ana.SourceTimestamp != firstMs {
		t.Errorf("analog 0: source timestamp = %d (present=%v), want %d",
			ana.SourceTimestamp, ana.HasSourceTimestamp, firstMs)
	}

	// Set the outstation's clock, which is what the driver's time-sync loop
	// does, and the next event is reported as synchronised.
	if err := m.SyncTime(ctx); err != nil {
		t.Fatalf("SyncTime: %v", err)
	}

	const secondMs = 1700000002000
	got = raise(secondMs, true, 654.25)

	bin, ok = got[[2]int{dnp3util.GroupBinaryInput, 0}]
	if !ok {
		t.Fatal("the second binary change was not delivered as an event")
	}
	if bin.Value != 1 {
		t.Errorf("binary 0: value = %v, want 1", bin.Value)
	}
	if bin.SourceTimestamp != secondMs {
		t.Errorf("binary 0: source timestamp = %d, want %d", bin.SourceTimestamp, secondMs)
	}
	if !bin.TimeTagOk {
		t.Error("binary 0: after a clock sync the event must be marked synchronised")
	}
}

// drainAll takes everything queued without blocking on an empty queue.
func drainAll(q *ValueQueue) []Dnp3Value {
	q.mu.Lock()
	defer q.mu.Unlock()
	batch := q.values
	q.values = nil
	return batch
}
