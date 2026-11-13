package trimmed

import (
	"errors"
	"math/big"

	"plainsolid/geom"
)

func graphPlaneTrimEquation(p *geom.SurfacePatch, plane polynomialPlaneEquation, freeAxis int, rational bool) (PCurve, int, error) {
	q, sign, err := polynomialPlaneTrimEquation(p, plane, freeAxis)
	if rational && errors.Is(err, geom.ErrConvergence) {
		r, sign, err := rationalGraphPlaneTrim(p, plane, freeAxis)
		return r, sign, err
	}
	return q, sign, err
}

// rationalGraphPlaneTrim handles a single polynomial knot cell with an
// affine projection and plane residual A(s)+B(s)t, where deg(A) <= 4 and
// 1 <= deg(B) <= 2. Positive homogeneous Bernstein controls certify that B
// never vanishes and the complete rational section stays strictly interior.
func rationalGraphPlaneTrim(p *geom.SurfacePatch, plane polynomialPlaneEquation, freeAxis int) (PCurve, int, error) {
	var zero PCurve
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
			if !(j == 0 || j == 1 && i <= 2) && c[plane.dropped].Sign() != 0 {
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
	denom := [3]*big.Rat{c(0, 1), c(1, 1), c(2, 1)}
	sign := denom[0].Sign()
	if (denom[1].Sign() == 0 && denom[2].Sign() == 0) || sign == 0 {
		return zero, 0, geom.ErrConvergence
	}
	numerator := make([]*big.Rat, 5)
	degree := 2 // The free-coordinate homogeneous numerator is s*B(s).
	if denom[2].Sign() != 0 {
		degree = 3
	}
	for i := range numerator {
		numerator[i] = new(big.Rat).Neg(c(i, 0))
		if numerator[i].Sign() != 0 {
			degree = max(degree, i)
		}
	}
	numerator[0].Add(numerator[0], plane.offset)
	if sign < 0 {
		for _, x := range denom {
			x.Neg(x)
		}
		for _, x := range numerator {
			x.Neg(x)
		}
	}
	d := p.Domain()
	lo, hi := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	a0, b00 := ratInset(lo[freeAxis]), ratInset(lo[1-freeAxis])
	da, db := new(big.Rat).Sub(ratInset(hi[freeAxis]), a0), new(big.Rat).Sub(ratInset(hi[1-freeAxis]), b00)
	controls := make([][3]*big.Rat, degree+1)
	for i := range controls {
		basis := func(j int) *big.Rat { return big.NewRat(polynomialChoose(i, j), polynomialChoose(degree, j)) }
		ww, free := new(big.Rat), new(big.Rat)
		for j := 0; j <= i && j < len(denom); j++ {
			ww.Add(ww, new(big.Rat).Mul(denom[j], basis(j)))
			if j < i {
				free.Add(free, new(big.Rat).Mul(denom[j], basis(j+1)))
			}
		}
		dep := new(big.Rat)
		for j := 0; j <= i; j++ {
			dep.Add(dep, new(big.Rat).Mul(numerator[j], basis(j)))
		}
		if ww.Sign() <= 0 || dep.Sign() <= 0 || dep.Cmp(ww) >= 0 {
			return zero, 0, geom.ErrConvergence
		}
		controls[i][freeAxis] = new(big.Rat).Add(new(big.Rat).Mul(a0, ww), new(big.Rat).Mul(da, free))
		controls[i][1-freeAxis] = new(big.Rat).Add(new(big.Rat).Mul(b00, ww), new(big.Rat).Mul(db, dep))
		controls[i][2] = ww
	}
	if degree == 2 {
		q, err := NewExactQuadraticUV([3][3]*big.Rat{controls[0], controls[1], controls[2]})
		return q, sign, err
	}
	q, err := NewExactBezierUV(controls)
	return q, sign, err
}
