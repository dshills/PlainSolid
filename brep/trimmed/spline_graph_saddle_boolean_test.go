package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestQuadraticSaddleSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	build := func(z, xy, cross float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for _, x := range []float64{0, 1} {
			for _, y := range []float64{0, 1} {
				p = append(p, geom.Vec3{X: x, Y: y, Z: z + xy*x*y + cross*(x-.5)*(y-.5)})
			}
		}
		k := []float64{0, 0, 1, 1}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0, 0, 0), build(2, .5, 1), build(2, .5, 0)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection} {
		s, h, e := CombineQuadraticSaddleSplineGraphs(ctx, base, a, b, uv, op, 1e-9, o, geom.GraphPointContactOptions{})
		if e != nil || h.Saddle == nil || len(h.FacesA) != 2 || len(h.FacesB) != 2 || len(h.ContourEdges) != 4 ||
			s.Topology().EulerCharacteristic != 2 {
			t.Fatal(s.Topology(), h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [2]float64{69.0 / 32, 67.0 / 32}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
		var native bytes.Buffer
		if e := WriteJSON(&native, s); e != nil {
			t.Fatal(e)
		}
		q, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := q.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !r.Complete {
			t.Fatal(r, e)
		}
	}
	if _, h, e := CombineSplineGraphSolids(ctx, base, b, a, parametric.Union, 1e-9, o); e != nil || h.Saddle == nil {
		t.Fatal("automatic midpoint saddle fallback", h, e)
	}
	if s, _, e := CombineQuadraticSaddleSplineGraphs(ctx, base, a, b, uv, parametric.Difference, 1e-9, o,
		geom.GraphPointContactOptions{}); !errors.Is(e, geom.ErrConvergence) || len(s.faces) != 0 {
		t.Fatal("pinched difference accepted", e)
	}
}

func TestQuadraticDiagonalSaddleSplineEnvelope(t *testing.T) {
	ctx := context.Background()
	makeTop := func(z float64, saddle bool) *geom.BSplineSurface {
		var poles []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				h := z
				if saddle {
					x2 := [3]float64{.25, -.25, .25}
					h += x2[i] - x2[j]
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: h})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := makeTop(0, false), makeTop(2, true), makeTop(2, false)
	s, h, e := CombineQuadraticSaddleSplineGraphs(ctx, base, a, b, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		parametric.Union, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{})
	if e != nil || len(h.ContourEdges) != 4 || len(h.WallFaces) != 4 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	// The two positive triangular sectors integrate (u-.5)^2-(v-.5)^2
	// to 1/24 in total. The exact rays end at the four square corners.
	if e != nil || math.Abs(p.Volume-(2+1.0/24)) > 1e-10 {
		t.Fatal(p, e)
	}
	for _, ray := range h.Saddle.Section.Terminals {
		if !ray.Exact || ray.SideMask&(ray.SideMask-1) == 0 {
			t.Fatal(ray)
		}
	}
}

func TestDiscoverOffCenterQuadraticSaddleEnvelope(t *testing.T) {
	ctx := context.Background()
	top := func(z float64, saddle bool) *geom.BSplineSurface {
		var poles []geom.Vec3
		for _, u := range []float64{0, 1} {
			for _, v := range []float64{0, 1} {
				h := z
				if saddle {
					h += (u - .25) * (v - .75)
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: h})
			}
		}
		k := []float64{0, 0, 1, 1}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	s, h, e := CombineSplineGraphSolids(ctx, top(0, false), top(2, true), top(2, false), parametric.Union, 1e-9, o)
	if e != nil || h.Saddle == nil || h.Saddle.Section.Certificate.Critical.Parameters[0].Cmp(big.NewRat(1, 4)) != 0 ||
		h.Saddle.Section.Certificate.Critical.Parameters[1].Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-(2+9.0/512)) > 1e-9 {
		t.Fatal(p, e)
	}
}
