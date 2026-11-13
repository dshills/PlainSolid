package plaincadbridge

import (
	"math"
	"testing"
)

func TestExactCylinderRimFillet(t *testing.T) {
	for _, n := range []vector{{0, 0, 1}, {0, -1, 0}, {1, 0, 0}} {
		b := New()
		source := circle(t, b, vector{}, n, 10, 10)
		end := coordinates(n.vec().Scale(10))
		out := shape(t, b, map[string]any{"op": "filletCylinderRims", "handle": source.Handle,
			"normal": n, "rimCenters": []vector{end}, "radius": 1})
		removed := 2 * math.Pi * (10*(1-math.Pi/4) - (5./6 - math.Pi/4))
		near(t, out.GeometryAssertions.Volume, 1000*math.Pi-removed)
		m, err := b.shapes[out.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}
