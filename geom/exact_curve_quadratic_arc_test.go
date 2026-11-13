package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func exactQuarterCircle(t *testing.T) *ExactSplineCurve {
	return sharedProjectionCurve(t, []Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, 1, 2})
}
func TestExactQuadraticArcRationalContactAndEndpoints(t *testing.T) {
	ctx := context.Background()
	arc := exactQuarterCircle(t)
	curve := sharedProjectionCurve(t, []Vec3{{0, .75, 0}, {1, .75, 0}}, nil)
	// The irrational X=sqrt(7)/4 contact is provably inside the rational arc.
	r, e := IntersectExactCurveQuadraticArc(ctx, curve, arc, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].ArcParameter != nil || r.Hits[0].ArcLower.Sign() <= 0 || r.Hits[0].ArcUpper.Cmp(big.NewRat(1, 1)) >= 0 {
		t.Fatal(r, e)
	}
	rational := sharedProjectionCurve(t, []Vec3{{0, .5, 0}, {1, 1, 0}}, nil)
	rr, re := IntersectExactCurveQuadraticArc(ctx, rational, arc, ExactCurveSegmentOptions{})
	if re != nil || len(rr.Hits) != 1 || rr.Hits[0].Curve.Parameter.Cmp(big.NewRat(3, 5)) != 0 || rr.Hits[0].ArcParameter.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(rr, re)
	}
	tangent := sharedProjectionCurve(t, []Vec3{{1, -1, 0}, {1, 1, 0}}, nil)
	r, e = IntersectExactCurveQuadraticArc(ctx, tangent, arc, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].ArcParameter.Sign() != 0 || !r.Hits[0].Curve.Tangent {
		t.Fatal(r, e)
	}
	end := sharedProjectionCurve(t, []Vec3{{-1, 1, 0}, {1, 1, 0}}, nil)
	r, e = IntersectExactCurveQuadraticArc(ctx, end, arc, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].ArcParameter.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r, e)
	}
}
func TestExactQuadraticArcNonlinearMembership(t *testing.T) {
	ctx := context.Background()
	arc := exactQuarterCircle(t)
	curve := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {0, .5, 0}, {1, 1, 0}}, nil)
	r, e := IntersectExactCurveQuadraticArc(ctx, curve, arc, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].Curve.Parameter != nil || r.Hits[0].ArcLower.Cmp(r.Hits[0].ArcUpper) >= 0 {
		t.Fatal(r, e)
	}
	outside, e := curve.Transform(Scaling(Vec3{-1, -1, -1}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactCurveQuadraticArc(ctx, outside, arc, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	reversed, e := arc.Reparameterize(-2, 4, true, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactCurveQuadraticArc(ctx, curve, reversed, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].ArcLower.Cmp(big.NewRat(-2, 1)) <= 0 || r.Hits[0].ArcUpper.Cmp(big.NewRat(4, 1)) >= 0 {
		t.Fatal(r, e)
	}
	_, e = IntersectExactCurveQuadraticArc(ctx, curve, arc, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
func TestExactQuadraticArcCoincidenceAndDegeneracy(t *testing.T) {
	ctx := context.Background()
	arc := exactQuarterCircle(t)
	r, e := IntersectExactCurveQuadraticArc(ctx, arc, arc, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Overlaps) != 1 || len(r.Coincident) != 0 {
		t.Fatal(r, e)
	}
	straight := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {.5, 0, 0}, {1, 0, 0}}, nil)
	_, e = IntersectExactCurveQuadraticArc(ctx, arc, straight, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectExactCurveQuadraticArc(canceled, arc, arc, ExactCurveSegmentOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
