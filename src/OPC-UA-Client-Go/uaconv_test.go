/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

package main

import (
	"testing"
	"time"

	"github.com/gopcua/opcua/ua"
)

func dv(v any) *ua.DataValue {
	return &ua.DataValue{
		EncodingMask: ua.DataValueValue,
		Value:        ua.MustVariant(v),
	}
}

func TestConvertScalars(t *testing.T) {
	guid := &ua.GUID{Data1: 0x72962B91, Data2: 0xFA75, Data3: 0x4AE6, Data4: []byte{0x8D, 0x28, 0xB4, 0x04, 0xDC, 0x7D, 0xAF, 0x63}}
	nid := ua.NewStringNodeID(2, "Demo.Static.Scalar.Double")

	cases := []struct {
		name    string
		in      any
		wantTp  string
		wantDbl float64
		wantStr string
	}{
		{"boolean true", true, "boolean", 1, "True"},
		{"boolean false", false, "boolean", 0, "False"},
		{"sbyte", int8(-5), "sbyte", -5, "-5"},
		{"byte", byte(200), "byte", 200, "200"},
		{"int16", int16(-1234), "int16", -1234, "-1234"},
		{"uint16", uint16(1234), "uint16", 1234, "1234"},
		{"int32", int32(-70000), "int32", -70000, "-70000"},
		{"uint32", uint32(70000), "uint32", 70000, "70000"},
		{"int64", int64(-5000000000), "int64", -5e9, "-5000000000"},
		{"uint64", uint64(5000000000), "uint64", 5e9, "5000000000"},
		{"float", float32(42.5), "float", 42.5, "42.5"},
		{"double", float64(42.5), "double", 42.5, "42.5"},
		{"string", "hello", "string", 0, "hello"},
		{"localizedtext", ua.NewLocalizedText("Pump 1"), "localizedtext", 0, "Pump 1"},
		{"qualifiedname ns0", &ua.QualifiedName{NamespaceIndex: 0, Name: "Foo"}, "qualifiedname", 0, "Foo"},
		{"qualifiedname ns2", &ua.QualifiedName{NamespaceIndex: 2, Name: "Foo"}, "qualifiedname", 0, "2:Foo"},
		{"guid", guid, "guid", 0, "72962B91-FA75-4AE6-8D28-B404DC7DAF63"},
		{"nodeid", nid, "nodeid", 0, "ns=2;s=Demo.Static.Scalar.Double"},
		{"xmlelement", ua.XMLElement("<a b=\"1\"/>"), "xmlelement", 0, `<a b="1"/>`},
		{"bytestring", []byte{0xDE, 0xAD, 0xBE, 0xEF}, "bytestring", 0, "3q2+7w=="},
		{"statuscode good", ua.StatusOK, "statuscode", 0, "Good"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tp, dbl, str, _, isArray := convertOPCValue(dv(c.in))
			if tp != c.wantTp {
				t.Errorf("asdu = %q, want %q", tp, c.wantTp)
			}
			if dbl != c.wantDbl {
				t.Errorf("value = %v, want %v", dbl, c.wantDbl)
			}
			if str != c.wantStr {
				t.Errorf("valueString = %q, want %q", str, c.wantStr)
			}
			if isArray {
				t.Error("scalar reported as array")
			}
		})
	}
}

// A DateTime is published as Unix milliseconds in value and as ISO-8601 in
// valueString; the command path converts it back from the milliseconds, so
// the two must agree.
func TestConvertDateTime(t *testing.T) {
	when := time.Date(2026, 8, 29, 12, 34, 56, 789000000, time.UTC)
	tp, dbl, str, _, isArray := convertOPCValue(dv(when))

	if tp != "datetime" {
		t.Errorf("asdu = %q", tp)
	}
	if isArray {
		t.Error("scalar reported as array")
	}
	if want := float64(when.UnixMilli()); dbl != want {
		t.Errorf("value = %v, want unix ms %v", dbl, want)
	}
	if got, err := time.Parse(time.RFC3339Nano, str); err != nil {
		t.Errorf("valueString %q is not RFC3339: %v", str, err)
	} else if !got.Equal(when) {
		t.Errorf("valueString %q round-tripped to %v, want %v", str, got, when)
	}
}

func TestConvertArrays(t *testing.T) {
	cases := []struct {
		name     string
		in       any
		wantTp   string
		wantJSON string
	}{
		{"double array", []float64{1, 2.5, 3}, "double[]", "[1,2.5,3]"},
		{"int32 array", []int32{-1, 0, 1}, "int32[]", "[-1,0,1]"},
		{"boolean array", []bool{true, false}, "boolean[]", "[true,false]"},
		{"string array", []string{"a", "b"}, "string[]", `["a","b"]`},
		{"empty double array", []float64{}, "double[]", "[]"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tp, dbl, str, jsonStr, isArray := convertOPCValue(dv(c.in))
			if !isArray {
				t.Error("array not reported as array")
			}
			if tp != c.wantTp {
				t.Errorf("asdu = %q, want %q", tp, c.wantTp)
			}
			if dbl != 0 {
				t.Errorf("value = %v, want 0 for an array", dbl)
			}
			if jsonStr != c.wantJSON {
				t.Errorf("valueJson = %s, want %s", jsonStr, c.wantJSON)
			}
			// Every array renders as its JSON in valueString.
			if str != c.wantJSON {
				t.Errorf("valueString = %s, want %s", str, c.wantJSON)
			}
			// Arrays become json tags whatever their element type.
			if got := tagTypeFor(tp, isArray); got != "json" {
				t.Errorf("tag type = %q, want json", got)
			}
		})
	}
}

