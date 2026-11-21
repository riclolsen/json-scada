package main

// Mirrors ../cs_data_processor/limits.test.js; the expected values must stay
// identical in both.

import (
	"math"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func limitsDoc(t *testing.T, d bson.D) rawDoc {
	t.Helper()
	raw, err := bson.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return rawDoc{raw: raw}
}

var allLimits = bson.D{
	{Key: "lololoLimit", Value: -30.0},
	{Key: "loloLimit", Value: -20.0},
	{Key: "loLimit", Value: -10.0},
	{Key: "hiLimit", Value: 10.0},
	{Key: "hihiLimit", Value: 20.0},
	{Key: "hihihiLimit", Value: 30.0},
}

func TestAlarmRangeOf(t *testing.T) {
	hihiOnly := bson.D{{Key: "hihiLimit", Value: 20.0}}
	nullLevels := bson.D{{Key: "hiLimit", Value: nil}, {Key: "loLimit", Value: nil}, {Key: "hihiLimit", Value: 20.0}}
	firstLevel := bson.D{{Key: "hiLimit", Value: 10.0}, {Key: "loLimit", Value: -10.0}}
	cases := []struct {
		limits    bson.D
		value     float64
		hyst      float64
		prev      any
		rng       int
		normal    bool
		escalated bool
	}{
		{allLimits, 5, 1, 0.0, 0, true, false},
		{allLimits, 11.5, 1, 0.0, 1, false, true},
		{allLimits, 10.5, 1, 0.0, 0, false, false},
		{allLimits, 10.5, 1, 1.0, 1, false, false},
		{allLimits, 9.5, 1, 1.0, 1, false, false},
		{allLimits, 8.5, 1, 1.0, 0, true, false},
		{allLimits, 21.5, 1, 1.0, 2, false, true},
		{allLimits, 20.5, 1, 1.0, 1, false, false},
		{allLimits, 19.5, 1, 2.0, 2, false, false},
		{allLimits, 18.5, 1, 2.0, 1, false, false},
		{allLimits, 35, 1, 0.0, 3, false, true},
		{allLimits, 35, 1, 2.0, 3, false, true},
		{allLimits, -11.5, 1, 0.0, -1, false, true},
		{allLimits, -21.5, 1, -1.0, -2, false, true},
		{allLimits, -19.5, 1, -2.0, -2, false, false},
		{allLimits, -18.5, 1, -2.0, -1, false, false},
		{allLimits, -11.5, 1, 2.0, -1, false, true},
		{allLimits, -10.5, 1, 1.0, 0, true, false},
		{allLimits, 5, 1, -3.0, 0, true, false},
		{allLimits, 35, 1, 7.0, 3, false, false},
		{allLimits, 10, 0, 0.0, 0, false, false},
		{allLimits, 10, 0, 1.0, 1, false, false},
		{hihiOnly, 15, 1, 0.0, 0, true, false},
		{hihiOnly, 25, 1, 0.0, 2, false, true},
		{nullLevels, -100, 1, 0.0, 0, true, false},
		{firstLevel, 50, 1, 0.0, 1, false, true},
		{firstLevel, 50, 1, nil, 1, false, true},
	}
	for _, c := range cases {
		d := append(bson.D{}, c.limits...)
		d = append(d, bson.E{Key: "alarmRange", Value: c.prev})
		got := alarmRangeOf(limitsDoc(t, d), c.value, c.hyst)
		want := alarmRangeResult{rng: c.rng, normal: c.normal, escalated: c.escalated}
		if got != want {
			t.Errorf("limits %v value %v hyst %v prev %v: got %+v, want %+v",
				c.limits, c.value, c.hyst, c.prev, got, want)
		}
	}
}

func TestHasLimits(t *testing.T) {
	cases := []struct {
		d    bson.D
		want bool
	}{
		{bson.D{}, false},
		{bson.D{{Key: "hiLimit", Value: nil}, {Key: "loLimit", Value: nil}}, false},
		{bson.D{{Key: "hiLimit", Value: math.NaN()}}, false},
		{bson.D{{Key: "lololoLimit", Value: -5.0}}, true},
		{bson.D{{Key: "hiLimit", Value: math.Inf(1)}}, true},
	}
	for _, c := range cases {
		if got := hasLimits(limitsDoc(t, c.d)); got != c.want {
			t.Errorf("hasLimits(%v) = %v, want %v", c.d, got, c.want)
		}
	}
}

func TestRangeLabel(t *testing.T) {
	want := map[int]string{-3: "LOLOLO", -2: "LOLO", -1: "LO", 0: "", 1: "HI", 2: "HIHI", 3: "HIHIHI"}
	for r, w := range want {
		if got := rangeLabel(r); got != w {
			t.Errorf("rangeLabel(%d) = %q, want %q", r, got, w)
		}
	}
}
