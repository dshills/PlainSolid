package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func testWarpedSplinePrism(t *testing.T, inward, crossing bool) Solid {
	t.Helper()
	ctx := context.Background()
	var surfaces []*geom.BSplineSurface
	var reversed []bool
	var domains []geom.ParameterRectangle
	add := func(f func(u, v float64) geom.Vec3, n int, reverse bool) {
		t.Helper()
		var knots []float64
		for i := 0; i < n; i++ {
			knots = append(knots, 0)
		}
		for i := 0; i < n; i++ {
			knots = append(knots, 1)
		}
		var poles []geom.Vec3
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				poles = append(poles, f(float64(i)/float64(n-1), float64(j)/float64(n-1)))
			}
		}
		s, e := geom.NewBSplineSurface(n-1, n-1, n, n, poles, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces, reversed = append(surfaces, s), append(reversed, reverse != inward)
		domains = append(domains, geom.ParameterRectangle{})
	}
	h := func(x, y float64) float64 { return 1 + x*y/4 }
	add(func(u, v float64) geom.Vec3 { return geom.Vec3{X: 2 * u, Y: 2 * v} }, 2, true)
	add(func(u, v float64) geom.Vec3 {
		x, y := 2*u, 2*v
		z := h(x, y)
		if crossing && u == .5 && v == .5 {
			z -= 8
		}
		return geom.Vec3{X: x, Y: y, Z: z}
	}, 3, false)
	for _, x := range []float64{0, 2} {
		add(func(u, v float64) geom.Vec3 { return geom.Vec3{X: x, Y: 2 * u, Z: v * h(x, 2*u)} }, 2, x == 0)
	}
	for _, y := range []float64{0, 2} {
		add(func(u, v float64) geom.Vec3 { return geom.Vec3{X: 2 * u, Y: y, Z: v * h(2*u, y)} }, 2, y == 2)
	}
	s, _, e := SewSplineRectangles(ctx, surfaces, domains, reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestSplineRectangleEmbeddingWarpedPrism(t *testing.T) {
	ctx := context.Background()
	s := testWarpedSplinePrism(t, false, false)
	q, e := s.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{})
	if e != nil || !q.Complete || q.OutwardFace < 0 || len(q.Faces) != 6 || len(q.Pairs) != 15 {
		t.Fatal(q, e)
	}
	m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-5) > 1e-9 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(m, e)
	}
	q, e = s.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{MaxFaces: 5})
	if e == nil || q.Complete {
		t.Fatal(q, e)
	}
}

func TestSplineRectangleEmbeddingRejectsInwardOrCrossing(t *testing.T) {
	ctx := context.Background()
	s := testWarpedSplinePrism(t, true, false)
	q, e := s.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{})
	if !errors.Is(e, geom.ErrDegenerate) || q.Complete || len(q.Pairs) != 15 {
		t.Fatal(q, e)
	}
	s = testWarpedSplinePrism(t, false, true)
	q, e = s.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{
		Separation: geom.SurfaceSurfaceIsolationOptions{MaxPairs: 32},
	})
	if !errors.Is(e, geom.ErrConvergence) || q.Complete {
		t.Fatal(q, e)
	}
}
