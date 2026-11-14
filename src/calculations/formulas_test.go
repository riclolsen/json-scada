package main

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b)) }

func TestParcelCounts(t *testing.T) {
	for code, f := range formulas {
		// a count that never fits must not be computed (and must not panic)
		for _, n := range []int{0, 1, 2, 3, 5, 40} {
			v := make([]float64, n)
			inv := make([]bool, n)
			_, _, _, ok := calculate(code, v, inv)
			if ok != f.parcelsOK(n) {
				t.Errorf("formula %d with %d parcels: ok=%v, parcelsOK=%v", code, n, ok, f.parcelsOK(n))
			}
		}
		if f.exact >= 0 {
			v := make([]float64, f.exact)
			inv := make([]bool, f.exact)
			if _, _, _, ok := calculate(code, v, inv); !ok {
				t.Errorf("formula %d not computed with exactly %d parcels", code, f.exact)
			}
		}
	}
	if _, _, _, ok := calculate(12, []float64{1}, []bool{false}); ok {
		t.Error("formula 12 is not implemented")
	}
}

func TestFormulas(t *testing.T) {
	cases := []struct {
		code      int
		v         []float64
		inv       []bool
		val       float64
		invalid   bool
		transient bool
	}{
		{1, []float64{3, 4, 10}, []bool{false, false, false}, 577.35027 * 5 / 10, false, false},
		{1, []float64{3, 4, 0}, []bool{false, true, false}, 0, true, false},
		{2, []float64{0, 0}, []bool{false, false}, 0, true, false}, // NaN -> 0, invalid
		{3, []float64{3, 4}, []bool{false, false}, 5, false, false},
		{4, []float64{1, 2, 3}, []bool{false, true, false}, 6, true, false},
		{5, []float64{-4}, []bool{false}, 0, true, false}, // NaN -> 0, invalid
		{6, []float64{1, 1, 0}, []bool{false, false, false}, 0, false, false},
		{7, []float64{0, 0, 2}, []bool{false, false, false}, 1, false, false},
		{10, []float64{1, 2}, []bool{false, false}, -3, false, false},
		{13, append(make([]float64, 22), 5, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0), make([]bool, 34), -1 - 5 - 7, false, false},
		{15, []float64{5, 3}, []bool{false, false}, 2, false, false},
		{50, []float64{1, 2, 3}, []bool{true, false, false}, 2, false, false}, // first valid
		{50, []float64{1, 2}, []bool{true, true}, 1, true, false},
		{52, []float64{1, 2}, []bool{true, false}, 1, false, false},
		{53, []float64{4, -1, 2}, []bool{false, false, false}, 5, false, false},
		{54, []float64{1, 0}, []bool{false, false}, 1, false, false},
		{54, []float64{0, 1}, []bool{false, false}, 0, false, false},
		{54, []float64{1, 1}, []bool{false, false}, 1, true, true},
		{55, []float64{1, 0}, []bool{false, false}, math.MaxFloat64, false, false},
		{202, []float64{2, 30}, []bool{false, false}, 150, false, false},
		{203, []float64{1, 10, 20}, []bool{true, true, false}, 20, false, false},
		{216, []float64{5, 1, 5}, []bool{false, true, true}, 1, false, false},
		{218, []float64{1, 1, 5}, []bool{false, false, false}, 0, false, false},
		{220, []float64{5, 3}, []bool{false, false}, 3, false, false},
		{221, []float64{5, 3}, []bool{false, false}, 5, false, false},
		{225, nil, nil, 1, false, false},
		{231, []float64{4}, []bool{false}, 0, false, false},
		{5022, []float64{10, 10}, []bool{false, false}, 16.5, false, false},
		{5172, []float64{10}, []bool{false}, -7.2, false, false},
	}
	for _, c := range cases {
		val, invalid, transient, ok := calculate(c.code, c.v, c.inv)
		if !ok || !approx(val, c.val) || invalid != c.invalid || transient != c.transient {
			t.Errorf("formula %d %v %v: got %v/%v/%v ok=%v, want %v/%v/%v",
				c.code, c.v, c.inv, val, invalid, transient, ok, c.val, c.invalid, c.transient)
		}
	}
}
