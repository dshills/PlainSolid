package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func coupledQuarticFixture(t *testing.T, center [2]*big.Rat, beta *big.Rat, weighted bool) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	u4, v4 := rootPower(t, center[0], 4), rootPower(t, center[1], 4)
	u2, v2 := rootPower(t, center[0], 2), rootPower(t, center[1], 2)
	power := map[[2]int]*big.Rat{}
	add := func(key [2]int, q *big.Rat) {
		if power[key] == nil {
			power[key] = new(big.Rat)
		}
		power[key].Add(power[key], q)
	}
	for i, q := range u4 {
		add([2]int{i, 0}, q)
	}
	for j, q := range v4 {
		add([2]int{0, j}, q)
	}
	for i, x := range u2 {
		for j, y := range v2 {
			add([2]int{i, j}, new(big.Rat).Mul(beta, new(big.Rat).Mul(x, y)))
		}
	}
	poles := make([]Vec3, 25)
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			poles[5*i+j] = Vec3{float64(i) / 4, float64(j) / 4, 0}
		}
	}
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	s, _ := NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			z := new(big.Rat)
			for ij, q := range power {
				if ij[0] > i || ij[1] > j {
					continue
				}
				fu := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(4, int64(ij[0])))
				fv := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(4, int64(ij[1])))
				z.Add(z, new(big.Rat).Mul(q, new(big.Rat).Mul(fu, fv)))
			}
			index := 5*i + j
			a.net[index][2] = z
			if weighted {
				w := big.NewRat(int64(4+i), 4)
				hx := new(big.Rat).Add(big.NewRat(int64(i), 4), big.NewRat(int64(i*(i-1)), 12))
				hy := new(big.Rat).Mul(big.NewRat(int64(j), 4), w)
				for _, p := range []*SurfacePatch{a, b} {
					p.net[index][0] = new(big.Rat).Set(hx)
					p.net[index][1] = new(big.Rat).Set(hy)
					p.net[index][3] = new(big.Rat).Set(w)
				}
			}
		}
	}
	for _, p := range []*SurfacePatch{a, b} {
		if e := p.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
			t.Fatal(e)
		}
	}
	return a, b
}
func TestCoupledQuarticIsolatedGraphTangency(t *testing.T) {
	ctx := context.Background()
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	for _, weighted := range []bool{false, true} {
		for _, beta := range []int64{-1, 1, 5} {
			a, b := coupledQuarticFixture(t, uv, big.NewRat(beta, 1), weighted)
			for side, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				r, e := CertifyCoupledQuarticGraphPointContact(ctx, pair[0], pair[1], uv, GraphPointContactOptions{})
				want := 1
				if side == 1 {
					want = -1
				}
				if e != nil || !r.Proven || r.Side != want || r.Parameters[0].Cmp(uv[0]) != 0 || r.Point[2].Sign() != 0 {
					t.Fatal(weighted, beta, r, e)
				}
			}
			found, e := DiscoverCoupledQuarticGraphPointContact(ctx, a, b, GraphPointContactOptions{})
			if e != nil || !found.Proven || found.Parameters[0].Cmp(uv[0]) != 0 || found.Parameters[1].Cmp(uv[1]) != 0 {
				t.Fatal("autonomous quartic", found, e)
			}
			if _, e := CertifySeparableSplineGraphPointContact(ctx, a, b, uv, SeparableGraphContactOptions{}); e == nil {
				t.Fatal("new fixture unexpectedly separable")
			}
		}
	}
	a, b := coupledQuarticFixture(t, uv, big.NewRat(-2, 1), false)
	if _, e := CertifyCoupledQuarticGraphPointContact(ctx, a, b, uv, GraphPointContactOptions{}); e == nil {
		t.Fatal("extra zero lines accepted")
	}
	a, b = coupledQuarticFixture(t, uv, big.NewRat(-3, 1), false)
	if _, e := CertifyCoupledQuarticGraphPointContact(ctx, a, b, uv, GraphPointContactOptions{}); e == nil {
		t.Fatal("saddle accepted")
	}
	a, b = coupledQuarticFixture(t, uv, big.NewRat(1, 1), false)
	if _, e := CertifyCoupledQuarticGraphPointContact(ctx, a, b, [2]*big.Rat{big.NewRat(1, 2), uv[1]}, GraphPointContactOptions{}); e == nil {
		t.Fatal("wrong witness")
	}
	if _, e := CertifyCoupledQuarticGraphPointContact(ctx, a, b, uv, GraphPointContactOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyCoupledQuarticGraphPointContact(cancelled, a, b, uv, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
