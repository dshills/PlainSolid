package geom

import (
	"context"
	"math/big"
)

// NormalBoundsRational encloses the continuous outward unit normal of this
// analytic face in its original source rectangle and displacement interval.
// A wall requires a positive-area UV collar incident to its fixed boundary.
// A layer requires equal displacements at its own exact boundary layer.
// An interval that can contain a zero normal returns ErrConvergence.
func (f VerifiedNormalOffsetSweepFace) NormalBoundsRational(ctx context.Context, rectangle ParameterRectangle,
	lower, upper *big.Rat, o SurfacePatchOptions) (r RationalSurfaceOffsetPoint, err error) {
	if f.sweep == nil || f.sweep.source == nil || f.side > NormalOffsetSweepUpper ||
		ctx == nil || lower == nil || upper == nil || lower.Cmp(upper) > 0 || !rectangle.Valid() {
		return r, ErrInvalid
	}
	s := f.sweep
	if lower.Cmp(s.lower) < 0 || upper.Cmp(s.upper) > 0 {
		return r, ErrDomain
	}
	d := s.source.Domain()
	if f.side == NormalOffsetSweepUMin && rectangle.UMin != d.UMin ||
		f.side == NormalOffsetSweepUMax && rectangle.UMax != d.UMax ||
		f.side == NormalOffsetSweepVMin && rectangle.VMin != d.VMin ||
		f.side == NormalOffsetSweepVMax && rectangle.VMax != d.VMax ||
		f.side == NormalOffsetSweepLower && upper.Cmp(s.lower) != 0 ||
		f.side == NormalOffsetSweepUpper && lower.Cmp(s.upper) != 0 {
		return r, ErrDomain
	}
	query := VerifiedNormalOffsetPatch{source: s.source, distance: lower}
	cell, budget, err := query.offsetBoundsCell(ctx, rectangle, o)
	if budget != nil {
		defer func() { r.ExactOperations = budget.operations }()
	}
	if err != nil {
		return r, err
	}
	if err := budget.check(upper); err != nil {
		return r, err
	}
	jet, err := offsetCartesianRanges(ctx, cell, budget)
	if err != nil {
		return r, err
	}
	a := offsetIntervalArithmetic{b: budget}
	n := a.cross(jet[1][0], jet[0][1])
	reverse := f.side == NormalOffsetSweepLower
	if f.side < NormalOffsetSweepLower {
		n, err = f.wallNormalRange(&a, jet, n, exactInterval{lower, upper})
		if err != nil {
			return r, err
		}
		reverse = f.ReverseWinding()
	}
	// The displacement column of the swept map is the original normal,
	// so the upper and lower layers point in its positive and negative
	// directions respectively, even when chart orientation is reversed.
	length, err := scaledOffsetNormalLength(&n, &a)
	if err != nil {
		return r, err
	}
	zero, one := offsetZeroInterval(), big.NewRat(1, 1)
	minusOne := new(big.Rat).Neg(one)
	var lo, hi [3]*big.Rat
	for axis := range n {
		unit := a.div(n[axis], length)
		if reverse {
			unit = a.sub(zero, unit)
		}
		if unit.lo.Cmp(minusOne) < 0 {
			unit.lo = new(big.Rat).Set(minusOne)
		}
		if unit.hi.Cmp(one) > 0 {
			unit.hi = new(big.Rat).Set(one)
		}
		lo[axis], hi[axis] = unit.lo, unit.hi
	}
	if a.err != nil {
		return r, a.err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Lower, r.Upper = lo, hi
	r.Bounds = Box{Min: rationalSurfaceVectorBounds(lo).Min, Max: rationalSurfaceVectorBounds(hi).Max}
	return r, nil
}

func (f VerifiedNormalOffsetSweepFace) wallNormalRange(a *offsetIntervalArithmetic, jet [3][3][3]exactInterval,
	n [3]exactInterval, displacement exactInterval) (wall [3]exactInterval, err error) {
	su, sv := jet[1][0], jet[0][1]
	free, du, dv := su, jet[2][0], jet[1][1]
	if f.side <= NormalOffsetSweepUMax {
		free, du, dv = sv, jet[1][1], jet[0][2]
	}
	first, second := a.cross(du, sv), a.cross(su, dv)
	scale := new(big.Rat)
	for _, interval := range n {
		for _, q := range []*big.Rat{interval.lo, interval.hi} {
			if size := new(big.Rat).Abs(q); size.Cmp(scale) > 0 {
				scale = size
			}
		}
	}
	length, err := scaledOffsetNormalLength(&n, a)
	if err != nil {
		return wall, err
	}
	// The derivative of the source cross product uses the same fixed
	// positive scale. Its component parallel to the normal drops out
	// of the wall cross product.
	factor := exactInterval{scale, scale}
	var derivative [3]exactInterval
	for axis := range derivative {
		derivative[axis] = a.div(a.add(first[axis], second[axis]), factor)
	}
	squareLength := a.norm2(n)
	initial, change := a.cross(free, n), a.cross(derivative, n)
	for axis := range wall {
		wall[axis] = a.add(a.div(initial[axis], length), a.div(a.mul(displacement, change[axis]), squareLength))
	}
	return wall, a.err
}
