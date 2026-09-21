package serverapp

import (
	"testing"

	dnp3 "github.com/dscsystems/go-dnp3"
)

// TestFamilyOf checks that each common address lands in the point family the
// C++ server's switch puts it in, including the default-to-analog fall-through.
func TestFamilyOf(t *testing.T) {
	cases := map[int]family{
		1: famBinary, 2: famBinary,
		3: famDoubleBit, 4: famDoubleBit,
		20: famCounter, 22: famCounter,
		21: famFrozenCounter, 23: famFrozenCounter,
		10: famBinaryOutputStatus, 11: famBinaryOutputStatus,
		30: famAnalog, 32: famAnalog,
		40: famAnalogOutputStatus, 42: famAnalogOutputStatus,
		110: famOctetString, 111: famOctetString,
		50: famTimeAndInterval, 52: famTimeAndInterval,
		// parity: anything unrecognised is distributed as an analog input.
		99: famAnalog, 0: famAnalog,
	}
	for addr, want := range cases {
		if got := familyOf(addr); got != want {
			t.Errorf("familyOf(%d) = %v, want %v", addr, got, want)
		}
	}
}

// TestVariationsFor checks the ASDU switch tables of DefineGroupVar().
func TestVariationsFor(t *testing.T) {
	cases := []struct {
		fam    family
		asdu   int
		static uint8
		event  uint8
	}{
		// Analog input: g30/g32.
		{famAnalog, 1, 1, 1},
		{famAnalog, 2, 2, 2},
		{famAnalog, 3, 3, 3},
		{famAnalog, 4, 4, 4},
		{famAnalog, 5, 5, 5},
		{famAnalog, 6, 6, 6},
		{famAnalog, 7, 5, 7},
		{famAnalog, 8, 6, 8},
		{famAnalog, 0, 5, 5},  // default
		{famAnalog, 99, 5, 5}, // default

		// Counter: g20/g22.
		{famCounter, 1, 1, 5},
		{famCounter, 2, 2, 6},
		{famCounter, 3, 1, 5},
		{famCounter, 4, 2, 6},
		{famCounter, 5, 5, 5},
		{famCounter, 6, 6, 6},
		{famCounter, 7, 5, 5},
		{famCounter, 8, 6, 6},
		{famCounter, 0, 1, 5}, // default

		// Frozen counter: g21/g23, which also accepts the delta and no-flag
		// variations 9 to 12.
		{famFrozenCounter, 1, 1, 5},
		{famFrozenCounter, 2, 2, 6},
		{famFrozenCounter, 3, 1, 5},
		{famFrozenCounter, 4, 2, 6},
		{famFrozenCounter, 5, 5, 5},
		{famFrozenCounter, 6, 6, 6},
		{famFrozenCounter, 7, 5, 5},
		{famFrozenCounter, 8, 6, 6},
		{famFrozenCounter, 9, 1, 5},
		{famFrozenCounter, 10, 2, 6},
		{famFrozenCounter, 11, 1, 5},
		{famFrozenCounter, 12, 2, 6},

		// Analog output status: g40/g42.
		{famAnalogOutputStatus, 1, 1, 3},
		{famAnalogOutputStatus, 2, 2, 4},
		{famAnalogOutputStatus, 3, 3, 7},
		{famAnalogOutputStatus, 4, 4, 8},
		{famAnalogOutputStatus, 0, 3, 7}, // default

		// The single-variation families ignore the ASDU.
		{famBinary, 0, 2, 2},
		{famBinary, 7, 2, 2},
		{famDoubleBit, 3, 2, 2},
		{famBinaryOutputStatus, 9, 2, 2},
		{famOctetString, 4, 0, 0},
	}

	for _, c := range cases {
		got := variationsFor(c.fam, c.asdu)
		if got.static != c.static || got.event != c.event {
			t.Errorf("variationsFor(%v, %d) = %d/%d, want %d/%d",
				c.fam, c.asdu, got.static, got.event, c.static, c.event)
		}
	}
}

// TestDefaultVariationsAreFloat pins the single most consequential default: the
// library's own analog default is a 32-bit integer variation, which would
// truncate every fractional measurement. The C++ server uses g30v5, single
// precision, and so must this one.
func TestDefaultVariationsAreFloat(t *testing.T) {
	if v := defaultVariations[famAnalog]; v.static != 5 || v.event != 7 {
		t.Errorf("analog defaults = %d/%d, want 5/7 (g30v5, g32v7)", v.static, v.event)
	}
	if v := defaultVariations[famAnalogOutputStatus]; v.static != 3 || v.event != 7 {
		t.Errorf("analog output defaults = %d/%d, want 3/7 (g40v3, g42v7)", v.static, v.event)
	}
}

// TestDefaultClasses checks the event classes the C++ server assigns per family.
func TestDefaultClasses(t *testing.T) {
	want := map[family]dnp3.Class{
		famBinary:             dnp3.Class1,
		famDoubleBit:          dnp3.Class2,
		famAnalog:             dnp3.Class2,
		famCounter:            dnp3.Class2,
		famFrozenCounter:      dnp3.Class3,
		famBinaryOutputStatus: dnp3.Class2,
		famAnalogOutputStatus: dnp3.Class2,
		famOctetString:        dnp3.Class3,
	}
	for fam, w := range want {
		if got := defaultClasses[fam]; got != w {
			t.Errorf("class of %v = %v, want %v", fam, got, w)
		}
	}
	// A zero class is ClassNone, which silently suppresses every event of the
	// point; no family may be left unassigned.
	for fam := famBinary; fam < famTimeAndInterval; fam++ {
		if _, ok := defaultClasses[fam]; !ok {
			t.Errorf("family %v has no default class", fam)
		}
	}
}
