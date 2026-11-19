package geom

import (
	"context"
	"math/big"
	"runtime"
	"sort"
)

// ExactCurvePointParameter proves a parameter whose model point equals the
// supplied rational point. Parameter is optional rational recovery; Lower/Upper
// always enclose the exact root. Multiplicity is the minimum vanishing order of
// all nonzero homogeneous coordinate residuals, local to Span.
type ExactCurvePointParameter struct {
	Parameter, Lower, Upper *big.Rat
	Span                    ParameterSpan
	Multiplicity            int
}

// ExactCurvePointParameters is exhaustive only when Complete. ConstantSpans
// identifies entire spans mapping exactly to Point; isolated roots occur once
// per incident knot span. All returned rational data belong to the caller.
type ExactCurvePointParameters struct {
	Point                    [3]*big.Rat
	Roots                    []ExactCurvePointParameter
	ConstantSpans            []ParameterSpan
	Complete                 bool
	Operations, Subdivisions int
}

// FindExactCurvePointParameters isolates all parameter preimages of an exact
// rational model point, using the GCD of three homogeneous coordinate residuals.
// It neither assumes injectivity nor substitutes approximate model distances.
// Unrecovered algebraic parameters retain certified isolating brackets and do
// not prevent completeness: their exact common-root existence is already proved.
func FindExactCurvePointParameters(ctx context.Context, c *ExactSplineCurve, point [3]*big.Rat, o ExactCurveSegmentOptions) (r ExactCurvePointParameters, err error) {
	if ctx == nil || c == nil || c.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 12
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxDegree < 1 || o.MaxDegree > 64 || o.MaxControlPoints < 2 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	if c.patch.degreeU > o.MaxDegree || c.patch.countU > o.MaxControlPoints {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		r.Operations = budget.operations
		sort.SliceStable(r.Roots, func(i, j int) bool {
			cmp := r.Roots[i].Lower.Cmp(r.Roots[j].Lower)
			if cmp == 0 {
				return r.Roots[i].Span.Index < r.Roots[j].Span.Index
			}
			return cmp < 0
		})
	}()
	for i, x := range point {
		if x == nil {
			return r, ErrInvalid
		}
		if e := budget.check(x); e != nil {
			return r, e
		}
		r.Point[i] = new(big.Rat).Set(x)
	}
	stack := []*SurfacePatch{c.patch}
	index := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if knot, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
			left, right, e := p.split(knot, true, budget)
			if e != nil {
				return r, e
			}
			stack = append(stack, right, left)
			continue
		}
		domain := p.Domain()
		span := ParameterSpan{Start: domain.UMin, End: domain.UMax, Index: index}
		index++
		var gcd rationalPolynomial
		for axis := 0; axis < 3; axis++ {
			coeff := make([]*big.Rat, p.countU)
			for i := range coeff {
				h := p.net[i*p.countV]
				coeff[i] = new(big.Rat).Sub(h[axis], new(big.Rat).Mul(point[axis], h[3]))
				runtime.KeepAlive(&h) // golang/go#82026: keeps h in the stack map. Remove once the Go fix is in use.
				if e := budget.check(coeff[i]); e != nil {
					return r, e
				}
			}
			polynomial, e := bernsteinToPower(coeff, budget)
			if e != nil {
				return r, e
			}
			gcd, e = polynomialGCD(gcd, polynomial, budget)
			if e != nil {
				return r, e
			}
			if len(gcd) == 1 {
				break
			}
		}
		if len(gcd) == 0 {
			r.ConstantSpans = append(r.ConstantSpans, span)
			continue
		}
		if len(gcd) == 1 {
			continue
		}
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(gcd, new(big.Rat), big.NewRat(1, 1), budget, remaining, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			candidate, e := recoverRationalPolynomialRoot(gcd, lo, hi, budget)
			if e != nil {
				return false, e
			}
			lower, upper := parameterMap(span, lo), parameterMap(span, hi)
			root := ExactCurvePointParameter{Lower: lower, Upper: upper, Span: span, Multiplicity: m}
			if candidate != nil {
				root.Parameter = parameterMap(span, candidate)
				root.Lower = new(big.Rat).Set(root.Parameter)
				root.Upper = new(big.Rat).Set(root.Parameter)
				evaluated, _, e := exactSurfaceJet(p, root.Parameter, new(big.Rat), budget)
				if e != nil {
					return false, e
				}
				for axis, x := range evaluated {
					if x.Cmp(point[axis]) != 0 {
						return false, ErrConvergence
					}
				}
			} else if new(big.Rat).Sub(upper, lower).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			r.Roots = append(r.Roots, root)
			return true, nil
		})
		r.Subdivisions += steps
		if e != nil {
			return r, e
		}
	}
	r.Complete = true
	return r, nil
}
