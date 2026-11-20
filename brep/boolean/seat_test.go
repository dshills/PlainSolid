package boolean

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// TestSeatedFaces combines a box with a second operand that has a planar
// face within ε of a face of the box, to either side of it (S7). The face
// is seated on the plane of the box: the result is measured as if the two
// had been one plane, to rounding and not to the gap, and no vertex of the
// result is off the planes of the box. A cylinder that ends there is a
// through bore; a block that starts there, with its face against the
// box's, is a boss.
func TestSeatedFaces(t *testing.T) {
	const pi = math.Pi
	stock := crossingStock(t, geom.Identity(), []geom.Vec2{{}, {X: 20}, {X: 20, Y: 10}, {Y: 10}}, 0, 8)
	for _, gap := range []float64{-9e-8, -5e-8, 5e-8, 9e-8, 1e-7} {
		// A bore from the bottom face that ends within ε of the top face.
		bore := crossingRod(t, geom.Identity(), geom.Vec3{X: 10, Y: 5, Z: (8 + gap) / 2}, geom.Vec3{Z: 1}, 2, 8+gap)
		// A block that stands on the top face, from within ε of it.
		boss := crossingStock(t, geom.Identity(), []geom.Vec2{{X: 4, Y: 2}, {X: 9, Y: 2}, {X: 9, Y: 6}, {X: 4, Y: 6}}, 8+gap, 3-gap)
		for _, c := range []struct {
			name         string
			other        trimmed.Solid
			op           Operation
			volume, area float64
		}{
			{"through bore", bore, Cut, 1600 - 32*pi, 880 + 32*pi - 8*pi},
			{"boss", boss, Union, 1600 + 60, 880 + 54},
		} {
			r, err := Combine(context.Background(), stock, c.other, c.op, crossingOptions)
			if err != nil {
				t.Fatalf("%s, gap %g: %v", c.name, gap, err)
			}
			if p := r.Properties; r.Solids != 1 || math.Abs(p.Volume-c.volume) > 1e-10 || math.Abs(p.Area-c.area) > 1e-10 {
				t.Errorf("%s, gap %g: %d solids, volume %.14g, area %.14g, want one, %.14g and %.14g", c.name, gap, r.Solids, p.Volume, p.Area, c.volume, c.area)
			}
			for _, v := range r.Solid.Vertices() {
				if v.Z != 0 && v.Z != 8 && v.Z != 11 {
					t.Errorf("%s, gap %g: a vertex of the result is at height %.17g", c.name, gap, v.Z)
				}
			}
			// The same operand again finds nothing to do.
			again, err := Combine(context.Background(), r.Solid, c.other, c.op, crossingOptions)
			if err != nil {
				t.Fatalf("%s, gap %g, again: %v", c.name, gap, err)
			}
			if math.Abs(again.Volume-r.Volume) > 1e-10 {
				t.Errorf("%s, gap %g: the same operand again changes the volume by %.3g", c.name, gap, again.Volume-r.Volume)
			}
		}
	}
}
