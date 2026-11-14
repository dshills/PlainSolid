package geom

import (
	"context"
	"math/big"
)

// WrappedSurfaceChartRelation binds two immutable wrapped supports proved equal
// under one exact affine chart map. It preserves the lifted parameter (including
// complete periods), not just a reduced or rounded representative. This proves
// quotient-chart correspondence, not surface injectivity or shell embedding.
type WrappedSurfaceChartRelation struct {
	source        *WrappedSurfacePatch
	target        *WrappedSurfacePatch
	axis          [2]int
	offset, scale [2]*big.Rat // indexed by source axis, emitted on target axis
	orientation   int
}

// CompareWrappedSurfaceCharts checks wrap-axis compatibility, then proves full
// rational surface identity using the existing bounded coefficient comparison.
// A complete unequal comparison returns nil relation without error. Any error
// returns neither a relation nor a completed proof. Independent knot partitions,
// weights and degrees are permitted; stored recipe identity is not required.
func CompareWrappedSurfaceCharts(ctx context.Context, a, b *WrappedSurfacePatch, o ExactSurfaceComparisonOptions) (*WrappedSurfaceChartRelation, ExactSurfaceComparison, error) {
	var zero ExactSurfaceComparison
	if ctx == nil || a == nil || b == nil || a.patch == nil || b.patch == nil {
		return nil, zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, zero, e
	}
	axis := [2]int{0, 1}
	if o.SwapB {
		axis = [2]int{1, 0}
	}
	for i, j := range axis {
		if (a.period[i] != nil) != (b.period[j] != nil) {
			return nil, zero, ErrDomain
		}
	}
	proof, e := CompareExactSurfaces(ctx, a.patch, b.patch, o)
	if e != nil {
		return nil, zero, e
	}
	if !proof.Complete || !proof.Equal {
		return nil, proof, nil
	}
	relation := &WrappedSurfaceChartRelation{source: a, target: b, axis: axis, orientation: 1}
	if o.SwapB {
		relation.orientation = -1
	}
	ad, bd := a.patch.Domain(), b.patch.Domain()
	ar := [2][2]float64{{ad.UMin, ad.UMax}, {ad.VMin, ad.VMax}}
	br := [2][2]float64{{bd.UMin, bd.UMax}, {bd.VMin, bd.VMax}}
	for i, j := range axis {
		amin, amax := new(big.Rat).SetFloat64(ar[i][0]), new(big.Rat).SetFloat64(ar[i][1])
		bmin, bmax := new(big.Rat).SetFloat64(br[j][0]), new(big.Rat).SetFloat64(br[j][1])
		scale := new(big.Rat).Quo(new(big.Rat).Sub(bmax, bmin), new(big.Rat).Sub(amax, amin))
		base := bmin
		if []bool{o.ReverseBU, o.ReverseBV}[i] {
			scale.Neg(scale)
			base = bmax
			relation.orientation = -relation.orientation
		}
		relation.scale[i] = scale
		relation.offset[i] = new(big.Rat).Sub(base, new(big.Rat).Mul(scale, amin))
	}
	// Bound the derived exact map too, including ratios of represented lengths.
	maxOps, maxBits := o.MaxOperations, o.MaxCoefficientBits
	if maxOps == 0 {
		maxOps = 8000000
	}
	if maxBits == 0 {
		maxBits = 16384
	}
	budget := &polynomialBudget{maxOperations: maxOps, maxBits: maxBits, operations: proof.ExactOperations}
	for i := 0; i < 2; i++ {
		for _, q := range []*big.Rat{relation.scale[i], relation.offset[i]} {
			if e = budget.check(q); e != nil {
				return nil, zero, e
			}
		}
	}
	proof.ExactOperations = budget.operations
	if e = ctx.Err(); e != nil {
		return nil, zero, e
	}
	return relation, proof, nil
}

// ChartOrientation is the sign of the lifted UV map determinant. At regular
// points it is the relative chart normal sign, not a regularity certificate.
func (r *WrappedSurfaceChartRelation) ChartOrientation() int {
	if r == nil {
		return 0
	}
	return r.orientation
}

// MapRational maps an exact source parameter without reducing wrapped axes.
// Unwrapped source coordinates must lie in the original closed domain. Returned
// values are independent copies; mutation cannot change the retained relation.
func (r *WrappedSurfaceChartRelation) MapRational(ctx context.Context, uv [2]*big.Rat, o SurfacePatchOptions) ([2]*big.Rat, error) {
	var zero [2]*big.Rat
	if ctx == nil || r == nil || r.source == nil || r.target == nil || uv[0] == nil || uv[1] == nil {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	limits, e := patchOptions(o)
	if e != nil {
		return zero, e
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	for _, q := range uv {
		if e = budget.check(q); e != nil {
			return zero, e
		}
	}
	d := r.source.patch.Domain()
	bounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	var out [2]*big.Rat
	for i, j := range r.axis {
		if r.source.period[i] == nil && (uv[i].Cmp(new(big.Rat).SetFloat64(bounds[i][0])) < 0 || uv[i].Cmp(new(big.Rat).SetFloat64(bounds[i][1])) > 0) {
			return zero, ErrDomain
		}
		v := new(big.Rat).Add(r.offset[i], new(big.Rat).Mul(r.scale[i], uv[i]))
		if e = budget.check(v); e != nil {
			return zero, e
		}
		out[j] = v
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
