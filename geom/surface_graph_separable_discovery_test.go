package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestDiscoverSeparableQuarticContact(t *testing.T) {
	u, v := big.NewRat(1, 3), big.NewRat(2, 5)
	a, c := separableContactFixture(t, rootPower(t, u, 4), rootPower(t, v, 4))
	r, e := DiscoverSeparableSplineGraphPointContact(context.Background(), a, c, SeparableGraphDiscoveryOptions{})
	if e != nil || !r.Contact.Proven || r.Contact.Parameters[0].Cmp(u) != 0 || r.Contact.Parameters[1].Cmp(v) != 0 || r.Contact.Orders != ([2]int{4, 4}) {
		t.Fatal(r, e)
	}
	a, c = separableContactFixture(t, rootPower(t, new(big.Rat), 3), rootPower(t, big.NewRat(1, 1), 4))
	r, e = DiscoverSeparableSplineGraphPointContact(context.Background(), a, c, SeparableGraphDiscoveryOptions{})
	if e != nil || !r.Contact.Proven {
		t.Fatal(r, e)
	}
}
func TestDiscoverSeparableUnrecoveredAlgebraicContact(t *testing.T) {
	// (u²-1/2)²+(v-1/2)^4 has a unique irrational interior zero.
	up := rationalPolynomial{big.NewRat(1, 4), new(big.Rat), big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}
	a, c := separableContactFixture(t, up, rootPower(t, big.NewRat(1, 2), 4))
	r, e := DiscoverSeparableSplineGraphPointContact(context.Background(), a, c, SeparableGraphDiscoveryOptions{})
	if !errors.Is(e, ErrConvergence) || r.Contact.Proven || len(r.Unrecovered[0]) != 1 {
		t.Fatal(r, e)
	}
	lo, hi := r.Unrecovered[0][0].Lower, r.Unrecovered[0][0].Upper
	if new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(1, 2)) >= 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(1, 2)) <= 0 {
		t.Fatal(r)
	}
}
func TestDiscoverSeparableRejectsExtraRootAndLimits(t *testing.T) {
	up, e := polynomialProduct(rootPower(t, big.NewRat(1, 4), 2), rootPower(t, big.NewRat(3, 4), 2), &polynomialBudget{maxOperations: 100000, maxBits: 16384})
	if e != nil {
		t.Fatal(e)
	}
	a, c := separableContactFixture(t, up, rootPower(t, big.NewRat(1, 2), 4))
	r, e := DiscoverSeparableSplineGraphPointContact(context.Background(), a, c, SeparableGraphDiscoveryOptions{})
	if !errors.Is(e, ErrConvergence) || r.Contact.Proven {
		t.Fatal(r, e)
	}
	_, e = DiscoverSeparableSplineGraphPointContact(context.Background(), a, c, SeparableGraphDiscoveryOptions{MaxCandidates: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = DiscoverSeparableSplineGraphPointContact(canceled, a, c, SeparableGraphDiscoveryOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
