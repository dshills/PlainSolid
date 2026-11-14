package geom

import (
	"context"
	"math/big"
)

// ComposedLinearCandidate retains the exact source parameter and the exact
// value of a linear functional, including a constant offset.
type ComposedLinearCandidate struct {
	Parameter, Value *ExactAlgebraicRoot
}

// ComposedLinearExtrema contains every endpoint and interior stationary
// candidate for a linear functional on one exact composed curve cell.
// Constant identifies the special case in which both endpoint values also
// describe every interior point of the cell.
type ComposedLinearExtrema struct {
	Candidates               []ComposedLinearCandidate
	Constant                 bool
	Operations, Subdivisions int
}

// LinearExtremaCandidates proves a finite exact candidate set for
// nx*x+ny*y+nz*z+d on the actual algebraic curve domain. The curve's
// homogeneous model coefficients must have degree at most three and the
// derivative numerator degree at most four. Nil functional coefficients
// represent zero. A nonzero functional is not required.
func (c *ExactComposedCurveCell) LinearExtremaCandidates(ctx context.Context, functional [4]*big.Rat, opts ExactCurvePlaneEventOptions) (r ComposedLinearExtrema, err error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 {
		return r, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return r, err
	}
	opts, err = graphRootOptions(opts)
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() {
		r.Operations = b.operations
		if err != nil {
			r.Candidates = nil
			r.Constant = false
		}
	}()
	var numerator rationalPolynomial
	for axis, co := range functional {
		p := trimPolynomial(c.power[axis])
		if len(p) > 4 {
			return r, ErrUnrepresentable
		}
		if co != nil {
			if err = b.check(co); err != nil {
				return r, err
			}
			numerator, err = subtractScaledPolynomial(numerator, p, new(big.Rat).Neg(co), b)
			if err != nil {
				return r, err
			}
		}
	}
	w := c.power[3]
	a, err := polynomialProduct(polynomialDerivative(numerator), w, b)
	if err != nil {
		return r, err
	}
	z, err := polynomialProduct(numerator, polynomialDerivative(w), b)
	if err != nil {
		return r, err
	}
	derivative, err := subtractScaledPolynomial(a, z, big.NewRat(1, 1), b)
	if err != nil {
		return r, err
	}
	if len(derivative) > 5 {
		return r, ErrUnrepresentable
	}
	r.Constant = len(derivative) == 0
	parameters := []*ExactAlgebraicRoot{{c.endpoints[0]}, {c.endpoints[1]}}
	if len(derivative) > 1 {
		hits, e := c.intersectComposedPolynomial(ctx, derivative, opts, b)
		r.Subdivisions += hits.Subdivisions
		if e != nil {
			return r, e
		}
		if !hits.Complete || hits.Coincident {
			return r, ErrConvergence
		}
		for _, h := range hits.Events {
			if h.EndpointMask() == 0 {
				parameters = append(parameters, h.Root())
			}
		}
	}
	image := *c
	image.power = [4]rationalPolynomial{numerator, nil, nil, w}
	for _, p := range parameters {
		work, steps := b.maxOperations-b.operations, opts.MaxSubdivisions-r.Subdivisions
		if work < 1 || steps < 1 {
			return r, ErrConvergence
		}
		value, n, m, e := image.CoordinateRootWithWork(ctx, p, 0,
			ExactCurvePlaneEventOptions{MaxOperations: work, MaxSubdivisions: steps, MaxCoefficientBits: b.maxBits})
		b.operations += n
		r.Subdivisions += m
		if e != nil {
			return r, e
		}
		r.Candidates = append(r.Candidates, ComposedLinearCandidate{p, value})
	}
	return r, ctx.Err()
}
