/*
 * Shared {json:scada} driver support library, in Go.
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

// Permissive accessors for bson.M documents. Ports of getDouble, getBoolean,
// getString and getDate of the C++ drivers and of the C# BsonDoubleSerializer:
// they accept any numeric BSON type where a number is wanted, because the
// collections hold a mixture of int32, int64, double and hand-edited strings
// written by different drivers over the years.
//
// This file replaces three parallel families that had drifted apart:
// GetDouble/GetInt/... (dnp3-go), ToFloat64/ToU32/... (iec60870-5) and
// mFloat/mInt/... (the flat drivers). The Get* shape wins because it takes
// the document and the key together, which removes the doc[key] boilerplate
// at every call site.
//
// DELIBERATE WIDENING: the dnp3-go family did not coerce strings or
// Decimal128 to numbers, and returned the caller's default for them. The
// iec60870-5 and flat-driver families did coerce. The union coerces, because
// that is what the C# reference implementations do and what a hand-edited
// document needs. For dnp3-go this means a numeric field stored as a string
// now reads as its number instead of as the default. There is no case where
// the old result was the intended one.

package jsmongo

import (
	"math"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ToFloat coerces a decoded BSON value to float64, returning def for anything
// it cannot read as a number.
func ToFloat(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		// TrimSpace matches iec60870-5's model.ParseU32, which tolerated
		// surrounding whitespace in a hand-edited document.
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return f
		}
	case bson.Decimal128:
		if f, err := strconv.ParseFloat(x.String(), 64); err == nil {
			return f
		}
	}
	return def
}

// ToBool coerces a decoded BSON value to bool, accepting a number.
func ToBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return ToFloat(v, 0) != 0
}

// ToString coerces a decoded BSON value to string. Like GetString it does not
// render numbers.
func ToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ToTime coerces a decoded BSON value to time.Time, zero when absent.
func ToTime(v any) time.Time {
	switch x := v.(type) {
	case bson.DateTime:
		return x.Time()
	case time.Time:
		return x
	}
	return time.Time{}
}

// GetDouble reads a numeric field.
func GetDouble(doc bson.M, key string, def float64) float64 {
	v, ok := doc[key]
	if !ok || v == nil {
		return def
	}
	return ToFloat(v, def)
}

// GetInt is GetDouble truncated, which is how every numeric configuration
// field of the C++ drivers is read.
func GetInt(doc bson.M, key string, def int) int {
	return int(GetDouble(doc, key, float64(def)))
}

// GetU32 reads an address-like field (ASDU type, common address, object
// address), clamping to [0, 2^32-1]. Same rules as iec60870-5's model.U32.
func GetU32(doc bson.M, key string, def uint32) uint32 {
	v, ok := doc[key]
	if !ok || v == nil {
		return def
	}
	f := ToFloat(v, math.NaN())
	if math.IsNaN(f) || f <= 0 {
		if math.IsNaN(f) {
			return def
		}
		return 0
	}
	if f >= math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(f)
}

// GetBool reads a boolean field, accepting a number as well.
func GetBool(doc bson.M, key string, def bool) bool {
	v, ok := doc[key]
	if !ok || v == nil {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	d := float64(0)
	if def {
		d = 1
	}
	return ToFloat(v, d) != 0
}

// GetString reads a string field.
//
// parity: unlike the C++ server's getString it does not render numbers,
// because every caller that wanted that was logging an _id and got an empty
// string instead; those call sites format the number themselves with
// FormatID.
func GetString(doc bson.M, key string, def string) string {
	v, ok := doc[key]
	if !ok || v == nil {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

// GetTime reads a date field, returning the zero time when absent.
func GetTime(doc bson.M, key string) time.Time {
	v, ok := doc[key]
	if !ok || v == nil {
		return time.Time{}
	}
	switch x := v.(type) {
	case bson.DateTime:
		return x.Time()
	case time.Time:
		return x
	case int64:
		return time.UnixMilli(x)
	}
	return time.Time{}
}

// GetDateMs reads a date field as milliseconds since the epoch.
func GetDateMs(doc bson.M, key string, def int64) int64 {
	v, ok := doc[key]
	if !ok || v == nil {
		return def
	}
	switch x := v.(type) {
	case bson.DateTime:
		return int64(x)
	case time.Time:
		return x.UnixMilli()
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	}
	return def
}

// GetStringArray reads an array field, keeping only its string elements.
func GetStringArray(doc bson.M, key string) []string {
	arr, ok := doc[key].(bson.A)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		if s, ok := el.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// GetDocArray reads an array of sub-documents, which is how rangeScans and
// protocolDestinations are stored.
func GetDocArray(doc bson.M, key string) []bson.M {
	arr, ok := doc[key].(bson.A)
	if !ok {
		return nil
	}
	out := make([]bson.M, 0, len(arr))
	for _, el := range arr {
		switch d := el.(type) {
		case bson.M:
			out = append(out, d)
		case bson.D:
			m := make(bson.M, len(d))
			for _, e := range d {
				m[e.Key] = e.Value
			}
			out = append(out, m)
		}
	}
	return out
}

// GetBinaryMap reads a sub-document of binary values, which is how
// iec61850_client persists the last report entry id per RCB.
func GetBinaryMap(doc bson.M, key string) map[string][]byte {
	sub, ok := doc[key].(bson.M)
	if !ok {
		return nil
	}
	out := make(map[string][]byte, len(sub))
	for k, v := range sub {
		switch b := v.(type) {
		case bson.Binary:
			out[k] = b.Data
		case []byte:
			out[k] = b
		}
	}
	return out
}

// AddrMatch builds a query predicate for an address-like protocol field that
// matches the value whether it is stored as a number or as its decimal string.
func AddrMatch(v int) bson.M {
	return bson.M{"$in": bson.A{v, strconv.Itoa(v)}}
}

// FormatID renders a tag _id for a log line. The ids are doubles holding whole
// numbers, so they read better without an exponent or a trailing ".000000".
func FormatID(id float64) string {
	return strconv.FormatFloat(id, 'f', -1, 64)
}
