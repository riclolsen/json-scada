/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

package main

import (
	"testing"

	"github.com/gopcua/opcua/ua"
)

// splitBrowsePath feeds group2, protocolSourceBrowsePath and the tag
// description, so its edge cases are visible to every operator.
func TestSplitBrowsePath(t *testing.T) {
	cases := []struct {
		full       string
		wantPath   string
		wantParent string
	}{
		{"/Objects/Boiler/Sensor", "Boiler", "Boiler"},
		{"/Objects/Boiler/Drum/Level", "Boiler/Drum", "Drum"},
		{"/Objects/Server/ServerStatus/State", "Server/ServerStatus", "ServerStatus"},

		// parity: a node directly under Objects has no "/Objects/" left to
		// strip once the last segment is removed, so the regex does not
		// match and the path keeps its leading "/Objects".
		{"/Objects/Boiler", "/Objects", "Objects"},

		// Only the first occurrence is replaced, as in the C# regex.
		{"/Objects/A/Objects/B", "A/Objects", "Objects"},

		{"", "", ""},
	}

	for _, c := range cases {
		path, parent := splitBrowsePath(c.full)
		if path != c.wantPath || parent != c.wantParent {
			t.Errorf("splitBrowsePath(%q) = (%q,%q), want (%q,%q)",
				c.full, path, parent, c.wantPath, c.wantParent)
		}
	}
}

// The tag document is built from the split path, so check the operator
// facing fields end to end.
func TestBrowsePathFeedsTagFields(t *testing.T) {
	path, parent := splitBrowsePath("/Objects/Boiler/Drum/Level")
	ov := OPCValue{
		Address:     "ns=2;s=Level",
		Asdu:        "double",
		ConnName:    "PLC1",
		ConnNumber:  81,
		DisplayName: "Level",
		Path:        path,
		ParentName:  parent,
	}
	doc := newRealtimeDoc(ov, 81000001, 0)

	if got := doc["group2"]; got != "Boiler/Drum" {
		t.Errorf("group2 = %v", got)
	}
	if got := doc["protocolSourceBrowsePath"]; got != "Boiler/Drum" {
		t.Errorf("protocolSourceBrowsePath = %v", got)
	}
	if got := doc["description"]; got != "PLC1~Boiler/Drum~Level" {
		t.Errorf("description = %v", got)
	}
}

func TestContinuationPoints(t *testing.T) {
	results := []*ua.BrowseResult{
		{ContinuationPoint: nil},
		{ContinuationPoint: []byte{1, 2}},
		{ContinuationPoint: []byte{}},
		{ContinuationPoint: []byte{3}},
	}
	cps := continuationPoints(results)
	if len(cps) != 2 {
		t.Fatalf("got %d continuation points, want 2", len(cps))
	}
	if string(cps[0]) != "\x01\x02" || string(cps[1]) != "\x03" {
		t.Errorf("continuation points came back in the wrong order: %v", cps)
	}
}

// References arriving through BrowseNext are merged back onto the result
// they continue. Losing them costs every reference past the first 1000 of a
// wide node — the bug this test guards against, which the C# driver had too
// until MergeContinuedReferences was added there. See D15 in README.md.
func TestMergeContinued(t *testing.T) {
	refA := &ua.ReferenceDescription{BrowseName: &ua.QualifiedName{Name: "A"}}
	refB := &ua.ReferenceDescription{BrowseName: &ua.QualifiedName{Name: "B"}}
	refC := &ua.ReferenceDescription{BrowseName: &ua.QualifiedName{Name: "C"}}

	results := []*ua.BrowseResult{
		{References: []*ua.ReferenceDescription{refA}},                               // no continuation
		{References: []*ua.ReferenceDescription{refB}, ContinuationPoint: []byte{9}}, // continues
	}
	continued := []*ua.BrowseResult{
		{References: []*ua.ReferenceDescription{refC}, ContinuationPoint: nil},
	}

	mergeContinued(results, continued)

	if len(results[0].References) != 1 {
		t.Errorf("a result without a continuation point must be untouched, got %d refs",
			len(results[0].References))
	}
	if len(results[1].References) != 2 {
		t.Fatalf("continued references were not merged: got %d, want 2", len(results[1].References))
	}
	if results[1].References[1].BrowseName.Name != "C" {
		t.Errorf("merged reference = %q, want C", results[1].References[1].BrowseName.Name)
	}
	if len(results[1].ContinuationPoint) != 0 {
		t.Error("the continuation point must be replaced by the new one, which is empty here")
	}
}

// A short continued slice must not panic or misalign.
func TestMergeContinuedShortResponse(t *testing.T) {
	results := []*ua.BrowseResult{
		{References: []*ua.ReferenceDescription{}, ContinuationPoint: []byte{1}},
		{References: []*ua.ReferenceDescription{}, ContinuationPoint: []byte{2}},
	}
	mergeContinued(results, nil)
	if len(results[0].ContinuationPoint) != 1 {
		t.Error("an empty response must leave the results alone")
	}
}

func TestIsEncodingLimit(t *testing.T) {
	if !isEncodingLimit(ua.StatusBadEncodingLimitsExceeded) {
		t.Error("BadEncodingLimitsExceeded must trigger the halving retry")
	}
	if !isEncodingLimit(ua.StatusBadResponseTooLarge) {
		t.Error("BadResponseTooLarge must trigger the halving retry")
	}
	if isEncodingLimit(ua.StatusBadNodeIDUnknown) {
		t.Error("an unrelated status must not trigger the halving retry")
	}
	if isEncodingLimit(errShortRead) {
		t.Error("a plain error must not trigger the halving retry")
	}
}
