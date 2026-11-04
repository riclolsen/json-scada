/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package mbcore holds the protocol-neutral parts of the Modbus drivers: the
// tag address and ASDU grammar, the byte-order codec and the trace formatting.
// It has no MongoDB, JSON-SCADA or transport imports so it stays unit-testable.
//
// # Byte orders
//
// On the wire, register contents are ALWAYS 16-bit words, big-endian within each
// word, in ascending register order. A logical N-byte value occupies ceil(N/2)
// registers whose raw bytes, in wire order, are labelled A B C D E F G H ...
// (A = high byte of the lowest-addressed register).
//
// A non-standard device may store the value with its bytes permuted. That is
// modelled as an explicit byte permutation: reorder the wire bytes into canonical
// big-endian order, then decode as big-endian. Encoding is the exact inverse.
// This single mechanism expresses every byte order (all N! permutations).
//
// The codec is ported from the Node.js driver (src/modbus/src/core/datacodec.ts)
// rather than taken from go-modbus's codec package so that every value, clamp,
// NaN path and string rule stays identical to the driver it replaces.
package mbcore

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValueType is a register value type.
type ValueType string

// The value types.
const (
	TypeBool        ValueType = "bool"
	TypeInt16       ValueType = "int16"
	TypeUint16      ValueType = "uint16"
	TypeInt32       ValueType = "int32"
	TypeUint32      ValueType = "uint32"
	TypeInt64       ValueType = "int64"
	TypeUint64      ValueType = "uint64"
	TypeFloat32     ValueType = "float32"
	TypeFloat64     ValueType = "float64"
	TypeBcd16       ValueType = "bcd16"
	TypeBcd32       ValueType = "bcd32"
	TypeBitstring16 ValueType = "bitstring16"
	TypeString      ValueType = "string"
)

// typeBytes is the width in bytes of each fixed-width register type.
var typeBytes = map[ValueType]int{
	TypeInt16: 2, TypeUint16: 2, TypeBcd16: 2, TypeBitstring16: 2,
	TypeInt32: 4, TypeUint32: 4, TypeBcd32: 4, TypeFloat32: 4,
	TypeInt64: 8, TypeUint64: 8, TypeFloat64: 8,
}

// aliases are the named byte orders per width, as permutation strings.
var aliases = map[int]map[string]string{
	2: {"BE": "AB", "LE": "BA", "SW": "AB", "SB": "BA"},
	4: {"BE": "ABCD", "LE": "DCBA", "SW": "CDAB", "SB": "BADC"},
	8: {"BE": "ABCDEFGH", "LE": "HGFEDCBA", "SW": "GHEFCDAB", "SB": "BADCFEHG"},
}

// synonyms are accepted for migration from the PLC4X/PLC4J drivers.
var synonyms = map[string]string{
	"BIG_ENDIAN": "BE", "LITTLE_ENDIAN": "LE", "REV_ENDIAN": "LE", "": "BE",
}

const letters = "ABCDEFGHIJKLMNOP"

// Perm is a compiled byte permutation: P[i] is the index of the wire byte that
// belongs at canonical (big-endian) position i.
type Perm struct {
	Width int
	P     []int
	inv   []int
}

func newPerm(width int, p []int) Perm {
	inv := make([]int, width)
	for i := 0; i < width; i++ {
		inv[p[i]] = i
	}
	return Perm{Width: width, P: p, inv: inv}
}

// ToCanonical reorders Width wire bytes at wire[off:] into big-endian order.
// Bytes past the end of wire read as zero.
func (p Perm) ToCanonical(wire []byte, off int) []byte {
	out := make([]byte, p.Width)
	for i := 0; i < p.Width; i++ {
		if j := off + p.P[i]; j < len(wire) {
			out[i] = wire[j]
		}
	}
	return out
}

// FromCanonical reorders a big-endian buffer back into wire order.
func (p Perm) FromCanonical(canonical []byte) []byte {
	out := make([]byte, p.Width)
	for i := 0; i < p.Width; i++ {
		out[i] = canonical[p.inv[i]]
	}
	return out
}

