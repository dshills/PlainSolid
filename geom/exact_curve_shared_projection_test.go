package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func sharedProjectionCurve(t *testing.T, poles []Vec3, weights []float64) *ExactSplineCurve {
	t.Helper()
	c, e := NewBezier(poles, weights)
	if e != nil {
		t.Fatal(e)
	}
	x, e := NewExactSplineCurve(c, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if weights != nil {
		// Author the exact rational net directly: float BSpline normalization by a
		// nonbinary maximum would otherwise alter these deliberately exact identities.
		for i, point := range poles {
			w := new(big.Rat).SetFloat64(weights[i])
			for j := 0; j < 2; j++ {
				x.patch.net[2*i+j] = exactHomogeneous{new(big.Rat).Mul(new(big.Rat).SetFloat64(point.X), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(point.Y), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(point.Z), w), new(big.Rat).Set(w)}
			}
		}
		if e := x.patch.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
			t.Fatal(e)
		}
	}
	return x
}
func TestSharedAffineCurveProjectionAlgebraic(t *testing.T) {
	ctx := context.Background()
	a := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {.5, 0, 0}, {1, 1, 0}}, nil)
	b := sharedProjectionCurve(t, []Vec3{{0, 1, 0}, {.5, 1, 0}, {1, 0, 0}}, nil)
	r, e := IntersectExactCurvesSharedAffineCoordinate(ctx, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	h := r.Hits[0]
	if h.Parameter != nil || !h.Transverse || h.Multiplicity != 1 || new(big.Rat).Mul(h.Lower, h.Lower).Cmp(big.NewRat(1, 2)) >= 0 || new(big.Rat).Mul(h.Upper, h.Upper).Cmp(big.NewRat(1, 2)) <= 0 {
		t.Fatal(h)
	}
	// A different second spatial residual eliminates the apparent XY crossing.
	lifted, e := b.Transform(Translation(Vec3{Z: math.Ldexp(1, -80)}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactCurvesSharedAffineCoordinate(ctx, a, lifted, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	_, e = IntersectExactCurvesSharedAffineCoordinate(ctx, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectExactCurvesSharedAffineCoordinate(canceled, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestSharedAffineCurveProjectionRationalAndKnots(t *testing.T) {
	ctx := context.Background()
	a := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {.25, 0, 0}, {1, 1, 0}}, []float64{1, 2, 3})
	b := sharedProjectionCurve(t, []Vec3{{0, .375, 0}, {.75, .375, 0}, {1, .375, 0}}, []float64{3, 2, 1})
	// Exact refinement gives different knot partitions without rounded poles.
	ap, e := a.patch.InsertUKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a = &ExactSplineCurve{patch: ap}
	bp, e := b.patch.InsertUKnot(.75, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b = &ExactSplineCurve{patch: bp}
	r, e := IntersectExactCurvesSharedAffineCoordinate(ctx, a, b, Vec3{X: 3}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].Parameter.Cmp(big.NewRat(1, 2)) != 0 || r.Hits[0].Point[1].Cmp(big.NewRat(3, 8)) != 0 {
		t.Fatal(r, e)
	}
	r, e = IntersectExactCurvesSharedAffineCoordinate(ctx, a, a, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Coincident) != 2 {
		t.Fatal(r, e)
	}
}
func TestSharedAffineCurveTangencyAndUnsupported(t *testing.T) {
	ctx := context.Background()
	a := sharedProjectionCurve(t, []Vec3{{0, .25, 0}, {.5, -.25, 0}, {1, .25, 0}}, nil)
	b := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {1, 0, 0}}, nil)
	r, e := IntersectExactCurvesSharedAffineCoordinate(ctx, a, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].Tangent || r.Hits[0].Multiplicity != 2 {
		t.Fatal(r, e)
	}
	nonlinear := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {0, 0, 0}, {1, 0, 0}}, nil)
	_, e = IntersectExactCurvesSharedAffineCoordinate(ctx, a, nonlinear, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	endpoint := sharedProjectionCurve(t, []Vec3{{0, 0, 0}, {.5, 0, 0}, {1, 1, 0}}, nil)
	r, e = IntersectExactCurvesSharedAffineCoordinate(ctx, endpoint, b, Vec3{X: 1}, ExactCurveSegmentOptions{})
	if e != nil || len(r.Hits) != 1 || !r.Hits[0].SharedEndpoint {
		t.Fatal(r, e)
	}
}
