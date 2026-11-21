package main

// Mirrors ../cs_data_processor/retention.test.js; the expected values must
// stay identical in both.

import (
	"math"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestRetentionDays(t *testing.T) {
	cases := []struct {
		v    float64
		ok   bool
		want int
	}{
		{0, false, 60}, // missing / null / a string
		{0, true, 60},
		{-5, true, 60},
		{math.NaN(), true, 60},
		{1, true, 1},
		{30, true, 30},
		{7.9, true, 7},
		{100000, true, 36500},
	}
	for _, c := range cases {
		if got := retentionDays(c.v, c.ok); got != c.want {
			t.Errorf("retentionDays(%v, %v) = %d, want %d", c.v, c.ok, got, c.want)
		}
	}
}

func TestRetentionSettings(t *testing.T) {
	if got := retentionSettings(rawDoc{}); got != (retentionConfig{60, 60, 60}) {
		t.Errorf("no document: %+v", got)
	}
	raw, _ := bson.Marshal(bson.D{
		{Key: "key", Value: "global"},
		{Key: "soeRetentionDays", Value: int32(10)},
		{Key: "alarmsRetentionDays", Value: 400.0},
		{Key: "histRetentionDays", Value: int64(2)},
	})
	if got := retentionSettings(rawDoc{raw: raw}); got != (retentionConfig{10, 400, 2}) {
		t.Errorf("settings: %+v", got)
	}
	raw, _ = bson.Marshal(bson.D{{Key: "soeRetentionDays", Value: "30"}})
	if got := retentionSettings(rawDoc{raw: raw}); got.soe != 60 {
		t.Errorf("string retention: %+v", got)
	}
}

func TestRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if got := retentionCutoff(now, 60).Format(time.RFC3339); got != "2025-12-31T12:00:00Z" {
		t.Errorf("cutoff = %s", got)
	}
}

func TestAlarmsCleanupFilter(t *testing.T) {
	c := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	want := bson.D{
		{Key: "timeTag", Value: bson.D{{Key: "$lt", Value: c}}},
		{Key: "ack", Value: bson.D{{Key: "$ne", Value: 0}}},
		{Key: "active", Value: bson.D{{Key: "$ne", Value: true}}},
	}
	if got := alarmsCleanupFilter(c); !reflect.DeepEqual(got, want) {
		t.Errorf("filter = %v", got)
	}
}
