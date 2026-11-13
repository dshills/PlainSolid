package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func graphContactFixture(t *testing.T, quartic, saddle bool) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	const n = 4
	var poles []Vec3
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			poles = append(poles, Vec3{float64(i) / n, float64(j) / n, 0})
		}
	}
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	s, e := NewBSplineSurface(n, n, n+1, n+1, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	c, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	choose := func(n, k int) int64 {
		x := int64(1)
		for i := 1; i <= k; i++ {
			x = x * int64(n-i+1) / int64(i)
		}
		return x
	}
	bern := func(i int, p []*big.Rat) *big.Rat {
		x := new(big.Rat)
		for k, v := range p {
			if k <= i {
				x.Add(x, new(big.Rat).Mul(v, big.NewRat(choose(i, k), choose(n, k))))
			}
		}
		return x
	}
	base := []*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
	u := []*big.Rat{big.NewRat(1, 4), big.NewRat(-1, 1), big.NewRat(1, 1)}
	v := u
	if quartic {
		u = []*big.Rat{big.NewRat(5, 16), big.NewRat(-3, 2), big.NewRat(5, 2), big.NewRat(-2, 1), big.NewRat(1, 1)}
	}
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			k := i*(n+1) + j
			z := new(big.Rat).Add(bern(i, base), bern(j, base))
			c.net[k][2] = new(big.Rat).Set(z)
			delta := new(big.Rat).Add(bern(i, u), bern(j, v))
			if saddle {
				delta.Sub(bern(i, u), bern(j, v))
			}
			a.net[k][2] = new(big.Rat).Add(z, delta)
		}
	}
	for _, p := range []*SurfacePatch{a, c} {
		if e := p.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
			t.Fatal(e)
		}
	}
	return a, c
}
func TestSplineGraphHigherDegreePointContact(t *testing.T) {
	ctx := context.Background()
	a, c := graphContactFixture(t, true, false)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	r, e := CertifySplineGraphPointContact(ctx, a, c, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Side != 1 || !r.Graph.ProvenInjective || r.Point[2].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(r, e)
	}
	r, e = CertifySplineGraphPointContact(ctx, c, a, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Side != -1 {
		t.Fatal(r, e)
	}
	uv[0] = big.NewRat(1, 3)
	r, e = CertifySplineGraphPointContact(ctx, a, c, uv, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
}
func TestSplineGraphPointContactRejectsSaddleAndLimits(t *testing.T) {
	ctx := context.Background()
	a, c := graphContactFixture(t, false, true)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	r, e := CertifySplineGraphPointContact(ctx, a, c, uv, GraphPointContactOptions{MaxDepth: 2})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	a, c = graphContactFixture(t, true, false)
	r, e = CertifySplineGraphPointContact(ctx, a, c, uv, GraphPointContactOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = CertifySplineGraphPointContact(canceled, a, c, uv, GraphPointContactOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
