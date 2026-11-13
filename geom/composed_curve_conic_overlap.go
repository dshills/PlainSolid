package geom

import (
	"context"
	"errors"
	"math/big"
)

// ComposedConicOverlap proves bounded same-image overlap under an exact Mobius
// parameter map B=(Map[0]+Map[1]*A)/(Map[2]+Map[3]*A). Boundary follows A order.
// Complete with neither Coincident nor PointOnly proves disjoint bounded arcs.
// PointOnly returns one paired event in Boundary[0], not an interval overlap.
type ComposedConicOverlap struct {
	Complete, Coincident, PointOnly bool
	Map                             [4]*big.Rat
	Orientation                     int
	Boundary                        [2]PairedComposedCurveEvent
	Operations                      int
}

// IntersectCoincidentComposedConics requires exact support-conic coincidence and
// a nonconstant Mobius correspondence with nonzero denominator across A's true
// closed domain. Algebraic bounds of degree <=4 are mapped and clipped exactly.
// MaxOperations is cumulative; MaxSubdivisions bounds each root proof.
func IntersectCoincidentComposedConics(ctx context.Context, a, c *ExactComposedCurveCell, o ExactCurvePlaneEventOptions) (r ComposedConicOverlap, err error) {
	if ctx == nil || a == nil || c == nil {
		return r, ErrInvalid
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	proof, e := IntersectComposedQuadraticConic(ctx, a, c, options)
	b.operations += proof.Operations
	if e != nil && !errors.Is(e, ErrConvergence) {
		return r, e
	}
	if !proof.CoincidentSupport {
		return r, ErrConvergence
	}
	lambda, e := quadraticConicInversePowers(a, c, b)
	if e != nil {
		return r, e
	}
	gcd, e := polynomialGCD(lambda[1], lambda[0], b)
	if e != nil {
		return r, e
	}
	numerator, _, e := polynomialDivide(lambda[1], gcd, b)
	if e != nil {
		return r, e
	}
	denominator, _, e := polynomialDivide(lambda[0], gcd, b)
	if e != nil {
		return r, e
	}
	if len(numerator) > 2 || len(denominator) > 2 || len(denominator) == 0 {
		return r, ErrConvergence
	}
	at := func(p rationalPolynomial, i int) *big.Rat {
		if i >= len(p) {
			return new(big.Rat)
		}
		return new(big.Rat).Set(p[i])
	}
	r.Map = [4]*big.Rat{at(numerator, 0), at(numerator, 1), at(denominator, 0), at(denominator, 1)}
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(r.Map[1], r.Map[2]), new(big.Rat).Mul(r.Map[0], r.Map[3]))
	if e := b.check(determinant); e != nil {
		return r, e
	}
	r.Orientation = determinant.Sign()
	if r.Orientation == 0 {
		return r, ErrDegenerate
	}
	signs := [2]int{}
	for i, root := range a.endpoints {
		sign, _, e := composedPolynomialRootSign(ctx, root, denominator, b, options.MaxSubdivisions)
		if e != nil {
			return r, e
		}
		signs[i] = sign
	}
	if signs[0] == 0 || signs[0] != signs[1] {
		return r, ErrConvergence
	}
	imageCell := *a
	imageCell.power = [4]rationalPolynomial{numerator, nil, nil, denominator}
	coordinate := func(cell *ExactComposedCurveCell, root *ExactGraphLineRoot) (*ExactGraphLineRoot, error) {
		child := options
		child.MaxOperations -= b.operations
		if child.MaxOperations < 1 {
			return nil, ErrConvergence
		}
		image, ops, e := cell.CoordinateRoot(ctx, &ExactAlgebraicRoot{root: root}, 0, child)
		b.operations += ops
		if e != nil {
			return nil, e
		}
		return image.root, nil
	}
	images := [2]*ExactGraphLineRoot{}
	for i, root := range a.endpoints {
		images[i], e = coordinate(&imageCell, root)
		if e != nil {
			return r, e
		}
	}
	if r.Orientation < 0 {
		images[0], images[1] = images[1], images[0]
	}
	compare := func(x, y *ExactGraphLineRoot) (int, error) {
		q, e := compareRootBudget(ctx, x, y, b, options.MaxSubdivisions)
		if e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, ErrConvergence
		}
		return q.Order, nil
	}
	lo, hi := images[0], images[1]
	order, e := compare(lo, c.endpoints[0])
	if e != nil {
		return r, e
	}
	if order < 0 {
		lo = c.endpoints[0]
	}
	order, e = compare(hi, c.endpoints[1])
	if e != nil {
		return r, e
	}
	if order > 0 {
		hi = c.endpoints[1]
	}
	order, e = compare(lo, hi)
	if e != nil {
		return r, e
	}
	if order > 0 {
		r.Complete = true
		return r, nil
	}
	r.PointOnly = order == 0
	inverseCell := *c
	inverseCell.power = [4]rationalPolynomial{{new(big.Rat).Neg(r.Map[0]), r.Map[2]}, nil, nil, {r.Map[1], new(big.Rat).Neg(r.Map[3])}}
	chosen := [2]*ExactGraphLineRoot{lo, hi}
	if r.Orientation < 0 {
		chosen[0], chosen[1] = chosen[1], chosen[0]
	}
	count := 2
	if r.PointOnly {
		count = 1
	}
	for i := 0; i < count; i++ {
		parameter, e := coordinate(&inverseCell, chosen[i])
		if e != nil {
			return r, e
		}
		maskA, maskB := uint8(0), uint8(0)
		for endpoint := 0; endpoint < 2; endpoint++ {
			order, e := compare(parameter, a.endpoints[endpoint])
			if e != nil {
				return r, e
			}
			if order == 0 {
				maskA |= 1 << uint(endpoint)
			}
			if endpoint == 0 && order < 0 || endpoint == 1 && order > 0 {
				return r, ErrConvergence
			}
			order, e = compare(chosen[i], c.endpoints[endpoint])
			if e != nil {
				return r, e
			}
			if order == 0 {
				maskB |= 1 << uint(endpoint)
			}
		}
		r.Boundary[i] = PairedComposedCurveEvent{A: &ExactComposedPlaneEvent{root: parameter, cell: a, endpointMask: maskA}, B: &ExactComposedPlaneEvent{root: copyExactRoot(chosen[i]), cell: c, endpointMask: maskB}}
	}
	r.Coincident = !r.PointOnly
	r.Complete = true
	return r, nil
}
