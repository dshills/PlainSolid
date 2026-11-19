package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestOriginalSplineGraphLineSection(t *testing.T) {
	ctx := context.Background()
	base, a, b := func() (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
		t.Helper()
		build := func(du int, u, knots, z []float64) *geom.BSplineSurface {
			var p []geom.Vec3
			for i, x := range u {
				for _, y := range []float64{0, 1} {
					p = append(p, geom.Vec3{X: x, Y: y, Z: z[i]})
				}
			}
			s, e := geom.NewBSplineSurface(du, 1, len(u), 2, p, knots, []float64{0, 0, 1, 1}, nil)
			if e != nil {
				t.Fatal(e)
			}
			return s
		}
		k := []float64{0, 0, 1, 1}
		return build(1, []float64{0, 1}, k, []float64{0, 0}),
			build(1, []float64{0, .5, 1}, []float64{0, 0, .5, 1, 1}, []float64{3, 2, 3}),
			build(2, []float64{0, .5, 1}, []float64{0, 0, 0, 1, 1, 1}, []float64{3, 1, 3})
	}()
	p, e := NewSharedBaseSplineGraphMaterial(ctx, base, a, b, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-11}
	r, e := p.SectionOriginalGraphsLine(ctx, geom.Vec3{X: -1, Y: .5, Z: 2}, geom.Vec3{X: 3}, 0, 1, o)
	if e != nil || r.Clip.Lower.Cmp(big.NewRat(1, 3)) != 0 || r.Clip.Upper.Cmp(big.NewRat(2, 3)) != 0 ||
		len(r.Graph[0].Hits) != 0 || len(r.Graph[1].Hits) != 1 || len(r.Graph[2].Hits) != 1 {
		t.Fatal(r, e)
	}
	h, z := r.Graph[1].Hits[0], r.Graph[2].Hits[0]
	if h.Parameter.Min.Cmp(big.NewRat(1, 2)) != 0 || h.Parameter.Max.Cmp(big.NewRat(1, 2)) != 0 ||
		len(h.Pieces) != 2 || z.Parameter.Multiplicity != 2 || z.Parameter.Min.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(h, z)
	}
	r, e = p.SectionOriginalGraphsLine(ctx, geom.Vec3{X: -1, Y: .5}, geom.Vec3{X: 3}, 0, 1, o)
	if e != nil || len(r.Graph[0].Coincident) != 1 || len(r.Graph[0].Hits) != 0 ||
		r.Graph[0].Coincident[0].Lower.Cmp(big.NewRat(1, 3)) != 0 || r.Graph[0].Coincident[0].Upper.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal(r, e)
	}
	r, e = p.SectionOriginalGraphsLine(ctx, geom.Vec3{X: -1, Y: .5, Z: 2.125}, geom.Vec3{X: 3}, 0, 1, o)
	if e != nil || len(r.Graph[2].Hits) != 2 {
		t.Fatal(r, e)
	}
	for i, hit := range r.Graph[2].Hits {
		want := (1.5 + [2]float64{-1, 1}[i]*math.Sqrt(.125/4)) / 3
		l, _ := hit.Parameter.Min.Float64()
		h, _ := hit.Parameter.Max.Float64()
		if want < l-1e-15 || want > h+1e-15 || h-l > 1.01*o.ParameterTolerance {
			t.Fatal(hit, want)
		}
	}
	hits, e := p.IntersectEnvelopeLine(ctx, geom.Vec3{X: -1, Y: .5, Z: 2}, geom.Vec3{X: 3}, 0, 1, parametric.Union, o)
	if e != nil || len(hits) != 3 || len(hits[0].Sides) != 1 || hits[0].Sides[0] != 3 ||
		len(hits[1].TopFaces) != 2 || hits[1].Parameter.Multiplicity != 2 || len(hits[2].Sides) != 1 || hits[2].Sides[0] != 1 {
		t.Fatal(hits, e)
	}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection} {
		hits, e := p.IntersectEnvelopeLine(ctx, geom.Vec3{X: -1, Y: .5, Z: 2.125}, geom.Vec3{X: 3}, 0, 1, op, o)
		if e != nil || len(hits) != 4 || len(hits[1].TopFaces) != 1 || hits[1].TopFaces[0] != i ||
			len(hits[2].TopFaces) != 1 || hits[2].TopFaces[0] != i {
			t.Fatal(op, hits, e)
		}
	}
	if hits, e := p.IntersectEnvelopeLine(ctx, geom.Vec3{X: -1, Y: .5}, geom.Vec3{X: 3}, 0, 1, parametric.Union, o); !errors.Is(e, geom.ErrConvergence) || len(hits) != 0 {
		t.Fatal("coplanar base was not flagged", hits, e)
	}

	material, e := p.ClipEnvelopeMaterialLine(ctx, geom.Vec3{X: -1, Y: .5, Z: 2.125}, geom.Vec3{X: 3}, 0, 1, parametric.Union, o)
	if e != nil || len(material.Intervals) != 2 || len(material.Hits) != 4 {
		t.Fatal(material, e)
	}
	material, e = p.ClipEnvelopeMaterialLine(ctx, geom.Vec3{X: -1, Y: .5, Z: 2}, geom.Vec3{X: 3}, 0, 1, parametric.Union, o)
	if e != nil || len(material.Intervals) != 1 || len(material.Hits) != 3 ||
		material.Intervals[0][0].Min.Cmp(big.NewRat(1, 3)) != 0 || material.Intervals[0][1].Max.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal(material, e)
	}

	side, e := p.IntersectEnvelopeSideLine(ctx, geom.Vec3{X: -1, Z: 2.125}, geom.Vec3{X: 3}, 0, 1, parametric.Union, o)
	if e != nil || side.Side != 0 || len(side.Hits) != 4 || len(side.Spans) != 2 ||
		side.Spans[0][0].Min.Cmp(big.NewRat(1, 3)) != 0 || side.Spans[1][1].Max.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal(side, e)
	}
	side, e = p.IntersectEnvelopeSideLine(ctx, geom.Vec3{X: -1, Z: 2}, geom.Vec3{X: 3}, 0, 1, parametric.Union, o)
	if e != nil || len(side.Hits) != 3 || len(side.Spans) != 1 {
		t.Fatal("tangency does not split side wall material", side, e)
	}
	side, e = p.IntersectEnvelopeSideLine(ctx, geom.Vec3{X: -1, Z: 1}, geom.Vec3{X: 3}, .5, .5, parametric.Union, o)
	if e != nil || len(side.Hits) != 1 || len(side.Spans) != 0 || len(side.Hits[0].Sides) != 1 {
		t.Fatal("single parameter in side interior", side, e)
	}

}

