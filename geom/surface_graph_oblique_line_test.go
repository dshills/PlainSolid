package geom

import (
	"context"
	"math/big"
	"testing"
)

func obliqueGraphFixture(t *testing.T, constant *big.Rat) (*SurfacePatch, *SurfacePatch) {
	a, b := graphContactFixture(t, false, false)
	coeff := map[[2]int]*big.Rat{}
	for _, term := range []struct {
		ij [2]int
		c  *big.Rat
	}{{[2]int{0, 0}, constant}, {[2]int{1, 0}, big.NewRat(1, 1)}, {[2]int{0, 1}, big.NewRat(2, 1)}} {
		for _, shift := range [][2]int{{0, 0}, {2, 0}, {0, 2}} {
			key := [2]int{term.ij[0] + shift[0], term.ij[1] + shift[1]}
			if coeff[key] == nil {
				coeff[key] = new(big.Rat)
			}
			coeff[key].Add(coeff[key], term.c)
		}
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			z := new(big.Rat)
			for ij, c := range coeff {
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
	return a, b
}
func TestObliqueGraphLineHigherDegree(t *testing.T) {
	for _, constant := range []*big.Rat{big.NewRat(-2, 3), big.NewRat(-1, 1)} {
		a, b := obliqueGraphFixture(t, constant)
		for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverSplineGraphObliqueLine(context.Background(), pair[0], pair[1], GraphFactorAxisLineOptions{})
			if e != nil || !r.Proven {
				t.Fatal(r, e)
			}
			for _, uv := range r.Terminals {
				if new(big.Rat).Add(constant, new(big.Rat).Add(uv[0], new(big.Rat).Mul(big.NewRat(2, 1), uv[1]))).Sign() != 0 {
					t.Fatal("wrong line")
				}
			}
			for _, region := range r.Regions {
				uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
				for _, vertex := range region.Vertices {
					uv[0].Add(uv[0], vertex[0])
					uv[1].Add(uv[1], vertex[1])
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
					t.Fatal("region sign")
				}
			}
		}
	}
}
func TestObliqueGraphLineRejectsPerturbationAndLimits(t *testing.T) {
	a, b := obliqueGraphFixture(t, big.NewRat(-2, 3))
	r, e := DiscoverSplineGraphObliqueLine(context.Background(), a, b, GraphFactorAxisLineOptions{MaxCandidates: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2].Add(a.net[i][2], tiny)
	}
	r, e = DiscoverSplineGraphObliqueLine(context.Background(), a, b, GraphFactorAxisLineOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}

func TestObliqueGraphLineMixedPartitions(t *testing.T) {
	a, b := obliqueGraphFixture(t, big.NewRat(-2, 3))
	var e error
	a, e = a.InsertUKnot(.5, a.degreeU, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := DiscoverSplineGraphObliqueLine(context.Background(), pair[0], pair[1], GraphFactorAxisLineOptions{})
		if e != nil || !r.Proven {
			t.Fatal(r, e)
		}
		for _, uv := range r.Terminals {
			if new(big.Rat).Sub(new(big.Rat).Add(uv[0], new(big.Rat).Mul(big.NewRat(2, 1), uv[1])), big.NewRat(2, 3)).Sign() != 0 {
				t.Fatal(uv)
			}
		}
	}
	// One interior control of the second U cell; all external boundary roots are
	// unchanged, so only the full cell-by-cell division can reject this proposal.
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	index := (a.degreeU+1)*a.countV + 1
	a.net[index][2].Add(a.net[index][2], tiny)
	r, e := DiscoverSplineGraphObliqueLine(context.Background(), a, b, GraphFactorAxisLineOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}

func TestObliqueGraphLinePiecewiseQuotient(t *testing.T) {
	a, b := graphContactFixture(t, false, false)
	// Reset A to the base B before adding a genuinely piecewise quotient.
	for i := range a.net {
		a.net[i][2].Set(b.net[i][2])
	}
	var e error
	a, e = a.InsertUKnot(.5, a.degreeU, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	n := a.degreeU
	for cell := 0; cell < 2; cell++ {
		origin := big.NewRat(int64(cell), 2)
		s0 := big.NewRat(1, 1)
		s1 := big.NewRat(int64(cell), 2)
		for i := 0; i <= n; i++ {
			t1 := big.NewRat(int64(i), int64(n))
			t2 := big.NewRat(int64(i*(i-1)), int64(n*(n-1)))
			c := new(big.Rat).Sub(origin, big.NewRat(2, 3))
			for j := 0; j < a.countV; j++ {
				delta := new(big.Rat).Mul(c, s0)
				delta.Add(delta, new(big.Rat).Mul(new(big.Rat).Add(new(big.Rat).Mul(c, s1), big.NewRat(1, 2)), t1))
				delta.Add(delta, new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(1, 2), s1), t2))
				delta.Add(delta, new(big.Rat).Mul(big.NewRat(int64(2*j), int64(a.degreeV)), new(big.Rat).Add(s0, new(big.Rat).Mul(s1, t1))))
				index := (cell*n+i)*a.countV + j
				// The shared knot row was already written by the first cell.
				if cell == 1 && i == 0 {
					continue
				}
				a.net[index][2].Add(a.net[index][2], delta)
			}
		}
	}
	r, e := DiscoverSplineGraphObliqueLine(context.Background(), a, b, GraphFactorAxisLineOptions{})
	if e != nil || !r.Proven {
		t.Fatal(r, e)
	}
}
