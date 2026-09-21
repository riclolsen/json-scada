package jsmongo

import (
	"math"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestGetDoubleAcceptsEveryNumericBSONType(t *testing.T) {
	doc := bson.M{
		"f64": float64(1.5),
		"f32": float32(2.5),
		"i":   int(3),
		"i32": int32(4),
		"i64": int64(5),
		"tru": true,
		"fal": false,
	}
	for key, want := range map[string]float64{
		"f64": 1.5, "f32": 2.5, "i": 3, "i32": 4, "i64": 5, "tru": 1, "fal": 0,
	} {
		if got := GetDouble(doc, key, -99); got != want {
			t.Errorf("GetDouble(%q) = %v, want %v", key, got, want)
		}
	}
}

// The deliberate widening documented at the top of bsonget.go: dnp3-go's
// family returned the default for a numeric string, the others coerced it.
func TestGetDoubleCoercesNumericStrings(t *testing.T) {
	doc := bson.M{"s": "1001", "sf": "1001.5", "junk": "not a number"}
	if got := GetDouble(doc, "s", -99); got != 1001 {
		t.Errorf(`GetDouble("1001") = %v, want 1001`, got)
	}
	if got := GetDouble(doc, "sf", -99); got != 1001.5 {
		t.Errorf(`GetDouble("1001.5") = %v, want 1001.5`, got)
	}
	if got := GetDouble(doc, "junk", -99); got != -99 {
		t.Errorf("GetDouble(junk) = %v, want the default", got)
	}
}

func TestGetDoubleMissingAndNil(t *testing.T) {
	doc := bson.M{"nil": nil}
	if got := GetDouble(doc, "nil", 7); got != 7 {
		t.Errorf("nil field = %v, want default", got)
	}
	if got := GetDouble(doc, "absent", 7); got != 7 {
		t.Errorf("absent field = %v, want default", got)
	}
}

func TestGetBoolAcceptsNumbers(t *testing.T) {
	doc := bson.M{"b": true, "one": int32(1), "zero": float64(0), "s": "1"}
	for key, want := range map[string]bool{"b": true, "one": true, "zero": false, "s": true} {
		if got := GetBool(doc, key, false); got != want {
			t.Errorf("GetBool(%q) = %v, want %v", key, got, want)
		}
	}
	if got := GetBool(bson.M{}, "absent", true); !got {
		t.Error("absent field should return the default")
	}
}

// parity: GetString must NOT render numbers. See the comment on GetString.
func TestGetStringDoesNotRenderNumbers(t *testing.T) {
	doc := bson.M{"s": "hello", "n": float64(42)}
	if got := GetString(doc, "s", "def"); got != "hello" {
		t.Errorf("GetString(s) = %q", got)
	}
	if got := GetString(doc, "n", "def"); got != "def" {
		t.Errorf("GetString(number) = %q, want the default, not a rendered number", got)
	}
}

func TestGetU32ClampsAndFloors(t *testing.T) {
	cases := []struct {
		v    any
		want uint32
	}{
		{float64(1001), 1001},
		{"1001", 1001},
		{"1001.0", 1001},
		{float64(-5), 0},
		{float64(math.MaxUint32) * 2, math.MaxUint32},
		{float64(1001.9), 1001},
	}
	for _, c := range cases {
		if got := GetU32(bson.M{"a": c.v}, "a", 0); got != c.want {
			t.Errorf("GetU32(%v) = %d, want %d", c.v, got, c.want)
		}
	}
	if got := GetU32(bson.M{}, "absent", 77); got != 77 {
		t.Errorf("absent = %d, want default 77", got)
	}
	if got := GetU32(bson.M{"a": "junk"}, "a", 77); got != 77 {
		t.Errorf("unparseable = %d, want default 77", got)
	}
}

// Ported from the deleted iec60870-5/internal/mongoutil/mongoutil_test.go,
// which covered the value-level ToU32 this package replaced. All the
// iec60870-5 call sites pass 0 as the default, so GetU32 with def=0 must
// reproduce ToU32 exactly.
func TestGetU32MatchesTheOldToU32(t *testing.T) {
	dec, err := bson.ParseDecimal128("1001")
	if err != nil {
		t.Fatalf("ParseDecimal128: %s", err)
	}
	cases := []struct {
		name string
		val  any
		want uint32
	}{
		{"string", "1001", 1001},
		{"string float with spaces", " 1001.0 ", 1001},
		{"string invalid", "abc", 0},
		{"double", float64(1001), 1001},
		{"int32", int32(1001), 1001},
		{"int64", int64(1001), 1001},
		{"decimal128", dec, 1001},
		{"bool", true, 1},
		{"nil", nil, 0},
		{"negative", float64(-1), 0},
		{"over range", float64(math.MaxUint32) * 2, math.MaxUint32},
	}
	for _, c := range cases {
		if got := GetU32(bson.M{"a": c.val}, "a", 0); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestValueLevelCoercions(t *testing.T) {
	if !ToBool(int32(1)) || ToBool(float64(0)) || !ToBool(true) {
		t.Error("ToBool")
	}
	if ToString(float64(42)) != "" || ToString("s") != "s" {
		t.Error("ToString must not render numbers")
	}
	now := time.Now().Truncate(time.Millisecond)
	if !ToTime(bson.NewDateTimeFromTime(now)).Equal(now) || !ToTime(nil).IsZero() {
		t.Error("ToTime")
	}
}

func TestGetTimeAndDateMs(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	doc := bson.M{"dt": bson.NewDateTimeFromTime(now), "tt": now}
	if got := GetTime(doc, "dt"); !got.Equal(now) {
		t.Errorf("GetTime(DateTime) = %v, want %v", got, now)
	}
	if got := GetTime(doc, "tt"); !got.Equal(now) {
		t.Errorf("GetTime(time.Time) = %v, want %v", got, now)
	}
	if got := GetTime(doc, "absent"); !got.IsZero() {
		t.Errorf("GetTime(absent) = %v, want zero", got)
	}
	if got := GetDateMs(doc, "dt", -1); got != now.UnixMilli() {
		t.Errorf("GetDateMs = %d, want %d", got, now.UnixMilli())
	}
}

func TestGetStringArraySkipsNonStrings(t *testing.T) {
	doc := bson.M{"a": bson.A{"x", float64(1), "y"}}
	got := GetStringArray(doc, "a")
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("GetStringArray = %v, want [x y]", got)
	}
	if GetStringArray(bson.M{"a": "notanarray"}, "a") != nil {
		t.Error("non-array should give nil")
	}
}

func TestGetDocArrayAcceptsBothDocumentForms(t *testing.T) {
	doc := bson.M{"a": bson.A{
		bson.M{"k": "fromM"},
		bson.D{{Key: "k", Value: "fromD"}},
	}}
	got := GetDocArray(doc, "a")
	if len(got) != 2 {
		t.Fatalf("GetDocArray len = %d, want 2", len(got))
	}
	if got[0]["k"] != "fromM" || got[1]["k"] != "fromD" {
		t.Errorf("GetDocArray = %v", got)
	}
}

func TestGetBinaryMap(t *testing.T) {
	doc := bson.M{"m": bson.M{
		"rcb1": bson.Binary{Data: []byte{1, 2}},
		"rcb2": []byte{3},
	}}
	got := GetBinaryMap(doc, "m")
	if len(got) != 2 || string(got["rcb1"]) != "\x01\x02" || string(got["rcb2"]) != "\x03" {
		t.Errorf("GetBinaryMap = %v", got)
	}
}

func TestAddrMatchAcceptsNumberOrString(t *testing.T) {
	m := AddrMatch(1001)
	arr, ok := m["$in"].(bson.A)
	if !ok || len(arr) != 2 || arr[0] != 1001 || arr[1] != "1001" {
		t.Errorf("AddrMatch = %v", m)
	}
}

func TestFormatIDHasNoExponentOrTrailingZeros(t *testing.T) {
	for in, want := range map[float64]string{
		1001: "1001", 0: "0", 1e21: "1000000000000000000000",
	} {
		if got := FormatID(in); got != want {
			t.Errorf("FormatID(%v) = %q, want %q", in, got, want)
		}
	}
}
