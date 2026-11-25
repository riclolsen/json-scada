package main

// Mirrors ../cs_data_processor/alarmlist.test.js; the expected values must
// stay identical in both.

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestDigitalAlarmActive(t *testing.T) {
	cases := []struct {
		st   float64
		stOk bool
		v    float64
		want bool
	}{
		{0, true, 0, true},
		{0, true, 1, false},
		{1, true, 1, true},
		{1, true, 0, false},
		{2, true, 1, false},
		{-1, true, 0, false},
		{0, false, 1, false},
	}
	for _, c := range cases {
		if got := digitalAlarmActive(c.st, c.stOk, c.v); got != c.want {
			t.Errorf("digitalAlarmActive(%v, %v, %v) = %v", c.st, c.stOk, c.v, got)
		}
	}
}

func TestDigitalAlarmText(t *testing.T) {
	fd := limitsDoc(t, bson.D{{Key: "eventTextTrue", Value: "TRIP"}, {Key: "eventTextFalse", Value: ""}})
	if got := digitalAlarmText(fd, 1, "ON"); got != "TRIP" {
		t.Errorf("true text = %q", got)
	}
	if got := digitalAlarmText(fd, 0, "OFF"); got != "OFF" {
		t.Errorf("false text = %q", got)
	}
	if got := digitalAlarmText(limitsDoc(t, bson.D{}), 0, "OFF [IV]"); got != "OFF [IV]" {
		t.Errorf("no text = %q", got)
	}
}

var alarmT0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func alarmFd(t *testing.T, typ string) rawDoc {
	return limitsDoc(t, bson.D{
		{Key: "_id", Value: 202.0},
		{Key: "tag", Value: "KAW2~TR1~MW"},
		{Key: "type", Value: typ},
		{Key: "group1", Value: "KAW2"},
		{Key: "group2", Value: "TR1"},
		{Key: "description", Value: "KAW2~TR1~MW"},
		{Key: "priority", Value: 2.0},
	})
}

func TestAlarmDocumentAnalog(t *testing.T) {
	d := alarmDocument(alarmFd(t, "analog"), alarmFields{
		value: 61, valueString: "61 MW", alarmText: "61 MW ⤉ 🚩 HIHI",
		alarmRange: 2, active: true, timeTag: alarmT0,
	})
	keys := []string{
		"tag", "pointKey", "group1", "group2", "description", "type", "priority",
		"value", "valueString", "alarmText", "timeTag", "timeTagAtSource",
		"timeTagAtSourceOk", "active", "ack", "timeTagAck", "ackUser",
		"timeTagNormal", "alarmRange",
	}
	if len(d) != len(keys) {
		t.Fatalf("fields = %d, want %d", len(d), len(keys))
	}
	for i, k := range keys {
		if d[i].Key != k {
			t.Errorf("field %d = %s, want %s", i, d[i].Key, k)
		}
	}
	if v, _ := setField(d, "value"); v != 61.0 {
		t.Errorf("value = %v", v)
	}
	if v, _ := setField(d, "alarmRange"); v != 2.0 {
		t.Errorf("alarmRange = %v", v)
	}
	if v, _ := setField(d, "ack"); v != int32(0) {
		t.Errorf("ack = %v", v)
	}
	if v, _ := setField(d, "timeTagNormal"); v != nil {
		t.Errorf("timeTagNormal = %v", v)
	}
	if v, _ := setField(d, "timeTagAtSource"); v != nil {
		t.Errorf("timeTagAtSource = %v", v)
	}
}

func TestAlarmDocumentDigitalInactive(t *testing.T) {
	d := alarmDocument(alarmFd(t, "digital"), alarmFields{
		value: 1, valueString: "ON", alarmText: "ON", active: false, timeTag: alarmT0,
	})
	if _, ok := setField(d, "alarmRange"); ok {
		t.Error("alarmRange on a digital alarm")
	}
	if v, _ := setField(d, "timeTagNormal"); v != alarmT0 {
		t.Errorf("timeTagNormal = %v", v)
	}
}

func TestAlarmOpModels(t *testing.T) {
	if m := (alarmOp{pointKey: 5.0, now: alarmT0}).models(); len(m) != 0 {
		t.Errorf("no change: %d models", len(m))
	}
	m := (alarmOp{pointKey: 5.0, deactivate: true, doc: bson.D{{Key: "tag", Value: "x"}}, now: alarmT0}).models()
	if len(m) != 2 {
		t.Fatalf("models = %d, want 2", len(m))
	}
	if _, ok := m[0].(*mongo.UpdateManyModel); !ok {
		t.Errorf("first model %T, want an update", m[0])
	}
	if _, ok := m[1].(*mongo.InsertOneModel); !ok {
		t.Errorf("second model %T, want an insert", m[1])
	}
	m = (alarmOp{pointKey: 5.0, doc: bson.D{{Key: "tag", Value: "x"}}, now: alarmT0}).models()
	if _, ok := m[0].(*mongo.InsertOneModel); !ok || len(m) != 1 {
		t.Errorf("insert only: %v", m)
	}
}
