package boolean

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
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

// laid runs the intersection stage on two solids as they are stored.
func laid(t *testing.T, x, y trimmed.Solid) (*arrangement, error) {
	t.Helper()
	o, err := matrixOptions.normalized()
	if err != nil {
		t.Fatal(err)
	}
	tol, err := working(o, x, y, [2]bool{})
	if err != nil {
		t.Fatal(err)
	}
	var ops [2]*operand
	for i, s := range [2]trimmed.Solid{x, y} {
		if ops[i], err = intake(i, s, tol); err != nil {
			t.Fatal(err)
		}
	}
	return arrange(context.Background(), ops, tol, o)
}

// leaning is a cylinder of radius 0.5 and length 6 whose axis passes
// through (2,2,1), turned from the vertical by the given angle about y.
func leaning(t *testing.T, angle float64) trimmed.Solid {
	t.Helper()
	turn, err := geom.Rotation(geom.Vec3{Y: 1}, angle)
	if err != nil {
		t.Fatal(err)
	}
	s, err := tube(t, 0, 0, -3, 0.5, 6).Transformed(geom.Translation(geom.Vec3{X: 2, Y: 2, Z: 1}).Mul(turn))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestFacePassCurved covers X6 where a cylinder is involved: the traces,
// same-surface pairs and touching contacts of plane with cylinder and of
// parallel cylinders, and the pieces they cut.
func TestFacePassCurved(t *testing.T) {
	cube := lifted(t, box(t, 0, 0, 0, 2, 2, 2))
	slab := lifted(t, box(t, 0, 0, 0, 4, 4, 2))
	for _, c := range []struct {
		name                    string
		a, b                    trimmed.Solid
		lines, conics           int // traces by carrier
		touching                int // traces that are touching contacts
		pairs, contacts, pieces int
		closed                  int // closed traces
	}{
		{"plane across the axis", cube, tube(t, 1, 1, 1, 0.5, 2), 0, 1, 0, 0, 0, 11, 1},
		{"oblique plane", slab, leaning(t, math.Pi/6), 0, 2, 0, 0, 0, 13, 2},
		// Two generators in the side face of A, and a chord of each cap.
		{"plane along the axis", cube, tube(t, 2.2, 1, 0.5, 0.5, 1), 4, 0, 0, 0, 0, 14, 0},
		{"overlapping parallel cylinders", tube(t, 0, 0, 0, 1, 2), tube(t, 1, 0, 0, 1, 2), 2, 0, 0, 2, 0, 20, 0},
		{"C11 cylinder touching a face from outside", cube, tube(t, 2.5, 1, 0.5, 0.5, 1), 1, 0, 1, 0, 3, 10, 0},
		{"C12 cylinder touching a face from inside", cube, tube(t, 0.5, 1, 0.5, 0.5, 1), 1, 0, 1, 0, 3, 10, 0},
		{"C13 cylinders touching from outside", tube(t, 0, 0, 0, 1, 2), tube(t, 2, 1, 0, 1.2360679774997898, 2), 1, 0, 1, 2, 1, 8, 0},
		{"C14 cylinders touching from inside", tube(t, 0, 0, 0, 1, 2), tube(t, 0, 0.5, 0, 0.5, 2), 1, 0, 1, 2, 1, 12, 0},
		{"C19 cylinder standing on a face", cube, tube(t, 1, 1, 2, 0.5, 1), 0, 0, 0, 1, 0, 10, 0},
		{"C17 coaxial cylinders", tube(t, 0, 0, 0, 1, 2), tube(t, 0, 0, -1, 0.5, 4), 0, 2, 0, 0, 0, 10, 2},
		// The base circle touches an edge of the top face: at the point
		// of contact two elements leave along one tangent, and the walk
		// orders them by curvature (F3).
		{"C20 base circle touching an edge", cube, tube(t, 1.5, 1, 2, 0.5, 1), 0, 0, 0, 1, 0, 10, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, err := laid(t, c.a, c.b)
			if err != nil {
				t.Fatal(err)
			}
			pieces, err := a.split()
			if err != nil {
				t.Fatal(err)
			}
			var got [4]int
			for _, tr := range a.traces {
				e := a.elems[tr.elem]
				if e.c.family() == lineFamily {
					got[0]++
				} else {
					got[1]++
				}
				if tr.touching {
					got[2]++
				}
				if e.a == e.b {
					got[3]++
				}
			}
			if want := [4]int{c.lines, c.conics, c.touching, c.closed}; got != want || len(a.pairs) != c.pairs || len(a.contacts) != c.contacts || len(pieces) != c.pieces {
				t.Fatalf("traces (lines, conics, touching, closed) %v, %d pairs, %d contacts, %d pieces; want %v, %d, %d, %d", got, len(a.pairs), len(a.contacts), len(pieces), want, c.pairs, c.contacts, c.pieces)
			}
			// The pieces of a face cover it: their chart areas add up to
			// the chart area of the face.
			for i := range a.ops {
				sums := filled(a.ops[i].faces, 0.)
				for _, p := range pieces {
					if p.op == i {
						sums[p.face] += chartArea(a.ops[i].faces[p.face].s.chart(), legsOf(a.elems, p.loops))
					}
				}
				for f, face := range a.ops[i].faces {
					if whole := chartArea(face.s.chart(), a.borders[i][f]); math.Abs(sums[f]-whole) > 1e-9 || !(whole > 0) {
						t.Fatalf("operand %d face %d: its pieces have area %.12g, the face %.12g", i, f, sums[f], whole)
					}
				}
			}
		})
	}
	t.Run("the base circle of a standing cylinder is recorded for both faces", func(t *testing.T) {
		a, err := laid(t, cube, tube(t, 1, 1, 2, 0.5, 1))
		if err != nil {
			t.Fatal(err)
		}
		// Besides the circle, the face holds the radial edge of the
		// polar cap of the cylinder.
		found := 0
		for f, list := range a.inner[0] {
			for _, e := range list {
				el := a.elems[e]
				k, ok := el.c.(conic)
				if !ok {
					continue
				}
				if el.a != el.b || math.Abs(k.a-0.5) > 1e-9 || a.ops[0].faces[f].s.offset(k.center) != 0 {
					t.Fatalf("face %d of A holds element %+v", f, el)
				}
				found++
			}
		}
		if found != 1 || len(a.traces) != 0 {
			t.Fatalf("%d circles recorded in faces of A and %d traces, want 1 and 0", found, len(a.traces))
		}
	})
	t.Run("cylinders with axes that are not parallel", func(t *testing.T) {
		upright := tube(t, 0, 0, 0, 0.5, 2)
		turn, err := geom.Rotation(geom.Vec3{Y: 1}, math.Pi/2)
		if err != nil {
			t.Fatal(err)
		}
		lying := func(y float64) trimmed.Solid {
			s, err := tube(t, 0, 0, -1, 0.5, 2).Transformed(geom.Translation(geom.Vec3{Y: y, Z: 1}).Mul(turn))
			if err != nil {
				t.Fatal(err)
			}
			return s
		}
		// Far enough apart for the proof of SP2: no contact.
		a, err := laid(t, upright, lying(1.2))
		if err != nil {
			t.Fatal(err)
		}
		if len(a.traces) != 0 || len(a.pairs) != 0 {
			t.Fatalf("%d traces and %d pairs", len(a.traces), len(a.pairs))
		}
		// Crossing: unsupported, naming both faces (SP3).
		if _, err := laid(t, upright, lying(0.3)); !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "of A") || !strings.Contains(err.Error(), "of B") {
			t.Fatalf("got %v, want ErrUnsupported naming a face of each operand", err)
		}
	})
}
