package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestGraphLineParabolaOrdinaryNode(t *testing.T) {
	ctx := context.Background()
	r, s, m, k := big.NewRat(1, 3), big.NewRat(2, 5), big.NewRat(1, 8), big.NewRat(1, 4)
	a, b := coupledQuarticFixture(t, [2]*big.Rat{r, s}, big.NewRat(1, 1), false)
	q0 := new(big.Rat).Add(new(big.Rat).Sub(s, new(big.Rat).Mul(m, r)), new(big.Rat).Mul(k, new(big.Rat).Mul(r, r)))
	q1 := new(big.Rat).Sub(m, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(k, r)))
	power := map[[2]int]*big.Rat{{0, 0}: new(big.Rat).Mul(r, q0), {1, 0}: new(big.Rat).Sub(new(big.Rat).Mul(r, q1), q0), {2, 0}: new(big.Rat).Sub(new(big.Rat).Mul(r, k), q1), {3, 0}: new(big.Rat).Neg(k), {0, 1}: new(big.Rat).Neg(r), {1, 1}: big.NewRat(1, 1)}
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			z := new(big.Rat)
			for ij, q := range power {
				if ij[0] > i || ij[1] > j {
					continue
				}
				x := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(4, int64(ij[0])))
				y := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(4, int64(ij[1])))
				z.Add(z, new(big.Rat).Mul(q, new(big.Rat).Mul(x, y)))
			}
			a.net[5*i+j][2] = z
		}
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	for side, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		got, e := CertifyGraphLineParabolaNode(ctx, pair[0], pair[1], r, s, m, k, GraphPointContactOptions{})
		if e != nil || !got.Proven || got.Side != 1-2*side {
			t.Fatal(got, e)
		}
		if got.Parameters[0].Cmp(r) != 0 || got.Parameters[1].Cmp(s) != 0 || got.Tangents[1][1].Cmp(m) != 0 {
			t.Fatal(got)
		}
		for i, uv := range got.Terminals {
			ja, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil || ja.Point[2].Sign() != 0 {
				t.Fatal(i, ja, e)
			}
			if i >= 2 {
				dx := new(big.Rat).Sub(uv[0], r)
				want := new(big.Rat).Add(s, new(big.Rat).Add(new(big.Rat).Mul(m, dx), new(big.Rat).Mul(k, new(big.Rat).Mul(dx, dx))))
				if uv[1].Cmp(want) != 0 {
					t.Fatal("wrong parabola endpoint")
				}
			}
		}
	}
	if _, e := CertifyGraphLineParabolaNode(ctx, a, b, r, s, m, big.NewRat(0, 1), GraphPointContactOptions{}); e == nil {
		t.Fatal("line-only accepted")
	}
	if _, e := CertifyGraphLineParabolaNode(ctx, a, b, r, s, m, k, GraphPointContactOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyGraphLineParabolaNode(cancelled, a, b, r, s, m, k, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	// Same exact polynomial over non-unit original chart coordinates.
	for _, p := range []*SurfacePatch{a, b} {
		for i := range p.knotsU {
			p.knotsU[i] = 2 + 3*p.knotsU[i]
		}
		for i := range p.knotsV {
			p.knotsV[i] = -1 + 2*p.knotsV[i]
		}
	}
	scaled, e := CertifyGraphLineParabolaNode(ctx, a, b, r, s, m, k, GraphPointContactOptions{})
	if e != nil || scaled.Parameters[0].Cmp(big.NewRat(3, 1)) != 0 || scaled.Parameters[1].Cmp(big.NewRat(-1, 5)) != 0 || scaled.Tangents[1][0].Cmp(big.NewRat(3, 1)) != 0 || scaled.Tangents[1][1].Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(scaled, e)
	}
	a.net[0][2].Add(a.net[0][2], big.NewRat(1, 1000000))
	if _, e := CertifyGraphLineParabolaNode(ctx, a, b, r, s, m, k, GraphPointContactOptions{}); e == nil {
		t.Fatal("extra perturbation accepted")
	}
}
