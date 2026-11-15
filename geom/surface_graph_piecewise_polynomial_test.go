package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestPiecewisePolynomialComposableModes(t *testing.T) {
	for _, mode := range []PolynomialGraphMode{FreeFactoredPolynomialGraph, BivariatePolynomialGraph} {
		var a, b *SurfacePatch
		if mode == FreeFactoredPolynomialGraph {
			a, b, _ = factoredGraphFixture(t, 3, 0, []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 1)}, true)
		} else {
			a, b, _ = bivariateGraphFixture(t, 4, 0, []*big.Rat{big.NewRat(1, 1)}, []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)})
		}
		o := PiecewisePolynomialOptions{Mode: mode}
		r, e := DiscoverPiecewiseSplineGraphPolynomial(context.Background(), a, b, o)
		if e != nil || !r.Proven || len(r.Pieces) != 2 || len(r.Joins) != 1 {
			t.Fatal(mode, r, e)
		}
		for _, p := range r.Pieces {
			if (mode == FreeFactoredPolynomialGraph) != (p.FreeFactor != nil) || (mode == BivariatePolynomialGraph) != (p.Bivariate != nil) {
				t.Fatal("wrong proof tag")
			}
			c := p.Controls()
			before := p.Controls()[0][0].RatString()
			c[0][0].SetInt64(91)
			if p.Controls()[0][0].RatString() != before {
				t.Fatal("control alias")
			}
		}
		for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
			o.Proof.MaxOperations = limit
			q, e := DiscoverPiecewiseSplineGraphPolynomial(context.Background(), a, b, o)
			if (e == nil) != (limit == r.ExactOperations) || (e != nil && (q.Proven || len(q.Joins) > 0 || len(q.Pieces) > 0)) {
				t.Fatal("cumulative quota", mode, e)
			}
		}
		if q, e := DiscoverPiecewiseSplineGraphPolynomial(context.Background(), a, b, PiecewisePolynomialOptions{}); e == nil || q.Proven {
			t.Fatal("unrequested fallback")
		}
	}
}
