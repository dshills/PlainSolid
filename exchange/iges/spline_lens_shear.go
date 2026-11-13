package iges

import (
	"math/big"
	"plainsolid/geom"
)

func lensAverage(x, y float64) (float64, error) {
	a, b := new(big.Rat).SetFloat64(x), new(big.Rat).SetFloat64(y)
	if a == nil || b == nil {
		return 0, geom.ErrInvalid
	}
	return geom.RepresentedExactScalar(a.Quo(a.Add(a, b), big.NewRat(2, 1)))
}
func igesLensQuadratic(m SplineLensSolid) (geom.GraphQuadraticEllipseHint, error) {
	rat := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	rx, ry, sh, cx, cy := rat(m.Radii.X), rat(m.Radii.Y), rat(m.Shear), rat(m.Center.X), rat(m.Center.Y)
	if rx == nil || ry == nil || sh == nil || cx == nil || cy == nil || rx.Sign() <= 0 || ry.Sign() <= 0 {
		return geom.GraphQuadraticEllipseHint{}, geom.ErrInvalid
	}
	tilt := rat(m.Tilt)
	if tilt == nil {
		return geom.GraphQuadraticEllipseHint{}, geom.ErrInvalid
	}
	det := new(big.Rat).Sub(new(big.Rat).Mul(rx, ry), new(big.Rat).Mul(sh, tilt))
	if det.Sign() <= 0 {
		return geom.GraphQuadraticEllipseHint{}, geom.ErrInvalid
	}
	den := new(big.Rat).Mul(det, det)
	a := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(ry, ry), new(big.Rat).Mul(tilt, tilt)), den)
	cross := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(-2, 1), new(big.Rat).Add(new(big.Rat).Mul(ry, sh), new(big.Rat).Mul(tilt, rx))), den)
	c := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(sh, sh), new(big.Rat).Mul(rx, rx)), den)
	lu := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(a, cx)), new(big.Rat).Mul(cross, cy)))
	lv := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(c, cy)), new(big.Rat).Mul(cross, cx)))
	constant := new(big.Rat).Add(new(big.Rat).Mul(a, new(big.Rat).Mul(cx, cx)), new(big.Rat).Mul(c, new(big.Rat).Mul(cy, cy)))
	constant.Add(constant, new(big.Rat).Mul(cross, new(big.Rat).Mul(cx, cy)))
	constant.Sub(constant, big.NewRat(1, 1))
	return geom.GraphQuadraticEllipseHint{Coefficients: [6]*big.Rat{constant, lu, a, lv, c, cross}}, nil
}
