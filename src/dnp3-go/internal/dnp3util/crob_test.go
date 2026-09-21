package dnp3util

import (
	"testing"

	dnp3 "github.com/dscsystems/go-dnp3"
)

// TestCROBFor checks every documented protocolSourceCommandDuration against the
// switch of the C++ client's executeCommand().
func TestCROBFor(t *testing.T) {
	const on, off = 1.0, 0.0

	cases := []struct {
		duration int
		value    float64
		code     dnp3.ControlCode
		pulsed   bool
	}{
		// PULSE 1=ON 0=OFF
		{1, on, dnp3.ControlPulseOn, true},
		{1, off, dnp3.ControlPulseOff, true},
		// PULSE 0=ON 1=OFF
		{2, on, dnp3.ControlPulseOff, true},
		{2, off, dnp3.ControlPulseOn, true},
		// LATCH 1=ON 0=OFF
		{3, on, dnp3.ControlLatchOn, false},
		{3, off, dnp3.ControlLatchOff, false},
		// LATCH 0=ON 1=OFF
		{4, on, dnp3.ControlLatchOff, false},
		{4, off, dnp3.ControlLatchOn, false},
		// PULSE 1=ON,CLOSE 0=OFF,TRIP
		{11, on, dnp3.ControlPulseOn | dnp3.ControlClose, true},
		{11, off, dnp3.ControlPulseOff | dnp3.ControlTrip, true},
		// LATCH 1=ON,CLOSE 0=OFF,TRIP
		{13, on, dnp3.ControlLatchOn | dnp3.ControlClose, false},
		{13, off, dnp3.ControlLatchOff | dnp3.ControlTrip, false},
		// PULSE 1=ON,TRIP 0=OFF,CLOSE
		{21, on, dnp3.ControlPulseOn | dnp3.ControlTrip, true},
		{21, off, dnp3.ControlPulseOff | dnp3.ControlClose, true},
		// LATCH 1=ON,TRIP 0=OFF,CLOSE
		{23, on, dnp3.ControlLatchOn | dnp3.ControlTrip, false},
		{23, off, dnp3.ControlLatchOff | dnp3.ControlClose, false},
	}

	for _, c := range cases {
		got := CROBFor(c.duration, c.value)
		if got.Code != c.code {
			t.Errorf("duration %d value %v: code = %s, want %s",
				c.duration, c.value, got.Code, c.code)
		}
		if got.Count != 1 {
			t.Errorf("duration %d value %v: count = %d, want 1", c.duration, c.value, got.Count)
		}
		wantOn, wantOff := uint32(0), uint32(0)
		if c.pulsed {
			wantOn, wantOff = CROBPulseOnTime, CROBPulseOffTime
		}
		if got.OnTime != wantOn || got.OffTime != wantOff {
			t.Errorf("duration %d value %v: on/off = %d/%d, want %d/%d",
				c.duration, c.value, got.OnTime, got.OffTime, wantOn, wantOff)
		}
	}
}

// TestCROBForUnimplemented pins quirk Q3: durations 10, 12, 20 and 22 are in
// the driver README's table but were never implemented by the C++ switch, and
// fall through to a block that operates nothing. Reproduced deliberately — a
// guess here would operate the wrong coil of a breaker.
func TestCROBForUnimplemented(t *testing.T) {
	for _, duration := range []int{0, 10, 12, 20, 22, 5, 99} {
		for _, value := range []float64{0, 1} {
			got := CROBFor(duration, value)
			if got.Code != dnp3.ControlNUL {
				t.Errorf("duration %d value %v: code = %s, want NUL", duration, value, got.Code)
			}
			if got.OnTime != 0 || got.OffTime != 0 {
				t.Errorf("duration %d value %v: expected no pulse times", duration, value)
			}
		}
	}
}
