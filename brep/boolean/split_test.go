package boolean

import (
	"errors"
	"math"
	"sort"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

// TestSplit covers F1 to F6: piece counts and chart areas of the faces of
// box pairs, and the errors of F2 and F6.
func TestSplit(t *testing.T) {
	cube := box(t, 0, 0, 0, 2, 2, 2)
	for _, c := range []struct {
		name  string
		b     brep.Solid
		areas [2][]float64 // the piece areas of A and of B, sorted
	}{
		{"C4", box(t, 0, 0, 2, 2, 2, 3), [2][]float64{{4, 4, 4, 4, 4, 4}, {2, 2, 2, 2, 4, 4}}},
		{"C5", box(t, 1, 0, 2, 3, 2, 3), [2][]float64{{2, 2, 4, 4, 4, 4, 4}, {2, 2, 2, 2, 2, 2, 4}}},
		{"C6", box(t, 0, 0, 0, 1, 1, 1), [2][]float64{{1, 1, 1, 3, 3, 3, 4, 4, 4}, {1, 1, 1, 1, 1, 1}}},
		{"corner overlap", box(t, 1, 1, 1, 3, 3, 3), [2][]float64{{1, 1, 1, 3, 3, 3, 4, 4, 4}, {1, 1, 1, 3, 3, 3, 4, 4, 4}}},
		{"standing inside a face", box(t, 0.5, 0.5, 2, 1.5, 1.5, 3), [2][]float64{{1, 3, 4, 4, 4, 4, 4}, {1, 1, 1, 1, 1, 1}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, err := whole(t, cube, c.b, nil)
			if err != nil {
				t.Fatal(err)
			}
			pieces, err := a.split()
			if err != nil {
				t.Fatal(err)
			}
			var got [2][]float64
			for _, p := range pieces {
				area := chartArea(a.ops[p.op].faces[p.face].s.chart(), legsOf(a.elems, p.loops))
				got[p.op] = append(got[p.op], math.Round(area*1e9)/1e9)
				if len(a.probes(p)) == 0 {
					t.Fatalf("piece %+v has no interior point", p)
				}
			}
			for i := range got {
				sort.Float64s(got[i])
				if len(got[i]) != len(c.areas[i]) {
					t.Fatalf("operand %d: piece areas %v, want %v", i, got[i], c.areas[i])
				}
				for k := range got[i] {
					if got[i][k] != c.areas[i][k] {
						t.Fatalf("operand %d: piece areas %v, want %v", i, got[i], c.areas[i])
					}
				}
			}
		})
	}
	t.Run("a hole is given to the piece around it", func(t *testing.T) {
		a, err := whole(t, cube, box(t, 0.5, 0.5, 2, 1.5, 1.5, 3), nil)
		if err != nil {
			t.Fatal(err)
		}
		pieces, err := a.split()
		if err != nil {
			t.Fatal(err)
		}
		holed := 0
		for _, p := range pieces {
			if len(p.loops) == 2 {
				holed++
				for _, q := range a.probes(p) {
					if q.Z != 2 || q.X > 0.5 && q.X < 1.5 && q.Y > 0.5 && q.Y < 1.5 {
						t.Fatalf("interior point %v is not in the piece", q)
					}
				}
			}
		}
		if holed != 1 {
			t.Fatalf("%d pieces with a hole, want 1", holed)
		}
	})
	t.Run("piece limit", func(t *testing.T) {
		a, err := whole(t, cube, box(t, 1, 0, 2, 3, 2, 3), func(o *Options) { o.MaxPieces = 13 })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.split(); !errors.Is(err, ErrBudget) {
			t.Fatalf("got %v, want ErrBudget", err)
		}
	})
	t.Run("an inconsistent arrangement", func(t *testing.T) {
		// A trace that ends inside a face and is not a touching contact.
		a, err := whole(t, cube, box(t, 5, 0, 0, 6, 1, 1), nil)
		if err != nil {
			t.Fatal(err)
		}
		first := len(a.points)
		a.points = append(a.points, point{geom.Vec3{X: 0.5, Y: 1, Z: 2}, a.t.eps}, point{geom.Vec3{X: 1.5, Y: 1, Z: 2}, a.t.eps})
		a.elems = append(a.elems, element{line{geom.Vec3{X: 0.5, Y: 1, Z: 2}, geom.Vec3{X: 1}}, 0, 1, first, first + 1})
		a.traces = append(a.traces, trace{elem: len(a.elems) - 1, faces: [2]int{1, 0}})
		a.inner[0][1] = append(a.inner[0][1], len(a.elems)-1)
		if _, err := a.split(); !errors.Is(err, ErrInconsistent) || !strings.Contains(err.Error(), "F2") {
			t.Fatalf("got %v, want ErrInconsistent from F2", err)
		}
	})
	t.Run("a sliver", func(t *testing.T) {
		// A prism stands on the top face of A. Its base is a triangle 1
		// long and 1.5ε high, with its corners far apart, which has no
		// interior point farther than ε from its boundary.
		const eps = 1e-7
		h := 1.5 * eps
		thin := polyhedron(t, []geom.Vec3{{X: 0.5, Y: 1, Z: 2}, {X: 1.5, Y: 1, Z: 2}, {X: 1, Y: 1 + h, Z: 2}, {X: 0.5, Y: 1, Z: 3}, {X: 1.5, Y: 1, Z: 3}, {X: 1, Y: 1 + h, Z: 3}},
			[][]int{{0, 2, 1}, {3, 4, 5}, {0, 1, 4, 3}, {1, 2, 5, 4}, {2, 0, 3, 5}})
		a, err := whole(t, cube, thin, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.split(); !errors.Is(err, ErrTolerance) || !strings.Contains(err.Error(), "F6") {
			t.Fatalf("got %v, want ErrTolerance from F6", err)
		}
	})
}

// TestLegBox holds the box of a leg, by which probes leaves legs that are
// too far unmeasured, to the two things that needs: every point of the leg
// is in the box, and the distance from a point to the box is no more than
// its distance to the leg.
func TestLegBox(t *testing.T) {
	arc := conic{center: geom.Vec3{X: 3, Y: -2, Z: 1}, x: geom.Vec3{X: 0.6, Y: 0.8}, y: geom.Vec3{X: -0.48, Y: 0.36, Z: 0.8}, a: 5, b: 2}
	for _, l := range []leg{
		{line{geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 2. / 3, Y: -1. / 3, Z: 2. / 3}}, -4, 9},
		{arc, 0.3, 2.9},
		{arc, 5, 1},
	} {
		box := l.box()
		for k := 0; k <= 64; k++ {
			p := l.c.at(l.t0 + (l.t1-l.t0)*float64(k)/64)
			if d := box.distance(p); d > 1e-12 {
				t.Errorf("a point of the leg is %.3g outside its box", d)
			}
		}
		for _, p := range []geom.Vec3{{X: 40, Y: 3, Z: -7}, {X: 3, Y: -2, Z: 1}, {X: -9, Y: -9, Z: 9}, {X: 4, Y: 0.5, Z: 30}} {
			if to, far := box.distance(p), p.Distance(l.nearest(p)); to > far+1e-12 {
				t.Errorf("the box is %.6g from a point that is %.6g from the leg", to, far)
			}
		}
	}
}