// The asdu of an array is what the command path parses back to find the
// element type, so the bracket form matters.
func TestArrayAsduIsParseableByCommandPath(t *testing.T) {
	tp, _, _, _, _ := convertOPCValue(dv([]float64{1, 2}))
	if tp != "double[]" {
		t.Fatalf("asdu = %q", tp)
	}
	if n := len(splitOnBracket(tp)) - 1; n != 1 {
		t.Errorf("asdu %q must contain exactly one '[' for the command path to see an array", tp)
	}
	if base := splitOnBracket(tp)[0]; base != "double" {
		t.Errorf("element type = %q, want double", base)
	}
}

func splitOnBracket(s string) []string {
	out := []string{""}
	for _, r := range s {
		if r == '[' {
			out = append(out, "")
			continue
		}
		out[len(out)-1] += string(r)
	}
	return out
}

func TestConvertNilAndNullValues(t *testing.T) {
	for _, c := range []struct {
		name string
		in   *ua.DataValue
	}{
		{"nil datavalue", nil},
		{"nil variant", &ua.DataValue{EncodingMask: ua.DataValueValue}},
		{"null variant", &ua.DataValue{EncodingMask: ua.DataValueValue, Value: &ua.Variant{}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			tp, dbl, str, jsonStr, isArray := convertOPCValue(c.in)
			if tp != "unknown" || dbl != 0 || str != "" || jsonStr != "" || isArray {
				t.Errorf("got (%q,%v,%q,%q,%v), want the empty conversion",
					tp, dbl, str, jsonStr, isArray)
			}
		})
	}
}

// An XML element is stored as a JSON string, so quotes inside the markup
// cannot break the document.
func TestConvertXMLElementJSONIsAString(t *testing.T) {
	_, _, str, jsonStr, _ := convertOPCValue(dv(ua.XMLElement(`<a b="1"/>`)))
	if str != `<a b="1"/>` {
		t.Errorf("valueString = %q", str)
	}
	if jsonStr != `"<a b=\"1\"/>"` {
		t.Errorf("valueJson = %s, want the XML as a quoted JSON string", jsonStr)
	}
}

func TestConvertLocalizedTextJSON(t *testing.T) {
	_, _, str, jsonStr, _ := convertOPCValue(dv(&ua.LocalizedText{
		EncodingMask: ua.LocalizedTextLocale | ua.LocalizedTextText,
		Locale:       "en", Text: "Pump 1",
	}))
	if str != "Pump 1" {
		t.Errorf("valueString = %q, want the text only", str)
	}
	// Locale and text both survive; the wire encoding mask must not.
	if jsonStr != `{"Locale":"en","Text":"Pump 1"}` {
		t.Errorf("valueJson = %s", jsonStr)
	}
}

func TestStatusIsGood(t *testing.T) {
	cases := map[ua.StatusCode]bool{
		ua.StatusOK:                        true,
		ua.StatusUncertain:                 false, // uncertain is not good, as in C#
		ua.StatusBad:                       false,
		ua.StatusBadNodeIDUnknown:          false,
		ua.StatusBadEncodingLimitsExceeded: false,
	}
	for code, want := range cases {
		if got := statusIsGood(code); got != want {
			t.Errorf("statusIsGood(0x%X) = %v, want %v", uint32(code), got, want)
		}
	}
}

func TestTagTypeFor(t *testing.T) {
	cases := []struct {
		asdu    string
		isArray bool
		want    string
	}{
		{"boolean", false, "digital"},
		{"double", false, "analog"},
		{"float", false, "analog"},
		{"int32", false, "analog"},
		{"statuscode", false, "analog"},
		{"datetime", false, "analog"},
		{"string", false, "string"},
		{"bytestring", false, "string"},
		{"localizedtext", false, "string"},
		{"qualifiedname", false, "string"},
		{"xmlelement", false, "string"},
		{"guid", false, "string"},
		{"nodeid", false, "json"},
		{"expandednodeid", false, "json"},
		{"extensionobject", false, "json"},
		{"boolean[]", true, "json"},
		{"double[]", true, "json"},
		{"string[]", true, "json"},
		// The C# switch lowercases first.
		{"Double", false, "analog"},
		{"BOOLEAN", false, "digital"},
	}
	for _, c := range cases {
		if got := tagTypeFor(c.asdu, c.isArray); got != c.want {
			t.Errorf("tagTypeFor(%q,%v) = %q, want %q", c.asdu, c.isArray, got, c.want)
		}
	}
}

func TestStatusCodeName(t *testing.T) {
	if got := statusCodeName(ua.StatusOK); got != "Good" {
		t.Errorf("statusCodeName(OK) = %q, want Good", got)
	}
	if got := statusCodeName(ua.StatusBadNodeIDUnknown); got == "" ||
		got == "0x0" || len(got) < 3 {
		t.Errorf("statusCodeName(BadNodeIdUnknown) = %q, want a symbolic name", got)
	}
}
