package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestDiscoverSeparableMultispanRationalAndKnot(t *testing.T) {
	for _, root := range [][2]*big.Rat{{big.NewRat(1, 3), big.NewRat(2, 5)}, {big.NewRat(1, 2), big.NewRat(1, 2)}} {
		a, c := separableContactFixture(t, rootPower(t, root[0], 4), rootPower(t, root[1], 4))
		var e error
		a, e = a.InsertUKnot(.25, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a, e = a.InsertVKnot(.5, 2, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, e = c.InsertUKnot(.5, 2, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, e = c.InsertVKnot(.75, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := DiscoverSeparableSplineGraphPointContact(context.Background(), a, c, SeparableGraphDiscoveryOptions{})
		if e != nil || !r.Contact.Proven || r.Contact.Parameters[0].Cmp(root[0]) != 0 || r.Contact.Parameters[1].Cmp(root[1]) != 0 {
			t.Fatal(r, e)
		}
	}
}
func TestDiscoverSeparableMultispanRejectsJumpAndMixed(t *testing.T) {
	for _, jump := range []bool{true, false} {
		a, c := separableContactFixture(t, rootPower(t, big.NewRat(1, 2), 4), rootPower(t, big.NewRat(1, 2), 4))
		var e error
		if !jump {
			for i, h := range a.net {
				a.net[i][2].Add(a.net[i][2], new(big.Rat).Mul(h[0], h[1]))
			}
		}
		a, e = a.InsertUKnot(.5, 4, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if jump {
			for i := 4; i < a.countU; i++ {
				for j := 0; j < a.countV; j++ {
					k := i*a.countV + j
					a.net[k][2].Add(a.net[k][2], new(big.Rat).Mul(big.NewRat(int64(i-4), 8), a.net[k][3]))
				}
			}
		}
		if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
			t.Fatal(e)
		}
		r, e := DiscoverSeparableSplineGraphPointContact(context.Background(), a, c, SeparableGraphDiscoveryOptions{})
		if !errors.Is(e, ErrConvergence) || r.Contact.Proven || r.ProofAttempts != 0 {
			t.Fatal(jump, r, e)
		}
	}
}
