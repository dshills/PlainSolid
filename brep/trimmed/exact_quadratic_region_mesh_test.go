package trimmed

import (
	"math/big"
	"plainsolid/geom"
	"testing"
)

func nativeNonbinaryRoundedLoop(t *testing.T, r *big.Rat, reverse bool) []ExactQuadraticUV {
	t.Helper()
	zero := new(big.Rat)
	negative := new(big.Rat).Neg(r)
	s := new(big.Rat).Mul(r, big.NewRat(10, 9))
	ns := new(big.Rat).Neg(s)
	ends := [][2]*big.Rat{{r, zero}, {zero, r}, {negative, zero}, {zero, negative}}
	mids := [][2]*big.Rat{{s, s}, {ns, s}, {ns, ns}, {s, ns}}
	var loop []ExactQuadraticUV
	for i, a := range ends {
		b := ends[(i+1)%4]
		var h [3][3]*big.Rat
		for j, p := range [][2]*big.Rat{a, mids[i], b} {
			h[j] = [3]*big.Rat{new(big.Rat).Add(big.NewRat(1, 2), p[0]), new(big.Rat).Add(big.NewRat(1, 2), p[1]), big.NewRat(1, 1)}
		}
		q, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	if reverse {
		for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
			loop[i], loop[j] = loop[j], loop[i]
		}
		for i := range loop {
			loop[i] = loop[i].reversed()
		}
	}
	return loop
}
func nativeObliquePatch(t *testing.T) *geom.SurfacePatch {
	t.Helper()
	var poles []geom.Vec3
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, geom.Vec3{X: 2 * u, Y: 3 * v, Z: u + v})
		}
	}
	s, e := geom.NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
