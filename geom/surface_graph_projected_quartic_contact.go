package geom

import (
	"context"
	"math/big"
)

// ProjectedCoupledQuarticContact retains original XYZ and both original UVs.
// Side*(A[ResidualAxis]-B[ResidualAxis]) is strictly positive away from the sole
// contact, over the complete common projected rectangle.
type ProjectedCoupledQuarticContact struct {
	CoupledQuarticGraphContact
	TargetParameters [2]*big.Rat
	ProjectionAxes   [2]int
	ResidualAxis     int
}

// DiscoverProjectedCoupledQuarticGraphPointContact supports independent affine
// projected rectangular charts, including signed axis permutations/scales. The
// exact aligned numerator is re-certified; nonrepresentable remapped knots and
// nonrectangular/sheared correspondences are explicitly unsupported.
func DiscoverProjectedCoupledQuarticGraphPointContact(ctx context.Context, a, c *SurfacePatch, axes [2]int, o GraphPointContactOptions) (r ProjectedCoupledQuarticContact, err error) {
	if ctx == nil || a == nil || c == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	order, e := modelProjectionPermutation(axes)
	if e != nil {
		return r, e
	}
	r.ProjectionAxes = axes
	r.ResidualAxis = order[2]
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	pa, e := permuteGraphPatch(a, order, budget)
	if e != nil {
		return r, e
	}
	pc, e := permuteGraphPatch(c, order, budget)
	if e != nil {
		return r, e
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	alignment, e := AlignAffineXYCharts(ctx, pa, pc, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	budget.operations += alignment.operations
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return r, ErrConvergence
	}
	r.CoupledQuarticGraphContact, e = DiscoverCoupledQuarticGraphPointContact(ctx, pa, alignment.target, o)
	budget.operations += r.ExactOperations
	if e != nil {
		return r, e
	}
	var point [3]*big.Rat
	for i, j := range order {
		point[j] = r.Point[i]
	}
	r.Point = point
	r.Graph.ProjectionAxes = axes
	for axis := 0; axis < 2; axis++ {
		value := new(big.Rat).Set(alignment.mapping[axis][2])
		for j, q := range r.Parameters {
			value.Add(value, new(big.Rat).Mul(alignment.mapping[axis][j], q))
		}
		if e := budget.check(value); e != nil {
			r.Proven = false
			return r, e
		}
		r.TargetParameters[axis] = value
	}
	return r, nil
}
