package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestRationalBezierCoefficientAndDerivativeCertificate(t *testing.T) {
	p, e := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	coeff, minW, ok := p.RationalBezierCoefficients(4)
	if !ok || minW.Sign() <= 0 {
		t.Fatal(ok, minW)
	}
	u, v := big.NewRat(1, 3), big.NewRat(2, 5)
	var h [4]*big.Rat
	for k := 0; k < 4; k++ {
		h[k] = new(big.Rat)
		up := big.NewRat(1, 1)
		for i := range coeff {
			vp := big.NewRat(1, 1)
			for j := range coeff[i] {
				h[k].Add(h[k], new(big.Rat).Mul(coeff[i][j][k], new(big.Rat).Mul(up, vp)))
				vp.Mul(vp, v)
			}
			up.Mul(up, u)
		}
	}
	jet, e := p.EvaluateRational(context.Background(), u, v, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for k := 0; k < 3; k++ {
		if new(big.Rat).Quo(h[k], h[3]).Cmp(jet.Point[k]) != 0 {
			t.Fatal("coefficient evaluation")
		}
	}
	squared, sum, ok := p.RationalBezierDerivativeBounds(4)
	if !ok || squared.Sign() <= 0 || sum.Sign() <= 0 {
		t.Fatal(squared, sum, ok)
	}
	plane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{2 * u, 3 * v, 0} })
	squared, sum, ok = plane.RationalBezierDerivativeBounds(4)
	if !ok || squared.Cmp(big.NewRat(13, 1)) != 0 || sum.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal(squared, sum, ok)
	}
	coeff[0][0][0].SetInt64(999)
	again, _, ok := p.RationalBezierCoefficients(4)
	if !ok || again[0][0][0].Cmp(coeff[0][0][0]) == 0 {
		t.Fatal("alias")
	}
	if _, _, ok := p.RationalBezierCoefficients(7); ok {
		t.Fatal("cap")
	}
	if _, _, ok := p.RationalBezierDerivativeBounds(0); ok {
		t.Fatal("cap")
	}
}
