package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCombineSeparatedSplineShellsNested(t *testing.T) {
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	m := geom.Identity()
	m[0][0], m[1][1], m[2][2] = .25, .25, .25
	m[0][3], m[1][3], m[2][3] = .5, .5, .25
	b, e := a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		op           parametric.BooleanOp
		volume       float64
		shells       int
		keepA, keepB bool
	}{{parametric.Union, 5, 1, true, false}, {parametric.Intersection, 5.0 / 64, 1, false, true},
		{parametric.Difference, 5 - 5.0/64, 2, true, true}} {
		s, h, e := CombineSeparatedSplineShells(ctx, a, b, tc.op, SplineShellBooleanOptions{})
		if e != nil || h.AInsideB || !h.BInsideA || h.BReversed != (tc.op == parametric.Difference) ||
			len(s.Topology().Shells) != tc.shells || (h.FacesA[0] >= 0) != tc.keepA || (h.FacesB[0] >= 0) != tc.keepB {
			t.Fatal(tc, h, e)
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-tc.volume) > 1e-9 {
			t.Fatal(tc, m, e)
		}
	}
	s, h, e := CombineSeparatedSplineShells(ctx, b, a, parametric.Difference, SplineShellBooleanOptions{})
	if e != nil || !h.AInsideB || h.BInsideA || len(s.Faces()) != 0 {
		t.Fatal(h, e)
	}
}

func TestCombineSeparatedSplineShellsDisjointAndContact(t *testing.T) {
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	m := geom.Identity()
	m[0][3] = 3
	b, e := a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineSeparatedSplineShells(ctx, a, b, parametric.Union, SplineShellBooleanOptions{})
	if e != nil || h.AInsideB || h.BInsideA || len(s.Topology().Shells) != 2 || h.FacesB[0] != 6 {
		t.Fatal(h, e)
	}
	s, _, e = CombineSeparatedSplineShells(ctx, a, b, parametric.Intersection, SplineShellBooleanOptions{})
	if e != nil || len(s.Faces()) != 0 {
		t.Fatal(e)
	}
	m[0][3] = 2
	b, e = a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = CombineSeparatedSplineShells(ctx, a, b, parametric.Union, SplineShellBooleanOptions{
		Separation: geom.SurfaceSurfaceIsolationOptions{MaxPairs: 16},
	})
	if !errors.Is(e, geom.ErrConvergence) || len(s.Faces()) != 0 {
		t.Fatal(e)
	}
}
