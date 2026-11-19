//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNestedSplineMaterialReusesCavityAndIsland(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	outer, _, e := NewWrappedCylinder(ctx, makeCanonicalWrappedCylinder(t, true, 0), 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cavity, _ := makeOpenSplineBox(t, [3]float64{-.5, -.5, 0}, [3]float64{.5, .5, 1}, 0)
	island, _ := makeOpenSplineBox(t, [3]float64{-.25, -.25, .25}, [3]float64{.25, .25, .75}, 0)
	a, h, e := CombineSplineShells(ctx, outer, cavity, parametric.Difference, SplineShellCombineOptions{})
	if e != nil || h.Method != "nested-components" || len(a.Topology().Shells) != 2 {
		t.Fatal(h, e)
	}
	out, h, e := CombineSplineShells(ctx, a, island, parametric.Union, SplineShellCombineOptions{})
	if e != nil || h.Method != "nested-components" || len(out.Topology().Shells) != 3 {
		t.Fatal(h, e)
	}
	p, e := out.ValidateNestedSplineComponents(ctx, SplineShellBooleanOptions{})
	if e != nil || len(p) != 3 || p[0].Parent != -1 || p[1].Parent != 0 || p[2].Parent != 1 || p[2].Depth != 2 {
		t.Fatal(p, e)
	}
	m, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 2e-6})
	if e != nil || math.Abs(m.Volume-(20*math.Pi-1+.125)) > 1e-5 {
		t.Fatal(m, e)
	}
	for _, c := range []struct {
		point geom.Vec3
		want  parametric.Location
	}{{geom.Vec3{Z: .5}, parametric.Inside},
		{geom.Vec3{X: .375, Z: .5}, parametric.Outside},
		{geom.Vec3{X: .5, Z: .5}, parametric.Boundary},
		{geom.Vec3{X: 1, Z: .5}, parametric.Inside},
		{geom.Vec3{X: 2.1}, parametric.Outside}} {
		got, e := out.LocateNestedSplineMaterial(ctx, c.point, SplineShellBooleanOptions{})
		if e != nil || got != c.want {
			t.Fatal(c, got, e)
		}
	}
	intersection, _, e := CombineNestedSplineComponents(ctx, a, island, parametric.Intersection, SplineShellBooleanOptions{})
	if e != nil || len(intersection.faces) != 0 {
		t.Fatal("island is inside original void", e)
	}
	bad, e := AssembleShells(ctx, []PlacedSolid{{outer, geom.Identity()}, {cavity, geom.Identity()}}, 9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := bad.ValidateNestedSplineComponents(ctx, SplineShellBooleanOptions{}); !errors.Is(e, geom.ErrDegenerate) {
		t.Fatal("inconsistent nested orientation accepted", e)
	}
}

func TestNestedSplineLocationCurvedGraphBoundary(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	p := geom.Vec3{X: .5, Y: .5, Z: 1 + 1.0/32}
	for _, c := range []struct {
		z    float64
		want parametric.Location
	}{{p.Z, parametric.Boundary}, {.5, parametric.Inside}, {1.0625, parametric.Outside}} {
		p.Z = c.z
		got, e := s.LocateNestedSplineMaterial(ctx, p, SplineShellBooleanOptions{})
		if e != nil || got != c.want {
			t.Fatal(c, got, e)
		}
	}
}
