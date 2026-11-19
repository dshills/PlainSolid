package boolean

import (
	"context"
	"errors"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

// whole runs the intersection stage on two planar solids.
func whole(t *testing.T, x, y brep.Solid, change func(*Options)) (*arrangement, error) {
	t.Helper()
	ops, tol, o := taken(t, x, y, change)
	return arrange(context.Background(), ops, tol, o)
}

// TestFacePass covers X6 and X8 on pairs of planar solids.
func TestFacePass(t *testing.T) {
	cube := box(t, 0, 0, 0, 2, 2, 2)
	t.Run("traces where faces cross", func(t *testing.T) {
		// Two boxes that overlap in a corner: each of the three faces of
		// A at the corner is crossed by two faces of B.
		a, err := whole(t, cube, box(t, 1, 1, 1, 3, 3, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.traces) != 6 || len(a.pairs) != 0 || len(a.contacts) != 0 {
			t.Fatalf("%d traces, %d same-surface pairs, %d contacts", len(a.traces), len(a.pairs), len(a.contacts))
		}
		for _, tr := range a.traces {
			e := a.elems[tr.elem]
			if tr.touching || tr.t1-tr.t0 != 1 || a.points[e.a].pos.Distance(a.points[e.b].pos) != 1 {
				t.Fatalf("trace %+v", tr)
			}
			for i, f := range tr.faces {
				found := false
				for _, x := range a.inner[i][f] {
					found = found || x == tr.elem
				}
				if !found {
					t.Fatalf("trace %+v is not recorded in face %d of operand %d", tr, f, i)
				}
			}
		}
	})
	t.Run("same-surface pairs", func(t *testing.T) {
		a, err := whole(t, cube, box(t, 0, 0, 2, 2, 2, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		opposite, same := 0, 0
		for _, p := range a.pairs {
			if p.opposite {
				opposite++
			} else {
				same++
			}
		}
		if opposite != 1 || same != 4 || len(a.traces) != 0 {
			t.Fatalf("%d opposite pairs, %d same pairs, %d traces; want 1, 4, 0", opposite, same, len(a.traces))
		}
	})
	t.Run("a span that already runs along the curve is reused", func(t *testing.T) {
		// C6: the faces of B inside A meet the faces of A along edges of
		// B that already lie in those faces. No trace is made.
		a, err := whole(t, cube, box(t, 0, 0, 0, 1, 1, 1), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.traces) != 0 || len(a.pairs) != 3 {
			t.Fatalf("%d traces and %d same-surface pairs, want 0 and 3", len(a.traces), len(a.pairs))
		}
	})
	t.Run("divergent faces", func(t *testing.T) {
		// Two faces parallel within Angular, coincident along one edge
		// and 10ε apart along the opposite one (T8, SP4).
		turn, err := geom.Rotation(geom.Vec3{Y: 1}, -5e-7)
		if err != nil {
			t.Fatal(err)
		}
		tilted, err := box(t, 0, 0, 0, 2, 2, 1).Transformed(geom.Translation(geom.Vec3{Z: 2}).Mul(turn))
		if err != nil {
			t.Fatal(err)
		}
		ops, tol, o := taken(t, cube, tilted, func(o *Options) { o.Angular = 1e-3 })
		a, err := newArrangement(context.Background(), ops, tol, o)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.faceWithFace(); !errors.Is(err, ErrTolerance) || !strings.Contains(err.Error(), "SP4") {
			t.Fatalf("got %v, want ErrTolerance from SP4", err)
		}
	})
	t.Run("X8", func(t *testing.T) {
		// Two traces in one face that cross where there is no node.
		ops, tol, o := taken(t, cube, box(t, 5, 0, 0, 6, 1, 1), nil)
		a, err := newArrangement(context.Background(), ops, tol, o)
		if err != nil {
			t.Fatal(err)
		}
		var ends [4]int
		for i, p := range []geom.Vec3{{X: 0.5, Y: 1, Z: 2}, {X: 1.5, Y: 1, Z: 2}, {X: 1, Y: 0.5, Z: 2}, {X: 1, Y: 1.5, Z: 2}} {
			if ends[i], err = a.add(p); err != nil {
				t.Fatal(err)
			}
		}
		a.traces = []trace{
			{c: line{geom.Vec3{X: 0.5, Y: 1, Z: 2}, geom.Vec3{X: 1}}, t1: 1, ends: [2]int{ends[0], ends[1]}, faces: [2]int{1, 0}},
			{c: line{geom.Vec3{X: 1, Y: 0.5, Z: 2}, geom.Vec3{Y: 1}}, t1: 1, ends: [2]int{ends[2], ends[3]}, faces: [2]int{1, 3}},
		}
		if err := a.tracesCross(); !errors.Is(err, ErrInconsistent) {
			t.Fatalf("got %v, want ErrInconsistent", err)
		}
		a.traces[1].faces[0] = 2 // not in one face any more
		if err := a.tracesCross(); err != nil {
			t.Fatal(err)
		}
	})
}
