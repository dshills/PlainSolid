package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// rationalGraphPlaneTrim handles a single polynomial knot cell with an
// affine projection and plane residual A(s)+B(s)t, where deg(A) <= 2 and
// deg(B) == 1. Positive homogeneous Bernstein controls certify that B
// never vanishes and the complete rational section stays strictly interior.
func rationalGraphPlaneTrim(p *geom.SurfacePatch, plane polynomialPlaneEquation, freeAxis int) (ExactQuadraticUV, int, error) {
	var zero ExactQuadraticUV
	if freeAxis < 0 || freeAxis > 1 {
		return zero, 0, geom.ErrInvalid
	}
	h, _, ok := p.RationalBezierCoefficients(4)
	if !ok {
		return zero, 0, geom.ErrConvergence
	}
	if freeAxis == 1 {
		h = transposePolynomialPlanePower(h)
	}
	w := h[0][0][3]
	for i, row := range h {
		for j, c := range row {
			if i+j != 0 && c[3].Sign() != 0 {
				return zero, 0, geom.ErrConvergence
			}
			for _, axis := range plane.axes {
				if i+j > 1 && c[axis].Sign() != 0 {
					return zero, 0, geom.ErrConvergence
				}
			}
			if !(j == 0 && i <= 2 || j == 1 && i <= 1) && c[plane.dropped].Sign() != 0 {
				return zero, 0, geom.ErrConvergence
			}
		}
	}
	c := func(i, j int) *big.Rat {
		if i >= len(h) || j >= len(h[i]) {
			return new(big.Rat)
		}
		x := h[i][j]
		return new(big.Rat).Quo(plane.dot([3]*big.Rat{x[0], x[1], x[2]}), w)
	}
	b0, b1 := c(0, 1), c(1, 1)
	sign := b0.Sign()
	if b1.Sign() == 0 || sign == 0 || new(big.Rat).Add(b0, b1).Sign() != sign {
		return zero, 0, geom.ErrConvergence
	}
	q0, q1, q2 := new(big.Rat).Sub(plane.offset, c(0, 0)), new(big.Rat).Neg(c(1, 0)), new(big.Rat).Neg(c(2, 0))
	if sign < 0 {
		for _, x := range []*big.Rat{b0, b1, q0, q1, q2} {
			x.Neg(x)
		}
	}
	half := big.NewRat(1, 2)
	weights := [3]*big.Rat{
		b0, new(big.Rat).Add(b0, new(big.Rat).Mul(b1, half)), new(big.Rat).Add(b0, b1),
	}
	dependent := [3]*big.Rat{
		q0, new(big.Rat).Add(q0, new(big.Rat).Mul(q1, half)), new(big.Rat).Add(new(big.Rat).Add(q0, q1), q2),
	}
	free := [3]*big.Rat{new(big.Rat), new(big.Rat).Mul(b0, half), weights[2]}
	d := p.Domain()
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	a0, b00 := ratInset(lo[freeAxis]), ratInset(lo[1-freeAxis])
	da, db := new(big.Rat).Sub(ratInset(hi[freeAxis]), a0), new(big.Rat).Sub(ratInset(hi[1-freeAxis]), b00)
	var controls [3][3]*big.Rat
	for i, ww := range weights {
		if ww.Sign() <= 0 || dependent[i].Sign() <= 0 || dependent[i].Cmp(ww) >= 0 {
			return zero, 0, geom.ErrConvergence
		}
		controls[i][freeAxis] = new(big.Rat).Add(new(big.Rat).Mul(a0, ww), new(big.Rat).Mul(da, free[i]))
		controls[i][1-freeAxis] = new(big.Rat).Add(new(big.Rat).Mul(b00, ww), new(big.Rat).Mul(db, dependent[i]))
		controls[i][2] = ww
	}
	q, err := NewExactQuadraticUV(controls)
	return q, sign, err
}
