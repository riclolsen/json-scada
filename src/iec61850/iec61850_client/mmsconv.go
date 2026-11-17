/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

// MMS value conversions. Every function here is a port of the equivalent in
// the C# driver (src/iec61850_client/AsduReceiveHandler.cs), including its
// quirks: the values written to MongoDB must match what the C# driver would
// have written for the same MMS value.

package main

import (
	"encoding/hex"
	"math"
	"strconv"
	"strings"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// fcScanNames is the functional constraint list in libiec61850's enum order
// (values 0..17), which is what the C# driver scanned for in a data
// reference. It excludes GO/GS, which go-iec61850's model.FC also has.
var fcScanNames = [...]string{
	"ST", "MX", "SP", "SV", "CF", "DC", "SG", "SE", "SR",
	"OR", "BL", "EX", "CO", "US", "MS", "RP", "BR", "LG",
}

// getRefFc splits an MMS-style data reference into an object reference and
// a functional constraint: "LD/LN$ST$Pos" -> ("LD/LN.Pos", ST).
// Unrecognised or absent FCs yield FCNone and the remaining '$' separators
// still become '.', exactly as in the C# driver.
func getRefFc(dataRef string) (string, model.FC) {
	for i, name := range fcScanNames {
		sfc := "$" + name + "$"
		if strings.Contains(dataRef, sfc) {
			fc := fcFromScanIndex(i)
			return strings.ReplaceAll(strings.Replace(dataRef, sfc, ".", 1), "$", "."), fc
		}
	}
	return strings.ReplaceAll(dataRef, "$", "."), model.FCNone
}

// getRefFc2 is the bracket form used for dataset member names:
// "LD/LN.Pos[ST]" -> ("LD/LN.Pos", ST).
func getRefFc2(dataRef string) (string, model.FC) {
	for i, name := range fcScanNames {
		sfc := "[" + name + "]"
		if strings.Contains(dataRef, sfc) {
			return strings.Replace(dataRef, sfc, "", 1), fcFromScanIndex(i)
		}
	}
	return dataRef, model.FCNone
}

// fcFromScanIndex maps a position in fcScanNames to a model.FC. Every name
// in the list parses; FCNone is only a guard.
func fcFromScanIndex(i int) model.FC {
	fc, err := model.ParseFC(fcScanNames[i])
	if err != nil {
		return model.FCNone
	}
	return fc
}

// bitStringToUint32BE is libiec61850's MmsValue_getBitStringAsIntegerBigEndian:
// the first bit is the most significant.
func bitStringToUint32BE(v *mms.Value) uint32 {
	if v == nil || v.Type() != mms.TypeBitString {
		return 0
	}
	n := v.BitLen()
	var out uint32
	for i := 0; i < n; i++ {
		if v.Bit(i) {
			out |= 1 << uint(n-1-i)
		}
	}
	return out
}

// bitStringToUint32 is MmsValue_getBitStringAsInteger: bit i has weight 2^i.
func bitStringToUint32(v *mms.Value) uint32 {
	if v == nil || v.Type() != mms.TypeBitString {
		return 0
	}
	var out uint32
	for i := 0; i < v.BitLen(); i++ {
		if v.Bit(i) {
			out |= 1 << uint(i)
		}
	}
	return out
}

// bitStringAsString renders a bit string as "0101", like the C# wrapper's
// GetBitStringAsString.
func bitStringAsString(v *mms.Value) string {
	if v == nil || v.Type() != mms.TypeBitString {
		return ""
	}
	var b strings.Builder
	for i := 0; i < v.BitLen(); i++ {
		if v.Bit(i) {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// mmsTestDoubleStateFailed reports a double-point in an inconsistent state
// (a 2-bit string with both bits equal: 00 intermediate or 11 bad).
func mmsTestDoubleStateFailed(v *mms.Value) bool {
	return v != nil && v.Type() == mms.TypeBitString && v.BitLen() == 2 && v.Bit(0) == v.Bit(1)
}

// mmsTestDoubleStateTransient reports a double-point in the intermediate
// (transient) position, 00.
func mmsTestDoubleStateTransient(v *mms.Value) bool {
	return v != nil && v.Type() == mms.TypeBitString && v.BitLen() == 2 && !v.Bit(0) && !v.Bit(1)
}

// mmsGetQualityFailed looks for an IEC 61850 Quality bit string inside an
// MMS value and reports whether its validity is other than good.
func mmsGetQualityFailed(v *mms.Value) bool {
	if v == nil {
		return false
	}
	switch v.Type() {
	case mms.TypeStructure:
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			if e != nil && e.Type() == mms.TypeBitString && e.BitLen() > 2 {
				return model.QualityFromValue(e).Validity() != model.ValidityGood
			}
		}
		// No quality attribute here: look inside the first member.
		if v.Len() > 0 {
			return mmsGetQualityFailed(v.Index(0))
		}
	case mms.TypeBitString:
		if mmsTestDoubleStateFailed(v) {
			return true
		}
		return bitStringToUint32BE(v) != 0
	}
	return false
}

// structQualityFailed reads the quality of a structured object: its q
// attribute when the attribute names are known, otherwise the first quality
// bit string found (mmsGetQualityFailed).
func structQualityFailed(v *mms.Value, childs []string) bool {
	if v != nil && v.Type() == mms.TypeStructure && len(childs) == v.Len() {
		for i, name := range childs {
			if name != "q" {
				continue
			}
			if q := v.Index(i); q != nil && q.Type() == mms.TypeBitString && q.BitLen() > 2 {
				return model.QualityFromValue(q).Validity() != model.ValidityGood
			}
		}
	}
	return mmsGetQualityFailed(v)
}

// mmsGetTimestamp looks for an IEC 61850 UtcTime inside an MMS value and
// returns it as milliseconds since the Unix epoch, or 0 when absent.
func mmsGetTimestamp(v *mms.Value) uint64 {
	if v == nil {
		return 0
	}
	switch v.Type() {
	case mms.TypeStructure:
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			if e != nil && e.Type() == mms.TypeUTCTime {
				return utcMillis(e)
			}
		}
		if v.Len() > 0 {
			return mmsGetTimestamp(v.Index(0))
		}
	case mms.TypeUTCTime:
		return utcMillis(v)
	}
	return 0
}

func utcMillis(v *mms.Value) uint64 {
	t := v.Time()
	if t.IsZero() {
		return 0
	}
	ms := t.UnixMilli()
	if ms < 0 {
		return 0
	}
	return uint64(ms)
}

// doubleStateValue maps a 2-bit double-point position to 0 or 1, the way
// the C# driver does by switching on the rendered bit string.
func doubleStateValue(v *mms.Value) float64 {
	switch bitStringAsString(v) {
	case "00", "01":
		return 0
	case "10", "11":
		return 1
	}
	return 0
}

// valueAttributeNames are the data attributes that carry the value of a
// common data class, most specific first. An MMS structure arrives without
// names, so the names come from the type description of the object; see
// namedValueAttribute.
var valueAttributeNames = []string{"stVal", "mag", "cVal", "mxVal", "instMag", "instCVal", "setVal", "general"}

// namedValueAttribute picks the value attribute of a structured object by
// name, given the attribute names of its type in order. It is what makes a
// double point read its position rather than, say, the operation counter a
// controllable object also carries under ST.
//
// The names are only trusted when there are exactly as many of them as the
// value has members, so a mismatch falls back to the type scan instead of
// reading the wrong member.
func namedValueAttribute(v *mms.Value, childs []string) *mms.Value {
	if v == nil || v.Type() != mms.TypeStructure || len(childs) != v.Len() {
		return nil
	}
	for _, want := range valueAttributeNames {
		for i, name := range childs {
			if name == want {
				return v.Index(i)
			}
		}
	}
	return nil
}

// doublePointAttribute returns the value attribute of a structured object
// when it is a double-point position (a two-bit string), and nil otherwise.
// It prefers the named attribute and falls back to the first two-bit
// member, which is what a double point carries.
func doublePointAttribute(v *mms.Value, childs []string) *mms.Value {
	if v == nil || v.Type() != mms.TypeStructure {
		return nil
	}
	if named := namedValueAttribute(v, childs); named != nil {
		if named.Type() == mms.TypeBitString && named.BitLen() == 2 {
			return named
		}
		return nil
	}
	for i := 0; i < v.Len(); i++ {
		if e := v.Index(i); e != nil && e.Type() == mms.TypeBitString && e.BitLen() == 2 {
			return e
		}
	}
	return nil
}

// mmsGetNumericVal looks for a numeric value inside an MMS value.
// isBinary reports that the value is a boolean or a double-point position,
// which makes the tag a digital one.
func mmsGetNumericVal(v *mms.Value) (val float64, isBinary bool) {
	if v == nil {
		return 0, false
	}
	switch v.Type() {
	case mms.TypeStructure:
		// A status attribute first: a controllable object carries counters
		// under the same constraint as its position, and the position is
		// the value. Only then the numeric attributes.
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			if e == nil {
				continue
			}
			switch {
			case e.Type() == mms.TypeBoolean:
				return mmsGetDoubleVal(e)
			case e.Type() == mms.TypeBitString && e.BitLen() == 2:
				return doubleStateValue(e), true
			}
		}
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			if e == nil {
				continue
			}
			switch e.Type() {
			case mms.TypeFloat32, mms.TypeFloat64:
				return e.Float64(), false
			case mms.TypeInteger:
				return float64(e.Int64()), false
			case mms.TypeUnsigned:
				return float64(uint32(e.Uint64())), false
			}
		}
		if v.Len() > 0 {
			return mmsGetNumericVal(v.Index(0))
		}
	case mms.TypeFloat32, mms.TypeFloat64:
		return v.Float64(), false
	case mms.TypeInteger:
		return float64(v.Int64()), false
	case mms.TypeUnsigned:
		return float64(uint32(v.Uint64())), false
	case mms.TypeBoolean:
		if v.Bool() {
			return 1, true
		}
		return 0, true
	case mms.TypeBitString:
		if v.BitLen() == 2 {
			return doubleStateValue(v), true
		}
		return float64(bitStringToUint32BE(v)), false
	}
	return 0, false
}

