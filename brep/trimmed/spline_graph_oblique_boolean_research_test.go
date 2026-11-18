//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestObliqueLineSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	graph := func(c, factor float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for _, v := range []float64{0, 1} {
				u := float64(i) / 2
				// z=c+factor*(1+u)*(2u+v-1).
				u2 := 0.
				if i == 2 {
					u2 = 1
				}
				p = append(p, geom.Vec3{X: u, Y: v, Z: c + factor*(2*u2+u+v+u*v-1)})
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := graph(0, 0), graph(5, 1), graph(5, 0)
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineObliqueLineSplineGraphs(ctx, base, a, b, op, 1e-9, o, geom.GraphFactorAxisLineOptions{})
		if e != nil || h.ObliqueLine == nil || len(h.ContourEdges) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [3]float64{5 + 97.0/96, 5 - 9.0/96, 97.0 / 96}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
	}
	// 3u+v=1 gives an exact but nonbinary bottom terminal.
	var p []geom.Vec3
	for _, u := range []float64{0, 1} {
		for _, v := range []float64{0, 1} {
			p = append(p, geom.Vec3{X: u, Y: v, Z: 5 + 3*u + v - 1})
		}
	}
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, p, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineObliqueLineSplineGraphs(ctx, base, a, b, parametric.Union, 1e-9, o, geom.GraphFactorAxisLineOptions{})
	if !errors.Is(e, geom.ErrConvergence) || h.ObliqueLine == nil || s.Topology().Faces != 0 {
		t.Fatal(h, s.Topology(), e)
	}
	ap, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bp, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	section, e := SectionObliqueSplineGraphs(ctx, ap, bp, geom.GraphFactorAxisLineOptions{})
	if e != nil || !section.Curves[0].Complete || !section.Curves[1].Complete {
		t.Fatal(section, e)
	}
	for i, p := range h.ObliqueLine.Terminals {
		if p[0].Cmp(big.NewRat(1, 3)) == 0 && p[1].Sign() == 0 {
			if section.Points[i][0].Cmp(p[0]) != 0 || section.Points[i][1].Sign() != 0 ||
				section.Points[i][2].Cmp(big.NewRat(5, 1)) != 0 {
				t.Fatal("wrong exact model terminal", section.Points[i])
			}
			return
		}
	}
	t.Fatal("missing exact 1/3 terminal", h.ObliqueLine.Terminals)
}

func TestMultispanObliqueSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	// A=5+(2u+v-1)(1+max(u-.25,0)). The quotient has a crease and
	// the contact crosses its knot at (1/4,1/2); B uses no knot there.
	const q = .25
	var poles []geom.Vec3
	for i, u := range []float64{0, q / 2, q, (1 + q) / 2, 1} {
		for _, v := range []float64{0, 1} {
			z := 5 + (2*u+v-1)*(1+max(u-q, 0))
			if i == 3 {
				// Bernstein midpoint control omits the quadratic
				// coefficient contribution of the last span.
				z -= (1 - q) * (1 - q) / 2
			}
			poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
		}
	}
	vk := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(2, 1, 5, 2, poles, []float64{0, 0, 0, q, q, 1, 1, 1}, vk, nil)
	if e != nil {
		t.Fatal(e)
	}
	flat := func(z float64) *geom.BSplineSurface {
		t.Helper()
		p := []geom.Vec3{{Z: z}, {Y: 1, Z: z}, {X: 1, Z: z}, {X: 1, Y: 1, Z: z}}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, vk, vk, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, b := flat(0), flat(5)
	o := geom.GraphFactorAxisLineOptions{}
	ap, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bp, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := SectionObliqueSplineGraphs(ctx, ap, bp, o)
	if e != nil || len(p.Curves[0].Pieces) != 2 || len(p.Curves[1].Pieces) != 1 ||
		p.Curves[0].Pieces[0].End.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(p, e)
	}
	r, h, e := CombineObliqueLineSplineGraphs(ctx, base, a, b, parametric.Intersection, 1e-9, intersect.SplinePlaneOptions{}, o)
	if e != nil || h.ObliqueLine == nil || len(h.ContourEdges) != 1 || r.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, r.Topology(), e)
	}
	m, e := r.MassProperties(ctx, parametric.IntegrationOptions{})
	// Negative contribution = integrate -.5*(1-2u)^2 *
	// (1+max(u-.25,0)) on [0,.5] = -129/1536.
	if e != nil || math.Abs(m.Volume-(5-129.0/1536)) > 1e-9 {
		t.Fatal(m, e)
	}
}

func TestSplineGraphCoincidentEnvelopeSpans(t *testing.T) {
	ctx := context.Background()
	graph := func(degree int, z []float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i, n := range z {
			for _, v := range []float64{0, 1} {
				p = append(p, geom.Vec3{X: float64(i) / float64(len(z)-1), Y: v, Z: n})
			}
		}
		k := []float64{0, 0, 1, 1}
		if degree == 2 {
			k = []float64{0, 0, 0, 1, 1, 1}
		} else if len(z) == 3 {
			k = []float64{0, 0, .5, 1, 1}
		}
		s, e := geom.NewBSplineSurface(degree, 1, len(z), 2, p, k, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := graph(1, []float64{0, 0}), graph(1, []float64{2, 2}), graph(2, []float64{1, 1, 3})
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
	m, e := NewSharedBaseSplineGraphMaterial(ctx, base, a, b, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	origin, direction := geom.Vec3{X: -1, Y: .5, Z: 2}, geom.Vec3{X: 3}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection} {
		r, e := m.CoincidentEnvelopeSpans(ctx, origin, direction, 0, 1, op, o)
		if e != nil || len(r) != 1 || len(r[0].TopFaces) != 1 || r[0].TopFaces[0] != 0 {
			t.Fatal(r, e)
		}
		h := [2]geom.ToroidLineRoot{r[0].End, r[0].Start}[i]
		l, _ := h.Min.Float64()
		u, _ := h.Max.Float64()
		root := (1 + math.Sqrt(.5)) / 3
		if l > root || u < root {
			t.Fatal(h, root)
		}
	}
	b = graph(1, []float64{2, 2, 3})
	m, e = NewSharedBaseSplineGraphMaterial(ctx, base, a, b, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := m.CoincidentEnvelopeSpans(ctx, origin, direction, 0, 1, parametric.Intersection, o)
	if e != nil || len(r) != 2 || len(r[0].TopFaces) != 2 || len(r[1].TopFaces) != 1 ||
		r[0].End.Min.Cmp(big.NewRat(1, 2)) != 0 || r[1].Start.Min.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(r, e)
	}
	origin.Z = 0
	r, e = m.CoincidentEnvelopeSpans(ctx, origin, direction, 0, 1, parametric.Union, o)
	if e != nil || len(r) != 1 || !r[0].Base || r[0].Start.Min.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
}
