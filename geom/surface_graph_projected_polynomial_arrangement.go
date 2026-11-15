package geom

import (
	"context"
	"math/big"
)

// ProjectedSplineGraphPolynomialArrangement records the complete contact
// arrangement in a third reference chart. Maps expresses each original
// roof UV as a function of reference U,V and a constant. Owners preserves
// their original UV restrictions in input order.
type ProjectedSplineGraphPolynomialArrangement struct {
	Arrangement     SplineGraphPolynomialArrangement
	Maps            [2][2][3]*big.Rat
	Owners          [2]SplineGraphPolynomialOwnerLedger
	ExactOperations int
}

// DiscoverProjectedSplineGraphPolynomialArrangement proves the original
// contact arrangement using a separate affine XY chart for station order.
// The reference and both roofs must cover the same projected domain. Each
// original chart must relate to the reference by a signed axis permutation
// and scaling with exactly representable remapped knots. Only the reference
// chart's XY projection participates in the proof. Its height is independent.
func DiscoverProjectedSplineGraphPolynomialArrangement(ctx context.Context, reference, a, b *SurfacePatch, axis int,
	o GraphPointContactOptions) (r ProjectedSplineGraphPolynomialArrangement, err error) {
	if ctx == nil || reference == nil || a == nil || b == nil || len(reference.net) == 0 || len(a.net) == 0 ||
		len(b.net) == 0 || axis < 0 || axis > 1 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := alignedPolynomialProofOptions(o)
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = ProjectedSplineGraphPolynomialArrangement{}
		}
		r.ExactOperations = budget.operations
	}()
	original := [2]*SurfacePatch{a, b}
	var onReference [2]*SurfacePatch
	for i, owner := range original {
		remaining := o.MaxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		alignment, work, e := AlignAffineXYChartsWithWork(ctx, reference, owner,
			SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += work
		if e != nil {
			return r, e
		}
		onReference[i] = alignment.TargetOnSource()
		r.Maps[i], e = alignment.MapCoefficients()
		if e != nil {
			return r, e
		}
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	r.Arrangement, e = DiscoverSplineGraphPolynomialArrangement(ctx, onReference[0], onReference[1], axis, o)
	budget.operations += r.Arrangement.ExactOperations
	if e != nil {
		return r, e
	}
	for i, owner := range original {
		r.Owners[i], e = polynomialArrangementOwner(ctx, owner, r.Arrangement, r.Maps[i], true, budget)
		if e != nil {
			return r, e
		}
	}
	return r, ctx.Err()
}