func TestParallelObliqueSplineGraphBoolean(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	graph := func(c, f float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				u, v := float64(i)/2, float64(j)/2
				x2, y2 := 0., 0.
				if i == 2 {
					x2 = 1
				}
				if j == 2 {
					y2 = 1
				}
				// 8(u+v-.5)(u+v-1.5) above a common graph.
				z := c + f*(8*(x2+2*u*v+y2)-16*(u+v)+6)
				p = append(p, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := graph(0, 0), graph(10, 1), graph(10, 0)
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineParallelObliqueLineSplineGraphs(ctx, base, a, b, op, 1e-9, o, geom.GraphFactorAxisLineOptions{})
		wantShells := 1
		if op == parametric.Difference {
			wantShells = 2
		}
		if e != nil || h.ParallelOblique == nil || len(h.ContourEdges) != 2 || len(s.Topology().Shells) != wantShells {
			t.Fatal(h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [3]float64{10 + 5.0/12, 10 - 13.0/12, 5.0 / 12}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
	}
	s, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Union, 1e-9, o)
	if e != nil || h.ParallelOblique == nil || s.Topology().EulerCharacteristic != 2 {
		t.Fatal("automatic parallel oblique fallback", h, s.Topology(), e)
	}
}

func TestIntersectingObliqueSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	build := func(c, f float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				u, v := float64(i)/2, float64(j)/2
				u2, v2 := 0., 0.
				if i == 2 {
					u2 = 1
				}
				if j == 2 {
					v2 = 1
				}
				// z=c+f[(u-.5)^2-(v-.5)^2].
				p = append(p, geom.Vec3{X: u, Y: v, Z: c + f*(u2-v2-u+v)})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0, 0), build(4, 1), build(4, 0)
	for _, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineIntersectingObliqueLineSplineGraphs(ctx, base, a, b, op, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphFactorAxisLineOptions{})
		if h.IntersectingOblique == nil || h.IntersectingOblique.Crossing[0].Cmp(big.NewRat(1, 2)) != 0 ||
			h.IntersectingOblique.Crossing[1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal(h, e)
		}
		if op == parametric.Difference {
			if !errors.Is(e, geom.ErrDegenerate) || s.Topology().Faces != 0 {
				t.Fatal(s.Topology(), e)
			}
			continue
		}
		if e != nil || len(h.ContourEdges) != 4 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := 4 + 1.0/24
		if op == parametric.Intersection {
			want = 4 - 1.0/24
		}
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
	}
}

