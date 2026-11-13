package geom

import (
	"context"
	"math/big"
)

// ComposedQuadraticConicIntersection retains independent original parameters.
// Pending holds support-conic events whose parameter image has unsupported
// algebraic degree. CoincidentSupport proves A lies on B’s supporting conic but
// does not assert bounded arc overlap. Complete never includes pending cases.
// Multiplicity on A is the source residual order; B multiplicity is zero
// because no order-transfer claim is made for the nonlinear inverse.
type ComposedQuadraticConicIntersection struct {
	Complete, CoincidentSupport bool
	Hits                        []PairedComposedCurveEvent
	Pending                     []*ExactComposedCurveEvent
	Operations                  int
}

// IntersectComposedQuadraticConic uses a rank-three homogeneous quadratic net
// to invert B projectively, then enforces its conic equation AND remaining
// spatial coordinate. Both curves must have homogeneous power degree <=2.
// Isolated roots of degree up to four are paired by exact coordinate images;
// failure to separate conjugate image roots remains unresolved. MaxSubdivisions bounds each root proof;
// MaxOperations is cumulative across all proofs. No common-plane approximation
// or spatial proximity is used to accept a pair.
func IntersectComposedQuadraticConic(ctx context.Context, a, c *ExactComposedCurveCell, o ExactCurvePlaneEventOptions) (r ComposedQuadraticConicIntersection, err error) {
	if ctx == nil || a == nil || c == nil || a.endpoints[0] == nil || a.endpoints[1] == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(a.power[3]) == 0 || len(c.power[3]) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	for _, cell := range []*ExactComposedCurveCell{a, c} {
		for _, p := range cell.power {
			if len(p) > 3 {
				return r, ErrConvergence
			}
		}
	}
	coefficient := func(p rationalPolynomial, i int) *big.Rat {
		if i >= len(p) {
			return new(big.Rat)
		}
		return p[i]
	}
	lambda, e := quadraticConicInversePowers(a, c, b)
	if e != nil {
		return r, e
	}
	square, e := polynomialProduct(lambda[1], lambda[1], b)
	if e != nil {
		return r, e
	}
	product, e := polynomialProduct(lambda[0], lambda[2], b)
	if e != nil {
		return r, e
	}
	common, e := subtractScaledPolynomial(square, product, big.NewRat(1, 1), b)
	if e != nil {
		return r, e
	}
	for field := 0; field < 4; field++ {
		predicted := rationalPolynomial{new(big.Rat), new(big.Rat), new(big.Rat)}
		for i := 0; i < 3; i++ {
			for j, x := range lambda[i] {
				predicted[j].Add(predicted[j], new(big.Rat).Mul(coefficient(c.power[field], i), x))
				if e := b.check(predicted[j]); e != nil {
					return r, e
				}
			}
		}
		residual, e := subtractScaledPolynomial(a.power[field], predicted, big.NewRat(1, 1), b)
		if e != nil {
			return r, e
		}
		if len(residual) > 0 {
			if len(common) == 0 {
				common = residual
			} else {
				common, e = polynomialGCD(common, residual, b)
				if e != nil {
					return r, e
				}
			}
		}
	}
	if len(common) == 0 {
		r.CoincidentSupport = true
		return r, ErrConvergence
	}
	contacts, e := a.intersectComposedPolynomial(ctx, common, options, b)
	if e != nil {
		return r, e
	}
	for _, event := range contacts.Events {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		sign, _, e := composedPolynomialRootSign(ctx, event.root, lambda[0], b, options.MaxSubdivisions)
		if e != nil {
			return r, e
		}
		if sign == 0 {
			continue
		} // finite B parameters require lambda0 !=0
		canonical, e := canonicalExactRoot(ctx, event.root.polynomial, event.root.bounds, ExactRootCodecOptions{ExactCurvePlaneEventOptions: options, MaxDegree: 4}, b)
		if e != nil {
			return r, e
		}
		if len(canonical.polynomial) > 5 {
			r.Pending = append(r.Pending, event)
			continue
		}
		imageCell := *a
		imageCell.power = [4]rationalPolynomial{lambda[1], nil, nil, lambda[0]}
		local := options
		local.MaxOperations -= b.operations
		if local.MaxOperations < 1 {
			return r, ErrConvergence
		}
		image, ops, e := imageCell.CoordinateRoot(ctx, &ExactAlgebraicRoot{root: canonical}, 0, local)
		b.operations += ops
		if e != nil {
			return r, e
		}
		inside := true
		mask := uint8(0)
		for endpoint := 0; endpoint < 2; endpoint++ {
			comparison, e := compareRootBudget(ctx, image.root, c.endpoints[endpoint], b, options.MaxSubdivisions)
			if e != nil {
				return r, e
			}
			if !comparison.Proven {
				return r, ErrConvergence
			}
			if endpoint == 0 && comparison.Order < 0 || endpoint == 1 && comparison.Order > 0 {
				inside = false
			}
			if comparison.Order == 0 {
				mask |= 1 << uint(endpoint)
			}
		}
		if !inside {
			continue
		}
		// Independent original-coordinate residual proof: substituting the rational
		// inverse s=lambda1/lambda0 into each B homogeneous polynomial must agree
		// with A at the selected source root. This rejects extraneous support roots.
		l00, e := polynomialProduct(lambda[0], lambda[0], b)
		if e != nil {
			return r, e
		}
		l01, e := polynomialProduct(lambda[0], lambda[1], b)
		if e != nil {
			return r, e
		}
		l11, e := polynomialProduct(lambda[1], lambda[1], b)
		if e != nil {
			return r, e
		}
		var substituted [4]rationalPolynomial
		for field := 0; field < 4; field++ {
			substituted[field] = make(rationalPolynomial, 5)
			for i := range substituted[field] {
				substituted[field][i] = new(big.Rat)
			}
			for power, base := range []rationalPolynomial{l00, l01, l11} {
				for i, x := range base {
					substituted[field][i].Add(substituted[field][i], new(big.Rat).Mul(coefficient(c.power[field], power), x))
					if e := b.check(substituted[field][i]); e != nil {
						return r, e
					}
				}
			}
			substituted[field] = trimPolynomial(substituted[field])
		}
		for field := 0; field < 3; field++ {
			left, e := polynomialProduct(a.power[field], substituted[3], b)
			if e != nil {
				return r, e
			}
			right, e := polynomialProduct(substituted[field], a.power[3], b)
			if e != nil {
				return r, e
			}
			residual, e := subtractScaledPolynomial(left, right, big.NewRat(1, 1), b)
			if e != nil {
				return r, e
			}
			if len(residual) > 0 {
				sign, _, e := composedPolynomialRootSign(ctx, event.root, residual, b, options.MaxSubdivisions)
				if e != nil {
					return r, e
				}
				if sign != 0 {
					return r, ErrConvergence
				}
			}
		}
		other := &ExactComposedPlaneEvent{root: image.root, cell: c, endpointMask: mask}
		r.Hits = append(r.Hits, PairedComposedCurveEvent{A: event, B: other})
	}
	r.Complete = len(r.Pending) == 0
	if !r.Complete {
		return r, ErrConvergence
	}
	return r, nil
}

