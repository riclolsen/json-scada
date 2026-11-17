/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package conv

import (
	"math"
	"testing"

	"github.com/riclolsen/go-iecp5/asdu"
)

func TestBits32(t *testing.T) {
	cases := []struct {
		in   float64
		want uint32
	}{
		{0, 0},
		{1, 1},
		{2147483648, 0x80000000}, // bit 31 kept
		{4294967295, 0xFFFFFFFF},
		{-1, 0xFFFFFFFF}, // two's complement
		{-2147483648, 0x80000000},
		{5e10, math.MaxUint32}, // saturates
		{math.NaN(), 0},
	}
	for _, c := range cases {
		if got := Bits32(c.in); got != c.want {
			t.Errorf("Bits32(%v) = %#x, want %#x", c.in, got, c.want)
		}
	}
}

func TestBuildInfoObjBitstringHighBit(t *testing.T) {
	obj := BuildInfoObj(7, 1, 3221225472, false, 0, Quality{}, nil, 1, 0, false)
	if v := obj.Info.(asdu.BitString32Info).Value; v != 0xC0000000 {
		t.Errorf("bitstring = %#x, want 0xc0000000", v)
	}
}

// Scaled and step values round to the nearest integer instead of truncating.
func TestBuildInfoObjRounds(t *testing.T) {
	if v := BuildInfoObj(11, 1, 28.9999, false, 0, Quality{}, nil, 1, 0, false).
		Info.(asdu.MeasuredValueScaledInfo).Value; v != 29 {
		t.Errorf("scaled 28.9999 = %d, want 29", v)
	}
	if v := BuildInfoObj(11, 1, -2.6, false, 0, Quality{}, nil, 1, 0, false).
		Info.(asdu.MeasuredValueScaledInfo).Value; v != -3 {
		t.Errorf("scaled -2.6 = %d, want -3", v)
	}
	if v := BuildInfoObj(5, 1, 4.5, false, 0, Quality{}, nil, 1, 0, false).
		Info.(asdu.StepPositionInfo).Value.Val; v != 5 {
		t.Errorf("step 4.5 = %d, want 5", v)
	}
	sc := BuildInfoObj(11, 1, math.NaN(), false, 0, Quality{}, nil, 1, 0, false).Info.(asdu.MeasuredValueScaledInfo)
	if sc.Value != 0 || sc.Qds&asdu.QDSInvalid == 0 {
		t.Errorf("NaN = %d qds %v, want 0 invalid", sc.Value, sc.Qds)
	}
}
