package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestOpenPlaneShellSimultaneous(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	b := splineSimultaneousTransform(t, a, geom.Affine{{0, .25, .25, .875}, {.25, 0, .25, .875}, {.25, .25, 0, .5}})
	pairs := [][2]int{{1, 0}, {1, 4}, {3, 2}, {3, 4}}

	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineOpenPlaneSplineShells(ctx, a, b, op, pairs, intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
		if e != nil {
			t.Fatal(op, e)
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [3]float64{1 + 1.0/32 - 1.0/768, 1.0 / 768, 1 - 1.0/768}[i]
		if e != nil || math.Abs(m.Volume-want) > 1e-10 || len(h.SectionEdges) == 0 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(m, e)
		}
	}
	// Omitting one of the four incident pairs must not turn the simultaneous
	// endpoint into an ordinary single-owner terminal or partial solid.
	bad, _, e := CombineOpenPlaneSplineShells(ctx, a, b, parametric.Intersection, pairs[:3], intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
	if e == nil || len(bad.Faces()) != 0 {
		t.Fatal("missing incident face", e)
	}
}

func TestAutomaticWarpedOpenPlaneShell(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}, .125)
	section, e := SectionSplineShells(ctx, a, b, SplineShellCombineOptions{})
	if e != nil || !section.Complete || len(section.Wires) != 1 {
		t.Fatal(e)
	}
	s, _, e := CombineSplineShells(ctx, a, b, parametric.Intersection, SplineShellCombineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-.125) > 1e-10 {
		t.Fatal(m, e)
	}
}
