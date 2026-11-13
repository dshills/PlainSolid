package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func rationalGraphMinimum(t *testing.T, u, v *big.Rat, edge bool) (*SurfacePatch, *SurfacePatch) {
	a, c := graphContactFixture(t, false, false)
	term := func(i int, origin *big.Rat, linear bool) *big.Rat {
		x := big.NewRat(int64(i), 4)
		r := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(int64(i*(i-1)), 12), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(origin, x))), new(big.Rat).Mul(origin, origin))
		if linear {
			r.Add(r, new(big.Rat).Sub(x, origin))
		}
		return r
	}
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			k := 5*i + j
			a.net[k][2] = new(big.Rat).Add(c.net[k][2], new(big.Rat).Add(term(i, u, edge), term(j, v, false)))
		}
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	return a, c
}
func TestDiscoverGraphRationalQuadraticMinimum(t *testing.T) {
	ctx := context.Background()
	for _, edge := range []bool{false, true} {
		u, v := big.NewRat(1, 3), big.NewRat(2, 5)
		if edge {
			u = new(big.Rat)
		}
		a, c := rationalGraphMinimum(t, u, v, edge)
		r, e := DiscoverSplineGraphPointContact(ctx, a, c, GraphPointContactDiscoveryOptions{})
		if e != nil || !r.Contact.Proven || r.Contact.Parameters[0].Cmp(u) != 0 || r.Contact.Parameters[1].Cmp(v) != 0 || r.CandidatesTested < 2 {
			t.Fatal(r, e)
		}
	}
}
func TestDiscoverGraphQuarticKnotVertex(t *testing.T) {
	ctx := context.Background()
	a, c := graphContactFixture(t, true, false)
	var e error
	for _, p := range []**SurfacePatch{&a, &c} {
		*p, e = (*p).InsertUKnot(.5, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		*p, e = (*p).InsertVKnot(.5, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		*p, e = (*p).Trim(ParameterRectangle{0, .75, 0, 1})
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := DiscoverSplineGraphPointContact(ctx, a, c, GraphPointContactDiscoveryOptions{})
	if e != nil || !r.Contact.Proven || r.Contact.Parameters[0].Cmp(big.NewRat(1, 2)) != 0 || r.CandidatesTested < 2 {
		t.Fatal(r, e)
	}
	_, e = DiscoverSplineGraphPointContact(ctx, a, c, GraphPointContactDiscoveryOptions{MaxCandidates: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = DiscoverSplineGraphPointContact(canceled, a, c, GraphPointContactDiscoveryOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestDiscoverGraphDoesNotInventContact(t *testing.T) {
	a, c := graphContactFixture(t, true, false)
	a, e := a.Transformed(Translation(Vec3{Z: .125}))
	if e != nil {
		t.Fatal(e)
	}
	r, e := DiscoverSplineGraphPointContact(context.Background(), a, c, GraphPointContactDiscoveryOptions{})
	if !errors.Is(e, ErrConvergence) || r.Contact.Proven || r.ProofAttempts != 0 {
		t.Fatal(r, e)
	}
}
