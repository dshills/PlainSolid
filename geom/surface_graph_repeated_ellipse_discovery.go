package geom

import (
	"context"
	"math/big"
)

// DiscoverSplineGraphRepeatedEllipse recovers an axis-aligned double ellipse
// from a pure quartic numerator on a union-knot cell. It independently
// verifies the inferred ellipse against both complete original supports.
// The explicit certifier also accepts nonconstant strict-sign cofactors.
func DiscoverSplineGraphRepeatedEllipse(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r SplineGraphEllipse, err error) {
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
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += b.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	cell := cells[0]
	if cell.degreeU > 8 || cell.degreeV > 8 {
		return r, ErrConvergence
	}
	var center [2]*big.Rat
	for axis := range center {
		q, e := graphNumeratorSlice(ctx, cell, axis, new(big.Rat), b)
		if e != nil {
			return r, e
		}
		if len(q) != 5 {
			return r, ErrConvergence
		}
		center[axis] = new(big.Rat).Quo(new(big.Rat).Neg(q[3]), new(big.Rat).Mul(big.NewRat(4, 1), q[4]))
		if e := b.check(center[axis]); e != nil {
			return r, e
		}
	}
	d := cell.Domain()
	var hint GraphEllipseHint
	for axis, limits := range [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
		p, e := graphNumeratorSlice(ctx, cell, axis, center[1-axis], b)
		if e != nil {
			return r, e
		}
		p, e = composeProjectionPolynomial(p, rationalPolynomial{center[axis], big.NewRat(1, 1)}, b)
		if e != nil {
			return r, e
		}
		if len(p) != 5 || p[1].Sign() != 0 || p[3].Sign() != 0 || p[0].Sign() != p[4].Sign() || p[2].Sign() != -p[4].Sign() {
			return r, ErrConvergence
		}
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(limits[1]), new(big.Rat).SetFloat64(limits[0]))
		hint.Center[axis] = new(big.Rat).Add(new(big.Rat).SetFloat64(limits[0]), new(big.Rat).Mul(width, center[axis]))
		radius := new(big.Rat).Quo(new(big.Rat).Neg(p[2]), new(big.Rat).Mul(big.NewRat(2, 1), p[4]))
		hint.RadiusSquared[axis] = new(big.Rat).Mul(radius, new(big.Rat).Mul(width, width))
		for _, q := range []*big.Rat{hint.Center[axis], hint.RadiusSquared[axis]} {
			if e := b.check(q); e != nil {
				return r, e
			}
		}
	}
	o.MaxOperations -= b.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	return CertifySplineGraphRepeatedEllipse(ctx, a, c, hint, o)
}

func graphNumeratorSlice(ctx context.Context, cell *SurfacePatch, axis int, fixed *big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	counts := [2]int{cell.countU, cell.countV}
	controls := make([]*big.Rat, counts[axis])
	row := make([]*big.Rat, counts[1-axis])
	for i := range controls {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j := range row {
			uv := [2]int{}
			uv[axis], uv[1-axis] = i, j
			row[j] = cell.net[uv[0]*cell.countV+uv[1]][2]
		}
		p, e := bernsteinToPower(row, b)
		if e != nil {
			return nil, e
		}
		controls[i], e = polynomialValue(p, fixed, b)
		if e != nil {
			return nil, e
		}
	}
	return bernsteinToPower(controls, b)
}
