package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// composedGraphPlaneTrim proves the complete homogeneous residual over one
// tensor span. A(s)+B*t with constant, nonzero B and constant positive weight
// determines a unique section regardless of the world projection.
func composedGraphPlaneTrim(p *geom.SurfacePatch, plane polynomialPlaneEquation, free int) (PCurve, int, error) {
	if free != 0 && free != 1 {
		return nil, 0, geom.ErrInvalid
	}
	h, _, ok := p.RationalBezierCoefficients(4)
	if !ok || len(h) == 0 {
		return nil, 0, geom.ErrConvergence
	}
	if free == 1 {
		h = transposePolynomialPlanePower(h)
	}
	if len(h[0]) < 2 || h[0][0][3].Sign() <= 0 {
		return nil, 0, geom.ErrConvergence
	}
	var b *big.Rat
	powers := make([]*big.Rat, len(h))
	for i, row := range h {
		for j, c := range row {
			if i+j > 0 && c[3].Sign() != 0 {
				return nil, 0, geom.ErrConvergence
			}
			x := new(big.Rat).Sub(plane.dot([3]*big.Rat{c[0], c[1], c[2]}), new(big.Rat).Mul(c[3], plane.offset))
			switch {
			case j == 0:
				powers[i] = x.Neg(x)
			case j == 1 && i == 0:
				b = x
			case x.Sign() != 0:
				return nil, 0, geom.ErrConvergence
			}
		}
	}
	if b == nil || b.Sign() == 0 {
		return nil, 0, geom.ErrConvergence
	}
	degree := 2
	for i, x := range powers {
		x.Quo(x, b)
		if i > degree && x.Sign() != 0 {
			degree = 4
		}
	}
	d := p.Domain()
	lo, hi := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}, [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
	width := [2]*big.Rat{new(big.Rat).Sub(hi[0], lo[0]), new(big.Rat).Sub(hi[1], lo[1])}
	controls := make([][3]*big.Rat, degree+1)
	for i := range controls {
		value := new(big.Rat)
		for j := 0; j <= i && j < len(powers); j++ {
			value.Add(value, new(big.Rat).Mul(powers[j], big.NewRat(polynomialChoose(i, j), polynomialChoose(degree, j))))
		}
		controls[i][free] = new(big.Rat).Add(lo[free], new(big.Rat).Mul(width[free], big.NewRat(int64(i), int64(degree))))
		controls[i][1-free] = value.Add(lo[1-free], value.Mul(value, width[1-free]))
		controls[i][2] = big.NewRat(1, 1)
	}
	if !strictComposedGraphControls(controls, free, lo[1-free], hi[1-free], 0) {
		return nil, 0, geom.ErrConvergence
	}
	if degree == 2 {
		q, e := NewExactQuadraticUV([3][3]*big.Rat{controls[0], controls[1], controls[2]})
		return q, b.Sign(), e
	}
	q, e := NewExactBezierUV(controls)
	return q, b.Sign(), e
}

func strictComposedGraphControls(h [][3]*big.Rat, free int, lo, hi *big.Rat, depth int) bool {
	inside := func(p [3]*big.Rat) bool { return p[1-free].Cmp(lo) > 0 && p[1-free].Cmp(hi) < 0 }
	if !inside(h[0]) || !inside(h[len(h)-1]) {
		return false
	}
	ok := true
	for _, p := range h[1 : len(h)-1] {
		ok = ok && inside(p)
	}
	if ok {
		return true
	}
	if depth >= 8 {
		return false
	}
	a, b, e := splitCapBezier(h, big.NewRat(1, 2), 16384)
	return e == nil && strictComposedGraphControls(a, free, lo, hi, depth+1) && strictComposedGraphControls(b, free, lo, hi, depth+1)
}

func composeProjectedGraphSectionControls(ctx context.Context, patch *geom.SurfacePatch, c PCurve, axes [2]int, limits geom.SurfacePatchOptions) ([][3]*big.Rat, error) {
	curve, e := polynomialTrimDescriptor(ctx, c, false, limits)
	if e != nil {
		return nil, e
	}
	cells, e := patch.BuildSurfaceCurveCompositionCells(ctx, curve, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: limits})
	if e != nil {
		return nil, e
	}
	if !cells.Complete || len(cells.Cells) != 1 {
		return nil, geom.ErrConvergence
	}
	power, _, e := cells.Cells[0].HomogeneousPower(ctx, geom.SurfacePatchEvaluationOptions{MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	degree := 2
	for _, axis := range []int{axes[0], axes[1], 3} {
		for i := len(power[axis]) - 1; i > degree; i-- {
			if power[axis][i].Sign() != 0 {
				degree = 4
				if i > degree {
					return nil, geom.ErrConvergence
				}
				break
			}
		}
	}
	bits := limits.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	h := make([][3]*big.Rat, degree+1)
	for i := range h {
		for j, axis := range []int{axes[0], axes[1], 3} {
			x := new(big.Rat)
			for k := 0; k <= i && k < len(power[axis]); k++ {
				x.Add(x, new(big.Rat).Mul(power[axis][k], big.NewRat(polynomialChoose(i, k), polynomialChoose(degree, k))))
			}
			if x.Num().BitLen() > bits || x.Denom().BitLen() > bits || j == 2 && x.Sign() <= 0 {
				return nil, geom.ErrConvergence
			}
			h[i][j] = x
		}
	}
	return h, nil
}
