package geom

import (
	"context"
	"math/big"
)

// PlanarCircleContact contains an immutable original curve event. SideSigns are
// -1 inside and +1 outside the disk immediately before/after the parameter;
// zero denotes an unavailable side at an actual cell endpoint. Order is the
// residual's vanishing order, not a claim that the curve has regular velocity.
type PlanarCircleContact struct {
	Event     *ExactComposedCurveEvent
	Order     int
	SideSigns [2]int
}

// PlanarCircleInterval is an open parameter interval with a strict disk sign.
// Its endpoints are retained independently even if their model points coincide.
type PlanarCircleInterval struct {
	Start, End *ExactAlgebraicRoot
	Sign       int
}

// PlanarCirclePartition exhausts the actual cell domain when Complete. Coincident
// means the entire domain lies on the circle; no artificial isolated events or
// strict intervals are returned. Error results publish only Operations.
type PlanarCirclePartition struct {
	Complete, Coincident bool
	Domain               [2]*ExactAlgebraicRoot
	Contacts             []PlanarCircleContact
	Intervals            []PlanarCircleInterval
	Operations           int
}

// ClassifyPlanarCircle proves circle incidence and inside/outside ordering for a
// rational composed curve of homogeneous degree <=4. The circle is specified by
// its exact XYZ center, positive physical radius squared, and two distinct
// ordered coordinate axes. The omitted model coordinate must equal the center's
// identically. No center/radius conversion to floating point is performed.
// MaxOperations is cumulative; MaxSubdivisions bounds each isolation/sign proof.
func (c *ExactComposedCurveCell) ClassifyPlanarCircle(ctx context.Context, axes [2]int, center [3]*big.Rat, radiusSquared *big.Rat, o ExactCurvePlaneEventOptions) (out PlanarCirclePartition, err error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 || radiusSquared == nil || radiusSquared.Sign() <= 0 || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() { out.Operations = b.operations }()
	if e = b.check(radiusSquared); e != nil {
		return out, e
	}
	for _, x := range center {
		if x == nil {
			return out, ErrInvalid
		}
		if e = b.check(x); e != nil {
			return out, e
		}
	}
	for _, p := range c.power {
		if len(p) > 5 {
			return out, ErrConvergence
		}
		for _, x := range p {
			if e = b.check(x); e != nil {
				return out, e
			}
		}
	}
	omitted := 3 - axes[0] - axes[1]
	plane, e := subtractScaledPolynomial(c.power[omitted], c.power[3], center[omitted], b)
	if e != nil {
		return out, e
	}
	if len(plane) != 0 {
		return out, ErrDomain
	}
	var residual rationalPolynomial
	for _, axis := range axes {
		p, e := subtractScaledPolynomial(c.power[axis], c.power[3], center[axis], b)
		if e != nil {
			return out, e
		}
		square, e := polynomialProduct(p, p, b)
		if e != nil {
			return out, e
		}
		residual, e = subtractScaledPolynomial(residual, square, big.NewRat(-1, 1), b)
		if e != nil {
			return out, e
		}
	}
	w2, e := polynomialProduct(c.power[3], c.power[3], b)
	if e != nil {
		return out, e
	}
	residual, e = subtractScaledPolynomial(residual, w2, radiusSquared, b)
	if e != nil {
		return out, e
	}
	domain := [2]*ExactAlgebraicRoot{{root: c.endpoints[0]}, {root: c.endpoints[1]}}
	if len(residual) == 0 {
		out.Complete = true
		out.Coincident = true
		out.Domain = domain
		return out, nil
	}
	isolated, e := c.intersectComposedPolynomial(ctx, residual, opts, b)
	if e != nil {
		return out, e
	}
	if !isolated.Complete || isolated.Coincident {
		return out, ErrConvergence
	}
	signAt := func(root *ExactGraphLineRoot, p rationalPolynomial) (int, error) {
		s, _, e := composedPolynomialRootSign(ctx, root, p, b, opts.MaxSubdivisions)
		return s, e
	}
	contacts := make([]PlanarCircleContact, 0, len(isolated.Events))
	for _, event := range isolated.Events {
		p := residual
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
		contacts = append(contacts, PlanarCircleContact{Event: event, Order: order, SideSigns: sides})
	}
	sign, e := signAt(c.endpoints[0], residual)
	if e != nil {
		return out, e
	}
	lower := domain[0]
	intervals := make([]PlanarCircleInterval, 0, len(contacts)+1)
	atEnd := false
	for _, contact := range contacts {
		if contact.Event.endpointMask&1 == 0 {
			if sign == 0 || sign != contact.SideSigns[0] {
				return out, ErrConvergence
			}
			intervals = append(intervals, PlanarCircleInterval{Start: lower, End: contact.Event.Root(), Sign: sign})
		}
		lower = contact.Event.Root()
		sign = contact.SideSigns[1]
		atEnd = contact.Event.endpointMask&2 != 0
	}
	if !atEnd {
		if sign == 0 {
			return out, ErrConvergence
		}
		intervals = append(intervals, PlanarCircleInterval{Start: lower, End: domain[1], Sign: sign})
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Complete = true
	out.Domain = domain
	out.Contacts = contacts
	out.Intervals = intervals
	return out, nil
}
