package geom

import (
	"context"
	"math/big"
)

// NormalBoundsRational encloses the continuous oriented unit normal of
// the verified offset over a closed parameter rectangle. The original
// and displaced tangents span the same tangent plane. The verified
// nonsingular displacement from zero preserves its orientation, so the
// offset uses the original unit normal. A loose source normal enclosure
// may require a smaller cell and returns ErrConvergence.
func (p *VerifiedNormalOffsetPatch) NormalBoundsRational(ctx context.Context, rectangle ParameterRectangle,
	o SurfacePatchOptions) (r RationalSurfaceOffsetPoint, err error) {
	cell, budget, err := p.offsetBoundsCell(ctx, rectangle, o)
	if budget != nil {
		defer func() { r.ExactOperations = budget.operations }()
	}
	if err != nil {
		return r, err
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
	var lower, upper [3]*big.Rat
	lo, hi := big.NewRat(-1, 1), big.NewRat(1, 1)
	for i := range n {
		box := a.div(n[i], length)
		lower[i], upper[i] = box.lo, box.hi
		if lower[i].Cmp(lo) < 0 {
			lower[i] = new(big.Rat).Set(lo)
		}
		if upper[i].Cmp(hi) > 0 {
			upper[i] = new(big.Rat).Set(hi)
		}
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
