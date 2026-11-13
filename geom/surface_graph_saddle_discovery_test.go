package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestDiscoverOffCenterExactQuadraticSaddle(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	center := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	n := a.degreeU
	q := func(i int, alpha *big.Rat) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).Mul(alpha, alpha), new(big.Rat).Sub(big.NewRat(int64(i*(i-1)), int64(n*(n-1))), new(big.Rat).Mul(alpha, big.NewRat(int64(2*i), int64(n)))))
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			a.net[k][2] = new(big.Rat).Add(b.net[k][2], new(big.Rat).Sub(q(i, center[0]), q(j, center[1])))
		}
	}
	r, e := DiscoverQuadraticGraphSaddle(context.Background(), a, b, GraphPointContactDiscoveryOptions{})
	if e != nil || !r.Certificate.Proven || r.CandidatesTested < 1 {
		t.Fatal(r, e)
	}
	for axis := 0; axis < 2; axis++ {
		if r.Certificate.Critical.Parameters[axis].Cmp(center[axis]) != 0 {
			t.Fatal(r.Certificate.Critical.Parameters)
		}
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2] = new(big.Rat).Add(a.net[i][2], tiny)
	}
	r, e = DiscoverQuadraticGraphSaddle(context.Background(), a, b, GraphPointContactDiscoveryOptions{})
	if !errors.Is(e, ErrConvergence) || r.Certificate.Proven {
		t.Fatal("nonzero critical level", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = DiscoverQuadraticGraphSaddle(ctx, a, b, GraphPointContactDiscoveryOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
