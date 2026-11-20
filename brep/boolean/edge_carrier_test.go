package boolean

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
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
		want, err := intake(0, plain, tol, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := intake(0, hidden, tol, nil)
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
		o, err := intake(0, split, tol, nil)
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
		faces, err := intake(0, plain, tol, nil)
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

// stored builds the cylinder of radius 0.5 and height 2 on the axis
// through (1,1,0) along z in one of the stored forms of section 11: sf1
// has line and ellipse edges, as a result has; sf2 is a parametric
// cylinder lifted with trimmed.FromParametric; sf3 a disc built by
// trimmed.ExtrudeDiskWithHoles. With placed set the solid is
// moved by a rigid placement that is not axis-aligned (SF4).
func stored(t testing.TB, form string, placed bool) (trimmed.Solid, geom.Affine) {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	var s trimmed.Solid
	switch form {
	case "sf1":
		s = formed(t, "sf1", 1, 1, 0, 0.5, 2)
	case "sf2":
		c, err := parametric.Cylinder(frame, 0.5, 2)
		if err != nil {
			t.Fatal(err)
		}
		if s, err = trimmed.FromParametric(c); err != nil {
			t.Fatal(err)
		}
	case "sf3":
		if s, err = trimmed.ExtrudeDiskWithHoles(context.Background(), frame, trimmed.ExtrusionCircle{Radius: 0.5}, nil, nil, 2); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown stored form %q", form)
	}
	m := geom.Identity()
	if placed {
		turn, err := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, 0.7)
		if err != nil {
			t.Fatal(err)
		}
		m = geom.Translation(geom.Vec3{X: 0.3, Y: -0.2, Z: 0.1}).Mul(turn)
		if s, err = s.Transformed(m); err != nil {
			t.Fatal(err)
		}
	}
	return s, m
}

// TestEdgeCarrierStoredForms: whatever the stored form and placement of
// a cylinder, every edge is reduced to a line along or across the axis or
// to a circle of the cylinder's radius about it, over the right interval.
func TestEdgeCarrierStoredForms(t *testing.T) {
	tol := tolerances{eps: 1e-7, angular: 1e-9, margin: 4e-7, size: 6}
	for _, form := range []string{"sf1", "sf2", "sf3"} {
		for _, placed := range []bool{false, true} {
			s, m := stored(t, form, placed)
			o, err := intake(0, s, tol, nil)
			if err != nil {
				t.Fatalf("%s placed %v: %v", form, placed, err)
			}
			axis := line{m.Apply(geom.Vec3{X: 1, Y: 1}), m.ApplyVector(geom.Vec3{Z: 1})}
			turns := [2]float64{}
			for i, e := range o.edges {
				a, b := e.c.at(e.t0), e.c.at(e.t1)
				if a.Distance(o.verts[e.start]) > tol.eps || b.Distance(o.verts[e.end]) > tol.eps || !(e.t0 < e.t1) {
					t.Fatalf("%s placed %v edge %d: runs from %v to %v, vertices %v and %v", form, placed, i, a, b, o.verts[e.start], o.verts[e.end])
				}
				switch k := e.c.(type) {
				case line:
					if along := math.Abs(k.dir.Dot(axis.dir)); along > 1e-9 && along < 1-1e-9 {
						t.Fatalf("%s placed %v edge %d: a line neither along nor across the axis", form, placed, i)
					}
				case conic:
					height := axis.param(k.center)
					if k.a != k.b && math.Abs(k.a-k.b) > tol.eps || math.Abs(k.a-0.5) > tol.eps || off(axis, k.center) > tol.eps || math.Abs(math.Abs(k.normal().Dot(axis.dir))-1) > 1e-9 || math.Min(math.Abs(height), math.Abs(height-2)) > tol.eps {
						t.Fatalf("%s placed %v edge %d: conic %+v", form, placed, i, k)
					}
					turns[int(math.Round(height/2))] += (e.t1 - e.t0) / (2 * math.Pi)
				}
			}
			if math.Abs(turns[0]-1) > 1e-9 || math.Abs(turns[1]-1) > 1e-9 {
				t.Fatalf("%s placed %v: the rim edges make %v turns, want one at each end", form, placed, turns)
			}
		}
	}
}

// helix is one turn of a helix on the cylinder of stored, from its
// bottom seam vertex to its top one.
type helix struct{}

