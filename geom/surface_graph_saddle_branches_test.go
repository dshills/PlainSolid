package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCertifiedQuadraticSaddleFourBranches(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	for _, pair := range [2][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := CertifyQuadraticGraphSaddleBranches(context.Background(), pair[0], pair[1], uv, GraphPointContactOptions{})
		if e != nil || !r.Proven || len(r.Rays) != 4 || !r.Embedding.ProvenInjective {
			t.Fatal(r, e)
		}
		for _, ray := range r.Rays {
			want := big.NewRat(-1, 1)
			if ray.Line == 1 {
				want.SetInt64(1)
			}
			if ray.Axis != 0 || ray.SlopeLower.Cmp(want) > 0 || ray.SlopeUpper.Cmp(want) < 0 || ray.Extent.Sign() <= 0 {
				t.Fatal(ray)
			}
			u := new(big.Rat).Add(uv[0], new(big.Rat).Mul(big.NewRat(int64(ray.Direction), 1), ray.Extent))
			v := new(big.Rat).Add(uv[1], new(big.Rat).Mul(want, new(big.Rat).Sub(u, uv[0])))
			x, e := pair[0].EvaluateRational(context.Background(), u, v, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			y, e := pair[1].EvaluateRational(context.Background(), u, v, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for i := range x.Point {
				if x.Point[i].Cmp(y.Point[i]) != 0 {
					t.Fatal("independent ray endpoint incidence")
				}
			}
		}
	}
}
func TestQuadraticSaddleVerticalAndHigherOrderRejection(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	// Replace height difference by (u-.5)(v-.5), keeping the curved base.
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			u := big.NewRat(int64(2*i-(a.countU-1)), int64(2*(a.countU-1)))
			v := big.NewRat(int64(2*j-(a.countV-1)), int64(2*(a.countV-1)))
			a.net[k][2] = new(big.Rat).Add(b.net[k][2], new(big.Rat).Mul(u, v))
		}
	}
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	r, e := CertifyQuadraticGraphSaddleBranches(context.Background(), a, b, uv, GraphPointContactOptions{})
	if e != nil || !r.Proven || len(r.Rays) != 4 || r.Rays[0].Axis != 1 || r.Rays[2].Axis != 0 {
		t.Fatal(r, e)
	}
	a, b = graphContactFixture(t, true, true)
	r, e = CertifyQuadraticGraphSaddleBranches(context.Background(), a, b, uv, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal("quartic saddle is outside exact factor subset", r, e)
	}
}
func TestQuadraticSaddleIrrationalSlopes(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	n := a.degreeU
	q := func(i int) *big.Rat {
		return new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).Sub(big.NewRat(int64(i*(i-1)), int64(n*(n-1))), big.NewRat(int64(i), int64(n))))
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			a.net[k][2] = new(big.Rat).Add(b.net[k][2], new(big.Rat).Sub(q(i), new(big.Rat).Mul(big.NewRat(2, 1), q(j))))
		}
	}
	r, e := CertifyQuadraticGraphSaddleBranches(context.Background(), a, b, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, GraphPointContactOptions{})
	if e != nil || !r.Proven || len(r.Rays) != 4 {
		t.Fatal(r, e)
	}
	negative, positive := r.Rays[0], r.Rays[2]
	if negative.SlopeUpper.Sign() >= 0 || positive.SlopeLower.Sign() <= 0 {
		t.Fatal("factor sectors not separated", negative, positive)
	}
	for _, ray := range r.Rays {
		lo, hi := new(big.Rat).Abs(ray.SlopeLower), new(big.Rat).Abs(ray.SlopeUpper)
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(1, 2)) > 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(1, 2)) < 0 {
			t.Fatal(ray)
		}
	}
}
