package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func boundaryGraphContactFixture(t *testing.T, u, v, gu, gv float64) (*SurfacePatch, *SurfacePatch) {
	a, c := graphContactFixture(t, false, false)
	contribution := func(i int, origin, gradient float64) *big.Rat {
		x := big.NewRat(int64(i), 4)
		x2 := big.NewRat(int64(i*(i-1)), 12)
		o, g := new(big.Rat).SetFloat64(origin), new(big.Rat).SetFloat64(gradient)
		linear := new(big.Rat).Sub(g, new(big.Rat).Mul(big.NewRat(2, 1), o))
		constant := new(big.Rat).Sub(new(big.Rat).Mul(o, o), new(big.Rat).Mul(g, o))
		return new(big.Rat).Add(new(big.Rat).Add(x2, new(big.Rat).Mul(linear, x)), constant)
	}
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			k := 5*i + j
			a.net[k][2] = new(big.Rat).Add(c.net[k][2], new(big.Rat).Add(contribution(i, u, gu), contribution(j, v, gv)))
		}
	}
	if e := a.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	return a, c
}
func TestSplineGraphBoundaryKKTContact(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		u, v, gu, gv float64
		mask         uint8
	}{{0, .5, 1, 0, 1}, {1, .5, -2, 0, 2}, {.5, 0, 0, 1, 4}, {.5, 1, 0, -1, 8}, {0, 1, 1, -1, 9}, {0, 0, 0, 0, 5}} {
		a, c := boundaryGraphContactFixture(t, test.u, test.v, test.gu, test.gv)
		uv := [2]*big.Rat{new(big.Rat).SetFloat64(test.u), new(big.Rat).SetFloat64(test.v)}
		r, e := CertifySplineGraphPointContact(ctx, a, c, uv, GraphPointContactOptions{})
		if e != nil || !r.Proven || r.Side != 1 || r.BoundaryMask != test.mask {
			t.Fatal(test, r, e)
		}
		reverse, e := CertifySplineGraphPointContact(ctx, c, a, uv, GraphPointContactOptions{})
		if e != nil || !reverse.Proven || reverse.Side != -1 || reverse.BoundaryMask != test.mask {
			t.Fatal(test, reverse, e)
		}
		// The exact witness must agree on both nonplanar supports.
		pa, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		pc, e := c.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil || pa.Point[2].Cmp(pc.Point[2]) != 0 {
			t.Fatal(pa, pc, e)
		}
	}
}
func TestSplineGraphBoundaryWrongKKTRejected(t *testing.T) {
	for _, gradient := range []Vec2{{-1, 0}, {0, 1}} {
		a, c := boundaryGraphContactFixture(t, 0, .5, gradient.X, gradient.Y)
		r, e := CertifySplineGraphPointContact(context.Background(), a, c, [2]*big.Rat{new(big.Rat), big.NewRat(1, 2)}, GraphPointContactOptions{})
		if !errors.Is(e, ErrConvergence) || r.Proven {
			t.Fatal(r, e)
		}
	}
}
