package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func localSewFaces(s Solid) []SewFace {
	var out []SewFace
	for _, f := range s.Faces() {
		p := SewFace{Face: f}
		v, e := map[int]int{}, map[int]int{}
		vertex := func(i int) int {
			if id, ok := v[i]; ok {
				return id
			}
			id := len(p.Vertices)
			p.Vertices = append(p.Vertices, s.vertices[i])
			v[i] = id
			return id
		}
		for _, l := range append([][]Use{p.Face.Outer}, p.Face.Holes...) {
			for i, u := range l {
				id, ok := e[u.Edge]
				if !ok {
					id = len(p.Edges)
					edge := s.edges[u.Edge]
					edge.Start, edge.End = vertex(edge.Start), vertex(edge.End)
					p.Edges = append(p.Edges, edge)
					e[u.Edge] = id
				}
				l[i].Edge = id
			}
		}
		out = append(out, p)
	}
	return out
}

func TestSewTrimmedPlanar(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	x, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	parts := localSewFaces(x)
	reversed := &parts[0].Edges[0]
	l := reversed.Curve.(geom.Line)
	start, e := l.Eval(reversed.Range.Max)
	if e != nil {
		t.Fatal(e)
	}
	reversed.Curve, _ = geom.NewLine(start, l.Direction().Scale(-1))
	reversed.Start, reversed.End = reversed.End, reversed.Start
	parts[0].Face.Outer[0].Reversed = !parts[0].Face.Outer[0].Reversed
	s, h, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-10})
	if e != nil || s.Topology().EulerCharacteristic != 2 || len(s.edges) != 12 || len(h.Edges) != 6 {
		t.Fatal(h, s.Topology(), e)
	}
	if _, _, e = Sew(ctx, parts[:5], SewOptions{Tolerance: 1e-10}); e == nil {
		t.Fatal("accepted open graph")
	}
	parts[0].Vertices[0].Z += 1e-13
	if _, _, e = Sew(ctx, parts, SewOptions{Tolerance: 1e-10}); e == nil {
		t.Fatal("implicitly welded near vertices")
	}
}

func TestSewCertifiedSplineSection(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
	r := [3]float64{.25, -.25, .25}
	var p, q []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			x := geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 1}
			q = append(q, x)
			x.Z += r[i] + r[j] - .1171875
			p = append(p, x)
		}
	}
	a, _ := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
	b, _ := geom.NewBSplineSurface(2, 2, 3, 3, q, k, k, nil)
	s, _, e := SealSplineGraphPair(ctx, a, b, 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{})
	if e != nil {
		t.Fatal(e)
	}
	parts := localSewFaces(s)
	s, h, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-7})
	if e != nil || len(h.Edges) != 2 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	props, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-5})
	if e != nil || math.Abs(props.Volume-math.Pi*.1171875*.1171875/2) > 1e-8 {
		t.Fatal(props, e)
	}
}

func TestSewIndependentSplineEdges(t *testing.T) {
	ctx := context.Background()
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	base, e := FromPlanar(box, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	parts := localSewFaces(base)
	var natural []SewFace
	for i := range parts {
		f := &parts[i]
		frame := f.Face.Surface.(geom.PlaneSurface).Frame()
		lo, hi := f.Face.Outer[0].UVStart, f.Face.Outer[0].UVStart
		for _, u := range f.Face.Outer {
			lo = geom.Vec2{X: min(lo.X, u.UVStart.X), Y: min(lo.Y, u.UVStart.Y)}
			hi = geom.Vec2{X: max(hi.X, u.UVStart.X), Y: max(hi.Y, u.UVStart.Y)}
		}
		point := func(x, y float64) geom.Vec3 { return frame.ToWorld(geom.Vec3{X: x, Y: y}) }
		p := []geom.Vec3{point(lo.X, lo.Y), point(lo.X, hi.Y), point(hi.X, lo.Y), point(hi.X, hi.Y)}
		k, l := []float64{lo.X, lo.X, hi.X, hi.X}, []float64{lo.Y, lo.Y, hi.Y, hi.Y}
		f.Face.Surface, e = geom.NewBSplineSurface(1, 1, 2, 2, p, k, l, nil)
		if e != nil {
			t.Fatal(e)
		}
		n, e := RectangularSplineFace(f.Face.Surface.(*geom.BSplineSurface), false)
		if e != nil {
			t.Fatal(e)
		}
		natural = append(natural, n)
		for j := range f.Edges {
			ed := &f.Edges[j]
			a, b := f.Vertices[ed.Start], f.Vertices[ed.End]
			if i%2 == 0 {
				ed.Curve, e = geom.NewBSpline(1, []geom.Vec3{a, b}, []float64{2, 2, 4, 4}, nil)
				ed.Range = parametric.Interval{Min: 2, Max: 4}
			} else {
				ed.Curve, e = geom.NewBezier([]geom.Vec3{a, a.Scale(.5).Add(b.Scale(.5)), b}, nil)
				ed.Range = parametric.Interval{Max: 1}
			}
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	rebuilt, _, e := Sew(ctx, natural, SewOptions{Tolerance: 1e-9})
	if e != nil || len(rebuilt.edges) != 12 || rebuilt.Topology().EulerCharacteristic != 2 {
		t.Fatal(rebuilt.Topology(), e)
	}
	var supports []*geom.BSplineSurface
	for _, f := range natural {
		supports = append(supports, f.Face.Surface.(*geom.BSplineSurface))
	}
	rebuilt, _, e = SewSplineRectangles(ctx, supports, make([]geom.ParameterRectangle, len(supports)),
		make([]bool, len(supports)), SewOptions{Tolerance: 1e-9})
	if e != nil || len(rebuilt.edges) != 12 {
		t.Fatal(rebuilt.Topology(), e)
	}
	chords, e := rebuilt.CertifiedEdgeChords(ctx, geom.CertifiedTessellationOptions{Deflection: 1e-6}, 12)
	if e != nil || len(chords) != 12 {
		t.Fatal(chords, e)
	}
	for _, c := range chords {
		edge := rebuilt.edges[c.Edge]
		if c.Start != rebuilt.vertices[edge.Start] || c.End != rebuilt.vertices[edge.End] || c.ErrorUpper != 0 {
			t.Fatal(c)
		}
	}
	s, h, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-9})
	if e != nil || len(h.Edges) != 6 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	props, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(props.Volume-1) > 1e-12 {
		t.Fatal(props, e)
	}
	edge := &parts[1].Edges[0]
	a, b := parts[1].Vertices[edge.Start], parts[1].Vertices[edge.End]
	edge.Curve, e = geom.NewBezier([]geom.Vec3{a, a.Scale(.5).Add(b.Scale(.5)).Add(geom.Vec3{X: 1e-11}), b}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = Sew(ctx, parts, SewOptions{Tolerance: 1e-9}); e == nil {
		t.Fatal("accepted distinct spline edges with the same endpoints")
	}
}
