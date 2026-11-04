/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbcore

// Tag address and ASDU grammar, identical to the Node.js driver.
//
// Object address:  <area>:<offset>[.b<bit>]
//   area   = co | di | hr | ir           (0-based PDU addressing)
//   offset = 0..65535
//   .b<bit>= optional bit 0..15 within a register (hr/ir only)
// Modicon compatibility (when enabled): plain numeric 1-based references such as
//   40001 -> hr:0, 30011 -> ir:10, 00001/1 -> co:0, 10005 -> di:4.
// PLC4X compatibility: holding-register:4 / input-register:4 / coil:4 / discrete-input:4.
//
// ASDU:  <type>[_<byteOrder>]   e.g. float32_cdab, int32_le, uint16, string:16_ba

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Area is a Modbus data area.
type Area string

// The four areas.
const (
	AreaCoil     Area = "co"
	AreaDiscrete Area = "di"
	AreaHolding  Area = "hr"
	AreaInput    Area = "ir"
)

// Address is a parsed object address. Bit is -1 when absent.
type Address struct {
	Area   Area
	Offset int
	Bit    int
}

// HasBit reports whether the address names one bit of a register.
func (a Address) HasBit() bool { return a.Bit >= 0 }

var areaAliases = map[string]Area{
	"co": AreaCoil, "coil": AreaCoil, "coils": AreaCoil,
	"di": AreaDiscrete, "discrete-input": AreaDiscrete, "discrete-inputs": AreaDiscrete, "discreteinput": AreaDiscrete,
	"hr": AreaHolding, "holding-register": AreaHolding, "holding-registers": AreaHolding, "holding": AreaHolding,
	"ir": AreaInput, "input-register": AreaInput, "input-registers": AreaInput, "input": AreaInput,
}

// IsBitArea reports whether the area holds single bits (coils, discrete inputs).
func IsBitArea(a Area) bool { return a == AreaCoil || a == AreaDiscrete }

// IsWritableArea reports whether a master can write the area.
func IsWritableArea(a Area) bool { return a == AreaCoil || a == AreaHolding }

var (
	reAddress = regexp.MustCompile(`^([a-zA-Z-]+)\s*:\s*(\d+)(?:\.b(\d+))?$`)
	reDigits  = regexp.MustCompile(`^\d+$`)
)

// ParseObjectAddress parses an object address. useModicon enables plain
// numeric 1-based references.
func ParseObjectAddress(raw string, useModicon bool) (Address, error) {
	s := strings.TrimSpace(raw)
	if useModicon && reDigits.MatchString(s) {
		return parseModicon(s)
	}
	m := reAddress.FindStringSubmatch(s)
	if m == nil {
		return Address{}, fmt.Errorf("Invalid Modbus object address: %q", raw)
	}
	area, ok := areaAliases[strings.ToLower(m[1])]
	if !ok {
		return Address{}, fmt.Errorf("Unknown Modbus area %q in %q", m[1], raw)
	}
	offset, err := strconv.Atoi(m[2])
	if err != nil || offset < 0 || offset > 0xffff {
		return Address{}, fmt.Errorf("Modbus address offset out of range in %q", raw)
	}
	bit := -1
	if m[3] != "" {
		if IsBitArea(area) {
			return Address{}, fmt.Errorf("Bit suffix .b not allowed on bit area in %q", raw)
		}
		bit, err = strconv.Atoi(m[3])
		if err != nil || bit < 0 || bit > 15 {
			return Address{}, fmt.Errorf("Bit index out of range 0..15 in %q", raw)
		}
	}
	return Address{Area: area, Offset: offset, Bit: bit}, nil
}

func parseModicon(s string) (Address, error) {
	var area Area
	var base int
	var err error
	lead := -1
	if len(s) >= 5 {
		lead = int(s[0] - '0')
	}
	if len(s) >= 5 && (lead == 0 || lead == 1 || lead == 3 || lead == 4) {
		base, err = strconv.Atoi(s[1:])
		switch lead {
		case 0:
			area = AreaCoil
		case 1:
			area = AreaDiscrete
		case 3:
			area = AreaInput
		default:
			area = AreaHolding
		}
	} else {
		// A bare number 1..N is a coil reference.
		area = AreaCoil
		base, err = strconv.Atoi(s)
	}
	offset := base - 1
	if err != nil || offset < 0 || offset > 0xffff {
		return Address{}, fmt.Errorf("Modicon reference out of range: %q", s)
	}
	return Address{Area: area, Offset: offset, Bit: -1}, nil
}

// Asdu is a parsed ASDU: value type plus compiled byte order.
type Asdu struct {
	Type      ValueType
	ByteOrder string // the order spec as resolved, for labels and logs
	Perm      Perm
	Str       *StringOptions
	RegCount  int // registers occupied (0 for bool)
}

