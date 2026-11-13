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

func TestCombineTangentSplineGraphSolids(t *testing.T) {
	ctx := context.Background()
	// 12*(u-.5)^4 + 3*(u-.5)^2 + 3*(v-.5)^2 has a unique zero.
	// These dyadic Bernstein coefficients represent it without rounding.
	u := [5]float64{1.5, -.75, .5, -.75, 1.5}
	v := [5]float64{.75, 0, -.25, 0, .75}
	build := func(height float64, bump bool) *geom.BSplineSurface {
		t.Helper()
		k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
		var p []geom.Vec3
		for i := range 5 {
			for j := range 5 {
				z := height
				if bump {
					z += u[i] + v[j]
				}
				p = append(p, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(4, 4, 5, 5, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0, false), build(2, true), build(2, false)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	_, automatic, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Union, 1e-9, intersect.SplinePlaneOptions{})
	if e != nil || automatic.PointContact == nil || !automatic.PointContact.Proven {
		t.Fatal("automatic contact discovery", automatic, e)
	}
	for _, c := range []struct {
		op   parametric.BooleanOp
		top  *geom.BSplineSurface
		mass float64
	}{{parametric.Union, a, 2.65}, {parametric.Intersection, b, 2}} {
		s, h, e := CombineTangentSplineGraphSolids(ctx, base, a, b, uv, c.op, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{})
		if e != nil || h.PointContact == nil || !h.PointContact.Proven || h.PointContact.Side != 1 || s.faces[0].Surface != c.top {
			t.Fatal(h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-c.mass) > 1e-9 {
			t.Fatal(p, c.mass, e)
		}
	}
	if s, _, e := CombineTangentSplineGraphSolids(ctx, base, a, b, uv, parametric.Difference, 1e-9,
		intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{}); len(s.faces) != 0 || !errors.Is(e, geom.ErrDegenerate) {
		t.Fatal("pinched difference accepted", e)
	}
	if s, h, e := CombineTangentSplineGraphSolids(ctx, base, b, a, uv, parametric.Difference, 1e-9,
		intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{}); len(s.faces) != 0 || e != nil || h.PointContact.Side != -1 {
		t.Fatal("lower minus upper should be empty", h, e)
	}
	raised := build(2+1.0/1024, true)
	s, ordered, e := CombineSplineGraphSolids(ctx, base, raised, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{
		Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}},
	})
	if e != nil || ordered.Ordering == nil || !ordered.Ordering.Proven || ordered.Ordering.MarginLower.Cmp(big.NewRat(1, 1024)) != 0 {
		t.Fatal("strict ordering fallback", ordered, e)
	}
	m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-(.65+1.0/1024)) > 1e-9 {
		t.Fatal(m, e)
	}
}

func TestDegenerateSplineGraphBooleanContact(t *testing.T) {
	ctx := context.Background()
	build := func(z float64, bump bool) *geom.BSplineSurface {
		k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
		h := [5]float64{.0625, -.0625, .0625, -.0625, .0625}
		var p []geom.Vec3
		for i := range 5 {
			for j := range 5 {
				v := z
				if bump {
					v += h[i] + h[j]
				}
				p = append(p, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: v})
			}
		}
		s, e := geom.NewBSplineSurface(4, 4, 5, 5, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0, false), build(2, true), build(2, false)
	s, h, e := CombineTangentSplineGraphSolids(ctx, base, a, b, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		parametric.Union, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{MaxDepth: 4})
	if e != nil || h.SeparableContact == nil || h.SeparableContact.Orders != ([2]int{4, 4}) {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-2.025) > 1e-9 {
		t.Fatal(p, e)
	}
	_, h, e = CombineSplineGraphSolids(ctx, base, a, b, parametric.Union, 1e-9, intersect.SplinePlaneOptions{
		Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}},
	})
	if e != nil || h.SeparableContact == nil || !h.SeparableContact.Proven {
		t.Fatal("automatic degenerate contact discovery", h, e)
	}
}

func TestCoupledQuarticSplineGraphBooleanContact(t *testing.T) {
	ctx := context.Background()
	// The gap is 9*(x^4+y^4-x²y²), for x=u-1/2,y=v-1/2.
	// Its exact tensor Bernstein net requires only binary coefficients.
	build := func(z float64, bump bool) *geom.BSplineSurface {
		k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
		h4 := [5]float64{1, -1, 1, -1, 1}
		h2 := [5]float64{3, 0, -1, 0, 3}
		var p []geom.Vec3
		for i := range h4 {
			for j := range h4 {
				v := z
				if bump {
					v += (9*(h4[i]+h4[j]) - h2[i]*h2[j]) / 16
				}
				p = append(p, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: v})
			}
		}
		s, e := geom.NewBSplineSurface(4, 4, 5, 5, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := build(0, false), build(2, true), build(2, false)
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 32}}}
	for _, c := range []struct {
		op   parametric.BooleanOp
		top  *geom.BSplineSurface
		mass float64
	}{{parametric.Union, a, 173.0 / 80}, {parametric.Intersection, b, 2}} {
		s, h, e := CombineSplineGraphSolids(ctx, base, a, b, c.op, 1e-9, o)
		if e != nil || h.CoupledQuarticContact == nil || !h.CoupledQuarticContact.Proven || h.CoupledQuarticContact.Side != 1 || s.faces[0].Surface != c.top {
			t.Fatal(c.op, h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-c.mass) > 1e-9 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(c.op, p, c.mass, e)
		}
	}
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	if s, h, e := CombineTangentSplineGraphSolids(ctx, base, a, b, uv, parametric.Difference, 1e-9, o,
		geom.GraphPointContactOptions{}); len(s.faces) != 0 || h.CoupledQuarticContact == nil || !errors.Is(e, geom.ErrDegenerate) {
		t.Fatal("pinched difference", h, e)
	}
	if s, h, e := CombineSplineGraphSolids(ctx, base, b, a, parametric.Difference, 1e-9, o); len(s.faces) != 0 ||
		h.CoupledQuarticContact == nil || h.CoupledQuarticContact.Side != -1 || e != nil {
		t.Fatal("lower minus upper", h, e)
	}
}