// ParseByteOrder compiles a byte-order spec (alias, synonym or explicit
// permutation string) for a value of the given byte width.
func ParseByteOrder(spec string, width int) (Perm, error) {
	if width == 1 {
		return newPerm(1, []int{0}), nil
	}
	s := strings.ToUpper(strings.TrimSpace(spec))
	if v, ok := synonyms[s]; ok {
		s = v
	}
	if t, ok := aliases[width]; ok {
		if v, ok := t[s]; ok {
			s = v
		}
	}
	if len(s) != width {
		return Perm{}, fmt.Errorf("Invalid byte order %q for %d-byte value: expected a %d-letter permutation or a named alias", spec, width, width)
	}
	expected := letters[:width]
	seen := map[int]bool{}
	perm := make([]int, 0, width)
	for _, ch := range s {
		idx := strings.IndexRune(expected, ch)
		if idx < 0 || seen[idx] {
			return Perm{}, fmt.Errorf("Invalid byte order %q: must be a permutation of %q", spec, expected)
		}
		seen[idx] = true
		perm = append(perm, idx)
	}
	return newPerm(width, perm), nil
}

// StringOptions describe a string value.
type StringOptions struct {
	Bytes    int    // characters (bytes); registers = ceil(Bytes/2)
	Encoding string // latin1 | utf8 | ascii
	RegOrder string // AB (default) or BA (bytes swapped within each register)
}

// Kind tells which field of a Value is meaningful.
type Kind int

// The value kinds. Int64 and Uint64 mirror JavaScript BigInt values: they are
// exact and are never scaled.
const (
	KindNumber Kind = iota
	KindInt64
	KindUint64
	KindBool
	KindString
)

// Value is a decoded or to-be-encoded value.
type Value struct {
	Kind Kind
	Num  float64
	I64  int64
	U64  uint64
	Bool bool
	Str  string
}

// Num returns a number value.
func Num(f float64) Value { return Value{Kind: KindNumber, Num: f} }

// Str returns a string value.
func Str(s string) Value { return Value{Kind: KindString, Str: s} }

// Bool returns a boolean value.
func Bool(b bool) Value { return Value{Kind: KindBool, Bool: b} }

// Float is the JavaScript Number(value) of the value.
func (v Value) Float() float64 {
	switch v.Kind {
	case KindInt64:
		return float64(v.I64)
	case KindUint64:
		return float64(v.U64)
	case KindBool:
		if v.Bool {
			return 1
		}
		return 0
	case KindString:
		return JSNumberFromString(v.Str)
	}
	return v.Num
}

// String is the JavaScript String(value) of the value.
func (v Value) String() string {
	switch v.Kind {
	case KindInt64:
		return fmt.Sprintf("%d", v.I64)
	case KindUint64:
		return fmt.Sprintf("%d", v.U64)
	case KindBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case KindString:
		return v.Str
	}
	return JSNumberString(v.Num)
}

// DecodeResult is the outcome of decoding one value.
type DecodeResult struct {
	Value Value
	// Invalid is true when the raw content is not a valid finite value
	// (float NaN/Inf, a BCD nibble above 9).
	Invalid bool
	// JSON is the bit array of a bitstring16 (LSB first), nil otherwise.
	JSON []int
}

// RegisterCount is the number of 16-bit registers a value type occupies.
func RegisterCount(t ValueType, str *StringOptions) int {
	if t == TypeBool {
		return 0
	}
	if t == TypeString {
		if str == nil {
			return 1
		}
		return (str.Bytes + 1) / 2
	}
	return typeBytes[t] / 2
}

