package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestExactOcticBezierCertificates(t *testing.T) {
	var controls [][4]*big.Rat
	for i := 0; i <= 8; i++ {
		for j := 0; j <= 8; j++ {
			z := int64(0)
			if i == 8 && j == 8 {
				z = 1
			}
			controls = append(controls, [4]*big.Rat{big.NewRat(int64(i), 8), big.NewRat(int64(j), 8), big.NewRat(z, 1), big.NewRat(1, 1)})
		}
	}
	p, e := NewExactHomogeneousBezierSurface(context.Background(), 8, 8, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, w, ok := p.RationalBezierCoefficients(8)
	if !ok || len(c) != 9 || len(c[0]) != 9 || c[8][8][2].Cmp(big.NewRat(1, 1)) != 0 || w.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("original mixed degree16", ok)
	}
	l2, l1, ok := p.RationalBezierDerivativeBounds(8)
	// X=u,Y=v,Z=u^8 v^8: component derivative maxima are 1,1,8,8.
	if !ok || l2.Cmp(big.NewRat(130, 1)) != 0 || l1.Cmp(big.NewRat(18, 1)) != 0 {
		t.Fatal(l2, l1, ok)
	}
	for _, limit := range []int{6, 7, 9} {
		if _, _, ok := p.RationalBezierCoefficients(limit); ok {
			t.Fatal("degree admission", limit)
		}
		if _, _, ok := p.RationalBezierDerivativeBounds(limit); ok {
			t.Fatal("derivative admission", limit)
		}
	}
	c[8][8][2].SetInt64(42)
	again, _, ok := p.RationalBezierCoefficients(8)
	if !ok || again[8][8][2].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("owned coefficients")
	}
}
