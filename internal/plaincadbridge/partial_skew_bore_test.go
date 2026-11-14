package plaincadbridge

import (
	"math"
	"strings"
	"testing"
)

func TestPartialSkewBoreScopeAndSTL(t *testing.T) {
	for _, radii := range [][2]float64{{5, 4}, {5, 5}} {
		b := New()
		stock := box(t, b, vector{}, vector{40, 40, 40})
		x := circle(t, b, vector{0, 16, 20}, vector{1, 0, 0}, radii[0], 40)
		z := circle(t, b, vector{20, 21, 40}, vector{0, 0, -1}, radii[1], 40)
		first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": x.Handle})
		next := shape(t, b, map[string]any{"op": "cut", "a": first.Handle, "b": z.Handle})
		wantVolume, wantArea := 59020.039219249535, 11408.296380716538
		if radii[0] == radii[1] {
			wantVolume, wantArea = 58007.189499960914, 11530.344011235435
		}
		m := next.GeometryAssertions
		if !m.Valid || m.SolidCount != 1 || math.Abs(m.Volume-wantVolume) > 1e-6 || math.Abs(m.SurfaceArea-wantArea) > 1e-6 {
			t.Fatal(radii, m)
		}
		near(t, overlapMeasure(t, b, first.Handle, z.Handle), first.GeometryAssertions.Volume-m.Volume)
		grouped, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle, z.Handle})
		if err != nil || grouped.GeometryAssertions != m || grouped.Bounds != stock.Bounds {
			t.Fatal(grouped, err)
		}
		near(t, overlapMeasure(t, b, grouped.Handle, x.Handle), 0)
		mesh, err := b.shapes[grouped.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := mesh.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		if _, err := b.exportSTEP([]string{grouped.Handle}); err == nil || !strings.Contains(err.Error(), "partially overlapping") {
			t.Fatalf("unexpected STEP result: %v", err)
		}
	}
}
