package jstags

import (
	"math"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// The base document is copied into a driver's own literal, so it must hand
// back a fresh map every time — a shared one would let one driver's overrides
// leak into another's tags.
func TestBaseDocIsAFreshMap(t *testing.T) {
	a := BaseDoc()
	a["unit"] = "MUTATED"
	if BaseDoc()["unit"] != "" {
		t.Fatal("BaseDoc must not return a shared map")
	}
}

// Pins the exact field set. Adding a field here changes every driver's stored
// tag documents at once, so it has to be deliberate.
func TestBaseDocFieldSet(t *testing.T) {
	want := map[string]any{
		"alarmDisabled":        false,
		"alerted":              false,
		"alarmed":              false,
		"alertState":           "",
		"annotation":           "",
		"commandBlocked":       false,
		"commissioningRemarks": "",
		"formula":              0.0,
		"frozen":               false,
		"frozenDetectTimeout":  0.0,
		"hiLimit":              math.MaxFloat64,
		"hihiLimit":            math.MaxFloat64,
		"hihihiLimit":          math.MaxFloat64,
		"historianDeadBand":    0.0,
		"historianPeriod":      0.0,
		"hysteresis":           0.0,
		"isEvent":              false,
		"kconv1":               1.0,
		"kconv2":               0.0,
		"loLimit":              -math.MaxFloat64,
		"location":             nil,
		"loloLimit":            -math.MaxFloat64,
		"lololoLimit":          -math.MaxFloat64,
		"notes":                "",
		"overflow":             false,
		"parcels":              nil,
		"priority":             0.0,
		"sourceDataUpdate":     nil,
		"substituted":          false,
		"timeTag":              nil,
		"timeTagAlarm":         nil,
		"timeTagAtSource":      nil,
		"timeTagAtSourceOk":    false,
		"transient":            false,
		"unit":                 "",
		"updatesCnt":           0.0,
		"valueDefault":         0.0,
		"zeroDeadband":         0.0,
	}
	got := BaseDoc()
	for k, v := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("missing %q", k)
			continue
		}
		if g != v {
			t.Errorf("%s = %v (%T), want %v (%T)", k, g, g, v, v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected field %q — adding one changes every driver", k)
		}
	}
}

// These fields differ between drivers and must stay out, or a driver that
// never set them would start writing someone else's default.
func TestBaseDocExcludesDriverSpecificDefaults(t *testing.T) {
	doc := BaseDoc()
	for _, k := range []string{
		"invalid", "invalidDetectTimeout", "protocolDestinations",
		"value", "valueString", "valueJson", "type", "alarmState",
		"stateTextFalse", "stateTextTrue", "eventTextFalse", "eventTextTrue",
		"tag", "description", "group1", "origin", "_id",
	} {
		if _, ok := doc[k]; ok {
			t.Errorf("%q must be set by the driver, not by BaseDoc", k)
		}
	}
}

// Numbers must be float64: the collections store them as BSON doubles, and an
// int would change the stored type.
func TestBaseDocNumbersAreFloat64(t *testing.T) {
	for k, v := range BaseDoc() {
		switch v.(type) {
		case int, int32, int64:
			t.Errorf("%s is %T, want float64", k, v)
		}
	}
}

func TestKeyAllocatorPartition(t *testing.T) {
	var k KeyAllocator
	if k.Base(3) != 3000000 {
		t.Errorf("Base(3) = %v", k.Base(3))
	}
	if k.Top(3) != 4000000 {
		t.Errorf("Top(3) = %v", k.Top(3))
	}
	custom := KeyAllocator{Multiplier: 100}
	if custom.Base(2) != 200 || custom.Top(2) != 300 {
		t.Errorf("custom multiplier: base=%v top=%v", custom.Base(2), custom.Top(2))
	}
}

// Without a database the lookup fails and the range starts at its base, then
// increments — the behaviour every driver had.
func TestKeyAllocatorIncrementsAfterSeed(t *testing.T) {
	var k KeyAllocator
	k.last = 3000000 // as if seeded
	if got := k.Next(t.Context(), nil, 3); got != 3000001 {
		t.Errorf("Next = %v, want 3000001", got)
	}
	if got := k.Next(t.Context(), nil, 3); got != 3000002 {
		t.Errorf("Next = %v, want 3000002", got)
	}
}

func TestKeyAllocatorReset(t *testing.T) {
	var k KeyAllocator
	k.last = 3000005
	k.Reset()
	if k.last != 0 {
		t.Errorf("last = %v after Reset, want 0", k.last)
	}
}

var _ = bson.M{}
