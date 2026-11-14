/*
 * Formula catalog of the cyclic calculations engine.
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
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

// Every formula code is a contract with README.md and with the MCP server
// catalog (src/mcp-json-scada-db/src/jsonscada/calculations.ts), which mirrors
// each entry to check and preview calculated points. Change all three together.

package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// first formula code reserved for user processes (custom change stream processors)
const userFormulaMin = 100000

type evalFunc func(v []float64, inv []bool) (val float64, invalid bool, transient bool)

type formulaDef struct {
	name string
	// parcel count: exact >= 0 requires exactly that many parcels; otherwise
	// minParcels..maxParcels (maxParcels < 0 means unlimited)
	exact      int
	minParcels int
	maxParcels int
	legacy     bool // site-specific formula kept for existing projects
	eval       evalFunc
}

// parcelsOK reports whether n parcels can be computed by the formula
func (f formulaDef) parcelsOK(n int) bool {
	if f.exact >= 0 {
		return n == f.exact
	}
	return n >= f.minParcels && (f.maxParcels < 0 || n <= f.maxParcels)
}

func (f formulaDef) parcelsText() string {
	if f.exact >= 0 {
		return fmt.Sprintf("exactly %d", f.exact)
	}
	if f.maxParcels < 0 {
		return fmt.Sprintf("%d or more", f.minParcels)
	}
	return fmt.Sprintf("%d to %d", f.minParcels, f.maxParcels)
}

func anyInvalid(inv []bool) bool {
	for _, b := range inv {
		if b {
			return true
		}
	}
	return false
}

func fixed(name string, n int, eval evalFunc) formulaDef {
	return formulaDef{name: name, exact: n, eval: eval}
}

func variadic(name string, min int, eval evalFunc) formulaDef {
	return formulaDef{name: name, exact: -1, minParcels: min, maxParcels: -1, eval: eval}
}

// weighted sum of a fixed number of parcels: sum(coeffs[i] * Pi) + constant;
// invalid if any parcel is invalid
func linear(coeffs []float64, constant float64) formulaDef {
	var sb strings.Builder
	for i, c := range coeffs {
		a := math.Abs(c)
		switch {
		case i == 0 && c < 0:
			sb.WriteString("-")
		case i > 0 && c < 0:
			sb.WriteString(" - ")
		case i > 0:
			sb.WriteString(" + ")
		}
		if a != 1 {
			sb.WriteString(fmt.Sprintf("%g*", a))
		}
		sb.WriteString(fmt.Sprintf("P%d", i+1))
	}
	if constant != 0 {
		sb.WriteString(fmt.Sprintf(" %+g", constant))
	}
	return fixed(sb.String(), len(coeffs), func(v []float64, inv []bool) (float64, bool, bool) {
		s := 0.0
		for i, c := range coeffs {
			s += c * v[i]
		}
		return s + constant, anyInvalid(inv), false
	})
}

// n coefficients of +1 followed by m of -1
func pm(plus, minus int) []float64 {
	c := make([]float64, 0, plus+minus)
	for i := 0; i < plus; i++ {
		c = append(c, 1)
	}
	for i := 0; i < minus; i++ {
		c = append(c, -1)
	}
	return c
}

func ones(n int) []float64 { return pm(n, 0) }

func cat(parts ...[]float64) []float64 {
	var c []float64
	for _, p := range parts {
		c = append(c, p...)
	}
	return c
}

func legacy(f formulaDef) formulaDef {
	f.legacy = true
	return f
}

func rangeCheck(test func(a, lo, hi float64) bool) evalFunc {
	return func(v []float64, inv []bool) (float64, bool, bool) {
		r := 0.0
		if test(v[0], v[1], v[2]) {
			r = 1
		}
		return r, inv[0], false // the limits P2, P3 do not affect quality
	}
}

func boolVal(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// value of the first valid parcel; P1 (invalid) when none is valid
func firstValid(v []float64, inv []bool) (float64, bool, bool) {
	for i := range v {
		if !inv[i] {
			return v[i], false, false
		}
	}
	return v[0], true, false
}

func logicalOr(v []float64, inv []bool) (float64, bool, bool) {
	r := 0.0
	for _, x := range v {
		if x != 0 {
			r = 1
		}
	}
	return r, anyInvalid(inv), false
}

var formulas = map[int]formulaDef{
	1: fixed("Current from P, Q and voltage: (1000/sqrt(3)) * sqrt(P1^2 + P2^2) / P3 (0 when P3 <= 0)", 3,
		func(v []float64, inv []bool) (float64, bool, bool) {
			r := 0.0
			if v[2] > 0 {
				r = 577.35027 * math.Sqrt(v[0]*v[0]+v[1]*v[1]) / v[2]
			}
			return r, anyInvalid(inv), false
		}),
	2: fixed("Power factor: P1 / sqrt(P1^2 + P2^2)", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return v[0] / math.Sqrt(v[0]*v[0]+v[1]*v[1]), anyInvalid(inv), false
		}),
	3: fixed("Apparent power: sqrt(P1^2 + P2^2)", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return math.Sqrt(v[0]*v[0] + v[1]*v[1]), anyInvalid(inv), false
		}),
	4: variadic("Sum: P1 + P2 + ... + Pn", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			s := 0.0
			for _, x := range v {
				s += x
			}
			return s, anyInvalid(inv), false
		}),
	5: fixed("Square root: sqrt(P1)", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return math.Sqrt(v[0]), anyInvalid(inv), false
		}),
	6: variadic("Logical AND", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			r := 1.0
			for _, x := range v {
				if x == 0 {
					r = 0
				}
			}
			return r, anyInvalid(inv), false
		}),
	7: variadic("Logical OR", 1, logicalOr),
	8: {name: "Timer: Unix time in seconds (parcels ignored)", exact: -1, minParcels: 0, maxParcels: -1,
		eval: func(v []float64, inv []bool) (float64, bool, bool) {
			return float64(time.Now().Unix()), false, false
		}},
	9: fixed("Apparent power from current and voltage: P1 * P2 * sqrt(3) / 1000", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return v[0] * v[1] * math.Sqrt(3) / 1000, anyInvalid(inv), false
		}),
	10: variadic("Negative sum: -(P1 + P2 + ... + Pn)", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			s := 0.0
			for _, x := range v {
				s -= x
			}
			return s, anyInvalid(inv), false
		}),
	11: legacy(linear(cat(ones(23), []float64{0.72}), 0)),
	13: legacy(linear(cat(
		pm(8, 1),          // P1..P8 - P9
		ones(3),           // P10..P12
		[]float64{0.52},   // P13
		pm(2, 2),          // P14 + P15 - P16 - P17
		ones(5),           // P18..P22
		[]float64{-1, -1}, // - P23 - P24
		ones(9),           // P25..P33
		[]float64{0.2},    // P34
	), -1)),
	14: fixed("Scale: (P1 * 10) / 6", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return (v[0] * 10) / 6, anyInvalid(inv), false
		}),
	15: linear(pm(1, 1), 0),
	16: linear(pm(1, 2), 0),
	17: linear(pm(1, 3), 0),
	18: linear(pm(1, 5), 0),
	19: linear(pm(2, 1), 0),
	20: linear(pm(3, 3), 0),
	21: linear(pm(4, 5), 0),
	22: linear(pm(6, 3), 0),
	23: linear(pm(10, 1), 0),
	24: linear(pm(3, 6), 0),
	26: fixed("Band selector: P2, but 0 when |P1| > 1.4 and 1 when |P1| <= 0.5", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			r := v[1]
			if math.Abs(v[0]) > 1.4 {
				r = 0
			}
			if math.Abs(v[0]) <= 0.5 {
				r = 1
			}
			return r, anyInvalid(inv), false
		}),
	50: variadic("Choice: value of the first valid parcel", 1, firstValid),
	51: variadic("Choice: value of the first valid parcel (same as 50)", 1, firstValid),
	52: variadic("Any parcel valid: 1 if any parcel is valid, else 0", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			r := 0.0
			for _, b := range inv {
				if !b {
					r = 1
				}
			}
			return r, false, false
		}),
	53: variadic("Span: max(P1..Pn) - min(P1..Pn)", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			max, min := v[0], v[0]
			for _, x := range v {
				max = math.Max(max, x)
				min = math.Min(min, x)
			}
			return max - min, anyInvalid(inv), false
		}),
	54: fixed("Double point from two single points (P1 = ON, P2 = OFF indication)", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			r, bad, tr := v[0], false, false
			if v[0] == 0 && v[1] != 0 {
				r = 0
			}
			if v[0] != 0 && v[1] == 0 {
				r = 1
			}
			if v[0] == v[1] { // both ON or both OFF: transition or failure
				bad, tr = true, true
			}
			return r, bad || anyInvalid(inv), tr
		}),
	55: fixed("Division: P1 / P2", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			if v[1] == 0 { // division by zero
				switch {
				case v[0] == 0:
					return 0, anyInvalid(inv), false
				case v[0] > 0:
					return math.MaxFloat64, anyInvalid(inv), false
				default:
					return -math.MaxFloat64, anyInvalid(inv), false
				}
			}
			return v[0] / v[1], anyInvalid(inv), false
		}),
	200: linear(pm(1, 7), 0),
	201: linear(pm(8, 3), 0),
	202: linear([]float64{60, 1}, 0),
	203: fixed("Selector: P1 != 0 ? P3 : P2", 3,
		func(v []float64, inv []bool) (float64, bool, bool) {
			if v[0] != 0 {
				return v[2], inv[2], false
			}
			return v[1], inv[1], false
		}),
	204: fixed("Half: P1 / 2", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return v[0] / 2, anyInvalid(inv), false
		}),
	205: linear(pm(2, 3), 0),
	206: linear(pm(1, 8), 0),
	207: linear(pm(2, 4), 0),
	208: linear(pm(1, 4), 0),
	209: linear(pm(2, 7), 0),
	210: linear(pm(1, 6), 0),
	211: linear(pm(5, 4), 0),
	213: linear(pm(3, 1), 0),
	214: linear(pm(4, 1), 0),
	215: linear(pm(2, 2), 0),
	216: fixed("Range check: 1 if P2 <= P1 <= P3, else 0", 3,
		rangeCheck(func(a, lo, hi float64) bool { return lo <= a && a <= hi })),
	217: linear(pm(5, 1), 0),
	218: fixed("Range check: 1 if P2 < P1 <= P3, else 0", 3,
		rangeCheck(func(a, lo, hi float64) bool { return lo < a && a <= hi })),
	219: fixed("Range check: 1 if P2 <= P1 < P3, else 0", 3,
		rangeCheck(func(a, lo, hi float64) bool { return lo <= a && a < hi })),
	220: fixed("Minimum: min(P1, P2)", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return math.Min(v[0], v[1]), anyInvalid(inv), false
		}),
	221: fixed("Maximum: max(P1, P2)", 2,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return math.Max(v[0], v[1]), anyInvalid(inv), false
		}),
	222: linear(pm(3, 4), 0),
	223: linear(pm(3, 5), 0),
	224: linear(pm(6, 1), 0),
	225: legacy(formulaDef{name: "Constant 1 (parcels ignored)", exact: -1, minParcels: 0, maxParcels: 1,
		eval: func(v []float64, inv []bool) (float64, bool, bool) { return 1, false, false }}),
	226: linear(pm(2, 6), 0),
	227: linear(pm(3, 2), 0),
	228: linear(pm(3, 9), 0),
	229: linear(pm(9, 3), 0),
	230: linear(pm(9, 1), 0),
	231: fixed("Negative part: P1 > 0 ? 0 : P1", 1,
		func(v []float64, inv []bool) (float64, bool, bool) {
			return math.Min(v[0], 0), anyInvalid(inv), false
		}),
	232:   linear(pm(5, 3), 0),
	233:   linear(pm(7, 4), 0),
	500:   legacy(variadic("Logical OR (same as 7)", 1, logicalOr)),
	680:   legacy(linear(pm(3, 2), 0)),
	5000:  legacy(linear([]float64{1, 1, 1, -1, -0.6, 1}, 0)),
	5022:  legacy(linear([]float64{1, 0.65}, 0)),
	5041:  legacy(linear([]float64{1, 1, -1, 1, -1}, 0)),
	5043:  legacy(linear([]float64{1, 1, 1, 1, -1, -1, 1, -1, -1}, 0)),
	5172:  legacy(linear([]float64{-0.72}, 0)),
	5173:  legacy(linear([]float64{0.38}, 0)),
	5174:  legacy(linear([]float64{0.14}, 0)),
	5406:  legacy(linear([]float64{-1, 1, 1, 1, 1, 1, -1, -1, 1, 1, 1}, 0)),
	5711:  legacy(linear(pm(0, 4), 0)),
	7374:  legacy(linear(pm(1, 11), 0)),
	8055:  legacy(linear(pm(2, 6), 0)),
	25673: legacy(linear(pm(1, 1), 0)),
}

// calculate computes a formula; ok is false when the formula is unknown or the
// parcel count does not fit. NaN and infinite results become 0, invalid.
func calculate(code int, v []float64, inv []bool) (val float64, invalid bool, transient bool, ok bool) {
	f, found := formulas[code]
	if !found || !f.parcelsOK(len(v)) {
		return 0, true, false, false
	}
	val, invalid, transient = f.eval(v, inv)
	if math.IsNaN(val) || math.IsInf(val, 0) {
		val, invalid = 0, true
	}
	return val, invalid, transient, true
}