// mmsGetDoubleVal converts any MMS value into a double.
func mmsGetDoubleVal(v *mms.Value) (val float64, isBinary bool) {
	if v == nil {
		return 0, false
	}
	switch v.Type() {
	case mms.TypeStructure:
		return mmsGetNumericVal(v)
	case mms.TypeBitString:
		if v.BitLen() == 2 {
			return doubleStateValue(v), true
		}
		return float64(bitStringToUint32BE(v)), false
	case mms.TypeBoolean:
		if v.Bool() {
			return 1, true
		}
		return 0, true
	case mms.TypeOctetString:
		b := v.Bytes()
		if len(b) > 0 {
			return float64(b[0]), false
		}
		return 0, false
	case mms.TypeFloat32, mms.TypeFloat64:
		return v.Float64(), false
	case mms.TypeInteger:
		return float64(v.Int64()), false
	case mms.TypeUnsigned:
		return float64(uint32(v.Uint64())), false
	case mms.TypeUTCTime:
		return float64(utcMillis(v)), false
	case mms.TypeArray:
		if v.Len() > 0 {
			return mmsGetNumericVal(v.Index(0))
		}
	}
	return 0, false
}

// mmsToString renders a leaf MMS value the way the .NET binding's
// MmsValue.ToString() does (libiec61850 dotnet/IEC61850forCSharp/MmsValue.cs:1111).
// The rendering ends up in valueBsonAtSource, so it has to match.
func mmsToString(v *mms.Value) string {
	if v == nil {
		return ""
	}
	switch v.Type() {
	case mms.TypeVisibleString, mms.TypeMMSString:
		return v.Text()
	case mms.TypeBoolean:
		if v.Bool() {
			return "True"
		}
		return "False"
	case mms.TypeInteger:
		return strconv.FormatInt(v.Int64(), 10)
	case mms.TypeUnsigned:
		return strconv.FormatUint(uint64(uint32(v.Uint64())), 10)
	case mms.TypeFloat32, mms.TypeFloat64:
		return formatDouble(v.Float64())
	case mms.TypeUTCTime, mms.TypeBinaryTime:
		t := v.Time()
		if t.IsZero() {
			return ""
		}
		// DateTimeOffset.ToString() under en-US, offset zero.
		return t.UTC().Format("1/2/2006 3:04:05 PM -07:00")
	case mms.TypeOctetString:
		b := v.Bytes()
		parts := make([]string, len(b))
		for i, x := range b {
			parts[i] = strings.ToUpper(hex.EncodeToString([]byte{x}))
		}
		return strings.Join(parts, "-")
	case mms.TypeBitString:
		return bitStringAsString(v)
	case mms.TypeDataAccessError:
		if code, isErr := v.AccessError(); isErr {
			return code.String()
		}
	}
	return v.Text()
}

