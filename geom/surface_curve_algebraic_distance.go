package geom

import (
	"context"
	"math/big"
)

// PolynomialCurveDistanceCandidate is a domain endpoint or a stationary point
// of squared distance, with both identities proved algebraically.
type PolynomialCurveDistanceCandidate struct {
	Parameter, SquaredDistance *ExactAlgebraicRoot
}

// PolynomialCurveDistance returns the full finite candidate set for a
// polynomial model curve of degree at most two, on its true algebraic
// interval. Flat distance returns the two endpoints, which suffice for a
// minimum value. Positive constant homogeneous weights are accepted.
type PolynomialCurveDistance struct {
	Candidates               []PolynomialCurveDistanceCandidate
	Operations, Subdivisions int
}

func (c *ExactComposedCurveCell) PolynomialDistanceCandidates(ctx context.Context, point [3]*big.Rat, options ExactCurvePlaneEventOptions) (PolynomialCurveDistance, error) {
	return c.quadraticDistanceCandidates(ctx, point, options, false)
}

// RationalQuadraticDistanceCandidates is the weighted counterpart. Power
// coefficients of each homogeneous coordinate must have degree at most two;
// the cancelled stationary polynomial has degree at most four.
func (c *ExactComposedCurveCell) RationalQuadraticDistanceCandidates(ctx context.Context, point [3]*big.Rat, options ExactCurvePlaneEventOptions) (PolynomialCurveDistance, error) {
	return c.quadraticDistanceCandidates(ctx, point, options, true)
}

func (c *ExactComposedCurveCell) quadraticDistanceCandidates(ctx context.Context, point [3]*big.Rat, options ExactCurvePlaneEventOptions, rational bool) (r PolynomialCurveDistance, err error) {
	defer func() {
		if err != nil {
			r.Candidates = nil
		}
	}()
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(options)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	w := trimPolynomial(c.power[3])
	if len(w) < 1 || len(w) > 3 || !rational && (len(w) != 1 || w[0].Sign() <= 0) {
		return r, ErrUnrepresentable
	}
	var d rationalPolynomial
	for axis, p := range point {
		if p == nil {
			return r, ErrInvalid
		}
		if e := b.check(p); e != nil {
			return r, e
		}
		coords := trimPolynomial(c.power[axis])
		if len(coords) > 3 {
			return r, ErrUnrepresentable
		}
		diff, e := subtractScaledPolynomial(coords, w, p, b)
		if e != nil {
			return r, e
		}
		s, e := polynomialProduct(diff, diff, b)
		if e != nil {
			return r, e
		}
		d, e = subtractScaledPolynomial(d, s, big.NewRat(-1, 1), b)
		if e != nil {
			return r, e
		}
	}
	if len(d) == 0 {
		d = rationalPolynomial{new(big.Rat)}
	}
	// For distance d/W², W stays strictly positive on the source cell.
	// The W' term cancels the degree-five leading coefficient.
	deriv, e := polynomialProduct(polynomialDerivative(d), w, b)
	if e != nil {
		return r, e
	}
	correction, e := polynomialProduct(d, polynomialDerivative(w), b)
	if e != nil {
		return r, e
	}
	deriv, e = subtractScaledPolynomial(deriv, correction, big.NewRat(2, 1), b)
	if e != nil {
		return r, e
	}
	if len(deriv) > 5 {
		return r, ErrConvergence
	}
	weightSquared, e := polynomialProduct(w, w, b)
	if e != nil {
		return r, e
	}
	roots := []*ExactAlgebraicRoot{{c.endpoints[0]}, {c.endpoints[1]}}
	if len(deriv) > 1 {
		h, e := c.intersectComposedPolynomial(ctx, deriv, o, b)
		if e != nil {
			return r, e
		}
		if !h.Complete || h.Coincident {
			return r, ErrConvergence
		}
		r.Subdivisions += h.Subdivisions
		for _, q := range h.Events {
			if q.EndpointMask() == 0 {
				roots = append(roots, q.Root())
			}
		}
	}
	distance := *c
	distance.power = [4]rationalPolynomial{d, nil, nil, weightSquared}
	for _, p := range roots {
		work, splits := b.maxOperations-b.operations, o.MaxSubdivisions-r.Subdivisions
		if work < 1 || splits < 1 {
			return r, ErrConvergence
		}
		q, used, e := distance.CoordinateRoot(ctx, p, 0, ExactCurvePlaneEventOptions{MaxOperations: work, MaxCoefficientBits: b.maxBits, MaxSubdivisions: splits})
		b.operations += used
		if e != nil {
			return r, e
		}
		r.Candidates = append(r.Candidates, PolynomialCurveDistanceCandidate{p, q})
	}
	return r, ctx.Err()
}
