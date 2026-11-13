package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSplineGraphContactDistinctKnotPartitions(t *testing.T) {
	ctx := context.Background()
	a, c := graphContactFixture(t, true, false)
	var e error
	a, e = a.InsertUKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e = a.InsertVKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, e = c.InsertUKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, e = c.InsertVKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	r, e := CertifySplineGraphPointContact(ctx, a, c, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || !r.NumeratorC1 || r.ContinuityInterfaces < 4 || r.Side != 1 {
		t.Fatal(r, e)
	}
	r, e = CertifySplineGraphPointContact(ctx, c, a, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Side != -1 {
		t.Fatal(r, e)
	}
}
func TestSplineGraphContactDerivativeJumpRejected(t *testing.T) {
	a, c := graphContactFixture(t, false, false)
	var e error
	a, e = a.InsertUKnot(.5, 4, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Add the exact piecewise linear max(u-.5,0) term to the right cell.
	// The position is continuous, but the first U derivative jumps by one.
	for i := 4; i < a.countU; i++ {
		shift := big.NewRat(int64(i-4), 8)
		for j := 0; j < a.countV; j++ {
			h := &a.net[i*a.countV+j]
			h[2] = new(big.Rat).Add(h[2], new(big.Rat).Mul(shift, h[3]))
		}
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	r, e := CertifySplineGraphPointContact(context.Background(), a, c, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven || r.NumeratorC1 {
		t.Fatal(r, e)
	}
}