// Label is the asduAtSource text: type, then "_" and the byte order lower-cased.
func (a Asdu) Label() string {
	if a.ByteOrder == "" {
		return string(a.Type)
	}
	return string(a.Type) + "_" + strings.ToLower(a.ByteOrder)
}

// AsduDefaults are the connection-level byte orders and string encoding.
type AsduDefaults struct {
	ByteOrder16    string
	ByteOrder32    string
	ByteOrder64    string
	ByteOrderStr   string
	StringEncoding string
}

var typeAliases = map[string]ValueType{
	"bool": TypeBool, "boolean": TypeBool, "bit": TypeBool, "coil": TypeBool, "digital": TypeBool,
	"int16": TypeInt16, "short": TypeInt16, "int": TypeInt16,
	"uint16": TypeUint16, "word": TypeUint16, "uint": TypeUint16,
	"int32": TypeInt32, "dint": TypeInt32,
	"uint32": TypeUint32, "udint": TypeUint32, "dword": TypeUint32,
	"int64": TypeInt64, "lint": TypeInt64,
	"uint64": TypeUint64, "ulint": TypeUint64, "lword": TypeUint64,
	"float32": TypeFloat32, "float": TypeFloat32, "real": TypeFloat32,
	"float64": TypeFloat64, "double": TypeFloat64, "lreal": TypeFloat64,
	"bcd16": TypeBcd16, "bcd32": TypeBcd32,
	"bitstring16": TypeBitstring16, "bits": TypeBitstring16,
	"string": TypeString,
}

var reString = regexp.MustCompile(`^string:(\d+)(.*)$`)

// ParseAsdu parses an ASDU. An empty ASDU means bool for bit areas and uint16
// for register areas; a bare byte-order token (the PLC4X driver used the ASDU
// for endianness only) keeps that default type.
func ParseAsdu(raw string, area Area, d AsduDefaults) (Asdu, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		if IsBitArea(area) {
			s = "bool"
		} else {
			s = "uint16"
		}
	}

	strBytes := -1
	if m := reString.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return Asdu{}, fmt.Errorf("Unknown Modbus ASDU type %q", raw)
		}
		strBytes = n
		s = "string" + m[2]
	}

	typeToken, orderToken := s, ""
	if i := strings.Index(s, "_"); i >= 0 {
		typeToken, orderToken = s[:i], s[i+1:]
	}

	if _, ok := typeAliases[typeToken]; !ok {
		if bare := strings.ToUpper(s); isByteOrderToken(bare) {
			orderToken = bare
			if IsBitArea(area) {
				typeToken = "bool"
			} else {
				typeToken = "uint16"
			}
		}
	}

	t, ok := typeAliases[typeToken]
	if !ok {
		return Asdu{}, fmt.Errorf("Unknown Modbus ASDU type %q", raw)
	}

	if t == TypeBool {
		p, _ := ParseByteOrder("", 1)
		return Asdu{Type: t, Perm: p}, nil
	}

	var str *StringOptions
	var width int
	if t == TypeString {
		n := strBytes
		if n < 0 {
			n = 2
		}
		enc := d.StringEncoding
		if enc == "" {
			enc = "latin1"
		}
		// The register byte order of a string comes from the ASDU suffix, or
		// from byteOrderStr when the ASDU names none.
		ro := strings.ToUpper(orderToken)
		if ro == "" {
			ro = strings.ToUpper(d.ByteOrderStr)
		}
		regOrder := "AB"
		if ro == "BA" || ro == "LE" {
			regOrder = "BA"
		}
		str = &StringOptions{Bytes: n, Encoding: enc, RegOrder: regOrder}
		width = 2
	} else {
		width = RegisterCount(t, nil) * 2
	}

	order := orderToken
	if order == "" {
		order = defaultOrderForWidth(width, d)
	}
	perm, err := ParseByteOrder(order, width)
	if err != nil {
		return Asdu{}, err
	}
	return Asdu{Type: t, ByteOrder: order, Perm: perm, Str: str, RegCount: RegisterCount(t, str)}, nil
}

func defaultOrderForWidth(width int, d AsduDefaults) string {
	pick := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}
	switch width {
	case 2:
		return pick(d.ByteOrder16, "AB")
	case 4:
		return pick(d.ByteOrder32, "ABCD")
	case 8:
		return pick(d.ByteOrder64, "ABCDEFGH")
	}
	return "AB"
}

func isByteOrderToken(s string) bool {
	switch s {
	case "BE", "LE", "SW", "SB", "BIG_ENDIAN", "LITTLE_ENDIAN", "REV_ENDIAN":
		return true
	}
	if s == "" {
		return false
	}
	seen := map[rune]bool{}
	for _, r := range s {
		if r < 'A' || r > 'H' || seen[r] {
			return false
		}
		seen[r] = true
	}
	return true
}
