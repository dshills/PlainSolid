package geom

import (
	"context"
	"math/big"
	"testing"
)

func factoredGraphFixture(t *testing.T, degree, axis int, g []*big.Rat, refine bool) (*SurfacePatch, *SurfacePatch, []*big.Rat) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	if degree == 4 {
		f = []*big.Rat{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	}
	n := len(f) + len(g) - 2
	product := make([]*big.Rat, n+1)
	for i := range product {
		product[i] = new(big.Rat)
	}
	for i, x := range f {
		for j, y := range g {
			product[i+j].Add(product[i+j], new(big.Rat).Mul(x, y))
		}
	}
	bern := func(p []*big.Rat) []*big.Rat {
		out := make([]*big.Rat, n+1)
		for i := range out {
			out[i] = new(big.Rat)
			for j := 0; j <= i && j < len(p); j++ {
				c := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(n), int64(j)))
				out[i].Add(out[i], new(big.Rat).Mul(p[j], c))
			}
		}
		return out
	}
	fg, gb := bern(product), bern(g)
	var out [2]*SurfacePatch
	for owner := 0; owner < 2; owner++ {
		h := make([][4]*big.Rat, 0, 2*(n+1))
		for i := 0; i <= n; i++ {
			for j := 0; j < 2; j++ {
				u, v := big.NewRat(int64(i), int64(n)), big.NewRat(int64(j), 1)
				z := new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Quo(new(big.Rat).Mul(u, v), big.NewRat(64, 1)))
				if owner == 0 {
					z.Add(z, new(big.Rat).Sub(new(big.Rat).Mul(v, gb[i]), fg[i]))
				}
				h = append(h, [4]*big.Rat{u, v, z, big.NewRat(1, 1)})
			}
		}
		du, dv := n, 1
		if axis == 1 {
			transposed := make([][4]*big.Rat, 0, len(h))
			for j := 0; j < 2; j++ {
				for i := 0; i <= n; i++ {
					p := h[2*i+j]
					p[0], p[1] = p[1], p[0]
					transposed = append(transposed, p)
				}
			}
			h = transposed
			du, dv = 1, n
		}
		p, e := NewExactHomogeneousBezierSurface(context.Background(), du, dv, h, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if refine {
			if (owner == 0) != (axis == 1) {
				p, e = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
			} else {
				p, e = p.InsertVKnot(.3125, 1, SurfacePatchOptions{})
			}
			if e != nil {
				t.Fatal(e)
			}
		}
		out[owner] = p
	}
	return out[0], out[1], f
}
func TestFactoredPolynomialOriginalCoverage(t *testing.T) {
	for _, degree := range []int{3, 4} {
		for _, axis := range []int{0, 1} {
			a, b, f := factoredGraphFixture(t, degree, axis, []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 1)}, true)
			for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Proven || r.Degree != degree || len(r.CertifiedCells) != 4 || len(r.Factors) != 4 {
					t.Fatal(degree, axis, r, e)
				}
				want := 1
				if order == 1 {
					want = -1
				}
				if r.PositiveSide != want {
					t.Fatal("occupancy")
				}
				for i, x := range f {
					if r.Power[i].Cmp(x) != 0 {
						t.Fatal("recovered polynomial", r.Power)
					}
				}
				for _, factor := range r.Factors {
					if factor.Sign != want || len(factor.Power) != 2 {
						t.Fatal("factor proof", factor)
					}
				}
				if q, e := DiscoverSplineGraphPolynomialContact(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || q.Proven {
					t.Fatal("strict constant mode widened")
				}
				for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
					q, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), pair[0], pair[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == r.ExactOperations) || (e != nil && (q.Proven || len(q.Factors) > 0)) {
						t.Fatal("atomic exact quota", e)
					}
				}
			}
		}
	}
}
func TestFactoredPolynomialSignSubdivision(t *testing.T) {
	a, b, _ := factoredGraphFixture(t, 3, 0, []*big.Rat{big.NewRat(3, 8), big.NewRat(-1, 1), big.NewRat(1, 1)}, false)
	if r, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("mixed hull admitted without proof")
	}
	r, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{MaxDepth: 1})
	if e != nil || !r.Proven || r.Factors[0].VisitedCells != 3 {
		t.Fatal(r, e)
	}
	if q, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{MaxDepth: 1, MaxCells: 2}); e == nil || q.Proven || len(q.Factors) > 0 {
		t.Fatal("partial sign cover", e)
	}
}
func TestFactoredPolynomialRejectsExtraBranches(t *testing.T) {
	for _, g := range [][]*big.Rat{{big.NewRat(-1, 2), big.NewRat(1, 1)}, {big.NewRat(1, 4), big.NewRat(-1, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(1, 1)}} {
		a, b, _ := factoredGraphFixture(t, 3, 0, g, false)
		r, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{MaxDepth: 4})
		if e == nil || r.Proven || len(r.Controls) > 0 || len(r.Factors) > 0 {
			t.Fatal("zero/constant factor accepted", g, e)
		}
	}
	a, b, _ := factoredGraphFixture(t, 3, 0, []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 1)}, true)
	// A true later mixed dependent term destroys the factor, not just its samples.
	s, e := a.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	d := s.Data()
	d.Poles[len(d.Poles)-1].Z += 1.0 / 1024
	changed, e := NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(changed, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), p, b, 0, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("later cell omitted")
	}
}

func TestFactoredPolynomialCancellationAndDegreeBounds(t *testing.T) {
	a, b, _ := factoredGraphFixture(t, 3, 0, []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 1)}, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := DiscoverSplineGraphFactoredPolynomialContact(ctx, a, b, 0, GraphPointContactOptions{}); e != context.Canceled || r.Proven || len(r.Factors) > 0 {
		t.Fatal(e)
	}
	if r, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{MaxDepth: 33}); e != ErrInvalid || r.Proven {
		t.Fatal(e)
	}
	g := []*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
	a, b, _ = factoredGraphFixture(t, 3, 0, g, false)
	if r, e := DiscoverSplineGraphFactoredPolynomialContact(context.Background(), a, b, 0, GraphPointContactOptions{}); e == nil || r.Proven {
		t.Fatal("factor degree cap", e)
	}
}
