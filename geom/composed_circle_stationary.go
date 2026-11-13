package geom

import (
	"context"
	"math/big"
)

// CircleStationaryEvent is a zero of angular velocity on a curve proved wholly
// coincident with a circle. It is never an isolated circle intersection. Signs
// describe increasing original parameter; zero means an unavailable domain side.
type CircleStationaryEvent struct {
	Event            *ExactComposedCurveEvent
	Order            int
	SideOrientations [2]int
	Reverses         bool
}

type CoincidentCircleOrientation struct {
	Complete   bool
	Domain     [2]*ExactAlgebraicRoot
	Pieces     []CoincidentCirclePiece
	Stationary []CircleStationaryEvent
	Operations int
}

// SplitCoincidentCircleOrientation partitions a wholly circle-coincident cell
// at all isolated angular stationary parameters. Each open piece has a proved
// strict orientation. Constant curves remain unresolved; noncoincident curves
// return ErrDomain. Homogeneous degree <=4 and exact algebraic domain cuts are
// supported. Arithmetic work is cumulative; subdivision limits apply per proof.
// Error publishes only Operations, never a partial orientation partition.
func (c *ExactComposedCurveCell) SplitCoincidentCircleOrientation(ctx context.Context, axes [2]int, center [3]*big.Rat, radiusSquared *big.Rat, o ExactCurvePlaneEventOptions) (out CoincidentCircleOrientation, err error) {
	part, e := c.ClassifyPlanarCircle(ctx, axes, center, radiusSquared, o)
	out.Operations = part.Operations
	if e != nil {
		return out, e
	}
	if !part.Complete || !part.Coincident {
		return out, ErrDomain
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits, operations: part.Operations}
	defer func() { out.Operations = b.operations }()
	x, e := subtractScaledPolynomial(c.power[axes[0]], c.power[3], center[axes[0]], b)
	if e != nil {
		return out, e
	}
	y, e := subtractScaledPolynomial(c.power[axes[1]], c.power[3], center[axes[1]], b)
	if e != nil {
		return out, e
	}
	xy, e := polynomialProduct(x, polynomialDerivative(y), b)
	if e != nil {
		return out, e
	}
	yx, e := polynomialProduct(y, polynomialDerivative(x), b)
	if e != nil {
		return out, e
	}
	angular, e := subtractScaledPolynomial(xy, yx, big.NewRat(1, 1), b)
	if e != nil {
		return out, e
	}
	if len(angular) == 0 {
		return out, ErrConvergence
	}
	roots, e := c.intersectComposedPolynomial(ctx, angular, opts, b)
	if e != nil {
		return out, e
	}
	if !roots.Complete || roots.Coincident {
		return out, ErrConvergence
	}
	signAt := func(root *ExactGraphLineRoot, p rationalPolynomial) (int, error) {
		s, _, e := composedPolynomialRootSign(ctx, root, p, b, opts.MaxSubdivisions)
		return s, e
	}
	stationary := make([]CircleStationaryEvent, 0, len(roots.Events))
	for _, event := range roots.Events {
		p := angular
		order := 0
		lead := 0
		for len(p) > 1 {
			order++
			p = polynomialDerivative(p)
			lead, e = signAt(event.root, p)
			if e != nil {
				return out, e
			}
			if lead != 0 {
				break
			}
		}
		if lead == 0 {
			return out, ErrConvergence
		}
		before := lead
		if order%2 != 0 {
			before = -before
		}
		sides := [2]int{before, lead}
		if event.endpointMask&1 != 0 {
			sides[0] = 0
		}
		if event.endpointMask&2 != 0 {
			sides[1] = 0
		}
		stationary = append(stationary, CircleStationaryEvent{Event: event, Order: order, SideOrientations: sides, Reverses: sides[0] != 0 && sides[1] != 0 && sides[0] != sides[1]})
	}
	sign, e := signAt(c.endpoints[0], angular)
	if e != nil {
		return out, e
	}
	lower := part.Domain[0]
	atEnd := false
	pieces := make([]CoincidentCirclePiece, 0, len(stationary)+1)
	for _, event := range stationary {
		if event.Event.endpointMask&1 == 0 {
			if sign == 0 || sign != event.SideOrientations[0] {
				return out, ErrConvergence
			}
			pieces = append(pieces, CoincidentCirclePiece{Start: lower, End: event.Event.Root(), Orientation: sign})
		}
		lower = event.Event.Root()
		sign = event.SideOrientations[1]
		atEnd = event.Event.endpointMask&2 != 0
	}
	if !atEnd {
		if sign == 0 {
			return out, ErrConvergence
		}
		pieces = append(pieces, CoincidentCirclePiece{Start: lower, End: part.Domain[1], Orientation: sign})
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Complete = true
	out.Domain = part.Domain
	out.Pieces = pieces
	out.Stationary = stationary
	return out, nil
}
