package geom

import (
	"context"
	"math/big"
)

// RationalOffsetProjection encloses the dot product of an entire offset
// cell with a caller-supplied exact direction. Values retain the scale
// of that direction. Lower and Upper are caller-owned rationals.
type RationalOffsetProjection struct {
	Lower, Upper    *big.Rat
	ExactOperations int
}

// ProjectionBoundsRational bounds a whole offset cell in an exact
// modeled direction. Projecting the homogeneous source controls before
// bounding retains correlated coordinates that an axis-aligned box loses.
// The normal displacement is enclosed independently by directed arithmetic.
func (p *VerifiedNormalOffsetPatch) ProjectionBoundsRational(ctx context.Context, rectangle ParameterRectangle,
	direction [3]*big.Rat, o SurfacePatchOptions) (r RationalOffsetProjection, err error) {
	nonzero := false
	for _, q := range direction {
		if q == nil {
			return r, ErrInvalid
		}
		nonzero = nonzero || q.Sign() != 0
	}
	if !nonzero {
		return r, ErrInvalid
	}
	cell, budget, err := p.offsetBoundsCell(ctx, rectangle, o)
	if budget != nil {
		defer func() { r.ExactOperations = budget.operations }()
	}
	if err != nil {
		return r, err
	}
	for _, q := range direction {
		if err := budget.check(q); err != nil {
			return r, err
		}
	}
	scalars := boundedRatArithmetic{b: budget}
	var source exactInterval
	for _, h := range cell.net {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		value := scalars.add(scalars.add(scalars.mul(direction[0], h[0]), scalars.mul(direction[1], h[1])),
			scalars.mul(direction[2], h[2]))
		value = scalars.quo(value, h[3])
		if scalars.err != nil {
			return r, scalars.err
		}
		if source.lo == nil || value.Cmp(source.lo) < 0 {
			source.lo = value
		}
		if source.hi == nil || value.Cmp(source.hi) > 0 {
			source.hi = value
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
	var directionBounds [3]exactInterval
	for i, q := range direction {
		directionBounds[i] = exactInterval{q, q}
	}
	q := a.dot(n, directionBounds)
	result := a.add(source, a.div(a.mul(exactInterval{p.distance, p.distance}, q), length))
	if a.err != nil {
		return r, a.err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Lower, r.Upper = result.lo, result.hi
	return r, nil
}
