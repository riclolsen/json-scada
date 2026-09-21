package jsredundancy

import (
	"testing"
	"time"
)

// simulateStandby drives the REAL takeover rule over a sequence of keep-alive
// readings and reports after how many the standby took over, or -1.
//
// It calls stallCounter.observe rather than reimplementing it, so these tests
// cannot pass against a copy that has drifted from the loop.
func simulateStandby(readings []time.Time) (tookOverAfter int) {
	var s stallCounter
	for i, r := range readings {
		if s.observe(r) {
			return i + 1
		}
	}
	return -1
}

func TestTakeoverAfterUnchangedReadings(t *testing.T) {
	frozen := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// First reading seeds lastKeepAlive without counting, so it takes
	// KeepAliveCountLimit+1 further identical readings to trip.
	readings := make([]time.Time, 10)
	for i := range readings {
		readings[i] = frozen
	}
	got := simulateStandby(readings)
	want := KeepAliveCountLimit + 2 // seed + (limit+1) counted readings
	if got != want {
		t.Errorf("took over after %d readings, want %d", got, want)
	}

	// At TickPeriod per reading that is the documented latency.
	latency := time.Duration(got) * TickPeriod
	if latency < 25*time.Second || latency > 35*time.Second {
		t.Errorf("takeover latency %v, expected roughly 25-30 s", latency)
	}
}

func TestNoTakeoverWhileKeepAliveAdvances(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	readings := make([]time.Time, 50)
	for i := range readings {
		readings[i] = base.Add(time.Duration(i) * TickPeriod)
	}
	if got := simulateStandby(readings); got != -1 {
		t.Errorf("took over after %d readings; a live active node must never be displaced", got)
	}
}

// The fix to the inherited Redundancy.cs quirk: the counter resets whenever
// the keep-alive advances, so separate short stalls no longer accumulate. An
// active node that stalls a cycle now and then is never displaced.
func TestStallsMustBeConsecutive(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var readings []time.Time
	for cycle := 0; cycle < 50; cycle++ {
		stamp := base.Add(time.Duration(cycle) * time.Minute)
		// KeepAliveCountLimit unchanged readings — one short of tripping —
		// then a fresh one. Under the old rule these accumulated and forced a
		// takeover; now each recovery wipes the count.
		for i := 0; i < KeepAliveCountLimit; i++ {
			readings = append(readings, stamp)
		}
		readings = append(readings, stamp.Add(time.Second))
	}
	if got := simulateStandby(readings); got != -1 {
		t.Errorf("took over after %d readings; a stall that recovers must not count towards the next one", got)
	}
}

// The reset must not slow down replacing a node that is genuinely gone: a
// dead node's keep-alive never advances, so the count is never reset.
func TestDeadNodeStillReplacedAtTheSameSpeed(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// A few healthy cycles, then the active node stops writing.
	var readings []time.Time
	for i := 0; i < 5; i++ {
		readings = append(readings, base.Add(time.Duration(i)*TickPeriod))
	}
	frozen := readings[len(readings)-1]
	for i := 0; i < 10; i++ {
		readings = append(readings, frozen)
	}

	got := simulateStandby(readings)
	if got == -1 {
		t.Fatal("a dead active node must be replaced")
	}
	// 5 healthy readings, then KeepAliveCountLimit+1 unchanged ones to trip.
	want := 5 + KeepAliveCountLimit + 1
	if got != want {
		t.Errorf("took over after %d readings, want %d", got, want)
	}
}

// The rule must not consult the local clock: a keep-alive far in the past or
// the future is irrelevant as long as it keeps changing. This is what makes
// the arbitration immune to clock skew between nodes.
func TestTakeoverIgnoresTheLocalClock(t *testing.T) {
	for _, skew := range []time.Duration{-100 * time.Hour, 100 * time.Hour} {
		base := time.Now().Add(skew)
		readings := make([]time.Time, 50)
		for i := range readings {
			readings[i] = base.Add(time.Duration(i) * TickPeriod)
		}
		if got := simulateStandby(readings); got != -1 {
			t.Errorf("skew %v: took over after %d readings; the local clock must not matter",
				skew, got)
		}
	}
}

func TestSetActiveOnlyFiresOnTransition(t *testing.T) {
	var activations, deactivations int
	c := &Controller{
		OnActivate:   func() { activations++ },
		OnDeactivate: func() { deactivations++ },
	}

	c.setActive(true, "up")
	c.setActive(true, "up")
	if activations != 1 {
		t.Errorf("OnActivate called %d times, want 1", activations)
	}
	if !c.Active() {
		t.Error("Active() should report true")
	}

	c.setActive(false, "down")
	c.setActive(false, "down")
	if deactivations != 1 {
		t.Errorf("OnDeactivate called %d times, want 1", deactivations)
	}
	if c.Active() {
		t.Error("Active() should report false")
	}
}

// A commands-only driver supplies no callbacks; flipping the flag must not
// panic on the nil funcs.
func TestSetActiveWithoutCallbacks(t *testing.T) {
	c := &Controller{}
	c.setActive(true, "up")
	c.setActive(false, "down")
	if c.Active() {
		t.Error("expected inactive")
	}
}

func TestMissingInstanceDefaultsInactive(t *testing.T) {
	var stalls stallCounter
	c := &Controller{}
	c.setActive(true, "up") // pretend it had taken over
	c.noInstance(t.Context(), &stalls)
	if c.Active() {
		t.Error("with no instance document the node must stand down")
	}
}

// dnp3-go keeps the C++ bootstrap behaviour so a single-node system with an
// incomplete configuration still runs.
func TestMissingInstanceActiveOptIn(t *testing.T) {
	var stalls stallCounter
	c := &Controller{MissingInstanceActive: true}
	c.noInstance(t.Context(), &stalls)
	if !c.Active() {
		t.Error("MissingInstanceActive must keep the node running")
	}
}

func TestForceActive(t *testing.T) {
	c := &Controller{}
	c.ForceActive(true)
	if !c.Active() {
		t.Error("ForceActive(true) should set the flag")
	}
	c.ForceActive(false)
	if c.Active() {
		t.Error("ForceActive(false) should clear the flag")
	}
}

func TestYieldDelayInRange(t *testing.T) {
	for i := 0; i < 200; i++ {
		d := yieldDelay()
		if d < time.Second || d >= 5*time.Second {
			t.Fatalf("yieldDelay = %v, want [1s, 5s)", d)
		}
	}
}