func quadraticConicInversePowers(a, c *ExactComposedCurveCell, b *polynomialBudget) (lambda [3]rationalPolynomial, err error) {
	var e error
	coefficient := func(p rationalPolynomial, i int) *big.Rat {
		if i >= len(p) {
			return new(big.Rat)
		}
		return p[i]
	}
	var inverse [3][3]*big.Rat
	var fields [3]int
	found := false
	for omitted := 0; omitted < 4 && !found; omitted++ {
		index := 0
		var matrix [3][3]*big.Rat
		for field := 0; field < 4; field++ {
			if field == omitted {
				continue
			}
			fields[index] = field
			for power := 0; power < 3; power++ {
				matrix[index][power] = coefficient(c.power[field], power)
			}
			index++
		}
		inverse, found, e = inverseExact3(matrix, b)
		if e != nil {
			return lambda, e
		}
	}
	if !found {
		return lambda, ErrDegenerate
	}
	for i := 0; i < 3; i++ {
		lambda[i] = make(rationalPolynomial, 3)
		for degree := 0; degree < 3; degree++ {
			lambda[i][degree] = new(big.Rat)
			for j := 0; j < 3; j++ {
				lambda[i][degree].Add(lambda[i][degree], new(big.Rat).Mul(inverse[i][j], coefficient(a.power[fields[j]], degree)))
				if e := b.check(lambda[i][degree]); e != nil {
					return lambda, e
				}
			}
		}
		lambda[i] = trimPolynomial(lambda[i])
	}

	return lambda, nil
}
