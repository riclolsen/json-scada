/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

import (
	"testing"

	"modbus-go/internal/mbcore"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNormalizeDefaults(t *testing.T) {
	c := Normalize(bson.M{"protocolConnectionNumber": 7.0, "maxReadRegisters": 500.0, "maxReadCoils": 9000.0})
	if c.Name != "MODBUS-7" || c.ConnectionMode != ModeTCP || c.TimeoutMs != 1000 || c.MaxRetries != 2 ||
		c.PollingIntervalMs != 1000 || c.GiIntervalS != 300 || c.MaxReadRegisters != 125 || c.MaxReadCoils != 2000 ||
		c.MaxAddressGap != 8 || c.ByteOrder32 != "ABCD" || !c.UseMaskWrite || !c.CommandsEnabled || !c.TLS.ChainValidation {
		t.Fatalf("defaults %+v", c)
	}
	if c.DefaultPort() != 502 {
		t.Fatal("port")
	}
	c.ConnectionMode = ModeRTUOverTLS
	if !c.IsTLS() || !c.IsRTU() || c.DefaultPort() != 802 {
		t.Fatal("rtu over tls")
	}
	c.ConnectionMode = "bogus"
	if c.Validate() == nil {
		t.Fatal("bogus mode accepted")
	}
}

func bind(t *testing.T, id, addr, asdu string) *Binding {
	t.Helper()
	b, err := BuildBinding(RawBinding{ID: id, ObjectAddress: addr, Asdu: asdu}, Normalize(bson.M{}).Defaults(), false)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPlanReadsCoalescing(t *testing.T) {
	bs := []*Binding{
		bind(t, "a", "hr:0", "uint16"),
		bind(t, "b", "hr:1", "float32"), // hr:1-2
		bind(t, "c", "hr:9", "uint16"),  // gap of 6 <= 8: same block
		bind(t, "d", "hr:30", "uint16"), // gap too large: new block
		bind(t, "e", "co:3", ""),
		bind(t, "f", "ir:3", "int16"),
	}
	blocks := PlanReads(bs, Limits{MaxReadRegisters: 125, MaxReadCoils: 2000, MaxAddressGap: 8})
	if len(blocks) != 4 {
		t.Fatalf("got %d blocks", len(blocks))
	}
	first := blocks[0]
	if first.Area != mbcore.AreaHolding || first.Start != 0 || first.Count != 10 || len(first.Slices) != 3 || first.FC != 3 {
		t.Fatalf("first block %+v", first)
	}
	// A smaller limit splits the run.
	blocks = PlanReads(bs[:3], Limits{MaxReadRegisters: 3, MaxReadCoils: 2000, MaxAddressGap: 8})
	if len(blocks) != 2 {
		t.Fatalf("limit split: %d blocks", len(blocks))
	}
}

func TestParseTopic(t *testing.T) {
	cases := []struct {
		in   string
		want TopicSpec
		ok   bool
	}{
		{"PLC1_FLOW|hr:4:float32_cdab", TopicSpec{Tag: "PLC1_FLOW", ObjectAddress: "hr:4", Asdu: "float32_cdab", Count: 1}, true},
		{"PLC1_STAT|hr:20:uint16[10]", TopicSpec{Tag: "PLC1_STAT", ObjectAddress: "hr:20", Asdu: "uint16", Count: 10}, true},
		{"X|hr:4:float32|badc", TopicSpec{Tag: "X", ObjectAddress: "hr:4", Asdu: "float32_badc", Count: 1}, true},
		{"no-separator", TopicSpec{}, false},
		{"X|garbage", TopicSpec{}, false},
	}
	for _, c := range cases {
		got, ok := ParseTopic(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseTopic(%q) = %+v %v, want %+v %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestTagDocShape(t *testing.T) {
	d := TagDoc(5, "T", 3, "C", "hr:1", "int16", 1, "analog")
	if d[0].Key != "_id" || d[1].Key != "tag" || len(d) != 35 {
		t.Fatalf("doc %v", d)
	}
}

func TestTagDocStateTexts(t *testing.T) {
	texts := func(d bson.D) map[string]any {
		m := map[string]any{}
		for _, e := range d {
			switch e.Key {
			case "stateTextTrue", "stateTextFalse", "eventTextTrue", "eventTextFalse":
				m[e.Key] = e.Value
			}
		}
		return m
	}
	cases := []struct {
		typ                 string
		wantTrue, wantFalse string
	}{
		{"digital", "TRUE", "FALSE"},
		{"analog", "", ""},
		{"string", "", ""},
	}
	for _, c := range cases {
		got := texts(TagDoc(5, "T", 3, "C", "co:1", "bool", 1, c.typ))
		want := map[string]any{
			"stateTextTrue": c.wantTrue, "stateTextFalse": c.wantFalse,
			"eventTextTrue": c.wantTrue, "eventTextFalse": c.wantFalse,
		}
		if len(got) != 4 {
			t.Fatalf("%s: texts %v", c.typ, got)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: %s = %v, want %q", c.typ, k, got[k], v)
			}
		}
	}
}
