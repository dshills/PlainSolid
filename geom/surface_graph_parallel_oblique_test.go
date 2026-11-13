package geom

import (
	"context"
	"math/big"
	"testing"
)

func parallelObliqueFixture(t *testing.T, levels []*big.Rat) (*SurfacePatch, *SurfacePatch) {
	a, b := graphContactFixture(t, false, false)
	power := map[[2]int]*big.Rat{{0, 0}: big.NewRat(1, 1)}
	for _, level := range levels {
		next := map[[2]int]*big.Rat{}
		for ij, c := range power {
			for _, term := range []struct {
				ij [2]int
				c  *big.Rat
			}{{[2]int{0, 0}, new(big.Rat).Neg(level)}, {[2]int{1, 0}, big.NewRat(1, 1)}, {[2]int{0, 1}, big.NewRat(2, 1)}} {
				key := [2]int{ij[0] + term.ij[0], ij[1] + term.ij[1]}
				if next[key] == nil {
					next[key] = new(big.Rat)
				}
				next[key].Add(next[key], new(big.Rat).Mul(c, term.c))
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
func TestParallelObliqueGraphLines(t *testing.T) {
	a, b := parallelObliqueFixture(t, []*big.Rat{big.NewRat(2, 3), big.NewRat(7, 3)})
	for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := DiscoverSplineGraphParallelObliqueLines(context.Background(), pair[0], pair[1], GraphFactorAxisLineOptions{})
		if e != nil || !r.Proven {
			t.Fatal(r, e)
		}
		for i, terminals := range r.Terminals {
			level := []*big.Rat{big.NewRat(2, 3), big.NewRat(7, 3)}[i]
			for _, uv := range terminals {
				if new(big.Rat).Sub(new(big.Rat).Add(uv[0], new(big.Rat).Mul(big.NewRat(2, 1), uv[1])), level).Sign() != 0 {
					t.Fatal(uv)
				}
			}
		}
		for _, region := range r.Regions {
			uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
			for _, p := range region.Vertices {
				uv[0].Add(uv[0], p[0])
				uv[1].Add(uv[1], p[1])
			}
			for axis := 0; axis < 2; axis++ {
				uv[axis].Quo(uv[axis], big.NewRat(int64(len(region.Vertices)), 1))
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
				t.Fatal("independent band sign")
			}
		}
	}
}
func TestParallelObliqueGraphLinesRejectsIncomplete(t *testing.T) {
	for _, levels := range [][]*big.Rat{{big.NewRat(2, 3), big.NewRat(2, 3)}, {big.NewRat(2, 3), big.NewRat(4, 3), big.NewRat(7, 3)}} {
		a, b := parallelObliqueFixture(t, levels)
		r, e := DiscoverSplineGraphParallelObliqueLines(context.Background(), a, b, GraphFactorAxisLineOptions{})
		if e == nil || r.Proven {
			t.Fatal(r, e)
		}
	}
	a, b := parallelObliqueFixture(t, []*big.Rat{big.NewRat(2, 3), big.NewRat(7, 3)})
	r, e := DiscoverSplineGraphParallelObliqueLines(context.Background(), a, b, GraphFactorAxisLineOptions{MaxCandidates: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2].Add(a.net[i][2], tiny)
	}
	r, e = DiscoverSplineGraphParallelObliqueLines(context.Background(), a, b, GraphFactorAxisLineOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
