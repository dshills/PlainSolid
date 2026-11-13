package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestRepeatedGraphAxisContact(t *testing.T) {
	for _, m := range []int{1, 2, 3, 4} {
		p := rationalPolynomial{big.NewRat(1, 1)}
		budget := &polynomialBudget{maxOperations: 100000, maxBits: 1024}
		for i := 0; i < m; i++ {
			var e error
			p, e = polynomialProduct(p, rationalPolynomial{big.NewRat(-1, 3), big.NewRat(1, 1)}, budget)
			if e != nil {
				t.Fatal(e)
			}
		}
		if m <= 2 {
			p, _ = polynomialProduct(p, rationalPolynomial{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(1, 1)}, budget)
		}
		for axis := 0; axis < 2; axis++ {
			a, b := factoredGraphLineFixture(t, axis, p)
			for swap, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphAxisContact(context.Background(), pair[0], pair[1], axis, GraphFactorAxisLineOptions{})
				sign := 1
				if swap == 1 {
					sign = -1
				}
				lower := sign
				if m%2 == 1 {
					lower = -sign
				}
				if e != nil || !r.Line.Proven || r.Multiplicity != m || r.Line.Coordinate.Cmp(big.NewRat(1, 3)) != 0 || r.Line.LowerSide != lower || r.Line.UpperSide != sign {
					t.Fatal(m, axis, r, e)
				}
			}
		}
	}
}
func TestRepeatedGraphAxisContactRejectsAdditionalAndPerturbed(t *testing.T) {
	p := rationalPolynomial{big.NewRat(-2, 27), big.NewRat(5, 9), big.NewRat(-4, 3), big.NewRat(1, 1)} // (u-1/3)^2(u-2/3)
	a, b := factoredGraphLineFixture(t, 0, p)
	r, e := DiscoverSplineGraphAxisContact(context.Background(), a, b, 0, GraphFactorAxisLineOptions{})
	if e == nil || r.Line.Proven {
		t.Fatal(r, e)
	}
	a, b = factoredGraphLineFixture(t, 0, rationalPolynomial{big.NewRat(1, 9), big.NewRat(-2, 3), big.NewRat(1, 1)})
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2].Add(a.net[i][2], new(big.Rat).Mul(tiny, a.net[i][1]))
	}
	r, e = DiscoverSplineGraphAxisContact(context.Background(), a, b, 0, GraphFactorAxisLineOptions{CandidateParameters: []*big.Rat{big.NewRat(1, 3)}})
	if e == nil || r.Line.Proven {
		t.Fatal(r, e)
	}
}
