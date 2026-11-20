package boolean

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// TestMirroredOperands combines operands that trimmed.Solid.Transformed
// has reflected (S4). Such a solid stores every surface under a placement
// of determinant −1 and every face reversed. The stock is an L-shaped
// prism with a through bore, in the form a result has; the tool is a
// cylinder in the lifted form, with caps in the polar chart, that stands
// in one arm of the L and out of its top. Neither has a mirror plane in
// common with the other, so a reading of the reflection that put a face
// elsewhere or turned it about would change the volumes.
//
// Each reflection is tried with both operands reflected, and with the
// stock reflected against a tool built where its image lies: the second
// holds the reflected stock to its place among solids that were not
// reflected. Volumes and areas are the closed forms.
func TestMirroredOperands(t *testing.T) {
	const pi = math.Pi
	outline := []geom.Vec2{{}, {X: 20}, {X: 20, Y: 10}, {X: 8, Y: 10}, {X: 8, Y: 16}, {Y: 16}}
	prism := crossingStock(t, geom.Identity(), outline, 0, 10)
	bore := crossingRod(t, geom.Identity(), geom.Vec3{X: 14, Y: 5, Z: 5}, geom.Vec3{Z: 1}, 2, 30)
	bored, err := Combine(context.Background(), prism, bore, Cut, crossingOptions)
	if err != nil {
		t.Fatal(err)
	}
	stock := bored.Solid
	centre, axis := geom.Vec3{X: 5, Y: 13, Z: 8}, geom.Vec3{Z: 1}
	tool := crossingRod(t, geom.Identity(), centre, axis, 2, 8)

	want := [3]struct{ volume, area float64 }{
		Union:  {2480 - 32*pi, 1216 + 40*pi},
		Common: {24 * pi, 32 * pi},
		Cut:    {2480 - 64*pi, 1216 + 56*pi},
	}
	across := geom.Affine{{-1, 0, 0, 3}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	for _, c := range []struct {
		name string
		m    geom.Affine
	}{
		{"across x", across},
		{"across z", geom.Affine{{1, 0, 0, 0}, {0, 1, 0, -2}, {0, 0, -1, 7}}},
		{"turned", crossingTurn(t).Mul(across)},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.m.Determinant() >= 0 {
				t.Fatal("the placement does not reflect")
			}
			mirrored := func(s trimmed.Solid) trimmed.Solid {
				out, err := s.Transformed(c.m)
				if err != nil {
					t.Fatal(err)
				}
				return out
			}
			a := mirrored(stock)
			for name, b := range map[string]trimmed.Solid{
				"both reflected":    mirrored(tool),
				"tool at its image": crossingRod(t, c.m, centre, axis, 2, 8),
			} {
				for op, w := range want {
					for _, swap := range []bool{false, true} {
						if swap && Operation(op) == Cut {
							continue
						}
						x, y := a, b
						if swap {
							x, y = b, a
						}
						r, err := Combine(context.Background(), x, y, Operation(op), crossingOptions)
						if err != nil {
							t.Fatalf("%s, operation %d, swapped %v: %v", name, op, swap, err)
						}
						if r.Solids != 1 || !crossingWithin(r.Volume, w.volume) || !crossingWithin(r.Properties.Area, w.area) {
							t.Errorf("%s, operation %d, swapped %v: %d solids, volume %.12g, area %.12g, want one, %.12g and %.12g", name, op, swap, r.Solids, r.Volume, r.Properties.Area, w.volume, w.area)
						}
					}
				}
			}
		})
	}
}

// TestMirroredIntake takes a reflected solid through intake: every loop
// is still a closed chain, each use ending at the vertex the next starts
// from, although the loop is taken backwards (S4).
func TestMirroredIntake(t *testing.T) {
	prism := crossingStock(t, geom.Identity(), []geom.Vec2{{}, {X: 20}, {X: 20, Y: 10}, {X: 8, Y: 10}, {X: 8, Y: 16}, {Y: 16}}, 0, 10)
	bore := crossingRod(t, geom.Identity(), geom.Vec3{X: 14, Y: 5, Z: 5}, geom.Vec3{Z: 1}, 2, 30)
	bored, err := Combine(context.Background(), prism, bore, Cut, crossingOptions)
	if err != nil {
		t.Fatal(err)
	}
	s, err := bored.Solid.Transformed(geom.Affine{{-1, 0, 0, 3}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	options, err := crossingOptions.normalized()
	if err != nil {
		t.Fatal(err)
	}
	tol, err := working(options, s, s, [2]bool{})
	if err != nil {
		t.Fatal(err)
	}
	o, err := intake(0, s, tol, nil)
	if err != nil {
		t.Fatal(err)
	}
	ends := func(u use) (from, to int) {
		e := o.edges[u.edge]
		if u.reversed {
			return e.end, e.start
		}
		return e.start, e.end
	}
	for fi, f := range o.faces {
		if f.reversed == s.Faces()[fi].Reversed {
			t.Errorf("face %d keeps its orientation against a carrier whose normal is turned", fi)
		}
		for li, loop := range f.loops {
			for k, u := range loop {
				_, to := ends(u)
				if from, _ := ends(loop[(k+1)%len(loop)]); from != to {
					t.Errorf("face %d loop %d: use %d ends at vertex %d and the next starts at %d", fi, li, k, to, from)
				}
			}
		}
	}
}