func TestSplineGraphEnvelopeSharedAlgebraicRoot(t *testing.T) {
	ctx := context.Background()
	graph := func(polesZ []float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i, z := range polesZ {
			for _, v := range []float64{0, 1} {
				p = append(p, geom.Vec3{X: float64(i) / float64(len(polesZ)-1), Y: v, Z: z})
			}
		}
		knots := make([]float64, 2*len(polesZ))
		for i := len(polesZ); i < len(knots); i++ {
			knots[i] = 1
		}
		s, e := geom.NewBSplineSurface(len(polesZ)-1, 1, len(polesZ), 2, p, knots, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := graph([]float64{0, 0}), graph([]float64{2, 2, 4}), graph([]float64{1, 1, 5})
	q, e := NewSharedBaseSplineGraphMaterial(ctx, base, a, b, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
	h, e := q.IntersectEnvelopeLine(ctx, geom.Vec3{X: -1, Y: .5, Z: 3}, geom.Vec3{X: 3}, 0, 1, parametric.Union, o)
	if e != nil || len(h) != 2 || len(h[0].TopFaces) != 2 || len(h[1].Sides) != 1 || h[1].Sides[0] != 1 {
		t.Fatal(h, e)
	}
	l, _ := h[0].Parameter.Min.Float64()
	r, _ := h[0].Parameter.Max.Float64()
	want := (1 + math.Sqrt(.5)) / 3
	if l > want || r < want || h[0].Parameter.Min.Cmp(h[0].Parameter.Max) == 0 {
		t.Fatal(h[0], want)
	}
}

func TestSplineGraphEnvelopeCloseBaseAndTopRoots(t *testing.T) {
	ctx := context.Background()
	graph := func(c, scale float64) *geom.BSplineSurface {
		t.Helper()
		k := []float64{0, 0, 0, 1, 1, 1}
		var p []geom.Vec3
		for i := range 3 {
			u := float64(i) / 2
			z := c
			if i == 2 {
				z += scale
			}
			for _, v := range []float64{0, 1} {
				p = append(p, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, p, k, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := graph(math.Ldexp(1, -44), 1)
	b := graph(math.Ldexp(1, -43), 1)
	p, e := NewSharedBaseSplineGraphMaterial(ctx, graph(0, 1), a, b, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := geom.ToroidLineOptions{ParameterTolerance: .05}
	h, e := p.IntersectEnvelopeLine(ctx, geom.Vec3{Y: .5, Z: .5}, geom.Vec3{X: 1}, 0, 1, parametric.Union, o)
	if e != nil || len(h) != 2 || len(h[0].TopFaces) != 1 || h[0].TopFaces[0] != 1 ||
		!h[1].Base || h[0].Parameter.Max.Cmp(h[1].Parameter.Min) >= 0 {
		t.Fatal(h, e)
	}
	o.MaxSubdivisions = 8
	if h, e := p.IntersectEnvelopeLine(ctx, geom.Vec3{Y: .5, Z: .5}, geom.Vec3{X: 1}, 0, 1, parametric.Union, o); !errors.Is(e, geom.ErrConvergence) || len(h) != 0 {
		t.Fatal("budget exhaustion must not return partial envelope", h, e)
	}
}
