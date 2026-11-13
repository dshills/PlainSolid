package occtcheck

import (
	"math"
	"testing"
)

// Native split-chart spline operations independently check these same analytic
// invariants in trimmed.TestCombineMultipleOpenSplineWires. This fixture uses
// OCCT primitive boxes, not exported native spline geometry.
func TestOCCTMultipleOpenSplineReference(t *testing.T) {
	refs := runOracle(t, "multiple-open-spline-box")
	for _, tc := range []struct {
		name         string
		volume, area float64
	}{
		{"multiple-open-union", 1.25, 8},
		{"multiple-open-common", .25, 2.5},
		{"multiple-open-cut", .75, 7.5},
	} {
		r, ok := refs[tc.name]
		if !ok || math.Abs(r.Volume-tc.volume) > 1e-10 || math.Abs(r.Area-tc.area) > 1e-10 {
			t.Fatal(tc, r)
		}
		for _, x := range r.Centroid {
			if math.Abs(x-.5) > 1e-10 {
				t.Fatal("centroid", r)
			}
		}
	}
}
