package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCombineFaceJoinedSplineShells(t *testing.T) {
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	m := geom.Identity()
	m[0][0], m[0][3] = -1, 4
	b, e := a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		op     parametric.BooleanOp
		faces  int
		volume float64
	}{{parametric.Union, 10, 10}, {parametric.Intersection, 0, 0}, {parametric.Difference, 6, 5}} {
		s, h, e := CombineFaceJoinedSplineShells(ctx, a, b, tc.op, SplineShellJoinOptions{})
		if e != nil || h.InterfaceA != 3 || h.InterfaceB != 3 || len(s.Faces()) != tc.faces {
			t.Fatal(tc, h, e)
		}
		if tc.faces == 0 {
			continue
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-tc.volume) > 1e-9 || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(tc, m, e)
		}
		if tc.op == parametric.Union && (h.FacesA[3] != -1 || h.FacesB[3] != -1 || h.FacesA[1] < 0 || h.FacesB[1] < 0) {
			t.Fatal(h)
		}
	}
	m[1][3] = .25
	b, e = a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := CombineFaceJoinedSplineShells(ctx, a, b, parametric.Union, SplineShellJoinOptions{})
	if !errors.Is(e, geom.ErrConvergence) || len(s.Faces()) != 0 {
		t.Fatal(e)
	}
}

func TestCombineInsetJoinedSplineShells(t *testing.T) {
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	m := geom.Identity()
	m[0][0], m[1][1], m[2][2] = .5, .5, -.5
	m[0][3], m[1][3] = .5, .5
	b, e := a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		op     parametric.BooleanOp
		faces  int
		volume float64
	}{{parametric.Union, 11, 5.625}, {parametric.Intersection, 0, 0}, {parametric.Difference, 6, 5}} {
		s, h, e := CombineInsetJoinedSplineShells(ctx, a, b, tc.op, SplineShellJoinOptions{})
		if e != nil || h.InterfaceA != 0 || h.InterfaceB != 0 || len(s.Faces()) != tc.faces {
			t.Fatal(tc, h, e)
		}
		if tc.faces == 0 {
			continue
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-tc.volume) > 1e-9 || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(tc, m, e)
		}
		if tc.op == parametric.Union && (h.FacesA[0] != 0 || h.FacesB[0] != -1 || len(s.Faces()[0].Holes) != 1) {
			t.Fatal(h)
		}
	}
	m[0][3] = 0 // The smaller face meets the larger one's outer wire.
	b, e = a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := CombineInsetJoinedSplineShells(ctx, a, b, parametric.Union, SplineShellJoinOptions{})
	if !errors.Is(e, geom.ErrConvergence) || len(s.Faces()) != 0 {
		t.Fatal(e)
	}
}

func TestCombineOverlappingInterfaceSplineShells(t *testing.T) {
	ctx := context.Background()
	a := testWarpedSplinePrism(t, false, false)
	m := geom.Identity()
	m[2][2] = -1
	m[0][3], m[1][3] = 1, .5
	b, e := a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineOverlappingInterfaceSplineShells(ctx, a, b, parametric.Union, SplineShellJoinOptions{})
	if e != nil || h.InterfaceA != 0 || h.InterfaceB != 0 || len(h.FacesA[0]) != 3 || len(h.FacesB[0]) != 3 || len(s.Faces()) != 16 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-10) > 1e-9 || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(p, e)
	}
	proof, e := s.ValidateEdgeAgreement(ctx, SplineRectangleEmbeddingOptions{}.EdgeAgreement)
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
	m[0][3] = 2
	b, e = a.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = CombineOverlappingInterfaceSplineShells(ctx, a, b, parametric.Union, SplineShellJoinOptions{})
	if !errors.Is(e, geom.ErrConvergence) || len(s.Faces()) != 0 {
		t.Fatal(e)
	}
}
