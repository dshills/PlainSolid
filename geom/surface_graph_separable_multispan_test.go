package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSeparableGraphDistinctPartitionsAtKnot(t *testing.T) {
	a, c := separableContactFixture(t, rootPower(t, big.NewRat(1, 2), 4), rootPower(t, big.NewRat(1, 2), 4))
	var e error
	a, e = a.InsertUKnot(.5, 4, SurfacePatchOptions{})
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
	// Make the U quartic genuinely piecewise, C3 at .5, by doubling only its
	// right half. Its local right-half quartic has only the last Bernstein pole.
	for j := 0; j < a.countV; j++ {
		k := (a.countU-1)*a.countV + j
		a.net[k][2].Add(a.net[k][2], new(big.Rat).Mul(big.NewRat(1, 16), a.net[k][3]))
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	r, e := CertifySeparableSplineGraphPointContact(context.Background(), a, c, uv, SeparableGraphContactOptions{})
	if e != nil || !r.Proven || r.Side != 1 || r.Orders != ([2]int{4, 4}) {
		t.Fatal(r, e)
	}
	r, e = CertifySeparableSplineGraphPointContact(context.Background(), c, a, uv, SeparableGraphContactOptions{})
	if e != nil || !r.Proven || r.Side != -1 {
		t.Fatal(r, e)
	}
}
func TestSeparableGraphMultispanRejectsJump(t *testing.T) {
	a, c := separableContactFixture(t, rootPower(t, big.NewRat(1, 2), 4), rootPower(t, big.NewRat(1, 2), 4))
	var e error
	a, e = a.InsertUKnot(.5, 4, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := 4; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			a.net[k][2].Add(a.net[k][2], new(big.Rat).Mul(big.NewRat(int64(i-4), 8), a.net[k][3]))
		}
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	r, e := CertifySeparableSplineGraphPointContact(context.Background(), a, c, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, SeparableGraphContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
}
