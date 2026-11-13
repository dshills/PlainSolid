package geom

import (
	"context"
	"math/big"
)

// Exact all-coordinate matching is the only exception to strict dependent
// containment. Equality of one wall coordinate or model display is insufficient.
func (t VerifiedSurfaceTube) matchesAnchorRoots(ctx context.Context, roots [4]*ExactAlgebraicRoot, b *polynomialBudget, maxSteps int) (bool, error) {
	if t.anchor == nil {
		return false, nil
	}
	for i, root := range roots {
		if root == nil || root.root == nil {
			return false, nil
		}
		q := t.anchor.parameters[i]
		target := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(q), big.NewRat(1, 1)}, bounds: [2]*big.Rat{q, q}}
		order, e := compareRootBudget(ctx, root.root, target, b, maxSteps)
		if e != nil {
			return false, e
		}
		if !order.Proven {
			return false, ErrConvergence
		}
		if order.Order != 0 {
			return false, nil
		}
	}
	return true, nil
}
