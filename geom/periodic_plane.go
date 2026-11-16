package geom

import (
	"context"
	"math/big"
)

// PeriodicSplinePlaneOptions bounds exact extraction, root isolation and
// conversion to physical parameters. MaxSubdivisions defaults to 8192.
type PeriodicSplinePlaneOptions struct {
	ExactBezierSpanOptions
	MaxSubdivisions int
}

// PeriodicSplinePlaneHit retains an exact physical parameter and the
// original knot span that owns the contact. Multiplicity is one-sided at
// a knot, including the cyclic seam. Incident spans report separate hits.
type PeriodicSplinePlaneHit struct {
	Parameter    *ExactAlgebraicRoot
	Cycle        *big.Int
	Span         int
	Multiplicity int
}

// PeriodicSplinePlaneResult contains isolated roots and coincident pieces in
// physical parameter order. Coincident pieces retain their exact control nets.
type PeriodicSplinePlaneResult struct {
	Hits                     []PeriodicSplinePlaneHit
	Coincident               []ExactPeriodicBezierSpan
	Operations, Subdivisions int
}

// IntersectPlaneExact exhaustively intersects a uniform periodic rational
// spline with the represented plane over a closed rational window. The
// spline degree must not exceed ExactPolynomialMaxDegree. All polynomial
// tests and interval endpoints use exact arithmetic. Both isolated hits
// and coincident pieces are discarded if the complete proof fails.
func (c *PeriodicBSpline) IntersectPlaneExact(ctx context.Context, plane Plane, start, end *big.Rat, options PeriodicSplinePlaneOptions) (result PeriodicSplinePlaneResult, err error) {
	if c == nil || c.curve == nil {
		return result, ErrInvalid
	}
	return intersectPeriodicCurvePlane(ctx, c, c.degree, plane, start, end, options)
}

// IntersectPlaneExact intersects the original nonuniform cyclic law with
// the represented plane. Edited knots keep distinct one-sided incidences.
func (c *ExactPeriodicSplineCurve) IntersectPlaneExact(ctx context.Context, plane Plane, start, end *big.Rat, options PeriodicSplinePlaneOptions) (result PeriodicSplinePlaneResult, err error) {
	if c == nil || len(c.net) == 0 {
		return result, ErrInvalid
	}
	return intersectPeriodicCurvePlane(ctx, c, c.degree, plane, start, end, options)
}

func intersectPeriodicCurvePlane(ctx context.Context, c exactPeriodicSpanSource, degree int, plane Plane, start, end *big.Rat, options PeriodicSplinePlaneOptions) (result PeriodicSplinePlaneResult, err error) {
	if ctx == nil || !plane.valid() || options.MaxSubdivisions < 0 {
		return result, ErrInvalid
	}
	if degree > ExactPolynomialMaxDegree {
		return result, ErrConvergence
	}
	o, err := patchOptions(options.SurfacePatchOptions)
	if err != nil {
		return result, err
	}
	limit := options.MaxSubdivisions
	if limit == 0 {
		limit = 8192
	}
	spans, work, err := c.ExactBezierSpans(ctx, start, end, options.ExactBezierSpanOptions)
	if err != nil {
		result.Operations = work
		return result, err
	}
	budget := &polynomialBudget{operations: work, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		result.Operations = budget.operations
		if err != nil {
			result.Hits, result.Coincident = nil, nil
		}
	}()
	var normal [3]*big.Rat
	distance := new(big.Rat)
	for axis := range normal {
		normal[axis] = new(big.Rat).SetFloat64(coordinate(plane.normal, axis))
		distance.Add(distance, new(big.Rat).Mul(normal[axis], new(big.Rat).SetFloat64(coordinate(plane.origin, axis))))
		if err = budget.check(distance); err != nil {
			return result, err
		}
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	for _, span := range spans {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		numerator := make([]*big.Rat, len(span.Control))
		coincident := true
		for i, h := range span.Control {
			value := new(big.Rat).Neg(new(big.Rat).Mul(distance, h[3]))
			for axis, n := range normal {
				value.Add(value, new(big.Rat).Mul(n, h[axis]))
				if err = budget.check(value); err != nil {
					return result, err
				}
			}
			numerator[i] = value
			coincident = coincident && value.Sign() == 0
		}
		if coincident {
			result.Coincident = append(result.Coincident, span)
			continue
		}
		power, err := bernsteinToPower(numerator, budget)
		if err != nil {
			return result, err
		}
		if len(power) <= 1 {
			continue
		}
		if budget.operations >= o.MaxOperations || result.Subdivisions >= limit {
			return result, ErrConvergence
		}
		roots, err := IsolateExactPolynomialRoots(ctx, power, zero, one, ExactCurvePlaneEventOptions{
			MaxOperations: o.MaxOperations - budget.operations, MaxCoefficientBits: o.MaxCoefficientBits,
			MaxSubdivisions: limit - result.Subdivisions,
		})
		budget.operations += roots.Operations
		result.Subdivisions += roots.Subdivisions
		if err != nil {
			return result, err
		}
		width := new(big.Rat).Sub(span.End, span.Start)
		for _, hit := range roots.Roots {
			parameter, err := affineMapRoot(hit.Root.root, span.Start, width, budget)
			if err != nil {
				return result, err
			}
			result.Hits = append(result.Hits, PeriodicSplinePlaneHit{
				Parameter: &ExactAlgebraicRoot{root: parameter}, Cycle: new(big.Int).Set(span.Cycle),
				Span: span.Span, Multiplicity: hit.Multiplicity,
			})
		}
	}
	return result, nil
}
