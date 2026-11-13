package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestProjectionKrawczykExactInteriorKnotRoot(t *testing.T) {
	p := nontriangularInversePatch(t)
	var e error
	p, e = p.InsertUKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := InvertSurfaceProjectionInterval(context.Background(), p, [2]*big.Rat{big.NewRat(9, 16), big.NewRat(9, 16)}, RationalSurfaceInverseOptions{})
	if e != nil || !r.Found || !r.Complete || r.KnotAwareTests == 0 {
		t.Fatal(r, e)
	}
	for axis := 0; axis < 2; axis++ {
		if r.Lower[axis].Cmp(big.NewRat(1, 2)) > 0 || r.Upper[axis].Cmp(big.NewRat(1, 2)) < 0 {
			t.Fatal(r)
		}
	}
}
func TestProjectionKrawczykContinuousDerivativeJump(t *testing.T) {
	p := nontriangularInversePatch(t)
	var e error
	p, e = p.InsertUKnot(.5, 2, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Exact C0 perturbation X += max(u-.5,0)/8. A single classical
	// Jacobian does not exist at the inverse, but all one-sided Jacobians
	// satisfy the verified contraction bound on the containing rectangle.
	for i := 2; i < p.countU; i++ {
		for j := 0; j < p.countV; j++ {
			k := i*p.countV + j
			p.net[k][0].Add(p.net[k][0], new(big.Rat).Mul(big.NewRat(int64(i-2), 32), p.net[k][3]))
		}
	}
	if e := p.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	r, e := InvertSurfaceProjectionInterval(context.Background(), p, [2]*big.Rat{big.NewRat(9, 16), big.NewRat(9, 16)}, RationalSurfaceInverseOptions{})
	if e != nil || !r.Found || !r.Complete || r.KnotAwareTests == 0 || !r.Injectivity.ProvenInjective {
		t.Fatal(r, e)
	}
	for axis := 0; axis < 2; axis++ {
		if r.Lower[axis].Cmp(big.NewRat(1, 2)) > 0 || r.Upper[axis].Cmp(big.NewRat(1, 2)) < 0 {
			t.Fatal(r)
		}
	}
}
