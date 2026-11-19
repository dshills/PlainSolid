package trimmed

import (
	"math/big"
	"testing"

	"plainsolid/geom"
)

func nativeThreePolynomialNodeFixture(t *testing.T, axis int, firstDenominator, gapDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=(1-firstDenominator*u)(1-2u)(3-4u)(1+u)/gapDenominator.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{
		big.NewRat(3, gapDenominator), big.NewRat(-3*firstDenominator-7, gapDenominator),
		big.NewRat(7*firstDenominator-2, gapDenominator), big.NewRat(2*firstDenominator+8, gapDenominator),
		big.NewRat(-8*firstDenominator, gapDenominator),
	}
	for i, c := range f {
		g[i].Add(g[i], c)
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func checkNativeThreePolynomialBranch(t *testing.T, c PCurve, axis int, power []*big.Rat) {
	t.Helper()
	q, ok := c.(ExactBezierUV)
	if !ok || len(q.control) != len(power) {
		t.Fatal("original polynomial degree and exact native curve")
	}
	left := q.control[0][axis]
	interval := new(big.Rat).Sub(q.control[len(power)-1][axis], left)
	for i, p := range q.control {
		want := new(big.Rat).Add(left, new(big.Rat).Mul(interval, big.NewRat(int64(i), int64(len(power)-1))))
		if p[2].Cmp(big.NewRat(1, 1)) != 0 || p[axis].Cmp(want) != 0 {
			t.Fatal("original affine free coordinate on the branch", i, p, want)
		}
	}
	// With unit weights and an affine free parameter, the difference from
	// the source branch is a polynomial of at most the original degree.
	for i := range power {
		uv, _, e := q.EvaluateRational(big.NewRat(int64(i), int64(len(power)-1)))
		if e != nil {
			t.Fatal(e)
		}
		want := new(big.Rat)
		for j := len(power) - 1; j >= 0; j-- {
			want.Mul(want, uv[axis]).Add(want, power[j])
		}
		if uv[1-axis].Cmp(want) != 0 {
			t.Fatal("reloaded curve differs from its original source polynomial", i, uv, want)
		}
	}
}
