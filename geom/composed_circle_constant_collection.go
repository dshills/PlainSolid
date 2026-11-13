package geom

import (
	"context"
	"math/big"
)

// ConstantCirclePiece is an entire source parameter interval mapping to the
// exact rational Point on the circle. Cell retains source ownership; this is
// not an isolated parameter event and carries no angular orientation. Point
// coordinates are caller-owned copies. Moving Pieces and ConstantPieces in a
// component are ordered jointly by original parameter (and source Cell).
type ConstantCirclePiece struct {
	Cell       int
	Start, End *ExactAlgebraicRoot
	Point      [3]*big.Rat
}

// ClassifyAllPlanarCircleComponents additionally admits constant circle-point
// spans, recorded in each component's ConstantPieces. Adjacent constant cells
// belong to the same coincident component after exact model seam verification.
// Finite stationary germs of moving neighbors remain in Stationary, with only
// their available side signs; no crossing through a constant span is inferred.
// No infinite event list is constructed. Existing strict and stationary-only
// methods continue rejecting constant continuations. Proof budgets and atomic
// result semantics are identical to ClassifyStationaryPlanarCircleComponents.
func (c SurfaceCurveCompositionCells) ClassifyAllPlanarCircleComponents(ctx context.Context, axes [2]int, center [3]*big.Rat, radiusSquared *big.Rat, o ComposedPlaneContactsOptions) (StationaryCircleContactComponents, error) {
	return c.circleContactComponents(ctx, axes, center, radiusSquared, o, true, true)
}

func (c *ExactComposedCurveCell) constantCirclePoint(ctx context.Context, o ExactCurvePlaneEventOptions) (point [3]*big.Rat, constant bool, err error) {
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	index := -1
	for i, w := range c.power[3] {
		if w.Sign() != 0 {
			index = i
			break
		}
	}
	if index < 0 {
		return point, false, ErrInvalid
	}
	for axis := 0; axis < 3; axis++ {
		if e := ctx.Err(); e != nil {
			return point, false, e
		}
		p := new(big.Rat)
		if index < len(c.power[axis]) {
			p.Quo(c.power[axis][index], c.power[3][index])
		}
		if e := b.check(p); e != nil {
			return point, false, e
		}
		residual, e := subtractScaledPolynomial(c.power[axis], c.power[3], p, b)
		if e != nil {
			return point, false, e
		}
		if len(residual) != 0 {
			return point, false, nil
		}
		point[axis] = p
	}
	return point, true, nil
}
