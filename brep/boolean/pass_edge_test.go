package boolean

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"plainsolid/geom"
)

// TestPassEdge covers X3 and X5 on pairs of planar solids.
func TestPassEdge(t *testing.T) {
	whole := box(t, 0, 0, 0, 2, 2, 2)
	t.Run("X3 makes a node where two edges cross", func(t *testing.T) {
		// The bottom edges of B cross the top edges of A at (1,2,2) and
		// (2,1,2); each crossing is a vertex of neither.
		a, err := arranged(t, context.Background(), whole, box(t, 1, 1, 2, 3, 3, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		crossings := 0
		for i, n := range a.nodes {
			if n.into < 0 && len(n.verts[0]) == 0 && len(n.verts[1]) == 0 {
				crossings++
				if p := n.pos; p != (geom.Vec3{X: 1, Y: 2, Z: 2}) && p != (geom.Vec3{X: 2, Y: 1, Z: 2}) {
					t.Fatalf("node %d at %v", i, p)
				}
			}
		}
		if crossings != 2 || len(a.bundles) != 0 {
			t.Fatalf("%d crossings and %d bundles, want 2 and 0", crossings, len(a.bundles))
		}
	})
	t.Run("X3 bundles the spans of an overlap", func(t *testing.T) {
		a, err := arranged(t, context.Background(), whole, box(t, 1, 0, 2, 3, 2, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range a.bundles {
			x, y := a.spans[b.spans[0]], a.spans[b.spans[1]]
			if x.op != 0 || y.op != 1 || x.elem != y.elem || x.t1-x.t0 != 1 || y.t1-y.t0 != 1 || b.reversed[1] != (x.a != y.a) {
				t.Fatalf("bundle %+v of spans %+v and %+v", b, x, y)
			}
		}
	})
	t.Run("X5 makes a node where an edge passes through a face", func(t *testing.T) {
		// B stands through the top face of A: its four vertical edges
		// pass through the face.
		a, err := arranged(t, context.Background(), whole, box(t, 0.5, 0.5, 1, 1.5, 1.5, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		through := 0
		for f, list := range a.free[0] {
			for _, p := range list {
				if a.points[p].pos.Z != 2 || a.ops[0].faces[f].s.offset(a.points[p].pos) != 0 {
					t.Fatalf("point %v in face %d of A", a.points[p].pos, f)
				}
				through++
			}
		}
		if through != 4 || len(a.spans) != 28 || len(a.contacts) != 0 {
			t.Fatalf("%d nodes inside faces of A, %d spans, %d contacts", through, len(a.spans), len(a.contacts))
		}
	})
	t.Run("X5 records an edge resting in a face as touching", func(t *testing.T) {
		// A prism with a triangle cross-section, resting on one edge in
		// the top face of A.
		wedge := polyhedron(t, []geom.Vec3{{X: 0.5, Y: 1, Z: 2}, {X: 1.5, Y: 1, Z: 2}, {X: 0.5, Y: 0.5, Z: 3}, {X: 1.5, Y: 0.5, Z: 3}, {X: 0.5, Y: 1.5, Z: 3}, {X: 1.5, Y: 1.5, Z: 3}},
			[][]int{{0, 2, 3, 1}, {0, 1, 5, 4}, {2, 4, 5, 3}, {0, 4, 2}, {1, 3, 5}})
		a, err := arranged(t, context.Background(), whole, wedge, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.contacts) != 1 || a.contacts[0].elem < 0 || len(a.contacts[0].faces[1]) != 2 {
			t.Fatalf("contacts %+v", a.contacts)
		}
		top := a.contacts[0].faces[0][0]
		if len(a.inner[0][top]) != 1 || a.inner[0][top][0] != a.contacts[0].elem || len(a.free[0][top]) != 2 {
			t.Fatalf("face %d of A holds elements %v and points %v", top, a.inner[0][top], a.free[0][top])
		}
	})
}

// TestPassEdgeOverlap covers the outcomes of CC4 for two line edges.
func TestPassEdgeOverlap(t *testing.T) {
	const eps = 1e-7
	whole := box(t, 0, 0, 0, 2, 2, 2)
	// tilted is a box on top of A whose bottom edges along x leave the
	// top edges of A: they coincide at x = 0 and are lift above them at
	// x = 2.
	tilted := func(lift float64) *arrangement {
		t.Helper()
		slope := math.Atan2(lift, 2)
		turn, err := geom.Rotation(geom.Vec3{Y: 1}, -slope)
		if err != nil {
			t.Fatal(err)
		}
		b, err := box(t, 0, 0, 0, 2, 2, 1).Transformed(geom.Translation(geom.Vec3{Z: 2}).Mul(turn))
		if err != nil {
			t.Fatal(err)
		}
		o, _ := matrixOptions.normalized()
		o.Angular = 1e-3
		sa, sb := lifted(t, whole), lifted(t, b)
		tol, err := working(o, sa, sb, [2]bool{})
		if err != nil {
			t.Fatal(err)
		}
		var ops [2]*operand
		if ops[0], err = intake(0, sa, tol); err != nil {
			t.Fatal(err)
		}
		if ops[1], err = intake(1, sb, tol); err != nil {
			t.Fatal(err)
		}
		a, err := newArrangement(context.Background(), ops, tol, o)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	t.Run("divergent", func(t *testing.T) {
		// Parallel within Angular, coincident at one end and 10ε apart
		// at the other: neither one edge nor two clear edges (T8).
		a := tilted(10 * eps)
		if err := a.lower(); !errors.Is(err, ErrTolerance) || !strings.Contains(err.Error(), "CC4") {
			t.Fatalf("got %v, want ErrTolerance from CC4", err)
		}
	})
	t.Run("coincident within the tolerance", func(t *testing.T) {
		a := tilted(0.5 * eps)
		if err := a.lower(); err != nil {
			t.Fatal(err)
		}
		if err := a.seal(); err != nil {
			t.Fatal(err)
		}
		if len(a.bundles) != 4 {
			t.Fatalf("%d bundles, want 4", len(a.bundles))
		}
	})
	t.Run("degenerate", func(t *testing.T) {
		// Collinear edges that share only an end point: no overlap.
		a, err := arranged(t, context.Background(), whole, box(t, 2, 0, 2, 4, 2, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.bundles) != 1 {
			t.Fatalf("%d bundles, want the one shared edge", len(a.bundles))
		}
	})
}
