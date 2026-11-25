/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

import (
	"fmt"
	"math"
	"strings"

	"modbus-go/internal/mbcore"

	"github.com/otfabric/go-sunspec"
)

// Value is what one decoded point publishes.
type Value struct {
	Num     float64
	Str     string
	Invalid bool
}

// isStringType reports the SunSpec types published as string tags.
func isStringType(t string) bool {
	switch t {
	case "string", "ipaddr", "ipv6addr", "eui48":
		return true
	}
	return false
}

// tagType is the json-scada type of a SunSpec point type.
func tagType(t string) string {
	if isStringType(t) {
		return "string"
	}
	return "analog"
}

// skippedType reports point types that get no tag: scale factors (metadata
// of other points, already applied) and padding.
func skippedType(t string) bool { return t == "sunssf" || t == "pad" }

// PointValue converts a decoded point: the scaled value when the point has a
// scale factor, the raw value otherwise; enum and bitfield points carry
// their symbol names in the string. A point the device reports as not
// implemented, or whose scale factor cannot be resolved, is invalid.
func PointValue(p *sunspec.DecodedPoint) Value {
	if p == nil {
		return Value{Invalid: true}
	}
	if regs, ok := p.RawValue.([]uint16); ok {
		var sb strings.Builder
		for _, r := range regs {
			fmt.Fprintf(&sb, "%04X", r)
		}
		return Value{Str: sb.String(), Invalid: true}
	}
	if isStringType(p.Type) {
		s, _ := p.RawValue.(string)
		return Value{Str: s, Invalid: !p.Implemented}
	}
	v := Value{Invalid: !p.Implemented}
	switch {
	case p.ScaledValue != nil:
		v.Num = *p.ScaledValue
	default:
		f, ok := rawFloat(p.RawValue)
		if !ok {
			return Value{Invalid: true}
		}
		v.Num = f
		if p.SFName != "" && p.Implemented {
			v.Invalid = true // the scale factor could not be resolved
		}
	}
	if math.IsNaN(v.Num) || math.IsInf(v.Num, 0) {
		v.Num, v.Invalid = 0, true
	}
	switch {
	case len(p.Symbols) > 0:
		v.Str = strings.Join(p.Symbols, ",")
	default:
		v.Str = mbcore.JSNumberString(v.Num)
	}
	return v
}

func rawFloat(x any) (float64, bool) {
	switch v := x.(type) {
	case int16:
		return float64(v), true
	case uint16:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}
