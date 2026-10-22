package serverapp

import (
	"fmt"
	"testing"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/outstation"
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
		// Analog input: g30/g32. The event is always a timed variation
		// (deviation D27).
		{famAnalog, 1, 1, 3},
		{famAnalog, 2, 2, 4},
		{famAnalog, 3, 3, 3},
		{famAnalog, 4, 4, 4},
		{famAnalog, 5, 5, 7},
		{famAnalog, 6, 6, 8},
		{famAnalog, 7, 5, 7},
		{famAnalog, 8, 6, 8},
		{famAnalog, 0, 5, 7},  // default
		{famAnalog, 99, 5, 7}, // default

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

// timedEvents lists, per family, the event variations that carry an absolute
// time: g2v2, g4v2, g11v2, g22v5/6, g23v5/6, g32v3/4/7/8 and g42v3/4/7/8.
var timedEvents = map[family][]uint8{
	famBinary:             {2},
	famDoubleBit:          {2},
	famBinaryOutputStatus: {2},
	famCounter:            {5, 6},
	famFrozenCounter:      {5, 6},
	famAnalog:             {3, 4, 7, 8},
	famAnalogOutputStatus: {3, 4, 7, 8},
}

// TestEveryEventCarriesTime checks that no ASDU selects an event variation
// without time, for the defaults and for every per-point choice, so a changed
// value always reaches the master with the moment it changed.
func TestEveryEventCarriesTime(t *testing.T) {
	for fam, allowed := range timedEvents {
		check := func(what string, ev uint8) {
			for _, a := range allowed {
				if ev == a {
					return
				}
			}
			t.Errorf("family %d %s: event variation %d carries no time", fam, what, ev)
		}
		check("default", defaultVariations[fam].event)
		for asdu := 0; asdu <= 20; asdu++ {
			check(fmt.Sprintf("ASDU %d", asdu), variationsFor(fam, asdu).event)
		}
	}
}

// TestEveryVariationIsAccepted checks that go-dnp3 takes the configuration the
// driver builds for every family and every ASDU. Database.Configure refuses an
// event variation the library cannot encode, and a refused point silently keeps
// its default, so a table entry the library does not know would otherwise show
// up only as a master reading a different variation than the one configured.
func TestEveryVariationIsAccepted(t *testing.T) {
	db := outstation.NewDatabase(outstation.DatabaseConfig{
		Binary: 1, DoubleBitBinary: 1, Counter: 1, FrozenCounter: 1, Analog: 1,
		BinaryOutputStatus: 1, AnalogOutputStatus: 1,
	}, outstation.NewEventBuffer(outstation.EventBufferConfig{}))

	for f := famBinary; f <= famAnalogOutputStatus; f++ {
		pt, ok := f.pointType()
		if !ok {
			continue
		}
		check := func(what string, vp variationPair) {
			cfg := outstation.PointConfig{Class: dnp3.Class2, StaticVariation: vp.static, EventVariation: vp.event}
			if !db.Configure(pt, 0, cfg) {
				t.Errorf("family %d %s: static v%d / event v%d refused by go-dnp3", f, what, vp.static, vp.event)
			}
		}
		check("default", defaultVariations[f])
		for asdu := 0; asdu <= 20; asdu++ {
			check(fmt.Sprintf("ASDU %d", asdu), variationsFor(f, asdu))
		}
	}
}
