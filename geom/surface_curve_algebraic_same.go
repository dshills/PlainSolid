package geom

import (
	"context"
	"math/big"
)

// ExactCurveContinuationAgreement proves coordinate equality of two exact
// rational continuations in the same parameter and over an equal domain.
// Original represented coefficients and exact roots are compared directly.
type ExactCurveContinuationAgreement struct {
	Equal                    bool
	Operations, Subdivisions int
}

func (c *ExactComposedCurveCell) CompareSameParameter(ctx context.Context, other *ExactComposedCurveCell, options ExactCurvePlaneEventOptions) (r ExactCurveContinuationAgreement, err error) {
	if ctx == nil || c == nil || other == nil || len(c.power[3]) == 0 || len(other.power[3]) == 0 ||
		c.endpoints[0] == nil || c.endpoints[1] == nil || other.endpoints[0] == nil || other.endpoints[1] == nil {
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
	for i := range c.endpoints {
		a, e := compareRootBudget(ctx, c.endpoints[i], other.endpoints[i], b, o.MaxSubdivisions-r.Subdivisions)
		r.Subdivisions += a.Subdivisions
		if e != nil {
			return r, e
		}
		if !a.Proven {
			return r, ErrConvergence
		}
		if a.Order != 0 {
			return r, nil
		}
	}
	for i := 0; i < 3; i++ {
		a, e := polynomialProduct(c.power[i], other.power[3], b)
		if e != nil {
			return r, e
		}
		z, e := polynomialProduct(other.power[i], c.power[3], b)
		if e != nil {
			return r, e
		}
		diff, e := subtractScaledPolynomial(a, z, big.NewRat(1, 1), b)
		if e != nil {
			return r, e
		}
		if len(diff) != 0 {
			return r, nil
		}
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	r.Equal = true
	return r, nil
}
