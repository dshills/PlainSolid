package geom

import (
	"context"
	"math/big"
)

// AffineProjectedGraphOrientation proves an invertible affine projection onto
// axes[0],axes[1] and returns the exact sign of its original UV Jacobian. The
// remaining model axis and permutation parity are not folded into this sign.
// Unsupported non-affine or singular projections return an error, never zero
// interpreted as orientation. Arithmetic includes permutation/bounds proof.
func AffineProjectedGraphOrientation(ctx context.Context, p *SurfacePatch, axes [2]int, o SurfacePatchOptions) (sign, operations int, err error) {
	if ctx == nil || p == nil {
		return 0, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return 0, 0, e
	}
	order, e := modelProjectionPermutation(axes)
	if e != nil {
		return 0, 0, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return 0, 0, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { operations = budget.operations }()
	projected, e := permuteGraphPatch(p, order, budget)
	if e != nil {
		return 0, 0, e
	}
	_, inverse, e := normalizeAffineXYGraph(ctx, projected, budget)
	if e != nil {
		return 0, 0, e
	}
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(inverse[0][0], inverse[1][1]), new(big.Rat).Mul(inverse[0][1], inverse[1][0]))
	if e := budget.check(determinant); e != nil {
		return 0, 0, e
	}
	return determinant.Sign(), 0, nil
}
