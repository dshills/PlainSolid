package geom

import (
	"context"
	"math/big"
)

type WrappedCurveModelComparison struct {
	Proven, Equivalent, Reversed bool
	Pieces, ExactOperations      int
}

// CompareModelBounded overlays constructor-owned complete partitions with a
// single arithmetic/subdivision budget. maxPieces (default256, max8192) bounds
// the common interval count. Immutable continuity/domain proofs are retained;
// no caller-supplied completeness or subcollection is trusted. False equality
// concerns the supplied parameter direction, not unparameterized images.
func (a *WrappedCurveCells) CompareModelBounded(ctx context.Context, b *WrappedCurveCells, reverseB bool, maxPieces int, o ExactCurvePlaneEventOptions) (WrappedCurveModelComparison, error) {
	var zero WrappedCurveModelComparison
	if ctx == nil || a == nil || b == nil || len(a.pieces) == 0 || len(b.pieces) == 0 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if maxPieces == 0 {
		maxPieces = 256
	}
	if maxPieces < 1 || maxPieces > 8192 {
		return zero, ErrInvalid
	}
	if len(a.pieces) > maxPieces || len(b.pieces) > maxPieces {
		return zero, ErrConvergence
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return zero, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, collection := range []*WrappedCurveCells{a, b} {
		for _, piece := range collection.pieces {
			for _, polynomial := range piece.Model.power {
				if len(polynomial) > 65 {
					return zero, ErrConvergence
				}
				for _, coefficient := range polynomial {
					if e := budget.check(coefficient); e != nil {
						return zero, e
					}
				}
			}
		}
	}
	steps := 0
	right := make([]*ExactComposedCurveCell, len(b.pieces))
	for i := range right {
		index := i
		if reverseB {
			index = len(right) - 1 - i
		}
		source := b.pieces[index].Model
		if !reverseB {
			right[i] = source
			continue
		}
		c := &ExactComposedCurveCell{}
		for axis := 0; axis < 4; axis++ {
			c.power[axis], e = composeProjectionPolynomial(source.power[axis], rationalPolynomial{big.NewRat(1, 1), big.NewRat(-1, 1)}, budget)
			if e != nil {
				return zero, e
			}
		}
		for side := 0; side < 2; side++ {
			if budget.operations >= opts.MaxOperations {
				return zero, ErrConvergence
			}
			root := &ExactAlgebraicRoot{root: source.endpoints[1-side]}
			mapped, n, e := root.AffineMap(ctx, big.NewRat(1, 1), big.NewRat(-1, 1), ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations - budget.operations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: opts.MaxSubdivisions})
			budget.operations += n
			if e != nil {
				return zero, e
			}
			c.endpoints[side] = mapped.root
		}
		right[i] = c
	}
	result := WrappedCurveModelComparison{Equivalent: true, Reversed: reverseB}
	i, j := 0, 0
	for i < len(a.pieces) && j < len(right) {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		result.Pieces++
		if result.Pieces > maxPieces {
			return zero, ErrConvergence
		}
		left, r := a.pieces[i].Model, right[j]
		// Every overlap has positive width: both original partitions cover [0,1]
		// without gaps, and the smaller preceding endpoint was advanced below.
		for axis := 0; axis < 3; axis++ {
			x, e := polynomialProduct(left.power[axis], r.power[3], budget)
			if e != nil {
				return zero, e
			}
			y, e := polynomialProduct(r.power[axis], left.power[3], budget)
			if e != nil {
				return zero, e
			}
			delta, e := subtractScaledPolynomial(x, y, big.NewRat(1, 1), budget)
			if e != nil {
				return zero, e
			}
			if len(delta) != 0 {
				result.Equivalent = false
			}
		}
		if steps >= opts.MaxSubdivisions {
			return zero, ErrConvergence
		}
		order, e := compareRootBudget(ctx, left.endpoints[1], r.endpoints[1], budget, opts.MaxSubdivisions-steps)
		steps += order.Subdivisions
		if e != nil {
			return zero, e
		}
		if !order.Proven {
			return zero, ErrConvergence
		}
		if order.Order <= 0 {
			i++
		}
		if order.Order >= 0 {
			j++
		}
	}
	if i != len(a.pieces) || j != len(right) {
		return zero, ErrDomain
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	result.Proven = true
	result.ExactOperations = budget.operations
	return result, nil
}
