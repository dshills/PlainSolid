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

// opaque is a curve evaluator of a type the package cannot name. It walks
// its base curve unevenly, agreeing with it at the eighths of its range
// where trimmed.NewSolid looks, and it can leave it sideways between two
// parameters.
type opaque struct {
	base       geom.Curve
	lo, hi     float64
	bump       geom.Vec3
	from, upTo float64 // the bump acts on this part of the normalized range
}

func (c opaque) warp(t float64) float64 {
	s := (t - c.lo) / (c.hi - c.lo)
	w := math.Sin(8 * math.Pi * s)
	return c.lo + (c.hi-c.lo)*(s+0.01*w*w)
}
func (c opaque) Eval(t float64) (geom.Vec3, error) {
	p, err := c.base.Eval(c.warp(t))
	if s := (t - c.lo) / (c.hi - c.lo); err == nil && s > c.from && s < c.upTo {
		w := math.Sin(math.Pi * (s - c.from) / (c.upTo - c.from))
		p = p.Add(c.bump.Scale(w * w))
	}
	return p, err
}
func (c opaque) Derivative(t float64) (geom.Vec3, error) { return c.base.Derivative(c.warp(t)) }

// restored rebuilds a solid with every edge curve replaced.
func restored(t *testing.T, s trimmed.Solid, replace func(i int, c geom.Curve, lo, hi float64) geom.Curve) trimmed.Solid {
	t.Helper()
	edges := s.Edges()
	for i, e := range edges {
		edges[i].Curve = replace(i, e.Curve, e.Range.Min, e.Range.Max)
	}
	out, err := trimmed.NewSolid(s.Vertices(), edges, s.Faces(), s.Tolerance())
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEdgeCarrier(t *testing.T) {
	tol := tolerances{eps: 1e-7, angular: 1e-9, margin: 4e-7, size: 4}
	plain := lifted(t, box(t, 0, 0, 0, 2, 2, 2))
	t.Run("stored form does not matter", func(t *testing.T) {
		hidden := restored(t, plain, func(_ int, c geom.Curve, lo, hi float64) geom.Curve { return opaque{base: c, lo: lo, hi: hi} })
		want, err := intake(0, plain, tol)
		if err != nil {
			t.Fatal(err)
		}
		got, err := intake(0, hidden, tol)
		if err != nil {
			t.Fatal(err)
		}
		for i, e := range got.edges {
			w := want.edges[i]
			if e.c.family() != lineFamily || e.c.at(e.t0).Distance(w.c.at(w.t0)) > tol.eps || e.c.at(e.t1).Distance(w.c.at(w.t1)) > tol.eps ||
				e.c.tangent(e.t0).Distance(w.c.tangent(w.t0)) > 1e-9 || !(e.t0 < e.t1) {
				t.Fatalf("edge %d: got %+v, want %+v", i, e, w)
			}
		}
	})
	t.Run("edge between two faces on one plane", func(t *testing.T) {
		// A box whose top is two triangles: their common edge lies on one
		// plane, so its carrier comes from its samples.
		split := lifted(t, polyhedron(t, []geom.Vec3{{}, {X: 2}, {X: 2, Y: 2}, {Y: 2}, {Z: 2}, {X: 2, Z: 2}, {X: 2, Y: 2, Z: 2}, {Y: 2, Z: 2}},
			[][]int{{0, 3, 2, 1}, {4, 5, 6}, {4, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}}))
		o, err := intake(0, split, tol)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range o.edges {
			if a, b := o.verts[e.start], o.verts[e.end]; a.Z == 2 && b.Z == 2 && a.X != b.X && a.Y != b.Y {
				found = true
				if e.c.at(e.t0).Distance(a) > tol.eps || e.c.at(e.t1).Distance(b) > tol.eps {
					t.Fatalf("the diagonal runs from %v to %v, want %v to %v", e.c.at(e.t0), e.c.at(e.t1), a, b)
				}
			}
		}
		if !found {
			t.Fatal("the diagonal edge is missing")
		}
		if _, err := Combine(context.Background(), split, trimmed.Solid{}, Union, matrixOptions); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("edge that leaves a face", func(t *testing.T) {
		// trimmed.NewSolid samples every edge at eighths of its range, so a
		// solid can hold a curve that leaves its faces only between them.
		// Sample 9 of 17 is the first past the middle.
		bent := restored(t, plain, func(i int, c geom.Curve, lo, hi float64) geom.Curve {
			if i != 0 {
				return c
			}
			return opaque{base: c, lo: lo, hi: hi, bump: geom.Vec3{X: 1e-6, Y: 1e-6, Z: 1e-6}, from: 0.5, upTo: 0.625}
		})
		_, err := Combine(context.Background(), bent, trimmed.Solid{}, Union, matrixOptions)
		if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "operand A edge 0") {
			t.Fatalf("got %v, want ErrInvalid naming operand A edge 0", err)
		}
		// The same at the very middle, on the edge alone.
		edges := plain.Edges()
		faces, err := intake(0, plain, tol)
		if err != nil {
			t.Fatal(err)
		}
		var on []surface
		for _, f := range faces.faces {
			for _, loop := range f.loops {
				for _, u := range loop {
					if u.edge == 0 {
						on = append(on, f.s)
					}
				}
			}
		}
		e := edges[0]
		e.Curve = opaque{base: e.Curve, lo: e.Range.Min, hi: e.Range.Max, bump: on[0].normal(geom.Vec3{}).Scale(10 * tol.eps), from: 0.25, upTo: 0.75}
		if _, err := edgeCarrier(e, on[0], on[1], tol); !errors.Is(err, ErrInvalid) {
			t.Fatalf("got %v, want ErrInvalid", err)
		}
	})
	t.Run("edge that runs back", func(t *testing.T) {
		back := restored(t, plain, func(i int, c geom.Curve, lo, hi float64) geom.Curve {
			if i != 0 {
				return c
			}
			return wiggle{c, lo, hi}
		})
		_, err := Combine(context.Background(), back, trimmed.Solid{}, Union, matrixOptions)
		if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "operand A edge 0") {
			t.Fatalf("got %v, want ErrUnsupported naming operand A edge 0", err)
		}
	})
}

// wiggle walks its base line forward, back and forward again between the
// eighths of its range, where trimmed.NewSolid does not look.
type wiggle struct {
	base   geom.Curve
	lo, hi float64
}

func (c wiggle) at(t float64) float64 {
	s := (t - c.lo) / (c.hi - c.lo)
	w := math.Sin(8 * math.Pi * s)
	return c.lo + (c.hi-c.lo)*(s-0.1*w*w)
}
func (c wiggle) Eval(t float64) (geom.Vec3, error)       { return c.base.Eval(c.at(t)) }
func (c wiggle) Derivative(t float64) (geom.Vec3, error) { return c.base.Derivative(c.at(t)) }

// polyhedron builds a planar solid from polygons over shared vertices.
func polyhedron(t testing.TB, vertices []geom.Vec3, polygons [][]int) brep.Solid {
	t.Helper()
	var edges []brep.Edge
	faces := make([]brep.Face, len(polygons))
	lookup := map[[2]int]int{}
	for f, p := range polygons {
		for i, a := range p {
			b := p[(i+1)%len(p)]
			key := [2]int{min(a, b), max(a, b)}
			id, ok := lookup[key]
			if !ok {
				id = len(edges)
				lookup[key] = id
				edges = append(edges, brep.Edge{Start: key[0], End: key[1]})
			}
			faces[f].Boundary = append(faces[f].Boundary, brep.EdgeUse{Edge: id, Reversed: a > b})
		}
	}
	s, err := brep.NewSolid(vertices, edges, faces)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
