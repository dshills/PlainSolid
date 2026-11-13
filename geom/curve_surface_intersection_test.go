package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestExactCurveSurfaceIntersection(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	makeCurve := func(a, b, c *big.Rat) *ExactSplineCurve {
		t.Helper()
		curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{
			{big.NewRat(1, 4), big.NewRat(1, 3), a, big.NewRat(1, 1)},
			{big.NewRat(1, 2), big.NewRat(1, 3), b, big.NewRat(1, 1)},
			{big.NewRat(3, 4), big.NewRat(1, 3), c, big.NewRat(1, 1)},
		}, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return curve
	}
	// z(t)=t²-t+3/16 has two simple contacts at t=1/4 and 3/4.
	curve := makeCurve(big.NewRat(3, 16), big.NewRat(-5, 16), big.NewRat(3, 16))
	assertHits := func(r CurveSurfaceIntersection, e error, want ...float64) {
		t.Helper()
		if e != nil || !r.Complete || len(r.Hits) != len(want) {
			t.Fatal(r, e)
		}
		for i, x := range want {
			h := r.Hits[i]
			if h.CurveParameter.Min > x || h.CurveParameter.Max < x || h.UV.Min.X > .25+x/2 || h.UV.Max.X < .25+x/2 ||
				h.UV.Min.Y > 1.0/3 || h.UV.Max.Y < 1.0/3 {
				t.Fatal(h, x)
			}
		}
	}
	r, e := IntersectExactSplineCurveSurface(ctx, curve, p, CurveSurfaceIntersectionOptions{})
	assertHits(r, e, .25, .75)
	if r.Hits[0].Direction != -1 || r.Hits[1].Direction != 1 {
		t.Fatal("incorrect oriented crossings", r.Hits)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "0", "2"}, Lower: "2/3", Upper: "3/4"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	zero, e := cell.EndpointRoot(0)
	if e != nil {
		t.Fatal(e)
	}
	cut, e := cell.RestrictAlgebraic(ctx, zero, root, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = cut.Cell.IntersectSplineSurface(ctx, p, CurveSurfaceIntersectionOptions{})
	assertHits(r, e, .25)
	bound, e := cut.Cell.EncloseEndpoint(ctx, 1, ComposedCurveEndpointOptions{})
	if e != nil || math.Abs(bound.Point.X-(.25+math.Sqrt2/4)) > 1e-9 {
		t.Fatal(bound, e)
	}
	// The surface projection intersects a singular tangent root and must
	// retain the unresolved region rather than reporting a certified miss.
	tan := makeCurve(big.NewRat(1, 4), big.NewRat(-1, 4), big.NewRat(1, 4))
	r, e = IntersectExactSplineCurveSurface(ctx, tan, p, CurveSurfaceIntersectionOptions{MaxDepth: 10})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) == 0 || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	r, e = IntersectExactSplineCurveSurface(ctx, curve, p, CurveSurfaceIntersectionOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
}
