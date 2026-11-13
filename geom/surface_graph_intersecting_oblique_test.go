package geom

import (
	"context"
	"math/big"
	"testing"
)

func intersectingObliqueFixture(t *testing.T, extra bool) (*SurfacePatch, *SurfacePatch) {
	a, b := graphContactFixture(t, false, false)
	// (u+2v-4/3)(u-v+1/6)(1+u); crossing is (1/3,1/2).
	factors := [][3]*big.Rat{{big.NewRat(-4, 3), big.NewRat(1, 1), big.NewRat(2, 1)}, {big.NewRat(1, 6), big.NewRat(1, 1), big.NewRat(-1, 1)}}
	factors = append(factors, [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(0, 1)})
	if extra {
		factors = append(factors, [3]*big.Rat{big.NewRat(-1, 2), big.NewRat(1, 1), big.NewRat(0, 1)})
	}
	power := map[[2]int]*big.Rat{{0, 0}: big.NewRat(1, 1)}
	for _, factor := range factors {
		next := map[[2]int]*big.Rat{}
		for ij, c := range power {
			for k, shift := range [][2]int{{0, 0}, {1, 0}, {0, 1}} {
				key := [2]int{ij[0] + shift[0], ij[1] + shift[1]}
				if next[key] == nil {
					next[key] = new(big.Rat)
				}
				next[key].Add(next[key], new(big.Rat).Mul(c, factor[k]))
			}
		}
		power = next
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			z := new(big.Rat)
			for ij, c := range power {
				if ij[0] > i || ij[1] > j {
					continue
				}
				fu := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(int64(a.degreeU), int64(ij[0])))
				fv := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(int64(a.degreeV), int64(ij[1])))
				z.Add(z, new(big.Rat).Mul(c, new(big.Rat).Mul(fu, fv)))
			}
			a.net[i*a.countV+j][2].Add(b.net[i*a.countV+j][2], z)
		}
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	return a, b
}
func TestIntersectingObliqueGraphLines(t *testing.T) {
	a, b := intersectingObliqueFixture(t, false)
	for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := DiscoverSplineGraphIntersectingObliqueLines(context.Background(), pair[0], pair[1], GraphFactorAxisLineOptions{})
		if e != nil || !r.Proven || r.Crossing[0].Cmp(big.NewRat(1, 3)) != 0 || r.Crossing[1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal(r, e)
		}
		for i, region := range r.Regions {
			uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
			for _, p := range region.Vertices {
				uv[0].Add(uv[0], p[0])
				uv[1].Add(uv[1], p[1])
			}
			for axis := 0; axis < 2; axis++ {
				uv[axis].Quo(uv[axis], big.NewRat(int64(len(region.Vertices)), 1))
			}
			for k, c := range r.Coefficients {
				value := new(big.Rat).Add(c[0], new(big.Rat).Add(new(big.Rat).Mul(c[1], uv[0]), new(big.Rat).Mul(c[2], uv[1])))
				if value.Sign() != r.RegionDirections[i][k] {
					t.Fatal("factor side")
				}
			}
			x, e := pair[0].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			y, e := pair[1].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != region.Side {
				t.Fatal("height side")
			}
		}
	}
}
func TestIntersectingObliqueGraphLinesRejectsThirdAndPerturbed(t *testing.T) {
	a, b := intersectingObliqueFixture(t, true)
	r, e := DiscoverSplineGraphIntersectingObliqueLines(context.Background(), a, b, GraphFactorAxisLineOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	a, b = intersectingObliqueFixture(t, false)
	r, e = DiscoverSplineGraphIntersectingObliqueLines(context.Background(), a, b, GraphFactorAxisLineOptions{MaxCandidates: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2].Add(a.net[i][2], tiny)
	}
	r, e = DiscoverSplineGraphIntersectingObliqueLines(context.Background(), a, b, GraphFactorAxisLineOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