// formatDouble renders a float the way .NET's double.ToString() does under
// en-US: shortest round-trip representation.
func formatDouble(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	return strconv.FormatFloat(f, 'G', -1, 64)
}

// mmsGetStringValue renders an MMS value as the JSON fragment stored in
// sourceDataUpdate.valueBsonAtSource. Leaves are quoted strings; structures
// and arrays of more than one member become arrays. A one-member container
// renders as its single member, which is what the C# driver does.
//
// Unlike the C# driver this emits strictly valid JSON: no trailing comma,
// and string contents are escaped (deviation D3).
func mmsGetStringValue(v *mms.Value) string {
	if v == nil {
		return `""`
	}
	switch v.Type() {
	case mms.TypeStructure, mms.TypeArray:
		if v.Len() == 1 {
			e := v.Index(0)
			if e != nil && e.Type() != mms.TypeStructure && e.Type() != mms.TypeArray {
				return quoteJSON(mmsToString(e))
			}
			return mmsGetStringValue(e)
		}
		var b strings.Builder
		b.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			e := v.Index(i)
			if e != nil && e.Type() != mms.TypeStructure && e.Type() != mms.TypeArray {
				b.WriteString(quoteJSON(mmsToString(e)))
			} else {
				b.WriteString(mmsGetStringValue(e))
			}
		}
		b.WriteByte(']')
		return b.String()
	}
	return quoteJSON(mmsToString(v))
}