// Decode decodes a value of type t from regs (big-endian wire registers),
// starting at register regIndex, applying the byte permutation.
func Decode(t ValueType, regs []byte, regIndex int, perm Perm, str *StringOptions) (DecodeResult, error) {
	off := regIndex * 2
	if t == TypeString {
		if str == nil {
			return DecodeResult{}, errors.New("string type requires StringOptions")
		}
		return decodeString(regs, off, str), nil
	}
	c := perm.ToCanonical(regs, off)
	u16 := func() uint16 { return uint16(c[0])<<8 | uint16(c[1]) }
	u32 := func() uint32 { return uint32(c[0])<<24 | uint32(c[1])<<16 | uint32(c[2])<<8 | uint32(c[3]) }
	u64 := func() uint64 {
		var x uint64
		for i := 0; i < 8; i++ {
			x = x<<8 | uint64(c[i])
		}
		return x
	}
	if len(c) < 2 {
		return DecodeResult{}, fmt.Errorf("Unsupported decode type: %s", t)
	}
	switch t {
	case TypeUint16:
		return DecodeResult{Value: Num(float64(u16()))}, nil
	case TypeInt16:
		return DecodeResult{Value: Num(float64(int16(u16())))}, nil
	case TypeUint32:
		if len(c) < 4 {
			break
		}
		return DecodeResult{Value: Num(float64(u32()))}, nil
	case TypeInt32:
		if len(c) < 4 {
			break
		}
		return DecodeResult{Value: Num(float64(int32(u32())))}, nil
	case TypeUint64:
		if len(c) < 8 {
			break
		}
		return DecodeResult{Value: Value{Kind: KindUint64, U64: u64()}}, nil
	case TypeInt64:
		if len(c) < 8 {
			break
		}
		return DecodeResult{Value: Value{Kind: KindInt64, I64: int64(u64())}}, nil
	case TypeFloat32:
		if len(c) < 4 {
			break
		}
		f := float64(math.Float32frombits(u32()))
		return DecodeResult{Value: Num(f), Invalid: math.IsNaN(f) || math.IsInf(f, 0)}, nil
	case TypeFloat64:
		if len(c) < 8 {
			break
		}
		f := math.Float64frombits(u64())
		return DecodeResult{Value: Num(f), Invalid: math.IsNaN(f) || math.IsInf(f, 0)}, nil
	case TypeBcd16:
		return decodeBcd(c, 2), nil
	case TypeBcd32:
		if len(c) < 4 {
			break
		}
		return decodeBcd(c, 4), nil
	case TypeBitstring16:
		raw := u16()
		bits := make([]int, 16)
		for b := 0; b < 16; b++ {
			bits[b] = int(raw>>b) & 1
		}
		return DecodeResult{Value: Num(float64(raw)), JSON: bits}, nil
	}
	return DecodeResult{}, fmt.Errorf("Unsupported decode type: %s", t)
}

// Encode encodes v as type t into wire register bytes (RegisterCount*2 bytes),
// applying the inverse permutation. Out-of-range integers are an error, as in
// the Node.js driver; NaN becomes 0 for the integer types.
func Encode(t ValueType, v Value, perm Perm, str *StringOptions) ([]byte, error) {
	if t == TypeString {
		if str == nil {
			return nil, errors.New("string type requires StringOptions")
		}
		return encodeString(v.String(), str), nil
	}
	if t == TypeBool {
		return nil, errors.New("bool has no register encoding")
	}
	width, ok := typeBytes[t]
	if !ok {
		return nil, fmt.Errorf("Unsupported encode type: %s", t)
	}
	if perm.Width != width {
		// The ASDU compiles the permutation for the value width; anything else is
		// a programming error, not a configuration one.
		return nil, fmt.Errorf("byte order width %d does not match %s", perm.Width, t)
	}
	c := make([]byte, width)
	put16 := func(x uint16) { c[0], c[1] = byte(x>>8), byte(x) }
	put32 := func(x uint32) {
		c[0], c[1], c[2], c[3] = byte(x>>24), byte(x>>16), byte(x>>8), byte(x)
	}
	put64 := func(x uint64) {
		for i := 7; i >= 0; i-- {
			c[i] = byte(x)
			x >>= 8
		}
	}
	switch t {
	case TypeUint16, TypeBitstring16:
		n, err := clampUint(v, 0xffff)
		if err != nil {
			return nil, err
		}
		put16(uint16(n))
	case TypeInt16:
		n, err := clampInt(v, -0x8000, 0x7fff)
		if err != nil {
			return nil, err
		}
		put16(uint16(int16(n)))
	case TypeUint32:
		n, err := clampUint(v, 0xffffffff)
		if err != nil {
			return nil, err
		}
		put32(uint32(n))
	case TypeInt32:
		n, err := clampInt(v, -0x80000000, 0x7fffffff)
		if err != nil {
			return nil, err
		}
		put32(uint32(int32(n)))
	case TypeUint64, TypeInt64:
		x, err := toWrapped64(v)
		if err != nil {
			return nil, err
		}
		put64(x)
	case TypeFloat32:
		put32(math.Float32bits(float32(v.Float())))
	case TypeFloat64:
		put64(math.Float64bits(v.Float()))
	case TypeBcd16:
		encodeBcd(c, v.Float(), 2)
	case TypeBcd32:
		encodeBcd(c, v.Float(), 4)
	}
	return perm.FromCanonical(c), nil
}

func decodeString(regs []byte, off int, str *StringOptions) DecodeResult {
	numRegs := (str.Bytes + 1) / 2
	raw := make([]byte, numRegs*2)
	if off < len(regs) {
		copy(raw, regs[off:])
	}
	if str.RegOrder == "BA" {
		for i := 0; i < numRegs; i++ {
			raw[i*2], raw[i*2+1] = raw[i*2+1], raw[i*2]
		}
	}
	sliced := raw[:str.Bytes]
	end := len(sliced)
	for i, b := range sliced {
		if b == 0 {
			end = i
			break
		}
	}
	s := decodeBytes(sliced[:end], str.Encoding)
	s = strings.TrimRightFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == 0xFEFF })
	return DecodeResult{Value: Str(s)}
}

