package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestIndependentAffineCurveProjectionReversal(t *testing.T) {
	ctx := context.Background()
	a := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {.5, 0, 0}, {1, 1, 0}}, nil)
	b := sharedProjectionCurve(t, []Vec3{{0, .25, 0}, {1, .25, 0}}, nil)
	a, e := a.Reparameterize(10, 12, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Reparameterize(-2, 4, true, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectExactCurvesAffineProjection(ctx, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	h := r.Hits[0]
	if h.Parameters[0].Cmp(big.NewRat(11, 1)) != 0 || h.Parameters[1].Cmp(big.NewRat(1, 1)) != 0 || r.Maps[0].Scale.Cmp(big.NewRat(1, 2)) != 0 || r.Maps[1].Scale.Cmp(big.NewRat(-1, 6)) != 0 {
		t.Fatal(r)
	}
	// Reversing the operands preserves the independent original parameters.
	r, e = IntersectExactCurvesAffineProjection(ctx, b, a, Vec3{X: -2}, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].Parameters[0].Cmp(big.NewRat(1, 1)) != 0 || r.Hits[0].Parameters[1].Cmp(big.NewRat(11, 1)) != 0 {
		t.Fatal(r, e)
	}
	_, e = IntersectExactCurvesAffineProjection(ctx, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectExactCurvesAffineProjection(canceled, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestIndependentAffineCurveProjectionAlgebraicAndOverlap(t *testing.T) {
	ctx := context.Background()
	a := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {.5, 0, 0}, {1, 1, 0}}, nil)
	b := sharedProjectionCurve(t, []Vec3{{0, .5, 0}, {1, .5, 0}}, nil)
	b, e := b.Reparameterize(2, 6, true, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectExactCurvesAffineProjection(ctx, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].Parameters[0] != nil || r.Hits[0].Parameters[1] != nil {
		t.Fatal(r, e)
	}
	h := r.Hits[0]
	if h.Lower[1].Cmp(h.Upper[1]) >= 0 || new(big.Rat).Sub(big.NewRat(6, 1), new(big.Rat).Mul(big.NewRat(4, 1), h.Upper[0])).Cmp(h.Lower[1]) != 0 {
		t.Fatal(h)
	}
	sub, e := a.Restrict(.25, .75, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	sub, e = sub.Reparameterize(-4, -2, true, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactCurvesAffineProjection(ctx, a, sub, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Coincident) != 1 || r.Coincident[0].Lower[0].Cmp(big.NewRat(1, 4)) != 0 || r.Coincident[0].Lower[1].Cmp(big.NewRat(-4, 1)) != 0 {
		t.Fatal(r, e)
	}
}
func TestIndependentAffineCurveProjectionNonbinaryKnot(t *testing.T) {
	a := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {1, 1, 0}}, nil)
	// An interior projected knot can be unrepresentable even with exact endpoints.
	domain, e := a.Reparameterize(0, 3, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e := domain.patch.InsertUKnot(1, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := IntersectExactCurvesAffineProjection(context.Background(), &ExactSplineCurve{patch: refined}, a, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Coincident) != 2 || r.Coincident[0].ExactProjectionSpan[1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	// Exact X=t/3 cannot export its endpoint as a float knot.
	p := a.patch
	for i := range p.net {
		p.net[i][0].Quo(p.net[i][0], big.NewRat(3, 1))
	}
	if e := p.computeBounds(&polynomialBudget{maxOperations: 1000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactCurvesAffineProjection(context.Background(), a, a, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Coincident) != 1 || r.Coincident[0].ExactProjectionSpan[1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
}

func TestRationalProjectedKnotNonlinearReversal(t *testing.T) {
	ctx := context.Background()
	a := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {.5, 0, 0}, {1, 1, 0}}, nil)
	a, e := a.Reparameterize(0, 3, true, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := a.patch.InsertUKnot(1, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a = &ExactSplineCurve{patch: p}
	b := sharedProjectionCurve(t, []Vec3{{0, .25, 0}, {1, .25, 0}}, nil)
	r, e := IntersectExactCurvesAffineProjection(ctx, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].Parameters[0].Cmp(big.NewRat(3, 2)) != 0 || r.Hits[0].Parameters[1].Cmp(big.NewRat(1, 2)) != 0 || r.Hits[0].ExactProjectionSpan[1].Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal(r, e)
	}
}
