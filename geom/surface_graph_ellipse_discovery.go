package geom

import (
	"context"
	"math/big"
)

// DiscoverSplineGraphEllipse recovers an exact axis-aligned ellipse from a
// genuinely quadratic height numerator on the first union-knot cell. Elevated
// polynomial degrees are removed by exact power conversion. The inferred hint
// is independently checked on the original supports by CertifySplineGraphEllipse;
// therefore later cells may have different strict quotient polynomials. Failure
// to discover a hint is unresolved, never a nonintersection certificate.
func DiscoverSplineGraphEllipse(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r SplineGraphEllipse, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += budget.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	p := cells[0]
	coefficients, quadratic, e := graphAxisQuadraticCoefficients(p, budget)
	if e != nil {
		return r, e
	}
	if !quadratic || coefficients[2].Sign() == 0 || coefficients[4].Sign() != coefficients[2].Sign() {
		return r, ErrConvergence
	}
	center := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Neg(coefficients[1]), new(big.Rat).Mul(big.NewRat(2, 1), coefficients[2])), new(big.Rat).Quo(new(big.Rat).Neg(coefficients[3]), new(big.Rat).Mul(big.NewRat(2, 1), coefficients[4]))}
	minimum := new(big.Rat).Set(coefficients[0])
	for axis := 0; axis < 2; axis++ {
		minimum.Sub(minimum, new(big.Rat).Mul(coefficients[2+2*axis], new(big.Rat).Mul(center[axis], center[axis])))
	}
	if minimum.Sign() == 0 || minimum.Sign() == coefficients[2].Sign() {
		return r, ErrConvergence
	}
	d := p.Domain()
	bounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	hint := GraphEllipseHint{}
	for axis := 0; axis < 2; axis++ {
		origin := new(big.Rat).SetFloat64(bounds[axis][0])
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][1]), origin)
		hint.Center[axis] = new(big.Rat).Add(origin, new(big.Rat).Mul(width, center[axis]))
		hint.RadiusSquared[axis] = new(big.Rat).Mul(new(big.Rat).Quo(new(big.Rat).Neg(minimum), coefficients[2+2*axis]), new(big.Rat).Mul(width, width))
		for _, value := range []*big.Rat{hint.Center[axis], hint.RadiusSquared[axis]} {
			if e := budget.check(value); e != nil {
				return r, e
			}
		}
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	return CertifySplineGraphEllipse(ctx, a, c, hint, o)
}

// graphAxisQuadraticCoefficients returns c0+c1*u+c2*u²+c3*v+c4*v² only when
// every mixed or higher power vanishes exactly, independent of nominal degree.
func graphAxisQuadraticCoefficients(p *SurfacePatch, b *polynomialBudget) (out [5]*big.Rat, proved bool, err error) {
	all, proved, err := graphQuadraticCoefficients(p, b)
	if err != nil || !proved || all[5].Sign() != 0 {
		return out, false, err
	}
	copy(out[:], all[:5])
	return out, true, nil
}
func graphQuadraticCoefficients(p *SurfacePatch, b *polynomialBudget) (out [6]*big.Rat, proved bool, err error) {
	for i := range out {
		out[i] = new(big.Rat)
	}
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		values := make([]*big.Rat, p.countV)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		rows[i], err = bernsteinToPower(values, b)
		if err != nil {
			return out, false, err
		}
	}
	for j := 0; j < p.countV; j++ {
		values := make([]*big.Rat, p.countU)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		column, e := bernsteinToPower(values, b)
		if e != nil {
			return out, false, e
		}
		for i, value := range column {
			if value.Sign() == 0 {
				continue
			}
			switch {
			case i == 0 && j == 0:
				out[0].Set(value)
			case i == 1 && j == 0:
				out[1].Set(value)
			case i == 2 && j == 0:
				out[2].Set(value)
			case i == 0 && j == 1:
				out[3].Set(value)
			case i == 0 && j == 2:
				out[4].Set(value)
			case i == 1 && j == 1:
				out[5].Set(value)
			default:
				return out, false, nil
			}
		}
	}
	return out, true, nil
}