func encodeString(value string, str *StringOptions) []byte {
	numRegs := (str.Bytes + 1) / 2
	raw := make([]byte, numRegs*2)
	b := encodeBytes(value, str.Encoding)
	if len(b) > str.Bytes {
		b = b[:str.Bytes]
	}
	copy(raw, b)
	if str.RegOrder == "BA" {
		for i := 0; i < numRegs; i++ {
			raw[i*2], raw[i*2+1] = raw[i*2+1], raw[i*2]
		}
	}
	return raw
}

// decodeBytes is Node's Buffer.toString(encoding).
func decodeBytes(b []byte, enc string) string {
	switch enc {
	case "utf8":
		if utf8.Valid(b) {
			return string(b)
		}
		return strings.ToValidUTF8(string(b), "�")
	case "ascii":
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c & 0x7f)
		}
		return string(r)
	default: // latin1
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c)
		}
		return string(r)
	}
}

// encodeBytes is Node's Buffer.from(string, encoding): latin1 and ascii keep
// the low byte of each UTF-16 code unit.
func encodeBytes(s string, enc string) []byte {
	if enc == "utf8" {
		return []byte(s)
	}
	var out []byte
	for _, r := range s {
		if r > 0xffff {
			// a surrogate pair is two UTF-16 code units
			r -= 0x10000
			out = append(out, byte(0xd800+(r>>10)), byte(0xdc00+(r&0x3ff)))
			continue
		}
		out = append(out, byte(r))
	}
	return out
}

func decodeBcd(c []byte, n int) DecodeResult {
	result := 0.0
	for i := 0; i < n; i++ {
		hi := (c[i] >> 4) & 0x0f
		lo := c[i] & 0x0f
		if hi > 9 || lo > 9 {
			return DecodeResult{Value: Num(0), Invalid: true}
		}
		result = result*100 + float64(hi)*10 + float64(lo)
	}
	return DecodeResult{Value: Num(result)}
}

func encodeBcd(c []byte, value float64, n int) {
	v := math.Max(0, JSRound(value))
	if math.IsNaN(value) {
		v = math.NaN()
	}
	for i := n - 1; i >= 0; i-- {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			c[i] = 0
			continue
		}
		lo := math.Mod(v, 10)
		v = math.Floor(v / 10)
		hi := math.Mod(v, 10)
		v = math.Floor(v / 10)
		c[i] = byte(int(hi))<<4 | byte(int(lo))
	}
}

// RangeError is an out-of-range value refused by the encoder.
type RangeError struct{ Msg string }

func (e *RangeError) Error() string { return e.Msg }

func clampUint(v Value, max float64) (float64, error) {
	n := JSRound(v.Float())
	if math.IsNaN(n) {
		return 0, nil
	}
	if n < 0 || n > max {
		return 0, &RangeError{fmt.Sprintf("value %s out of range [0, %s]", JSNumberString(n), JSNumberString(max))}
	}
	return n, nil
}

func clampInt(v Value, min, max float64) (float64, error) {
	n := JSRound(v.Float())
	if math.IsNaN(n) {
		return 0, nil
	}
	if n < min || n > max {
		return 0, &RangeError{fmt.Sprintf("value %s out of range [%s, %s]", JSNumberString(n), JSNumberString(min), JSNumberString(max))}
	}
	return n, nil
}

// toWrapped64 is BigInt(Math.round(value)) wrapped to 64 bits, as the Node.js
// driver's BigInt.asUintN / asIntN do.
func toWrapped64(v Value) (uint64, error) {
	switch v.Kind {
	case KindInt64:
		return uint64(v.I64), nil
	case KindUint64:
		return v.U64, nil
	case KindBool:
		if v.Bool {
			return 1, nil
		}
		return 0, nil
	}
	f := JSRound(v.Float())
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, &RangeError{fmt.Sprintf("The number %s cannot be converted to a BigInt because it is not an integer", JSNumberString(f))}
	}
	bf := new(big.Float).SetFloat64(f)
	bi, _ := bf.Int(nil)
	mod := new(big.Int).Lsh(big.NewInt(1), 64)
	bi.Mod(bi, mod) // Euclidean: always non-negative
	return bi.Uint64(), nil
}
