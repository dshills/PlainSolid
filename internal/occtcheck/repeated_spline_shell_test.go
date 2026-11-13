package occtcheck

import (
	"math"
	"testing"
)

// The independent OCCT shell uses one quadratic roof, not a native export.
// Native explicit repeated-pair assembly checks the same exact volume targets
// through a bounded sampled mesh; this oracle verifies analytic integrals.
func TestOCCTRepeatedSplineShellReference(t *testing.T) {
	refs := runOracle(t, "repeated-spline-shell")
	roof := .5*math.Sqrt(2) - .25*math.Sqrt(1.25) + .5*(math.Asinh(1)-math.Asinh(.5))
	for _, tc := range []struct {
		name            string
		volume, area, z float64
	}{
		{"repeated-common", 1. / 24, 23./24 + roof, 19. / 320},
		{"repeated-cut", 23. / 24, 121./24 + roof, 3821. / 7360},
		{"repeated-union", 319. / 24, refs["repeated-b"].Area + 121./24 - roof, -115187. / 102080},
	} {
		r, ok := refs[tc.name]
		if !ok || math.Abs(r.Volume-tc.volume) > 1e-8 || math.Abs(r.Area-tc.area) > 1e-5 || math.Abs(r.Centroid[2]-tc.z) > 1e-8 || math.Abs(r.Centroid[0]-.5) > 1e-9 || math.Abs(r.Centroid[1]-.5) > 1e-9 {
			t.Fatal(tc, r)
		}
	}
	if math.Abs(refs["repeated-b"].Volume-37./3) > 1e-10 {
		t.Fatal(refs["repeated-b"])
	}
}
