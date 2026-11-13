package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactQuadraticArcOverlapMaps(t *testing.T) {
	ctx := context.Background()
	arc := exactQuarterCircle(t)
	source, e := arc.Reparameterize(-2, 4, true, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e := source.patch.InsertUKnot(1, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	source = &ExactSplineCurve{patch: refined}
	r, e := IntersectExactCurveQuadraticArc(ctx, source, arc, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Overlaps) != 2 {
		t.Fatal(r, e)
	}
	for _, x := range r.Overlaps {
		if x.ArcScale.Cmp(big.NewRat(-1, 6)) != 0 || x.ArcOffset.Cmp(big.NewRat(2, 3)) != 0 {
			t.Fatal(x)
		}
	}
	// Exact nonbinary endpoint restriction without exporting rounded poles.
	net := []exactHomogeneous{arc.patch.net[0], arc.patch.net[2], arc.patch.net[4]}
	restricted, e := restrictHomogeneousBezier(net, new(big.Rat), big.NewRat(1, 3), &polynomialBudget{maxOperations: 100000, maxBits: 16384})
	if e != nil {
		t.Fatal(e)
	}
	p := *arc.patch
	p.net = make([]exactHomogeneous, 6)
	for i, h := range restricted {
		for j := 0; j < 2; j++ {
			for axis := 0; axis < 4; axis++ {
				p.net[2*i+j][axis] = new(big.Rat).Set(h[axis])
			}
		}
	}
	if e := p.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactCurveQuadraticArc(ctx, &ExactSplineCurve{patch: &p}, arc, ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Overlaps) != 1 || r.Overlaps[0].ArcEnd.Cmp(big.NewRat(1, 3)) != 0 || r.Overlaps[0].ArcScale.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
}
func TestExactQuadraticArcOverlapNonAffineRemainsPending(t *testing.T) {
	arc := exactQuarterCircle(t)
	// Circle parameter s=t² is geometrically coincident, but not affine speed.
	source := sharedProjectionCurve(t, []Vec3{{}, {}, {}, {}, {}}, nil)
	x := []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat)}
	y := []*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1), big.NewRat(2, 1)}
	for i := 0; i < 5; i++ {
		w := big.NewRat(1, 1)
		if i == 4 {
			w = big.NewRat(2, 1)
		}
		for j := 0; j < 2; j++ {
			source.patch.net[2*i+j] = exactHomogeneous{new(big.Rat).Set(x[i]), new(big.Rat).Set(y[i]), new(big.Rat), new(big.Rat).Set(w)}
		}
	}
	if e := source.patch.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	r, e := IntersectExactCurveQuadraticArc(context.Background(), source, arc, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Overlaps) != 0 || len(r.Coincident) != 1 {
		t.Fatal(r, e)
	}
	outside, e := arc.Transform(Scaling(Vec3{-1, -1, 1}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = IntersectExactCurveQuadraticArc(context.Background(), outside, arc, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}
