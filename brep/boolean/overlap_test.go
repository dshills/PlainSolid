package boolean

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

// arcs is an arrangement of one edge of A and one edge of B on the unit
// circle of the plane z = 0, each from one angle to another in degrees.
// An edge of a whole turn is closed.
func arcs(t *testing.T, from, to [2]float64, second conic) *arrangement {
	t.Helper()
	const eps = 1e-7
	first := conic{geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Y: 1}, 1, 1}
	var ops [2]*operand
	for i, c := range [2]conic{first, second} {
		t0, t1 := from[i]*math.Pi/180, to[i]*math.Pi/180
		o := &operand{verts: []geom.Vec3{c.at(t0)}}
		e := edge{c: c, t0: t0, t1: t1, box: around(geom.Vec3{X: -2, Y: -2}, geom.Vec3{X: 2, Y: 2})}
		if to[i]-from[i] < 360 {
			o.verts = append(o.verts, c.at(t1))
			e.end = 1
		}
		o.edges = []edge{e}
		ops[i] = o
	}
	a, err := newArrangement(context.Background(), ops, tolerances{eps: eps, angular: 1e-9, margin: 4 * eps, size: 4}, Options{MaxNodes: 64, MaxPairs: 64})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// TestOverlap covers CC4 on a closed carrier: two circle edges of one
// carrier.
func TestOverlap(t *testing.T) {
	const eps = 1e-7
	unit := conic{geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Y: 1}, 1, 1}
	turned := func(degrees float64) conic {
		s, c := math.Sincos(degrees * math.Pi / 180)
		return conic{geom.Vec3{}, geom.Vec3{X: c, Y: s}, geom.Vec3{X: -s, Y: c}, 1, 1}
	}
	for _, c := range []struct {
		name                 string
		from, to             [2]float64
		second               conic
		laps, bundles, spans int
	}{
		{"two full circles with one vertex", [2]float64{0, 0}, [2]float64{360, 360}, unit, 1, 1, 2},
		{"two full circles with different vertices", [2]float64{0, 0}, [2]float64{360, 360}, turned(90), 1, 2, 4},
		{"two full circles in opposite directions", [2]float64{0, 0}, [2]float64{360, 360}, unit.flipped().(conic), 1, 1, 2},
		{"two arcs that overlap at both ends", [2]float64{0, 180}, [2]float64{270, 450}, unit, 2, 2, 6},
		{"an overlap through the parameter origin", [2]float64{300, 330}, [2]float64{420, 390}, unit, 1, 1, 4},
		{"an arc on a full circle, through its vertex", [2]float64{0, 300}, [2]float64{360, 420}, unit, 1, 2, 5},
		{"all but a gap shorter than the tolerance", [2]float64{0, 0}, [2]float64{360, 360 - eps/2*180/math.Pi}, unit, 1, 1, 2},
		{"arcs that share only an end", [2]float64{0, 90}, [2]float64{90, 180}, unit, 0, 0, 2},
		{"arcs apart", [2]float64{0, 200}, [2]float64{90, 300}, unit, 0, 0, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := arcs(t, c.from, c.to, c.second)
			if err := a.vertexWithVertex(); err != nil {
				t.Fatal(err)
			}
			if err := a.vertexWithEdge(); err != nil {
				t.Fatal(err)
			}
			if err := a.overlap(0, 0); err != nil {
				t.Fatal(err)
			}
			if len(a.laps) != c.laps {
				t.Fatalf("%d overlaps, want %d", len(a.laps), c.laps)
			}
			if c.name == "all but a gap shorter than the tolerance" {
				return // the two ends of the arc are then one node
			}
			if err := a.seal(); err != nil {
				t.Fatal(err)
			}
			if len(a.bundles) != c.bundles || len(a.spans) != c.spans {
				t.Fatalf("%d bundles and %d spans, want %d and %d", len(a.bundles), len(a.spans), c.bundles, c.spans)
			}
			for _, b := range a.bundles {
				x, y := a.spans[b.spans[0]], a.spans[b.spans[1]]
				closed := x.a == x.b
				if x.elem != y.elem || closed != (y.a == y.b) || math.Abs((x.t1-x.t0)-(y.t1-y.t0)) > 1e-9 {
					t.Fatalf("bundle of spans %+v and %+v", x, y)
				}
			}
		})
	}
	t.Run("coincident at some points only", func(t *testing.T) {
		// A circle 0.9ε larger and 0.9ε off centre: on the first circle
		// at two of the three test points and 1.8ε from it at the third.
		a := arcs(t, [2]float64{0, 0}, [2]float64{360, 360}, conic{geom.Vec3{X: 0.9 * eps}, geom.Vec3{X: 1}, geom.Vec3{Y: 1}, 1 + 0.9*eps, 1 + 0.9*eps})
		if err := a.overlap(0, 0); !errors.Is(err, ErrTolerance) {
			t.Fatalf("got %v, want ErrTolerance", err)
		}
	})
}
