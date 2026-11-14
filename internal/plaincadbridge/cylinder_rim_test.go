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
		fine, err := b.shapes[out.Handle].tessellate(request{
			LinearDeflection:  optionalNumber{value: .05, set: true},
			AngularDeflection: optionalNumber{value: .1, set: true},
		})
		if err != nil || len(fine.Triangles) >= 100000 {
			t.Fatal("fine rim mesh exceeds PlainCAD's per-body limit", len(fine.Triangles), err)
		}
	}
}

func TestExactCylinderRimChamfer(t *testing.T) {
	for _, normal := range []vector{{0, 0, 1}, {0, -1, 0}} {
		b := New()
		base := shape(t, b, map[string]any{"op": "circle", "origin": vector{}, "normal": normal, "radius": 10, "height": -10})
		top := vector{-10 * normal[0], -10 * normal[1], -10 * normal[2]}
		q := map[string]any{"op": "chamferCylinderRims", "handle": base.Handle, "normal": normal, "rimCenters": []vector{top}, "distance": 1}
		cut := shape(t, b, q)
		near(t, cut.GeometryAssertions.Volume, math.Pi*(1000-10+1./3))
		near(t, cut.GeometryAssertions.SurfaceArea, math.Pi*(100+81+180+19*math.Sqrt2))
		invoke(t, b, map[string]any{"op": "mesh", "handle": cut.Handle, "linearDeflection": .05, "angularDeflection": .1})
		q["rimCenters"] = []vector{{}, top}
		two := shape(t, b, q)
		near(t, two.GeometryAssertions.Volume, math.Pi*(1000-2*(10-1./3)))
		q["distance"] = 5
		rejectGroup(t, b, q)
	}
}
