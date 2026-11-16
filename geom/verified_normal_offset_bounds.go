package geom

import (
	"context"
	"math/big"
)

// BoundsRational encloses the whole verified offset over a closed,
// positive-area parameter rectangle. It restricts the exact source net,
// encloses the continuous original normal, and adds the signed displacement
// with directed rational arithmetic. A cell whose normal enclosure includes
// degeneracy needs subdivision; it returns ErrConvergence.
func (p *VerifiedNormalOffsetPatch) BoundsRational(ctx context.Context, rectangle ParameterRectangle,
	o SurfacePatchOptions) (r RationalSurfaceOffsetPoint, err error) {
	if ctx == nil || p == nil || p.source == nil || p.distance == nil || !rectangle.Valid() {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	domain := p.source.Domain()
	if rectangle.UMin < domain.UMin || rectangle.UMax > domain.UMax ||
		rectangle.VMin < domain.VMin || rectangle.VMax > domain.VMax {
		return r, ErrDomain
	}
	o, err = patchOptions(o)
	if err != nil {
		return r, err
	}
	if len(p.source.net) > o.MaxPoles {
		return r, ErrConvergence
	}
	source := *p.source
	source.options.MaxPoles = min(source.options.MaxPoles, o.MaxPoles)
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	if err := budget.check(p.distance); err != nil {
		return r, err
	}
	cell := &source
	for _, side := range []struct {
		parameter, original float64
		u, low              bool
	}{{rectangle.UMin, domain.UMin, true, true}, {rectangle.UMax, domain.UMax, true, false},
		{rectangle.VMin, domain.VMin, false, true}, {rectangle.VMax, domain.VMax, false, false}} {
		if side.parameter == side.original {
			continue
		}
		if err := ctx.Err(); err != nil {
			return r, err
		}
		l, h, err := cell.split(side.parameter, side.u, budget)
		if err != nil {
			return r, err
		}
		cell = l
		if side.low {
			cell = h
		}
	}
	jet, err := offsetCartesianRanges(ctx, cell, budget)
	if err != nil {
		return r, err
	}
	a := offsetIntervalArithmetic{b: budget}
	n := a.cross(jet[1][0], jet[0][1])
	length, err := scaledOffsetNormalLength(&n, &a)
	if err != nil {
		return r, err
	}
	d := exactInterval{p.distance, p.distance}
	var lower, upper [3]*big.Rat
	for i := range n {
		x := a.add(jet[0][0][i], a.div(a.mul(d, n[i]), length))
		lower[i], upper[i] = x.lo, x.hi
	}
	if a.err != nil {
		return r, a.err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Lower, r.Upper = lower, upper
	r.Bounds = Box{Min: rationalSurfaceVectorBounds(lower).Min, Max: rationalSurfaceVectorBounds(upper).Max}
	return r, nil
}

// scaledOffsetNormalLength divides the interval normals by one positive
// constant and encloses their scaled length. The unit normal is unchanged.
func scaledOffsetNormalLength(n *[3]exactInterval, a *offsetIntervalArithmetic) (exactInterval, error) {
	scale := new(big.Rat)
	for _, x := range n {
		for _, q := range []*big.Rat{x.lo, x.hi} {
			if magnitude := new(big.Rat).Abs(q); magnitude.Cmp(scale) > 0 {
				scale = magnitude
			}
		}
	}
	if a.err != nil {
		return exactInterval{}, a.err
	}
	if scale.Sign() == 0 {
		return exactInterval{}, ErrConvergence
	}
	s := exactInterval{scale, scale}
	for i := range n {
		n[i] = a.div(n[i], s)
	}
	length2 := a.norm2(*n)
	if a.err != nil {
		return exactInterval{}, a.err
	}
	if length2.lo.Sign() <= 0 {
		return exactInterval{}, ErrConvergence
	}
	lo, _, err := rationalSqrtBounds(length2.lo, 96, a.b)
	if err != nil {
		return exactInterval{}, err
	}
	_, hi, err := rationalSqrtBounds(length2.hi, 96, a.b)
	if err != nil {
		return exactInterval{}, err
	}
	if lo.Sign() <= 0 {
		return exactInterval{}, ErrConvergence
	}
	return boundedExactInterval(lo, hi, a.b)
}
