package main

import (
	"testing"
	"time"
)

func TestCsSanitize(t *testing.T) {
	cases := map[string]string{
		"KAW2":           "KAW2",
		" RTU 1 / main":  "RTU_1___main",
		"IEC60870-5-104": "IEC60870-5-104",
		"a.b":            "a_b",
		"":               "_",
	}
	for in, want := range cases {
		if got := csSanitize(in); got != want {
			t.Errorf("csSanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCsStallObserve(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var s csStall

	if alive, _ := s.observe(time.Time{}, false, now); alive {
		t.Fatal("no keep-alive must not be alive")
	}
	ka := now.Add(-2 * time.Second)
	if alive, age := s.observe(ka, true, now); !alive || age != 2 {
		t.Fatalf("fresh first reading: alive=%v age=%v", alive, age)
	}
	// unchanged readings up to the limit stay alive
	for i := 1; i <= commStatusStallLimit; i++ {
		now = now.Add(commStatusCycle)
		if alive, _ := s.observe(ka, true, now); !alive {
			t.Fatalf("stopped too early at reading %d", i)
		}
	}
	now = now.Add(commStatusCycle)
	if alive, _ := s.observe(ka, true, now); alive {
		t.Fatal("must be stopped after the stall limit")
	}
	// an advance revives it immediately, whatever the clocks say
	ka = ka.Add(-time.Hour) // skewed remote clock, still a change
	if alive, age := s.observe(ka, true, now); !alive || age != 0 {
		t.Fatalf("advance must revive: alive=%v age=%v", alive, age)
	}
}

func TestCsStallStaleAtStart(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var s csStall
	ka := now.Add(-10 * time.Minute)
	if alive, _ := s.observe(ka, true, now); alive {
		t.Fatal("keep-alive stale at start must be stopped")
	}
	if alive, _ := s.observe(ka, true, now.Add(commStatusCycle)); alive {
		t.Fatal("still stopped while unchanged")
	}
}

func pointsByTag(pts []csPoint) map[string]csPoint {
	m := map[string]csPoint{}
	for _, p := range pts {
		m[p.Spec.Tag] = p
	}
	return m
}

func TestCsBuildStatus(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	up, down := true, false
	snap := csSnapshot{
		Instances: []csInstance{
			{ID: "i1", Driver: "DNP3", Number: 1, Enabled: true, ActiveNode: "mainNode",
				Version: "0.8.2", KeepAlive: now.Add(-time.Second), HasKeepAlive: true},
			{ID: "i2", Driver: "OPC-UA", Number: 1, Enabled: true,
				KeepAlive: now.Add(-time.Hour), HasKeepAlive: true},
		},
		Connections: []csConnection{
			{ID: "c1", Number: 10, Name: "RTU 1", Driver: "DNP3", InstNumber: 1, Enabled: true,
				IsConnected: &up, NumStats: map[string]float64{"numBytesRx": 5}},
			{ID: "c2", Number: 11, Name: "RTU2", Driver: "DNP3", InstNumber: 1, Enabled: true,
				IsConnected: &down, NumStats: map[string]float64{}},
			{ID: "c3", Number: 12, Name: "PLC", Driver: "OPC-UA", InstNumber: 1, Enabled: true,
				NumStats: map[string]float64{}},
			{ID: "c4", Number: 13, Name: "ALL_BAD", Driver: "DNP3", InstNumber: 1, Enabled: true,
				NumStats: map[string]float64{}},
			{ID: "c5", Number: 14, Name: "NoDriver", Driver: "MODBUS", InstNumber: 1, Enabled: true,
				NumStats: map[string]float64{}},
		},
		Counts: map[float64]csTagCount{
			10: {Total: 4, Invalid: 1},
			13: {Total: 2, Invalid: 2},
		},
	}
	cs := newCommStatus(Config{Instance: 1})
	pts := pointsByTag(cs.build(snap, true, now))

	expect := func(tag string, v float64, invalid bool) {
		t.Helper()
		p, ok := pts[tag]
		if !ok {
			t.Fatalf("missing %s", tag)
		}
		if p.Val.Value != v || p.Val.Invalid != invalid {
			t.Errorf("%s = %v invalid=%v, want %v invalid=%v", tag, p.Val.Value, p.Val.Invalid, v, invalid)
		}
	}
	expect("_System.Driver.DNP3.1.Alive", 1, false)
	expect("_System.Driver.OPC-UA.1.Alive", 0, false)
	expect("_System.Conn.RTU_1.Status", 1, false)
	expect("_System.Conn.RTU_1.Health", 75, false)
	expect("_System.Conn.RTU_1.LinkUp", 1, false)
	expect("_System.Conn.RTU_1.Stats.numBytesRx", 5, false)
	expect("_System.Conn.RTU2.Status", 0, false)    // link down
	expect("_System.Conn.PLC.Status", 0, false)     // driver stopped
	expect("_System.Conn.PLC.LinkUp", 0, true)      // not reported
	expect("_System.Conn.ALL_BAD.Status", 0, false) // every tag invalid
	expect("_System.Conn.NoDriver.Status", 0, false)
	expect("_System.Conn.RTU2.Health", 0, true) // no tags
	expect("_System.Comm.DriversDown", 1, false)
	expect("_System.Comm.ConnectionsDown", 4, false)
	expect("_System.Comm.ConnectionsTotal", 5, false)

	if p := pts["_System.Driver.DNP3.1.ActiveNode"]; p.Val.Str != "mainNode" || p.Spec.Type != "string" {
		t.Errorf("ActiveNode = %+v", p)
	}
	if p := pts["_System.Conn.RTU_1.Status"]; p.Spec.Group3 != "RTU 1" || !p.Spec.Alarm {
		t.Errorf("Status spec = %+v", p.Spec)
	}

	// without mirroring there are no Stats points
	cs2 := newCommStatus(Config{Instance: 1})
	if _, ok := pointsByTag(cs2.build(snap, false, now))["_System.Conn.RTU_1.Stats.numBytesRx"]; ok {
		t.Error("stats mirrored while disabled")
	}
}

func TestCsBuildKeepsCountsBetweenRefreshes(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cs := newCommStatus(Config{Instance: 1})
	snap := csSnapshot{
		Connections: []csConnection{{ID: "c1", Number: 10, Name: "A", NumStats: map[string]float64{}}},
		Counts:      map[float64]csTagCount{10: {Total: 10, Invalid: 5}},
	}
	cs.build(snap, false, now)
	snap.Counts = nil // a cycle without the aggregation
	pts := pointsByTag(cs.build(snap, false, now.Add(commStatusCycle)))
	if p := pts["_System.Conn.A.Health"]; p.Val.Value != 50 || p.Val.Invalid {
		t.Errorf("Health = %+v, want cached 50", p.Val)
	}
}

func TestCsChanged(t *testing.T) {
	analog := csSpec{Type: "analog", Deadband: 5}
	if csChanged(analog, csValue{Value: 1}, csValue{Value: 4}) {
		t.Error("within dead band must not change")
	}
	if !csChanged(analog, csValue{Value: 1}, csValue{Value: 6}) {
		t.Error("outside dead band must change")
	}
	if !csChanged(analog, csValue{Value: 1}, csValue{Value: 1, Invalid: true}) {
		t.Error("quality change must change")
	}
	str := csSpec{Type: "string"}
	if !csChanged(str, csValue{Str: "a"}, csValue{Str: "b"}) || csChanged(str, csValue{Str: "a"}, csValue{Str: "a"}) {
		t.Error("string comparison")
	}
}

func TestCsTagDocument(t *testing.T) {
	s := csSpec{Kind: csKindConn, Ref: "c1", Point: "Status", Tag: "_System.Conn.A.Status",
		Type: "digital", Group2: "Connections", Group3: "A", Text: "Status",
		StateTrue: "Online", StateFalse: "Offline", Alarm: true, Priority: 1}
	doc := csTagDocument(-1001, s)
	got := map[string]any{}
	for _, e := range doc {
		if _, dup := got[e.Key]; dup {
			t.Fatalf("duplicate field %s", e.Key)
		}
		got[e.Key] = e.Value
	}
	want := map[string]any{
		"_id": -1001.0, "tag": "_System.Conn.A.Status", "type": "digital",
		"group1": "_System", "group2": "Connections", "group3": "A",
		"description": "_System~Connections~A~Status", "alarmDisabled": false,
		"priority": 1.0, "stateTextTrue": "Online", "origin": "system",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	if _, ok := got[commStatusField]; !ok {
		t.Error("missing commStatus identity")
	}
}