func (helix) Eval(t float64) (geom.Vec3, error) {
	s, c := math.Sincos(2 * math.Pi * t)
	return geom.Vec3{X: 1 + 0.5*c, Y: 1 + 0.5*s, Z: 2 * t}, nil
}
func (helix) Derivative(t float64) (geom.Vec3, error) {
	s, c := math.Sincos(2 * math.Pi * t)
	return geom.Vec3{X: -math.Pi * s, Y: math.Pi * c, Z: 2}, nil
}

// TestEdgeCarrierFailures covers the edges I5 refuses on a curved face.
func TestEdgeCarrierFailures(t *testing.T) {
	tol := tolerances{eps: 1e-7, angular: 1e-9, margin: 4e-7, size: 6}
	t.Run("a helical edge", func(t *testing.T) {
		// The wall of the sf2 cylinder, split in two faces along a helix
		// from one end of its seam to the other.
		s, _ := stored(t, "sf2", false)
		edges, faces := s.Edges(), s.Faces()
		edges = append(edges, parametric.Edge{Start: 0, End: 1, Curve: helix{}, Range: parametric.Interval{Max: 1}})
		split := len(edges) - 1
		wall := faces[0]
		turn, top := 2*math.Pi, 2.
		lower := trimmed.Face{Surface: wall.Surface, Outer: []trimmed.Use{
			{Edge: 0, UVStart: geom.Vec2{}, UVEnd: geom.Vec2{X: turn}},
			{Edge: 1, UVStart: geom.Vec2{X: turn}, UVEnd: geom.Vec2{X: turn, Y: top}},
			{Edge: split, Reversed: true, UVStart: geom.Vec2{X: turn, Y: top}, UVEnd: geom.Vec2{}},
		}}
		upper := trimmed.Face{Surface: wall.Surface, Outer: []trimmed.Use{
			{Edge: split, UVStart: geom.Vec2{}, UVEnd: geom.Vec2{X: turn, Y: top}},
			{Edge: 2, UVStart: geom.Vec2{X: turn, Y: top}, UVEnd: geom.Vec2{Y: top}},
			{Edge: 1, Reversed: true, UVStart: geom.Vec2{Y: top}, UVEnd: geom.Vec2{}},
		}}
		twisted, err := trimmed.NewSolid(s.Vertices(), edges, append([]trimmed.Face{lower, upper}, faces[1:]...), 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Combine(context.Background(), twisted, trimmed.Solid{}, Union, matrixOptions)
		if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "operand A edge 7") {
			t.Fatalf("got %v, want ErrUnsupported naming operand A edge 7", err)
		}
	})
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	round, err := geom.NewEllipse(frame, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	floor, wall := plane{frame}, cylinder{frame, 1}
	whole := parametric.Edge{Curve: round, Range: parametric.Interval{Max: 2 * math.Pi}}
	t.Run("an uneven full circle", func(t *testing.T) {
		// Three quarters of the turn lie in the first sixteenth of the
		// range: the first step between samples is refined.
		even, err := edgeCarrier(whole, floor, wall, tol)
		if err != nil {
			t.Fatal(err)
		}
		uneven := whole
		uneven.Curve = warped{round, func(s float64) float64 {
			if s < 1./16 {
				return 12 * s
			}
			return 0.75 + (s-1./16)*0.25/(15./16)
		}}
		got, err := edgeCarrier(uneven, floor, wall, tol)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(got.t0-even.t0) > 1e-9 || math.Abs(got.t1-even.t1) > 1e-9 || got.c.at(1).Distance(even.c.at(1)) > tol.eps {
			t.Fatalf("got %+v, want %+v", got, even)
		}
	})
	t.Run("a circle walked forward and back", func(t *testing.T) {
		there := whole
		there.Curve = warped{round, func(s float64) float64 { return 0.25 - math.Abs(s-0.5)/2 }}
		there.Start, there.End = 0, 0
		if _, err := edgeCarrier(there, floor, wall, tol); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("got %v, want ErrUnsupported", err)
		}
	})
}

// warped walks a curve of range 2π by a map of the unit interval to
// turns.
type warped struct {
	base geom.Curve
	turn func(float64) float64
}

func (c warped) Eval(t float64) (geom.Vec3, error) {
	return c.base.Eval(2 * math.Pi * c.turn(t/(2*math.Pi)))
}
func (c warped) Derivative(t float64) (geom.Vec3, error) {
	return c.base.Derivative(2 * math.Pi * c.turn(t/(2*math.Pi)))
}
