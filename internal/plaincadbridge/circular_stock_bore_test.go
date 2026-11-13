package plaincadbridge

import (
	"math"
	"os"
	"testing"
)

func TestCircularStockFiniteEccentricHoleAndSTEP(t *testing.T) {
	for _, tc := range []struct {
		origin, axis  vector
		height, depth float64
		through       bool
	}{
		{vector{2, 0, -1}, vector{0, 0, 1}, 10, 8, true},
		{vector{2, 0, 8}, vector{0, 0, -1}, 3, 3, false},
		{vector{2, 0, 0}, vector{0, 0, 1}, 3, 3, false},
		{vector{2, 0, 5}, vector{0, 0, 1}, 3, 3, false},
	} {
		b := New()
		base := circle(t, b, vector{}, vector{0, 0, 1}, 5, 8)
		tool := circle(t, b, tc.origin, tc.axis, 1, tc.height)
		near(t, overlapMeasure(t, b, base.Handle, tool.Handle), math.Pi*tc.depth)
		near(t, overlapMeasure(t, b, tool.Handle, base.Handle), math.Pi*tc.depth)
		for _, q := range []map[string]any{
			{"op": "cut", "a": base.Handle, "b": tool.Handle},
			{"op": "cutMany", "a": base.Handle, "tools": []string{tool.Handle}},
		} {
			hole := shape(t, b, q)
			near(t, hole.GeometryAssertions.Volume, (200-tc.depth)*math.Pi)
			area := 130*math.Pi + 2*math.Pi*tc.depth
			if tc.through {
				area -= 2 * math.Pi
			}
			near(t, hole.GeometryAssertions.SurfaceArea, area)
			s := b.shapes[hole.Handle]
			mesh, err := s.tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if _, err := b.exportSTEP([]string{hole.Handle}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSTEPIndependentlyAuthoredRoundBlindAndThroughBores(t *testing.T) {
	data, err := os.ReadFile("../../exchange/step/testdata/disk-pocket-occt.step")
	if err != nil {
		t.Fatal(err)
	}
	b := New()
	s, err := b.importSTEP(string(data))
	if err != nil {
		t.Fatal(err)
	}
	p := b.shapes[s.Handle].analytic.stepDisk
	if p == nil || len(p.Holes) != 1 || len(p.Blind) != 1 || p.Outer.Radius != 4 || p.Height != 3 || p.Blind[0].Depth != 1.25 {
		t.Fatal("independent STEP round stock was not preserved")
	}
	if _, err := b.exportSTEP([]string{s.Handle}); err != nil {
		t.Fatal(err)
	}
}
