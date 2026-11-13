package geom

import (
	"context"
	"math/big"
	"testing"
)

func crossingGraphFixture(t *testing.T) (*SurfacePatch, *SurfacePatch) {
	a, b := graphContactFixture(t, false, false)
	bern := func(power []*big.Rat, i, n int) *big.Rat {
		r := new(big.Rat)
		for k, c := range power {
			if k > i {
				break
			}
			f := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(n), int64(k)))
			r.Add(r, new(big.Rat).Mul(c, f))
		}
		return r
	}
	// (u-1/3)(1+u^2)(v-2/5)(1+v).
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			x := bern([]*big.Rat{big.NewRat(-1, 3), big.NewRat(1, 1), big.NewRat(-1, 3), big.NewRat(1, 1)}, i, a.degreeU)
			y := bern([]*big.Rat{big.NewRat(-2, 5), big.NewRat(3, 5), big.NewRat(1, 1)}, j, a.degreeV)
			a.net[i*a.countV+j][2].Add(b.net[i*a.countV+j][2], new(big.Rat).Mul(x, y))
		}
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	return a, b
}
func TestHigherDegreeGraphCrossingLines(t *testing.T) {
	a, b := crossingGraphFixture(t)
	for swap, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := DiscoverSplineGraphCrossingLines(context.Background(), pair[0], pair[1], GraphCrossingLinesOptions{})
		if e != nil || !r.Proven || r.Crossing[0].Cmp(big.NewRat(1, 3)) != 0 || r.Crossing[1].Cmp(big.NewRat(2, 5)) != 0 {
			t.Fatal(r, e)
		}
		for _, sector := range r.Sectors {
			sign := sector.Direction[0] * sector.Direction[1]
			if swap == 1 {
				sign = -sign
			}
			if sector.Side != sign {
				t.Fatal(sector)
			}
			uv := [2]*big.Rat{new(big.Rat).Add(r.Crossing[0], big.NewRat(int64(sector.Direction[0]), 10)), new(big.Rat).Add(r.Crossing[1], big.NewRat(int64(sector.Direction[1]), 10))}
			x, e := pair[0].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			y, e := pair[1].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != sign {
				t.Fatal("independent sector")
			}
		}
	}
}
func TestGraphCrossingLinesRejectsPerturbationsAndLimits(t *testing.T) {
	a, b := crossingGraphFixture(t)
	r, e := DiscoverSplineGraphCrossingLines(context.Background(), a, b, GraphCrossingLinesOptions{GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2].Add(a.net[i][2], tiny)
	}
	r, e = DiscoverSplineGraphCrossingLines(context.Background(), a, b, GraphCrossingLinesOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = DiscoverSplineGraphCrossingLines(ctx, a, b, GraphCrossingLinesOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}

func TestGraphCrossingLinesRejectsThirdContact(t *testing.T) {
	a, b := graphContactFixture(t, false, false)
	// Extra full contact u=2/3 must prevent a two-line completeness claim.
	up := []*big.Rat{big.NewRat(2, 9), big.NewRat(-1, 1), big.NewRat(1, 1)}
	for i := 0; i < a.countU; i++ {
		u := new(big.Rat)
		for k, c := range up {
			if k > i {
				break
			}
			f := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(a.degreeU), int64(k)))
			u.Add(u, new(big.Rat).Mul(c, f))
		}
		for j := 0; j < a.countV; j++ {
			v := new(big.Rat).Sub(big.NewRat(int64(j), int64(a.degreeV)), big.NewRat(2, 5))
			a.net[i*a.countV+j][2].Add(b.net[i*a.countV+j][2], new(big.Rat).Mul(u, v))
		}
	}
	r, e := DiscoverSplineGraphCrossingLines(context.Background(), a, b, GraphCrossingLinesOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
