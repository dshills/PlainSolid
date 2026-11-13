package geom

import (
	"context"
	"math/big"
)

// PairedComposedCurveEvent retains independent original parameter identities
// and model enclosures through each event's Enclose method.
type PairedComposedCurveEvent struct{ A, B *ExactComposedCurveEvent }

// ComposedCurvesAffineProjectionIntersection proves all contacts when Complete.
// Hits are ordered in A parameter. CoincidentBoundary is ordered in A (B may
// decrease). PointOnly marks a singleton projected-domain overlap; its hit has
// multiplicity zero, since no isolated residual order is asserted there.
type ComposedCurvesAffineProjectionIntersection struct {
	Complete, Coincident, PointOnly bool
	Hits                            []PairedComposedCurveEvent
	CoincidentBoundary              [2]PairedComposedCurveEvent
	Offset, Scale                   *big.Rat // B parameter = Offset + Scale*A parameter
	Operations, Subdivisions        int
}

// IntersectComposedCurvesAffineProjection proves exact affine, strictly monotone
// projection onto the selected model coordinate in BOTH input parameterizations.
// Other coordinates may be arbitrary supported rational polynomials. Exact
// residual GCD roots and algebraic domain clipping provide exhaustive pairing;
// neither endpoint boxes nor rounded spatial proximity establish coincidence.
func IntersectComposedCurvesAffineProjection(ctx context.Context, a, c *ExactComposedCurveCell, axis int, o ExactCurvePlaneEventOptions) (r ComposedCurvesAffineProjectionIntersection, err error) {
	if ctx == nil || a == nil || c == nil || axis < 0 || axis > 2 || len(a.power[3]) == 0 || len(c.power[3]) == 0 || a.endpoints[0] == nil || a.endpoints[1] == nil || c.endpoints[0] == nil || c.endpoints[1] == nil {
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
	var projections [2]rationalPolynomial
	for i, cell := range []*ExactComposedCurveCell{a, c} {
		q, rem, e := polynomialDivide(cell.power[axis], cell.power[3], b)
		if e != nil {
			return r, e
		}
		if len(rem) != 0 || len(q) != 2 || q[1].Sign() == 0 {
			return r, ErrConvergence
		}
		projections[i] = q
	}
	r.Offset = new(big.Rat).Quo(new(big.Rat).Sub(projections[0][0], projections[1][0]), projections[1][1])
	r.Scale = new(big.Rat).Quo(projections[0][1], projections[1][1])
	for _, q := range []*big.Rat{r.Offset, r.Scale} {
		if e := b.check(q); e != nil {
			return r, e
		}
	}
	inverse := new(big.Rat).Inv(r.Scale)
	inverseOffset := new(big.Rat).Neg(new(big.Rat).Mul(r.Offset, inverse))
	var mapped [2]*ExactGraphLineRoot
	for i := 0; i < 2; i++ {
		mapped[i], e = affineMapRoot(c.endpoints[i], inverseOffset, inverse, b)
		if e != nil {
			return r, e
		}
	}
	if r.Scale.Sign() < 0 {
		mapped[0], mapped[1] = mapped[1], mapped[0]
	}
	compare := func(x, y *ExactGraphLineRoot) (int, error) {
		remaining := options.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return 0, ErrConvergence
		}
		q, e := compareRootBudget(ctx, x, y, b, remaining)
		r.Subdivisions += q.Subdivisions
		if e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, ErrConvergence
		}
		return q.Order, nil
	}
	lo, hi := copyExactRoot(a.endpoints[0]), copyExactRoot(a.endpoints[1])
	order, e := compare(lo, mapped[0])
	if e != nil {
		return r, e
	}
	if order < 0 {
		lo = mapped[0]
	}
	order, e = compare(hi, mapped[1])
	if e != nil {
		return r, e
	}
	if order > 0 {
		hi = mapped[1]
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
	var other [4]rationalPolynomial
	for i := 0; i < 4; i++ {
		other[i], e = composeProjectionPolynomial(c.power[i], rationalPolynomial{r.Offset, r.Scale}, b)
		if e != nil {
			return r, e
		}
	}
	var residuals [3]rationalPolynomial
	var common rationalPolynomial
	for i := 0; i < 3; i++ {
		left, e := polynomialProduct(a.power[i], other[3], b)
		if e != nil {
			return r, e
		}
		right, e := polynomialProduct(other[i], a.power[3], b)
		if e != nil {
			return r, e
		}
		residuals[i], e = subtractScaledPolynomial(left, right, big.NewRat(1, 1), b)
		if e != nil {
			return r, e
		}
		if len(residuals[i]) == 0 {
			continue
		}
		if len(common) == 0 {
			common = residuals[i]
		} else {
			common, e = polynomialGCD(common, residuals[i], b)
			if e != nil {
				return r, e
			}
		}
	}
	modelA, modelB := *a, *c
	modelA.endpoints = [2]*ExactGraphLineRoot{lo, hi}
	for i, root := range modelA.endpoints {
		modelB.endpoints[i], e = affineMapRoot(root, r.Offset, r.Scale, b)
		if e != nil {
			return r, e
		}
	}
	if r.Scale.Sign() < 0 {
		modelB.endpoints[0], modelB.endpoints[1] = modelB.endpoints[1], modelB.endpoints[0]
	}
	paired := func(event *ExactComposedPlaneEvent) (PairedComposedCurveEvent, error) {
		root, e := affineMapRoot(event.root, r.Offset, r.Scale, b)
		if e != nil {
			return PairedComposedCurveEvent{}, e
		}
		mask := event.endpointMask
		if r.Scale.Sign() < 0 {
			mask = (mask&1)<<1 | (mask&2)>>1
		}
		return PairedComposedCurveEvent{event, &ExactComposedPlaneEvent{root: root, cell: &modelB, multiplicity: event.multiplicity, endpointMask: mask}}, nil
	}
	if r.PointOnly {
		for _, polynomial := range residuals {
			if len(polynomial) == 0 {
				continue
			}
			remaining := options.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			sign, steps, e := composedPolynomialRootSign(ctx, lo, polynomial, b, remaining)
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
			if sign != 0 {
				r.Complete = true
				return r, nil
			}
		}
		event := &ExactComposedPlaneEvent{root: copyExactRoot(lo), cell: &modelA, endpointMask: 3}
		pair, e := paired(event)
		if e != nil {
			return r, e
		}
		r.Hits = append(r.Hits, pair)
		r.Complete = true
		return r, nil
	}
	remaining := options.MaxSubdivisions - r.Subdivisions
	if remaining < 1 {
		return r, ErrConvergence
	}
	child := options
	child.MaxSubdivisions = remaining
	contact, e := modelA.intersectComposedPolynomial(ctx, common, child, b)
	r.Subdivisions += contact.Subdivisions
	if e != nil {
		return r, e
	}
	if contact.Coincident {
		for i, event := range contact.Boundary {
			r.CoincidentBoundary[i], e = paired(event)
			if e != nil {
				return r, e
			}
		}
	} else {
		for _, event := range contact.Events {
			pair, e := paired(event)
			if e != nil {
				return r, e
			}
			r.Hits = append(r.Hits, pair)
		}
	}
	r.Coincident = contact.Coincident
	r.Complete = contact.Complete
	return r, nil
}
