package geom

import (
	"context"
	"math/big"
)

type tubeCoincidentTrimSource struct {
	chart      *ExactComposedCurveCell
	coordinate int
}

// ParameterAtFreeRoot recovers the original normalized trim parameter for
// an exact free coordinate on this certified coincident span. It proves the
// preimage in the original directed source interval, including when both
// coordinates are algebraic. Work is bounded to a substituted degree of 64.
func (s TubeCoincidentTrimSpan) ParameterAtFreeRoot(ctx context.Context, free *ExactAlgebraicRoot,
	o ExactCurvePlaneEventOptions) (parameter *ExactAlgebraicRoot, operations int, err error) {
	if ctx == nil || free == nil || free.root == nil || s.source == nil {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	o, err = graphRootOptions(o)
	if err != nil {
		return nil, 0, err
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	chart := s.source.chart
	numerator, weight := chart.power[s.source.coordinate], chart.power[3]
	target := free.root.polynomial
	lo, hi := free.root.bounds[0], free.root.bounds[1]
	if e := b.check(lo); e != nil {
		return nil, 0, e
	}
	if e := b.check(hi); e != nil {
		return nil, 0, e
	}
	if lo.Cmp(hi) == 0 {
		target = rationalPolynomial{new(big.Rat).Neg(lo), big.NewRat(1, 1)}
	}
	if (len(target)-1)*(max(len(numerator), len(weight))-1) > 64 {
		return nil, 0, ErrConvergence
	}
	if e := b.check(target[len(target)-1]); e != nil {
		return nil, 0, e
	}
	f := rationalPolynomial{target[len(target)-1]}
	weightPower := rationalPolynomial{big.NewRat(1, 1)}
	for i := len(target) - 2; i >= 0; i-- {
		if e := ctx.Err(); e != nil {
			return nil, 0, e
		}
		if e := b.check(target[i]); e != nil {
			return nil, 0, e
		}
		weightPower, err = polynomialProduct(weightPower, weight, b)
		if err != nil {
			return nil, 0, err
		}
		f, err = polynomialProduct(f, numerator, b)
		if err != nil {
			return nil, 0, err
		}
		f, err = subtractScaledPolynomial(f, weightPower, new(big.Rat).Neg(target[i]), b)
		if err != nil {
			return nil, 0, err
		}
	}
	candidates, e := chart.intersectComposedPolynomial(ctx, f, o, b)
	if e != nil {
		return nil, 0, e
	}
	if !candidates.Complete || candidates.Coincident {
		return nil, 0, ErrConvergence
	}
	o.MaxSubdivisions -= candidates.Subdivisions
	var bounds [2]rationalPolynomial
	if lo.Cmp(hi) != 0 {
		for i, bound := range []*big.Rat{lo, hi} {
			bounds[i], e = subtractScaledPolynomial(numerator, weight, bound, b)
			if e != nil {
				return nil, 0, e
			}
		}
	}
	for _, event := range candidates.Events {
		inTarget := true
		if lo.Cmp(hi) != 0 {
			for side, comparison := range bounds {
				if o.MaxSubdivisions < 1 {
					return nil, 0, ErrConvergence
				}
				sign, steps, e := composedPolynomialRootSign(ctx, event.root, comparison, b, o.MaxSubdivisions)
				o.MaxSubdivisions -= steps
				if e != nil {
					return nil, 0, e
				}
				inTarget = inTarget && (side == 0 && sign >= 0 || side == 1 && sign <= 0)
			}
		}
		if inTarget {
			if parameter != nil {
				return nil, 0, ErrConvergence
			}
			parameter = event.Root()
		}
	}
	if parameter == nil {
		return nil, 0, ErrDomain
	}
	return parameter, 0, nil
}
