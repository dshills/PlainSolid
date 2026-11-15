package geom

import (
	"context"
	"math/big"
	"testing"
)

func bivariateGraphFixture(t *testing.T, degree, axis int, a, b []*big.Rat) (*SurfacePatch, *SurfacePatch, []*big.Rat) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	if degree == 4 {
		f = []*big.Rat{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	}
	n := len(f) + max(len(a), len(b)) - 2
	product := func(x, y []*big.Rat) []*big.Rat {
		p := make([]*big.Rat, n+1)
		for i := range p {
			p[i] = new(big.Rat)
		}
		for i, u := range x {
			for j, v := range y {
				p[i+j].Add(p[i+j], new(big.Rat).Mul(u, v))
			}
		}
		return p
	}
	constant, linear := product(f, b), product(f, a)
	for i := range constant {
		constant[i].Neg(constant[i])
		linear[i].Neg(linear[i])
		if i < len(b) {
			linear[i].Add(linear[i], b[i])
		}
	}
	bern := func(p []*big.Rat) []*big.Rat {
		out := make([]*big.Rat, n+1)
		for i := range out {
			out[i] = new(big.Rat)
			for j := 0; j <= i && j < len(p); j++ {
				x := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(n), int64(j)))
				out[i].Add(out[i], new(big.Rat).Mul(p[j], x))
			}
		}
		return out
	}
	cc, bb, aa := bern(constant), bern(linear), bern(a)
	var out [2]*SurfacePatch
	for owner := 0; owner < 2; owner++ {
		var controls [][4]*big.Rat
		for i := 0; i <= n; i++ {
			for j := 0; j < 3; j++ {
				u, v := big.NewRat(int64(i), int64(n)), big.NewRat(int64(j), 2)
				z := new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Quo(new(big.Rat).Mul(u, v), big.NewRat(64, 1)))
				if owner == 0 {
					z.Add(z, cc[i])
					z.Add(z, new(big.Rat).Mul(v, bb[i]))
					if j == 2 {
						z.Add(z, aa[i])
					}
				}
				controls = append(controls, [4]*big.Rat{u, v, z, big.NewRat(1, 1)})
			}
		}
		du, dv := n, 2
		if axis == 1 {
			var transpose [][4]*big.Rat
			for j := 0; j < 3; j++ {
				for i := 0; i <= n; i++ {
					p := controls[3*i+j]
					p[0], p[1] = p[1], p[0]
					transpose = append(transpose, p)
				}
			}
			controls = transpose
			du, dv = 2, n
		}
		p, e := NewExactHomogeneousBezierSurface(context.Background(), du, dv, controls, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		k := .5
		if owner == 1 {
			k = .3125
		}
		if (owner == 0) != (axis == 1) {
			p, e = p.InsertUKnot(k, 1, SurfacePatchOptions{})
		} else {
			p, e = p.InsertVKnot(k, 1, SurfacePatchOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
		out[owner] = p
	}
	return out[0], out[1], f
}
func TestBivariatePolynomialOriginalCoverage(t *testing.T) {
	for _, degree := range []int{3, 4} {
		for _, axis := range []int{0, 1} {
			a, b, f := bivariateGraphFixture(t, degree, axis, []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}, []*big.Rat{big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 8)})
			for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphBivariatePolynomialContact(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.Degree != degree || len(r.Factors) != 4 {
					t.Fatal(degree, axis, order, r, e)
				}
				want := 1
				if order == 1 {
					want = -1
				}
				if r.PositiveSide != want {
					t.Fatal("sign")
				}
				for i, x := range f {
					if r.Power[i].Cmp(x) != 0 {
						t.Fatal("original polynomial root", r.Power)
					}
				}
				for _, factor := range r.Factors {
					if factor.Sign != want || factor.VisitedCells < 2 {
						t.Fatal(factor)
					}
				}
				if q, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || q.Proven {
					t.Fatal("strict free-only scope widened")
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphBivariatePolynomialContact(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) || (e != nil && (q.Proven || len(q.Factors) > 0)) {
						t.Fatal("atomic quota", e)
					}
				}
			}
		}
	}
}
func TestBivariatePolynomialRejectsCurvedExtraBranch(t *testing.T) {
	for _, axis := range []int{0, 1} {
		a, b, _ := bivariateGraphFixture(t, 3, axis, []*big.Rat{big.NewRat(1, 1)}, []*big.Rat{big.NewRat(-1, 2), new(big.Rat), big.NewRat(-1, 8)})
		r, e := DiscoverSplineGraphBivariatePolynomialContact(context.Background(), a, b, axis, GraphPointContactOptions{MaxDepth: 4})
		if e == nil || r.Proven || len(r.Factors) > 0 {
			t.Fatal("second curved branch", e)
		}
	}
}
func TestExactPolynomialDiscriminantSquare(t *testing.T) {
	b := &polynomialBudget{maxOperations: 100000, maxBits: 1024}
	p := rationalPolynomial{big.NewRat(1, 4), big.NewRat(-1, 1), big.NewRat(1, 1)}
	r, e := graphExactPolynomialSqrt(p, b)
	if e != nil || len(r) != 2 || r[0].Cmp(big.NewRat(-1, 2)) != 0 || r[1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r, e)
	}
	p[0] = big.NewRat(1, 3)
	if _, e = graphExactPolynomialSqrt(p, b); e == nil {
		t.Fatal("nonsquare accepted")
	}
}

func TestBivariatePolynomialLeadingCoefficientZeroInside(t *testing.T) {
	// The quadratic coefficient vanishes at u=1/2. Exact polynomial division
	// removes the apparent quadratic-formula pole; no pointwise division occurs.
	a, b, _ := bivariateGraphFixture(t, 3, 0, []*big.Rat{big.NewRat(-1, 2), big.NewRat(1, 1)}, []*big.Rat{big.NewRat(1, 1)})
	r, e := DiscoverSplineGraphBivariatePolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{})
	if e != nil || !r.Proven {
		t.Fatal(r, e)
	}
	if q, e := DiscoverSplineGraphBivariatePolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{MaxCells: 7}); e == nil || q.Proven || len(q.Factors) > 0 {
		t.Fatal("partial tensor sign proof", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := DiscoverSplineGraphBivariatePolynomialContact(ctx, a, b, 0, GraphPointContactOptions{}); e != context.Canceled || q.Proven {
		t.Fatal(e)
	}
}
