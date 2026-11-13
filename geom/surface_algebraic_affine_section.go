package geom

import (
	"context"
	"math/big"
)

// BuildAffineSection constructs the exact model curve along UV=origin+t*direction
// between two ordered algebraic t endpoints. The finite rectangle clips the
// supporting line exactly; both requested endpoints must lie in that clip.
// Returned cells retain original t units, surface knot ancestry, and exact
// algebraic endpoints. No event or coefficient is rounded for construction.
func (p *SurfacePatch) BuildAffineSection(ctx context.Context, origin, direction [2]*big.Rat, lower, upper *ExactAlgebraicRoot, o SurfaceCurveAgreementOptions) (r SurfaceCurveCompositionCells, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || lower == nil || upper == nil || lower.root == nil || upper.root == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	d := p.Domain()
	mins, maxs := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	var lo, hi *big.Rat
	for axis := 0; axis < 2; axis++ {
		if origin[axis] == nil || direction[axis] == nil {
			return r, ErrInvalid
		}
		for _, q := range []*big.Rat{origin[axis], direction[axis]} {
			if e := b.check(q); e != nil {
				return r, e
			}
		}
		a, c := new(big.Rat).SetFloat64(mins[axis]), new(big.Rat).SetFloat64(maxs[axis])
		if direction[axis].Sign() == 0 {
			if origin[axis].Cmp(a) < 0 || origin[axis].Cmp(c) > 0 {
				return r, ErrDomain
			}
			continue
		}
		a.Quo(new(big.Rat).Sub(a, origin[axis]), direction[axis])
		c.Quo(new(big.Rat).Sub(c, origin[axis]), direction[axis])
		if a.Cmp(c) > 0 {
			a, c = c, a
		}
		for _, q := range []*big.Rat{a, c} {
			if e := b.check(q); e != nil {
				return r, e
			}
		}
		if lo == nil || a.Cmp(lo) > 0 {
			lo = a
		}
		if hi == nil || c.Cmp(hi) < 0 {
			hi = c
		}
	}
	if lo == nil {
		return r, ErrDegenerate
	}
	if lo.Cmp(hi) >= 0 {
		return r, ErrDomain
	}
	width := new(big.Rat).Sub(hi, lo)
	net := make([]exactHomogeneous, 2)
	for i, t := range []*big.Rat{lo, hi} {
		for axis := 0; axis < 2; axis++ {
			net[i][axis] = new(big.Rat).Add(origin[axis], new(big.Rat).Mul(direction[axis], t))
			if e := b.check(net[i][axis]); e != nil {
				return r, e
			}
		}
		net[i][2] = new(big.Rat)
		net[i][3] = big.NewRat(1, 1)
	}
	uv, e := exactCurveFromHomogeneousBezier(net, TubeAffineLineOptions{MaxPoles: options.MaxPoles, MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits}, b)
	if e != nil {
		return r, e
	}
	inverse := new(big.Rat).Inv(width)
	offset := new(big.Rat).Neg(new(big.Rat).Mul(lo, inverse))
	l, e := affineMapRoot(lower.root, offset, inverse, b)
	if e != nil {
		return r, e
	}
	h, e := affineMapRoot(upper.root, offset, inverse, b)
	if e != nil {
		return r, e
	}
	child := o
	child.SurfacePatchOptions = options
	child.MaxOperations -= b.operations
	if child.MaxOperations < 1 {
		return r, ErrConvergence
	}
	composed, e := p.BuildSurfaceCurveCompositionCells(ctx, uv, child)
	b.operations += composed.ExactOperations
	r.Subdivisions = composed.Subdivisions
	if e != nil {
		return r, e
	}
	remainingOps, remainingSubs := options.MaxOperations-b.operations, o.MaxSubdivisions-r.Subdivisions
	if remainingOps < 1 || remainingSubs < 1 {
		return r, ErrConvergence
	}
	trimmed, e := composed.RestrictAlgebraic(ctx, &ExactAlgebraicRoot{root: l}, &ExactAlgebraicRoot{root: h}, ExactCurvePlaneEventOptions{MaxOperations: remainingOps, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: remainingSubs})
	b.operations += trimmed.ExactOperations
	r.Subdivisions += trimmed.Subdivisions
	if e != nil {
		return r, e
	}
	for _, cell := range trimmed.Cells {
		mapped := &ExactComposedCurveCell{cell: cell.cell}
		for axis := 0; axis < 4; axis++ {
			mapped.power[axis], e = composeProjectionPolynomial(cell.power[axis], rationalPolynomial{offset, inverse}, b)
			if e != nil {
				return r, e
			}
		}
		for i := 0; i < 2; i++ {
			mapped.endpoints[i], e = affineMapRoot(cell.endpoints[i], lo, width, b)
			if e != nil {
				return r, e
			}
		}
		r.Cells = append(r.Cells, mapped)
	}
	if len(r.Cells) > 0 {
		r.Events = append(r.Events, r.Cells[0].endpoints[0].Interval())
		for _, cell := range r.Cells {
			r.Events = append(r.Events, cell.endpoints[1].Interval())
		}
	}
	r.Complete = trimmed.Complete
	return r, nil
}
