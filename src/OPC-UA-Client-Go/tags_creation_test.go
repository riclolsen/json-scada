/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

package main

import (
	"math"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func sampleValue() OPCValue {
	return OPCValue{
		AccessLevels: 3,
		Address:      "ns=2;s=Boiler1.Temp",
		Asdu:         "double",
		Value:        42.5,
		ValueString:  "42.5",
		ValueJSON:    "42.5",
		ConnNumber:   81,
		ConnName:     "PLC1",
		DisplayName:  "Temp",
		Path:         "Boiler1",
	}
}

func field(t *testing.T, doc bson.M, key string) any {
	t.Helper()
	v, ok := doc[key]
	if !ok {
		t.Fatalf("document has no %q field; every realtimeData field must be written", key)
	}
	return v
}

// Fields present in every shape, whatever the type of the point.
func TestNewRealtimeDocCommonFields(t *testing.T) {
	doc := newRealtimeDoc(sampleValue(), 81000005, 0)

	want := map[string]any{
		"_id":                              81000005.0,
		"protocolSourceBrowsePath":         "Boiler1",
		"protocolSourceAccessLevel":        "3",
		"protocolSourceASDU":               "double",
		"protocolSourceCommonAddress":      "",
		"protocolSourceConnectionNumber":   81.0,
		"protocolSourceObjectAddress":      "ns=2;s=Boiler1.Temp",
		"protocolSourceCommandUseSBO":      false,
		"protocolSourceCommandDuration":    0.0,
		"protocolSourceDiscardOldest":      true,
		"protocolSourcePublishingInterval": 5.0,
		"protocolSourceSamplingInterval":   2.0,
		"protocolSourceQueueSize":          10.0,
		"origin":                           "supervised",
		"tag":                              "PLC1;ns=2;s=Boiler1.Temp",
		"description":                      "PLC1~Boiler1~Temp",
		"ungroupedDescription":             "Temp",
		"group1":                           "PLC1",
		"group2":                           "Boiler1",
		"group3":                           "",
		"invalid":                          true,
		"invalidDetectTimeout":             60000.0,
		"kconv1":                           1.0,
		"kconv2":                           0.0,
		"hiLimit":                          math.MaxFloat64,
		"loLimit":                          -math.MaxFloat64,
		"supervisedOfCommand":              0.0,
		"commandOfSupervised":              0.0,
	}
	for k, w := range want {
		if got := field(t, doc, k); got != w {
			t.Errorf("%s = %v (%T), want %v (%T)", k, got, got, w, w)
		}
	}

	for _, k := range []string{"location", "parcels", "sourceDataUpdate", "timeTag", "timeTagAlarm", "timeTagAtSource"} {
		if got := field(t, doc, k); got != nil {
			t.Errorf("%s = %v, want nil", k, got)
		}
	}
	if _, ok := field(t, doc, "protocolDestinations").(bson.A); !ok {
		t.Error("protocolDestinations must be an empty array, not nil")
	}
}

// The access level is stored as the decimal digits of the byte, which is
// what C#'s Convert.ToString(byte) produces — not hex, not a flag name.
func TestNewRealtimeDocAccessLevelIsDecimal(t *testing.T) {
	ov := sampleValue()
	ov.AccessLevels = 255
	doc := newRealtimeDoc(ov, 1, 0)
	if got := doc["protocolSourceAccessLevel"]; got != "255" {
		t.Errorf("protocolSourceAccessLevel = %v, want \"255\"", got)
	}
}

func TestNewRealtimeDocAnalog(t *testing.T) {
	doc := newRealtimeDoc(sampleValue(), 1, 7)

	if got := field(t, doc, "type"); got != "analog" {
		t.Fatalf("type = %v", got)
	}
	if got := field(t, doc, "value"); got != 42.5 {
		t.Errorf("value = %v, want the acquired value", got)
	}
	if got := field(t, doc, "valueString"); got != "" {
		t.Errorf("valueString = %q, want empty for analog", got)
	}
	// parity: the analog literal of TagsCreation.cs never assigns
	// valueJson, so the stored field is null rather than "".
	if got := field(t, doc, "valueJson"); got != nil {
		t.Errorf("valueJson = %v, want nil for analog", got)
	}
	if got := field(t, doc, "alarmState"); got != -1.0 {
		t.Errorf("alarmState = %v, want -1", got)
	}
	if got := field(t, doc, "commandOfSupervised"); got != 7.0 {
		t.Errorf("commandOfSupervised = %v, want the command key", got)
	}
}

func TestNewRealtimeDocDigital(t *testing.T) {
	ov := sampleValue()
	ov.Asdu = "boolean"
	ov.Value = 1
	doc := newRealtimeDoc(ov, 1, 0)

	if got := field(t, doc, "type"); got != "digital" {
		t.Fatalf("type = %v", got)
	}
	if got := field(t, doc, "value"); got != 1.0 {
		t.Errorf("value = %v", got)
	}
	if got := field(t, doc, "alarmState"); got != 2.0 {
		t.Errorf("alarmState = %v, want 2 for digital", got)
	}
	for _, k := range []string{"stateTextTrue", "eventTextTrue"} {
		if got := field(t, doc, k); got != "TRUE" {
			t.Errorf("%s = %v, want TRUE", k, got)
		}
	}
	for _, k := range []string{"stateTextFalse", "eventTextFalse"} {
		if got := field(t, doc, k); got != "FALSE" {
			t.Errorf("%s = %v, want FALSE", k, got)
		}
	}
	if got := field(t, doc, "valueJson"); got != "" {
		t.Errorf("valueJson = %v, want empty string for digital", got)
	}
}

func TestNewRealtimeDocString(t *testing.T) {
	ov := sampleValue()
	ov.Asdu = "string"
	ov.ValueString = "running"
	doc := newRealtimeDoc(ov, 1, 0)

	if got := field(t, doc, "type"); got != "string" {
		t.Fatalf("type = %v", got)
	}
	if got := field(t, doc, "valueString"); got != "running" {
		t.Errorf("valueString = %v", got)
	}
	if got := field(t, doc, "value"); got != 0.0 {
		t.Errorf("value = %v, want 0 for a string tag", got)
	}
	if got := field(t, doc, "valueJson"); got != "" {
		t.Errorf("valueJson = %v, want empty for a string tag", got)
	}
	if got := field(t, doc, "alarmState"); got != -1.0 {
		t.Errorf("alarmState = %v", got)
	}
	if got := field(t, doc, "stateTextTrue"); got != "" {
		t.Errorf("stateTextTrue = %v, want empty for a non-digital tag", got)
	}
}

func TestNewRealtimeDocJSON(t *testing.T) {
	ov := sampleValue()
	ov.Asdu = "double[]"
	ov.IsArray = true
	ov.ValueString = "[1,2,3]"
	ov.ValueJSON = "[1,2,3]"
	doc := newRealtimeDoc(ov, 1, 0)

	if got := field(t, doc, "type"); got != "json" {
		t.Fatalf("type = %v", got)
	}
	if got := field(t, doc, "valueJson"); got != "[1,2,3]" {
		t.Errorf("valueJson = %v", got)
	}
	if got := field(t, doc, "valueString"); got != "[1,2,3]" {
		t.Errorf("valueString = %v", got)
	}
}

// A writable variable gets a command twin whose key is one below the
// supervised point it drives; the link is what lets an operator command it.
func TestNewRealtimeDocCommandForWritableVariable(t *testing.T) {
	ov := sampleValue()
	ov.CreateCommandForSupervised = true
	doc := newRealtimeDoc(ov, 81000005, 0)

	if got := field(t, doc, "origin"); got != "command" {
		t.Fatalf("origin = %v", got)
	}
	if got := field(t, doc, "tag"); got != "PLC1;ns=2;s=Boiler1.Temp;cmd" {
		t.Errorf("tag = %v", got)
	}
	if got := field(t, doc, "description"); got != "PLC1~Boiler1~Temp-Command" {
		t.Errorf("description = %v", got)
	}
	if got := field(t, doc, "supervisedOfCommand"); got != 81000006.0 {
		t.Errorf("supervisedOfCommand = %v, want the next key", got)
	}
	if got := field(t, doc, "commandOfSupervised"); got != 0.0 {
		t.Errorf("commandOfSupervised = %v, want 0 on a command tag", got)
	}
	if got := field(t, doc, "alarmState"); got != 2.0 {
		t.Errorf("alarmState = %v, want 2 on a command tag", got)
	}
	// A command carries no value of its own.
	if got := field(t, doc, "value"); got != 0.0 {
		t.Errorf("value = %v, want 0", got)
	}
	for _, k := range []string{"protocolSourcePublishingInterval", "protocolSourceSamplingInterval", "protocolSourceQueueSize"} {
		if got := field(t, doc, k); got != 0.0 {
			t.Errorf("%s = %v, want 0 on a command tag", k, got)
		}
	}
}

// A method is a command with no supervised twin at all.
func TestNewRealtimeDocCommandForMethod(t *testing.T) {
	ov := sampleValue()
	ov.CreateCommandForMethod = true
	ov.Asdu = "method"
	ov.Value = 0
	doc := newRealtimeDoc(ov, 81000005, 0)

	if got := field(t, doc, "origin"); got != "command" {
		t.Fatalf("origin = %v", got)
	}
	if got := field(t, doc, "protocolSourceASDU"); got != "method" {
		t.Errorf("protocolSourceASDU = %v, want method", got)
	}
	if got := field(t, doc, "supervisedOfCommand"); got != 0.0 {
		t.Errorf("supervisedOfCommand = %v, want 0 for a method", got)
	}
}

// A digital command still gets the TRUE/FALSE texts.
func TestNewRealtimeDocCommandDigitalTexts(t *testing.T) {
	ov := sampleValue()
	ov.Asdu = "boolean"
	ov.CreateCommandForSupervised = true
	doc := newRealtimeDoc(ov, 1, 0)

	if got := field(t, doc, "stateTextTrue"); got != "TRUE" {
		t.Errorf("stateTextTrue = %v, want TRUE on a digital command", got)
	}
}

func TestTagFromOPCParameters(t *testing.T) {
	if got := tagFromOPCParameters(sampleValue()); got != "PLC1;ns=2;s=Boiler1.Temp" {
		t.Errorf("tag = %q", got)
	}
}

// Every value written to MongoDB must be a float64 so it lands as a BSON
// double: an int would make the AdminUI and cs_data_processor see a
// different type than the C# driver produces.
func TestNewRealtimeDocNumbersAreFloat64(t *testing.T) {
	doc := newRealtimeDoc(sampleValue(), 1, 0)
	for k, v := range doc {
		switch v.(type) {
		case int, int32, int64, float32:
			t.Errorf("%s is %T, must be float64", k, v)
		}
	}
}