// quoteJSON quotes and escapes a string for embedding in JSON.
func quoteJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u`)
				b.WriteString(strings.ToLower(hex.EncodeToString([]byte{0, byte(r)})))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// mmsTypeName renders an MMS type with the name libiec61850's MmsType enum
// uses, which is what the C# driver stored in asduAtSource.
func mmsTypeName(t mms.Type) string {
	switch t {
	case mms.TypeArray:
		return "MMS_ARRAY"
	case mms.TypeStructure:
		return "MMS_STRUCTURE"
	case mms.TypeBoolean:
		return "MMS_BOOLEAN"
	case mms.TypeBitString:
		return "MMS_BIT_STRING"
	case mms.TypeInteger:
		return "MMS_INTEGER"
	case mms.TypeUnsigned:
		return "MMS_UNSIGNED"
	case mms.TypeFloat32, mms.TypeFloat64:
		return "MMS_FLOAT"
	case mms.TypeOctetString:
		return "MMS_OCTET_STRING"
	case mms.TypeVisibleString:
		return "MMS_VISIBLE_STRING"
	case mms.TypeGeneralizedTime:
		return "MMS_GENERALIZED_TIME"
	case mms.TypeBinaryTime:
		return "MMS_BINARY_TIME"
	case mms.TypeMMSString:
		return "MMS_STRING"
	case mms.TypeUTCTime:
		return "MMS_UTC_TIME"
	}
	return "MMS_DATA_ACCESS_ERROR"
}

// entryIDString renders a report EntryID like BitConverter.ToString does:
// uppercase hex bytes separated by '-'.
func entryIDString(b []byte) string {
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = strings.ToUpper(hex.EncodeToString([]byte{x}))
	}
	return strings.Join(parts, "-")
}
