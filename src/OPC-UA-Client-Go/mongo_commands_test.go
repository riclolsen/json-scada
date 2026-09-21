/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopcua/opcua/ua"
)

func TestCommandVariantScalars(t *testing.T) {
	cases := []struct {
		asdu        string
		value       float64
		valueString string
		want        any
	}{
		{"boolean", 1, "", true},
		{"boolean", 0, "", false},
		// Any non-zero is true, as in Convert.ToBoolean(value != 0.0).
		{"boolean", -3.5, "", true},
		{"sbyte", -5, "", int8(-5)},
		{"byte", 200, "", byte(200)},
		{"int16", -1234, "", int16(-1234)},
		{"uint16", 1234, "", uint16(1234)},
		{"int32", -70000, "", int32(-70000)},
		{"integer", -70000, "", int32(-70000)},
		{"uint32", 70000, "", uint32(70000)},
		{"int64", -5000000000, "", int64(-5000000000)},
		{"uint64", 5000000000, "", uint64(5000000000)},
		{"float", 42.5, "", float32(42.5)},
		{"double", 42.5, "", float64(42.5)},
		{"string", 0, "hello", "hello"},
		{"bytestring", 0, "3q2+7w==", "3q2+7w=="},
		{"localizedtext", 0, "Pump 1", "Pump 1"},
		{"qualifiedname", 0, "2:Foo", "2:Foo"},
		{"nodeid", 0, "ns=2;s=Foo", "ns=2;s=Foo"},
		{"guid", 0, "72962B91-FA75-4AE6-8D28-B404DC7DAF63", "72962B91-FA75-4AE6-8D28-B404DC7DAF63"},

		// The ASDU is matched case-insensitively, as the C# switch
		// lowercases first.
		{"Double", 1.25, "", float64(1.25)},
		{"BOOLEAN", 1, "", true},
	}

	for _, c := range cases {
		t.Run(c.asdu, func(t *testing.T) {
			v, reason, err := commandVariant(c.asdu, c.value, c.valueString)
			if reason != "" {
				t.Fatalf("unexpected cancel reason %q", reason)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v == nil {
				t.Fatal("no variant produced")
			}
			if got := v.Value(); !reflect.DeepEqual(got, c.want) {
				t.Errorf("value = %v (%T), want %v (%T)", got, got, c.want, c.want)
			}
		})
	}
}

// A datetime travels as Unix milliseconds, the same encoding acquisition
// publishes, so a value read back and commanded again must survive.
func TestCommandVariantDateTimeRoundTrip(t *testing.T) {
	when := time.Date(2026, 8, 29, 12, 34, 56, 789000000, time.UTC)

	// What acquisition would have published for this instant.
	_, dbl, _, _, _ := convertOPCValue(dv(when))

	v, reason, err := commandVariant("datetime", dbl, "")
	if reason != "" || err != nil {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	got, ok := v.Value().(time.Time)
	if !ok {
		t.Fatalf("variant holds %T, want time.Time", v.Value())
	}
	if !got.Equal(when) {
		t.Errorf("round-tripped to %v, want %v", got, when)
	}
}

// Structured types cannot be built from a number or a string, so the
// command must be refused rather than written wrong.
func TestCommandVariantComplexTypesRefused(t *testing.T) {
	for _, asdu := range []string{"extensionobject", "numericrange", "variant", "diagnosticinfo", "datavalue"} {
		v, reason, err := commandVariant(asdu, 1, "x")
		if err == nil {
			t.Errorf("%s: expected an error, got variant %v reason %q", asdu, v, reason)
		}
	}
}

// An ASDU no branch matches yields no variant and no error; the caller
// turns that into "unsupported command type".
func TestCommandVariantUnknownASDU(t *testing.T) {
	v, reason, err := commandVariant("somethingelse", 1, "")
	if v != nil || reason != "" || err != nil {
		t.Errorf("got (%v,%q,%v), want (nil,\"\",nil)", v, reason, err)
	}
}

func TestArrayVariantTypes(t *testing.T) {
	cases := []struct {
		asdu  string
		value string
		want  any
	}{
		{"double[]", "[1,2.5,3]", []float64{1, 2.5, 3}},
		{"float[]", "[1,2.5]", []float32{1, 2.5}},
		{"int32[]", "[-1,0,1]", []int32{-1, 0, 1}},
		{"integer[]", "[-1,0,1]", []int32{-1, 0, 1}},
		{"int16[]", "[-1,2]", []int16{-1, 2}},
		{"uint16[]", "[1,2]", []uint16{1, 2}},
		{"uint32[]", "[1,2]", []uint32{1, 2}},
		{"int64[]", "[-1,2]", []int64{-1, 2}},
		{"uint64[]", "[1,2]", []uint64{1, 2}},
		{"boolean[]", "[true,false]", []bool{true, false}},
		{"string[]", `["a","b"]`, []string{"a", "b"}},
		{"localizedtext[]", `["a"]`, []string{"a"}},
		{"empty", "[]", nil}, // handled below
	}

	for _, c := range cases {
		if c.asdu == "empty" {
			continue
		}
		t.Run(c.asdu, func(t *testing.T) {
			v, reason, err := arrayVariant(c.asdu, c.value)
			if reason != "" || err != nil {
				t.Fatalf("reason=%q err=%v", reason, err)
			}
			if got := v.Value(); !reflect.DeepEqual(got, c.want) {
				t.Errorf("value = %v (%T), want %v (%T)", got, got, c.want, c.want)
			}
		})
	}
}

// The array ASDU produced by acquisition must be understood by the command
// path; this is the round trip an operator actually exercises.
func TestArrayVariantAcceptsAcquisitionASDU(t *testing.T) {
	asdu, _, valueString, _, isArray := convertOPCValue(dv([]float64{1, 2.5, 3}))
	if !isArray {
		t.Fatal("expected an array")
	}
	v, reason, err := arrayVariant(asdu, valueString)
	if reason != "" || err != nil {
		t.Fatalf("acquisition asdu %q / valueString %q were refused: reason=%q err=%v",
			asdu, valueString, reason, err)
	}
	if got := v.Value(); !reflect.DeepEqual(got, []float64{1, 2.5, 3}) {
		t.Errorf("value = %v", got)
	}
}

func TestArrayVariantDateTime(t *testing.T) {
	// parity: array elements are ISO-8601 strings, unlike a scalar
	// datetime, which travels as Unix milliseconds.
	v, reason, err := arrayVariant("datetime[]", `["2026-08-29T12:34:56Z"]`)
	if reason != "" || err != nil {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	got, ok := v.Value().([]time.Time)
	if !ok || len(got) != 1 {
		t.Fatalf("variant holds %T", v.Value())
	}
	if !got[0].Equal(time.Date(2026, 8, 29, 12, 34, 56, 0, time.UTC)) {
		t.Errorf("parsed %v", got[0])
	}
}

func TestArrayVariantCancelReasons(t *testing.T) {
	if _, reason, _ := arrayVariant("double[]", ""); reason != "empty array json error" {
		t.Errorf("empty valueString reason = %q", reason)
	}
	if _, reason, _ := arrayVariant("double[]", "not json"); reason != "array invalid json format error" {
		t.Errorf("bad json reason = %q", reason)
	}
	if _, reason, _ := arrayVariant("double[]", `{"a":1}`); reason != "array invalid json format error" {
		t.Errorf("json object reason = %q", reason)
	}
}

func TestArrayVariantErrors(t *testing.T) {
	// A wrong element type is a conversion error, not a cancel reason.
	if _, reason, err := arrayVariant("double[]", `["a"]`); err == nil {
		t.Errorf("strings in a double array must fail, got reason %q", reason)
	}
	if _, reason, err := arrayVariant("boolean[]", `[1]`); err == nil {
		t.Errorf("numbers in a boolean array must fail, got reason %q", reason)
	}
	if _, _, err := arrayVariant("extensionobject[]", `[1]`); err == nil {
		t.Error("an unsupported array element type must fail")
	}
}

// parity: a multi-dimensional ASDU such as "double[,]" still holds exactly
// one '[', so the C# driver treats it as a flat array of the element type
// and so does this. Writing a flat array to a matrix node is then the
// server's problem to reject, which is the C# behaviour too.
func TestCommandVariantMultiDimensionalIsFlattened(t *testing.T) {
	v, reason, err := commandVariant("double[,]", 1, "[1,2]")
	if reason != "" || err != nil {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if got := v.Value(); !reflect.DeepEqual(got, []float64{1, 2}) {
		t.Errorf("value = %v (%T), want a flat []float64", got, got)
	}
}

// The Call service needs the object that owns the method, which the driver
// finds by browsing the method's hierarchical references backwards.
func TestMethodParentResolution(t *testing.T) {
	cli, _, _ := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	methodID := ua.NewStringNodeID(1, "Boiler.Reset")
	parent, err := methodParent(ctx, cli, methodID)
	if err != nil {
		t.Fatalf("methodParent: %v", err)
	}
	if got, want := parent.String(), "ns=1;s=Boiler"; got != want {
		t.Errorf("parent = %s, want %s", got, want)
	}
}

// A node with no owning object must be reported, not guessed at.
func TestMethodParentMissing(t *testing.T) {
	cli, _, _ := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := methodParent(ctx, cli, ua.NewStringNodeID(1, "Direct")); err == nil {
		t.Error("a node without a parent object must fail to resolve")
	}
}

func TestMethodArguments(t *testing.T) {
	args, err := methodArguments(`[true, 12, 2.5, "abc"]`)
	if err != nil {
		t.Fatalf("methodArguments: %v", err)
	}
	if len(args) != 4 {
		t.Fatalf("got %d arguments, want 4", len(args))
	}

	want := []any{true, int64(12), 2.5, "abc"}
	for i, w := range want {
		if got := args[i].Value(); !reflect.DeepEqual(got, w) {
			t.Errorf("argument %d = %v (%T), want %v (%T)", i, got, got, w, w)
		}
	}
}

func TestMethodArgumentsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "not json", `{"a":1}`} {
		args, err := methodArguments(in)
		if err != nil {
			t.Errorf("methodArguments(%q) errored: %v", in, err)
		}
		if len(args) != 0 {
			t.Errorf("methodArguments(%q) = %d args, want none", in, len(args))
		}
	}
}

// The variant types must be ones gopcua can actually encode; NewVariant
// rejects anything else, so this guards the whole command path.
func TestCommandVariantsAreEncodable(t *testing.T) {
	for _, asdu := range []string{
		"boolean", "sbyte", "byte", "int16", "uint16", "int32", "uint32",
		"int64", "uint64", "float", "double", "datetime", "string",
	} {
		v, _, err := commandVariant(asdu, 1, "x")
		if err != nil || v == nil {
			t.Fatalf("%s: %v", asdu, err)
		}
		if _, err := v.Encode(); err != nil {
			t.Errorf("%s: variant cannot be encoded: %v", asdu, err)
		}
	}
	for _, asdu := range []string{
		"boolean[]", "int16[]", "uint16[]", "int32[]", "uint32[]",
		"int64[]", "uint64[]", "float[]", "double[]", "string[]",
	} {
		body := "[1]"
		switch {
		case strings.HasPrefix(asdu, "boolean"):
			body = "[true]"
		case strings.HasPrefix(asdu, "string"):
			body = `["a"]`
		}
		v, reason, err := arrayVariant(asdu, body)
		if err != nil || reason != "" || v == nil {
			t.Fatalf("%s: reason=%q err=%v", asdu, reason, err)
		}
		if _, err := v.Encode(); err != nil {
			t.Errorf("%s: variant cannot be encoded: %v", asdu, err)
		}
	}
}
