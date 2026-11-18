package trimmed

import (
	"context"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCombineSplineShellsDispatch(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	check := func(b Solid, op parametric.BooleanOp, method string, faces int) {
		t.Helper()
		s, h, e := CombineSplineShells(ctx, a, b, op, SplineShellCombineOptions{})
		if e != nil || h.Method != method || len(s.faces) != faces {
			t.Fatal(method, len(s.faces), h, e)
		}
	}
	check(a, parametric.Union, "coincident", 6)
	check(a, parametric.Difference, "coincident", 0)
	m := geom.Identity()
	m[0][3] = 3
	b, e := a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	check(b, parametric.Union, "separated", 12)
	m[0][0], m[0][3] = -1, 4
	b, e = a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	check(b, parametric.Union, "shared-face", 10)
}

func TestCombineSplineShellsDispatchClosed(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b, _ := transverseSplineCaps(t)
	o := SplineShellCombineOptions{}
	o.Transverse.Trace.PlaneFallback.UCuts = []float64{.25, .3125, .6875, .75}
	o.Transverse.Trace.PlaneFallback.VCuts = o.Transverse.Trace.PlaneFallback.UCuts
	o.Transverse.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Transverse.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Transverse.Trace.Coverage.Cell.MaxOperations = 16000000
	s, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, o)
	if e != nil || h.Method != "closed-contours" || len(s.faces) != 2 || len(h.SectionEdges) == 0 {
		t.Fatal(h, e)
	}
}

func TestCombineSplineShellsDispatchAffineOpen(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}, 0)
	s, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, SplineShellCombineOptions{})
	if e != nil || h.Method != "affine-open-wire" || len(h.SectionEdges) != 12 || len(s.faces) != 6 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || p.Volume < .125-1e-10 || p.Volume > .125+1e-10 {
		t.Fatal(p, e)
	}
}

func TestCombineSplineShellsBoundaryContact(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	for _, lo := range [][3]float64{{1, 1, 0}, {1, 1, 1}} {
		b, _ := makeOpenSplineBox(t, lo, [3]float64{2, 2, 2}, 0)
		o := SplineShellCombineOptions{}
		o.Transverse.Shells.Separation.MaxPairs = 16
		s, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, o)
		if e != nil || h.Method != "boundary-contact" || len(s.faces) != 0 {
			t.Fatal(lo, h, e)
		}
		s, h, e = CombineSplineShells(ctx, a, b, parametric.Difference, o)
		if e != nil || h.Method != "boundary-contact" || len(s.faces) != 6 {
			t.Fatal(lo, h, e)
		}
		if _, _, e := CombineSplineShells(ctx, a, b, parametric.Union, o); e == nil {
			t.Fatal("touch-only union would not be a manifold boundary")
		}
	}
}
