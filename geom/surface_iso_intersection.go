package geom

import (
	"context"
	"math"
	"math/big"
	"sort"
)

// IntersectPlane isolates roots of the retained exact homogeneous plane
// numerator. No conversion through float poles occurs. Multiplicities are
// exact within each knot span; shared knot endpoints intentionally appear once
// per incident span. Coincident spans are exact, not tolerance based. Returned
// points/residuals are floating diagnostics. Any error makes the result partial.
func (c *SurfaceIsoCurve) IntersectPlane(plane Plane, options SplinePlaneOptions) (SplinePlaneResult, error) {
	return c.IntersectPlaneContext(context.Background(), plane, options)
}

// IntersectPlaneContext checks cancellation during refinement and root isolation.
// Arithmetic within a bounded polynomial step is not interrupted.
func (c *SurfaceIsoCurve) IntersectPlaneContext(ctx context.Context, plane Plane, o SplinePlaneOptions) (result SplinePlaneResult, err error) {
	if ctx == nil || c == nil || c.patch == nil || !plane.valid() {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.DistanceTolerance == 0 {
		o.DistanceTolerance = 1e-8
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 12
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if !finite(o.ParameterTolerance) || !finite(o.DistanceTolerance) || o.ParameterTolerance <= 0 || o.DistanceTolerance <= 0 || o.MaxDegree < 1 || o.MaxDegree > 64 || o.MaxControlPoints < 2 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return result, ErrInvalid
	}
	if c.patch.degreeU > o.MaxDegree || c.patch.countU > o.MaxControlPoints {
		return result, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		result.Operations = budget.operations
		sort.SliceStable(result.Hits, func(i, j int) bool {
			if result.Hits[i].Parameter == result.Hits[j].Parameter {
				return result.Hits[i].Span.Index < result.Hits[j].Span.Index
			}
			return result.Hits[i].Parameter < result.Hits[j].Parameter
		})
	}()
	stack := []*SurfacePatch{c.patch}
	spanIndex := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if knot, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
			a, b, e := p.split(knot, true, budget)
			if e != nil {
				return result, e
			}
			stack = append(stack, b, a)
			continue
		}
		d := p.Domain()
		span := ParameterSpan{Start: d.UMin, End: d.UMax, Index: spanIndex}
		spanIndex++
		coefficients := make([]*big.Rat, p.countU)
		for i := range coefficients {
			h := p.net[i*p.countV]
			value := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				offset := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(plane.origin, axis)), h[3]))
				value.Add(value, new(big.Rat).Mul(offset, new(big.Rat).SetFloat64(coordinate(plane.normal, axis))))
				if e := budget.check(value); e != nil {
					return result, e
				}
			}
			coefficients[i] = value
		}
		polynomial, e := bernsteinToPower(coefficients, budget)
		if e != nil {
			return result, e
		}
		if len(polynomial) == 0 {
			result.Coincident = append(result.Coincident, span)
			continue
		}
		remaining := o.MaxSubdivisions - result.Subdivisions
		if remaining < 1 {
			return result, ErrConvergence
		}
		subdivisions, e := isolatePolynomialRoots(polynomial, new(big.Rat), big.NewRat(1, 1), budget, remaining, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			a, b := parameterMap(span, lo), parameterMap(span, hi)
			if new(big.Rat).Sub(b, a).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			middle := new(big.Rat).Quo(new(big.Rat).Add(a, b), big.NewRat(2, 1))
			parameter, _ := middle.Float64()
			exact, _, e := exactSurfaceJet(p, new(big.Rat).SetFloat64(parameter), new(big.Rat), budget)
			if e != nil {
				return false, e
			}
			x, _ := exact[0].Float64()
			y, _ := exact[1].Float64()
			z, _ := exact[2].Float64()
			point := Vec3{x, y, z}
			residual := math.Abs(plane.SignedDistance(point))
			if !point.IsFinite() || !finite(residual) {
				return false, ErrInvalid
			}
			if residual > o.DistanceTolerance {
				return false, nil
			}
			result.Hits = append(result.Hits, PlaneCurveHit{Parameter: parameter, ParameterMin: rationalFloatBound(a, true), ParameterMax: rationalFloatBound(b, false), Point: point, Residual: residual, Multiplicity: m, Tangent: m > 1, Crossing: m%2 == 1, Span: span})
			return true, nil
		})
		result.Subdivisions += subdivisions
		if e != nil {
			return result, e
		}
	}
	return result, nil
}
