package geom

import (
	"context"
	"math/big"
)

// BindEndpointRoot rederives identity with one immutable source-cell endpoint.
// A numerically valid persisted root is not accepted merely because its bracket
// overlaps: exact unique-root GCD comparison must prove equality.
func (c *ExactComposedCurveCell) BindEndpointRoot(ctx context.Context, endpoint int, persisted *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (ExactCurveEventOrder, error) {
	if c == nil || endpoint < 0 || endpoint > 1 || c.endpoints[endpoint] == nil || persisted == nil || persisted.root == nil {
		return ExactCurveEventOrder{}, ErrInvalid
	}
	r, e := CompareExactGraphLineRoots(ctx, c.endpoints[endpoint], persisted.root, o)
	if e != nil {
		return r, e
	}
	if !r.Proven || r.Order != 0 {
		return r, ErrDomain
	}
	return r, nil
}

// BindPlaneRoot reconstructs all current isolated plane contacts from the source
// cell and finds the exact persisted parameter among them. Imported coefficients
// are never substituted for the model constraint. Coincident spans are rejected
// here because they do not define an isolated plane event. A distinct root,
// even arbitrarily nearby, returns ErrDomain rather than a false association.
func (c *ExactComposedCurveCell) BindPlaneRoot(ctx context.Context, plane [4]*big.Rat, persisted *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (event *ExactComposedPlaneEvent, err error) {
	if ctx == nil || c == nil || persisted == nil || persisted.root == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return nil, e
	}
	intersections, e := c.IntersectRawPlane(ctx, plane, o)
	if e != nil {
		return nil, e
	}
	if !intersections.Complete || intersections.Coincident {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{operations: intersections.Operations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, x := range persisted.root.polynomial {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	for _, x := range persisted.root.bounds {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	subdivisions := intersections.Subdivisions
	for _, candidate := range intersections.Events {
		remaining := o.MaxSubdivisions - subdivisions
		if remaining < 1 {
			return nil, ErrConvergence
		}
		comparison, e := compareExactPolynomialEvents(ctx, [2]rationalPolynomial{candidate.root.polynomial, persisted.root.polynomial}, [2][2]*big.Rat{candidate.root.bounds, persisted.root.bounds}, b, remaining)
		subdivisions += comparison.Subdivisions
		if e != nil {
			return nil, e
		}
		if comparison.Proven && comparison.Order == 0 {
			return candidate, nil
		}
	}
	return nil, ErrDomain
}
