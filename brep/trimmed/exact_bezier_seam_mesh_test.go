package trimmed

import (
	"math/big"
	"plainsolid/geom"
	"testing"
)

func bezierSeamFixture(t *testing.T, n int) (*geom.SurfacePatch, ExactBezierUV, *geom.SurfacePatch, ExactBezierUV) {
	t.Helper()
	patch := func(swap bool) *geom.SurfacePatch {
		p := []geom.Vec3{}
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				x, y := float64(i), float64(j)
				if swap {
					x, y = y, x
				}
				p = append(p, geom.Vec3{X: x, Y: y, Z: x * y})
			}
		}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		q, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	h := make([][3]*big.Rat, n+1)
	for i := range h {
		w := big.NewRat(1, 1)
		if i == 1 {
			w = big.NewRat(3, 2)
		}
		y := big.NewRat(int64(1+2*i), int64(4*n))
		if i == 0 {
			y = big.NewRat(1, 4)
		}
		if i == n {
			y = big.NewRat(3, 4)
		}
		h[i] = [3]*big.Rat{new(big.Rat).Mul(big.NewRat(int64(i), int64(n)), w), new(big.Rat).Mul(y, w), w}
	}
	a, e := NewExactBezierUV(h)
	if e != nil {
		t.Fatal(e)
	}
	for i := range h {
		h[i][0], h[i][1] = h[i][1], h[i][0]
	}
	b, e := NewExactBezierUV(h)
	if e != nil {
		t.Fatal(e)
	}
	return patch(false), a, patch(true), b.reversed()
}
