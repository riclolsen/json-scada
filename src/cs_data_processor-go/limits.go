package main

// Out-of-range alarm levels of analog tags.
//
// Port of ../cs_data_processor/limits.js: change both together and keep
// limits_test.go and limits.test.js with the same expected values.
//
// alarmRange: 1, 2, 3 = above hiLimit, hihiLimit, hihihiLimit;
// -1, -2, -3 = below loLimit, loloLimit, lololoLimit; 0 = normal.
// A level whose limit is missing, null or not a number is not checked.

import "math"

var (
	hiLimitFields = [...]string{"hiLimit", "hihiLimit", "hihihiLimit"}
	loLimitFields = [...]string{"loLimit", "loloLimit", "lololoLimit"}
)

// limitOf returns a limit as a number, false when the level is not configured.
func limitOf(d rawDoc, key string) (float64, bool) {
	v, ok := d.num(key)
	if !ok || math.IsNaN(v) {
		return 0, false
	}
	return v, true
}

// hasLimits reports whether the document has at least one limit configured.
func hasLimits(d rawDoc) bool {
	for _, k := range hiLimitFields {
		if _, ok := limitOf(d, k); ok {
			return true
		}
	}
	for _, k := range loLimitFields {
		if _, ok := limitOf(d, k); ok {
			return true
		}
	}
	return false
}

// limitLevel returns the most severe level the value violates, the limits
// moved by offset (outwards when positive). inclusive: a value equal to a
// moved limit violates it.
func limitLevel(d rawDoc, value, offset float64, inclusive bool) int {
	for i := len(hiLimitFields) - 1; i >= 0; i-- {
		l, ok := limitOf(d, hiLimitFields[i])
		if !ok {
			continue
		}
		if (inclusive && value >= l+offset) || (!inclusive && value > l+offset) {
			return i + 1
		}
	}
	for i := len(loLimitFields) - 1; i >= 0; i-- {
		l, ok := limitOf(d, loLimitFields[i])
		if !ok {
			continue
		}
		if (inclusive && value <= l-offset) || (!inclusive && value < l-offset) {
			return -(i + 1)
		}
	}
	return 0
}

type alarmRangeResult struct {
	rng       int
	normal    bool // range 0, the value left an alarm range or is clear of every hysteresis band
	escalated bool // moved to a more severe level (or to the other side): a new alarm
}

func signOf(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// alarmRangeOf returns the new alarm range of a value from the previous
// range (the document's alarmRange) and the hysteresis. A level is entered
// when the value passes its limit by more than the hysteresis and left when
// it comes back by more than the hysteresis, so a value in a hysteresis
// band keeps the previous range.
func alarmRangeOf(d rawDoc, value, hysteresis float64) alarmRangeResult {
	prev := 0
	if p, ok := limitOf(d, "alarmRange"); ok {
		// levels go from -3 to 3
		prev = int(math.Max(-3, math.Min(3, math.Trunc(p))))
	}
	entry := limitLevel(d, value, hysteresis, false)
	hold := limitLevel(d, value, -hysteresis, true)
	r := alarmRangeResult{}
	switch {
	case entry != 0 && (prev == 0 || signOf(entry) != signOf(prev) || absInt(entry) > absInt(prev)):
		r.rng = entry
		r.escalated = true
	case prev > 0:
		r.rng = min(max(hold, 0), prev)
	case prev < 0:
		r.rng = max(min(hold, 0), prev)
	}
	r.normal = r.rng == 0 && (hold == 0 || prev != 0)
	return r
}

// rangeLabel is the text of an alarm range for events: HI, HIHI, ..., "" for normal.
func rangeLabel(rng int) string {
	switch rng {
	case 1:
		return "HI"
	case 2:
		return "HIHI"
	case 3:
		return "HIHIHI"
	case -1:
		return "LO"
	case -2:
		return "LOLO"
	case -3:
		return "LOLOLO"
	}
	return ""
}
