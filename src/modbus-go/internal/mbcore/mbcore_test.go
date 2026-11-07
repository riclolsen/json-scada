/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbcore

import (
	"math"
	"reflect"
	"testing"
)

var defaults = AsduDefaults{ByteOrder16: "AB", ByteOrder32: "ABCD", ByteOrder64: "ABCDEFGH", ByteOrderStr: "AB", StringEncoding: "latin1"}

func mustOrder(t *testing.T, spec string, width int) Perm {
	t.Helper()
	p, err := ParseByteOrder(spec, width)
	if err != nil {
		t.Fatalf("ParseByteOrder(%q,%d): %v", spec, width, err)
	}
	return p
}

func TestParseByteOrderRejectsNonPermutations(t *testing.T) {
	for _, s := range []string{"ABAB", "ABC", "ZZZZ"} {
		if _, err := ParseByteOrder(s, 4); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestParseByteOrderAliases(t *testing.T) {
	cases := map[string][]int{
		"BE": {0, 1, 2, 3}, "LE": {3, 2, 1, 0}, "SW": {2, 3, 0, 1}, "SB": {1, 0, 3, 2},
		"BIG_ENDIAN": {0, 1, 2, 3}, "LITTLE_ENDIAN": {3, 2, 1, 0}, "": {0, 1, 2, 3},
	}
	for spec, want := range cases {
		if got := mustOrder(t, spec, 4).P; !reflect.DeepEqual(got, want) {
			t.Errorf("%q = %v, want %v", spec, got, want)
		}
	}
}

func TestFloat32OneUnderEveryNamedOrder(t *testing.T) {
	one := []byte{0x3f, 0x80, 0x00, 0x00}
	for _, order := range []string{"BE", "LE", "SW", "SB", "ABCD", "DCBA", "CDAB", "BADC"} {
		p := mustOrder(t, order, 4)
		wire := p.FromCanonical(one)
		res, err := Decode(TypeFloat32, wire, 0, p, nil)
		if err != nil || res.Value.Num != 1.0 || res.Invalid {
			t.Errorf("order %s: %+v %v", order, res, err)
		}
	}
}

func perms(width int) []string {
	letters := []byte("ABCDEFGH"[:width])
	var out []string
	var rec func(rest, cur []byte)
	rec = func(rest, cur []byte) {
		if len(rest) == 0 {
			out = append(out, string(cur))
			return
		}
		for i := range rest {
			r := append(append([]byte{}, rest[:i]...), rest[i+1:]...)
			rec(r, append(append([]byte{}, cur...), rest[i]))
		}
	}
	rec(letters, nil)
	return out
}

func TestRoundTripEveryTypeUnderPermutations(t *testing.T) {
	type tc struct {
		typ    ValueType
		width  int
		values []Value
	}
	cases := []tc{
		{TypeInt16, 2, []Value{Num(0), Num(-1), Num(32767), Num(-32768), Num(1234)}},
		{TypeUint16, 2, []Value{Num(0), Num(65535), Num(40000)}},
		{TypeInt32, 4, []Value{Num(0), Num(-1), Num(2147483647), Num(-2147483648)}},
		{TypeUint32, 4, []Value{Num(0), Num(4294967295), Num(123456)}},
		{TypeFloat32, 4, []Value{Num(1), Num(-3.5), Num(3.14159), Num(0)}},
		{TypeFloat64, 8, []Value{Num(1), Num(-2.718281828), Num(1e10)}},
		{TypeInt64, 8, []Value{{Kind: KindInt64, I64: 0}, {Kind: KindInt64, I64: -1}, {Kind: KindInt64, I64: math.MaxInt64}}},
		{TypeUint64, 8, []Value{{Kind: KindUint64, U64: 0}, {Kind: KindUint64, U64: math.MaxUint64}}},
	}
	for _, c := range cases {
		all := perms(c.width)
		for i, ps := range all {
			if c.width == 8 && i%500 != 0 {
				continue
			}
			p := mustOrder(t, ps, c.width)
			for _, v := range c.values {
				wire, err := Encode(c.typ, v, p, nil)
				if err != nil {
					t.Fatalf("%s %s %v: %v", c.typ, ps, v, err)
				}
				dec, err := Decode(c.typ, wire, 0, p, nil)
				if err != nil {
					t.Fatal(err)
				}
				switch v.Kind {
				case KindInt64, KindUint64:
					if dec.Value != v {
						t.Errorf("%s %s: %+v != %+v", c.typ, ps, dec.Value, v)
					}
				default:
					if math.Abs(dec.Value.Num-v.Num) >= math.Max(1e-3, math.Abs(v.Num)*1e-6) {
						t.Errorf("%s %s v=%v got=%v", c.typ, ps, v.Num, dec.Value.Num)
					}
				}
			}
		}
	}
}

func TestFloatNaNInfInvalid(t *testing.T) {
	p := mustOrder(t, "BE", 4)
	for _, w := range [][]byte{{0x7f, 0xc0, 0, 0}, {0x7f, 0x80, 0, 0}} {
		if r, _ := Decode(TypeFloat32, w, 0, p, nil); !r.Invalid {
			t.Errorf("% X not invalid", w)
		}
	}
}

func TestBcd16(t *testing.T) {
	p := mustOrder(t, "BE", 2)
	if r, _ := Decode(TypeBcd16, []byte{0x12, 0x34}, 0, p, nil); r.Value.Num != 1234 || r.Invalid {
		t.Errorf("decode %+v", r)
	}
	if r, _ := Decode(TypeBcd16, []byte{0x1a, 0x34}, 0, p, nil); !r.Invalid {
		t.Error("invalid nibble accepted")
	}
	enc, _ := Encode(TypeBcd16, Num(5678), p, nil)
	if !reflect.DeepEqual(enc, []byte{0x56, 0x78}) {
		t.Errorf("encode % X", enc)
	}
}

func TestStringRegisterOrders(t *testing.T) {
	p := mustOrder(t, "BE", 2)
	ab, _ := Decode(TypeString, []byte("Hello\x00\x00\x00"), 0, p, &StringOptions{Bytes: 8, Encoding: "latin1", RegOrder: "AB"})
	if ab.Value.Str != "Hello" {
		t.Errorf("AB %q", ab.Value.Str)
	}
	ba, _ := Decode(TypeString, []byte("BADC"), 0, p, &StringOptions{Bytes: 4, Encoding: "latin1", RegOrder: "BA"})
	if ba.Value.Str != "ABCD" {
		t.Errorf("BA %q", ba.Value.Str)
	}
	enc, _ := Encode(TypeString, Str("ABC"), p, &StringOptions{Bytes: 4, Encoding: "latin1", RegOrder: "BA"})
	if string(enc) != "BA\x00C" {
		t.Errorf("encode %q", enc)
	}
}

func TestBitstring16(t *testing.T) {
	p := mustOrder(t, "BE", 2)
	r, _ := Decode(TypeBitstring16, []byte{0, 5}, 0, p, nil)
	if r.Value.Num != 5 || !reflect.DeepEqual(r.JSON, []int{1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Errorf("%+v", r)
	}
}

func TestEncodeOutOfRange(t *testing.T) {
	p := mustOrder(t, "BE", 2)
	if _, err := Encode(TypeUint16, Num(70000), p, nil); err == nil {
		t.Error("uint16 70000 accepted")
	}
	if _, err := Encode(TypeInt16, Num(40000), p, nil); err == nil {
		t.Error("int16 40000 accepted")
	}
	// NaN encodes as 0 for integers, as the Node.js DataView does.
	if b, err := Encode(TypeUint16, Num(math.NaN()), p, nil); err != nil || b[0] != 0 || b[1] != 0 {
		t.Errorf("NaN -> % X %v", b, err)
	}
	// 64-bit integers wrap like BigInt.asUintN.
	p8 := mustOrder(t, "BE", 8)
	b, _ := Encode(TypeUint64, Num(-1), p8, nil)
	if !reflect.DeepEqual(b, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) {
		t.Errorf("uint64 -1 -> % X", b)
	}
}

func TestParseObjectAddress(t *testing.T) {
	ok := map[string]Address{
		"hr:0": {AreaHolding, 0, -1}, "ir:100": {AreaInput, 100, -1}, "co:5": {AreaCoil, 5, -1},
		"di:3": {AreaDiscrete, 3, -1}, "hr:10.b7": {AreaHolding, 10, 7},
		"holding-register:4": {AreaHolding, 4, -1}, "input-register:20": {AreaInput, 20, -1}, "coil:1": {AreaCoil, 1, -1},
	}
	for s, want := range ok {
		got, err := ParseObjectAddress(s, false)
		if err != nil || got != want {
			t.Errorf("%q = %+v %v", s, got, err)
		}
	}
	for _, s := range []string{"xx:5", "hr:99999", "co:5.b3", "hr:5.b16", "40001"} {
		if _, err := ParseObjectAddress(s, false); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
	modicon := map[string]Address{
		"40001": {AreaHolding, 0, -1}, "30011": {AreaInput, 10, -1}, "10005": {AreaDiscrete, 4, -1},
		"00001": {AreaCoil, 0, -1}, "1": {AreaCoil, 0, -1}, "400001": {AreaHolding, 0, -1},
	}
	for s, want := range modicon {
		got, err := ParseObjectAddress(s, true)
		if err != nil || got != want {
			t.Errorf("modicon %q = %+v %v", s, got, err)
		}
	}
}

func TestParseAsdu(t *testing.T) {
	a, _ := ParseAsdu("float32_cdab", AreaHolding, defaults)
	if a.Type != TypeFloat32 || !reflect.DeepEqual(a.Perm.P, []int{2, 3, 0, 1}) || a.RegCount != 2 || a.Label() != "float32_cdab" {
		t.Errorf("%+v", a)
	}
	b, _ := ParseAsdu("int32_le", AreaHolding, defaults)
	if !reflect.DeepEqual(b.Perm.P, []int{3, 2, 1, 0}) {
		t.Errorf("%+v", b)
	}
	c, _ := ParseAsdu("uint64_ghefcdab", AreaHolding, defaults)
	if c.Type != TypeUint64 || c.RegCount != 4 {
		t.Errorf("%+v", c)
	}
	d2 := defaults
	d2.ByteOrder32 = "CDAB"
	e, _ := ParseAsdu("float32", AreaHolding, d2)
	if !reflect.DeepEqual(e.Perm.P, []int{2, 3, 0, 1}) || e.Label() != "float32_cdab" {
		t.Errorf("%+v", e)
	}
	if x, _ := ParseAsdu("", AreaCoil, defaults); x.Type != TypeBool || x.Label() != "bool" {
		t.Errorf("%+v", x)
	}
	if x, _ := ParseAsdu("", AreaHolding, defaults); x.Type != TypeUint16 || x.Label() != "uint16_ab" {
		t.Errorf("%+v", x)
	}
	if x, _ := ParseAsdu("LITTLE_ENDIAN", AreaHolding, defaults); x.Type != TypeUint16 || !reflect.DeepEqual(x.Perm.P, []int{1, 0}) {
		t.Errorf("%+v", x)
	}
	s, _ := ParseAsdu("string:16", AreaHolding, defaults)
	if s.Type != TypeString || s.RegCount != 8 || s.Str.Bytes != 16 || s.Str.RegOrder != "AB" {
		t.Errorf("%+v", s)
	}
	sba, _ := ParseAsdu("string:4_ba", AreaHolding, defaults)
	if sba.Str.RegOrder != "BA" {
		t.Errorf("%+v", sba)
	}
	d3 := defaults
	d3.ByteOrderStr = "BA"
	sdef, _ := ParseAsdu("string:4", AreaHolding, d3)
	if sdef.Str.RegOrder != "BA" {
		t.Errorf("byteOrderStr not applied: %+v", sdef)
	}
	if _, err := ParseAsdu("nonsense", AreaHolding, defaults); err == nil {
		t.Error("nonsense accepted")
	}
}

func TestJSNumberString(t *testing.T) {
	cases := map[float64]string{
		0: "0", 1: "1", -1.5: "-1.5", 12.34: "12.34", 1e21: "1e+21", 1.5e-7: "1.5e-7",
		123456789012: "123456789012", 0.30000000000000004: "0.30000000000000004", 1e-7: "1e-7", 1e-6: "0.000001", 2.5e-6: "0.0000025",
	}
	for f, want := range cases {
		if got := JSNumberString(f); got != want {
			t.Errorf("%v = %q, want %q", f, got, want)
		}
	}
	if JSNumberString(math.NaN()) != "NaN" || JSNumberString(math.Inf(-1)) != "-Infinity" {
		t.Error("special values")
	}
	if JSRound(-2.5) != -2 || JSRound(2.5) != 3 || JSRound(0.49999999999999994) != 0 {
		t.Error("JSRound")
	}
	if JSNumberFromString(" 12 ") != 12 || JSNumberFromString("") != 0 || !math.IsNaN(JSNumberFromString("abc")) || JSNumberFromString("0x10") != 16 {
		t.Error("JSNumberFromString")
	}
}

func TestCRCAndRTUDecoder(t *testing.T) {
	// Read holding registers, unit 1, addr 0, qty 2: 01 03 00 00 00 02 C4 0B
	frame := EncodeRTU(1, []byte{3, 0, 0, 0, 2})
	if Hex(frame, 0) != "01 03 00 00 00 02 C4 0B" {
		t.Fatalf("frame %s", Hex(frame, 0))
	}
	var d RTURequestDecoder
	// split across chunks, with a garbage byte in front
	out := d.Push(append([]byte{0x55}, frame[:3]...))
	out = append(out, d.Push(frame[3:])...)
	if len(out) != 1 || out[0].Unit != 1 || Hex(out[0].PDU, 0) != "03 00 00 00 02" {
		t.Fatalf("decoded %+v", out)
	}
	// FC16 with byte count
	pdu := []byte{16, 0, 10, 0, 2, 4, 0, 1, 0, 2}
	out = d.Push(EncodeRTU(7, pdu))
	if len(out) != 1 || out[0].Unit != 7 || len(out[0].PDU) != len(pdu) {
		t.Fatalf("fc16 %+v", out)
	}
	// unknown function code: idle gap delimits it
	out = d.Push(EncodeRTU(1, []byte{0x11}))
	if len(out) != 0 {
		t.Fatal("fc17 decoded before idle")
	}
	out = d.OnIdle()
	if len(out) != 1 || out[0].PDU[0] != 0x11 {
		t.Fatalf("idle %+v", out)
	}
	if RTUIdleGapMs(9600, 0) != 5 || RTUIdleGapMs(115200, 0) != 2 || RTUIdleGapMs(9600, 20) != 20 {
		t.Error("idle gap")
	}
}

func TestDescribe(t *testing.T) {
	if s := DescribeRequest([]byte{3, 0, 0, 0, 2}); s != "fc=3 READ_HOLDING_REGISTERS addr=0 qty=2" {
		t.Error(s)
	}
	if s := DescribeResponse([]byte{0x83, 2}); s != "fc=3 READ_HOLDING_REGISTERS(exception) code=0x02 ILLEGAL DATA ADDRESS" {
		t.Error(s)
	}
	if s := ExceptionText(3, 2); s != "Modbus exception 02 ILLEGAL DATA ADDRESS (FC 3)" {
		t.Error(s)
	}
}
