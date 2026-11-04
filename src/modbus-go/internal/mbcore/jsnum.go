/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbcore

import (
	"math"
	"strconv"
	"strings"
)

// JSNumberString renders a float64 exactly as JavaScript's String(number)
// does, so valueStringAtSource and the relayed command valueString keep the
// text the Node.js driver wrote ("12.5", "1e+21", "1.5e-7", "NaN").
func JSNumberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0" // also -0
	}
	a := math.Abs(f)
	if a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	s := strconv.FormatFloat(f, 'e', -1, 64) // e.g. 1.5e-07, 1e+21
	mant, exp, _ := strings.Cut(s, "e")
	sign := exp[0]
	digits := strings.TrimLeft(exp[1:], "0")
	if digits == "" {
		digits = "0"
	}
	return mant + "e" + string(sign) + digits
}

// JSNumberFromString is JavaScript's Number(string) for the forms that matter
// here: decimal, exponent and hex literals, surrounding whitespace, "" as 0,
// anything else NaN.
func JSNumberFromString(s string) float64 {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0
	}
	switch t {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	lower := strings.ToLower(t)
	if strings.HasPrefix(lower, "0x") || strings.HasPrefix(lower, "0b") || strings.HasPrefix(lower, "0o") {
		if n, err := strconv.ParseUint(lower[2:], map[byte]int{'x': 16, 'b': 2, 'o': 8}[lower[1]], 64); err == nil {
			return float64(n)
		}
		return math.NaN()
	}
	if strings.ContainsAny(lower, "xpn_") || strings.Contains(lower, "inf") {
		return math.NaN()
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return f
		}
		return math.NaN()
	}
	return f
}

// JSRound is JavaScript's Math.round: halves round towards +Infinity.
func JSRound(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	f := math.Floor(x)
	if x-f >= 0.5 {
		return f + 1
	}
	return f
}
