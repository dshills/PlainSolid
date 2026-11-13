package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func factoredGraphLineFixture(t *testing.T, axis int, power []*big.Rat) (*SurfacePatch, *SurfacePatch) {
	a, b := graphContactFixture(t, false, false)
	n := a.degreeU
	bern := func(i int) *big.Rat {
		v := new(big.Rat)
		for k, c := range power {
			if k > i {
				break
			}
			factor := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(n), int64(k)))
			v.Add(v, new(big.Rat).Mul(c, factor))
		}
		return v
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			primary, other := i, j
			if axis == 1 {
				primary, other = j, i
			}
			delta := new(big.Rat).Mul(bern(primary), new(big.Rat).Add(big.NewRat(1, 1), big.NewRat(int64(other), int64(n))))
			a.net[i*a.countV+j][2] = new(big.Rat).Add(b.net[i*a.countV+j][2], delta)
		}
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	return a, b
}
func TestDiscoverHigherDegreeGraphAxisFactor(t *testing.T) {
	power := []*big.Rat{big.NewRat(-1, 3), big.NewRat(1, 1), big.NewRat(-1, 3), big.NewRat(1, 1)}
	for axis := 0; axis < 2; axis++ {
		a, b := factoredGraphLineFixture(t, axis, power)
		if legacy, e := CertifySplineGraphAxisLine(context.Background(), a, b, axis, GraphPointContactOptions{}); e == nil || legacy.Proven {
			t.Fatal("legacy affine subset unexpectedly accepted cubic")
		}
		for swap, pair := range [2][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverSplineGraphFactorAxisLine(context.Background(), pair[0], pair[1], axis, GraphFactorAxisLineOptions{})
			sign := 1
			if swap == 1 {
				sign = -1
			}
			if e != nil || !r.Line.Proven || r.Line.Coordinate.Cmp(big.NewRat(1, 3)) != 0 || r.Line.LowerSide != -sign || r.Line.UpperSide != sign {
				t.Fatal(r, e)
			}
			for _, coordinate := range []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 3), big.NewRat(3, 4)} {
				uv := [2]*big.Rat{big.NewRat(2, 5), big.NewRat(2, 5)}
				uv[axis] = coordinate
				x, e := pair[0].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				y, e := pair[1].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != sign*coordinate.Cmp(big.NewRat(1, 3)) {
					t.Fatal("independent side")
				}
			}
		}
	}
}
func TestHigherDegreeGraphFactorRejectsRepeatedAndPerturbed(t *testing.T) {
	for _, power := range [][]*big.Rat{{big.NewRat(1, 9), big.NewRat(-2, 3), big.NewRat(1, 1)}, {big.NewRat(2, 9), big.NewRat(-1, 1), big.NewRat(1, 1)}} {
		a, b := factoredGraphLineFixture(t, 0, power)
		r, e := DiscoverSplineGraphFactorAxisLine(context.Background(), a, b, 0, GraphFactorAxisLineOptions{})
		if !errors.Is(e, ErrConvergence) || r.Line.Proven {
			t.Fatal(r, e)
		}
	}
	power := []*big.Rat{big.NewRat(-1, 3), big.NewRat(1, 1), big.NewRat(-1, 3), big.NewRat(1, 1)}
	a, b := factoredGraphLineFixture(t, 0, power)
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2] = new(big.Rat).Add(a.net[i][2], new(big.Rat).Mul(tiny, a.net[i][1]))
	}
	r, e := DiscoverSplineGraphFactorAxisLine(context.Background(), a, b, 0, GraphFactorAxisLineOptions{CandidateParameters: []*big.Rat{big.NewRat(1, 3)}})
	if !errors.Is(e, ErrConvergence) || r.Line.Proven {
		t.Fatal(r, e)
	}
	a, b = factoredGraphLineFixture(t, 0, power)
	r, e = DiscoverSplineGraphFactorAxisLine(context.Background(), a, b, 0, GraphFactorAxisLineOptions{GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}})
	if e == nil || r.Line.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = DiscoverSplineGraphFactorAxisLine(ctx, a, b, 0, GraphFactorAxisLineOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestGraphAxisFactorNonconstantRationalWeights(t *testing.T) {
	a, b := mixedBasisPartialCoincidence(t)
	for i, h := range a.net {
		base := new(big.Rat).Quo(new(big.Rat).Mul(h[0], h[1]), h[3])
		a.net[i][2] = new(big.Rat).Sub(new(big.Rat).Add(base, new(big.Rat).Mul(big.NewRat(3, 1), h[0])), h[3])
	}
	r, e := DiscoverSplineGraphFactorAxisLine(context.Background(), a, b, 0, GraphFactorAxisLineOptions{})
	if e != nil || !r.Line.Proven || r.Line.Coordinate.Cmp(big.NewRat(1, 3)) != 0 || r.Line.UpperSide != 1 {
		t.Fatal(r, e)
	}
}
