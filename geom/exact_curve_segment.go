package geom

import (
	"context"
	"math/big"
	"runtime"
	"sort"
)

// ExactCurveSegmentOptions bounds exact GCD/root isolation. Zero selects
// parameter tolerance 1e-10, degree 12, 4096 controls, 2M operations, 16384 bits
// and 8192 subdivisions. No geometric tolerance participates in incidence.
type ExactCurveSegmentOptions struct {
	ParameterTolerance                                                              float64
	MaxDegree, MaxControlPoints, MaxOperations, MaxCoefficientBits, MaxSubdivisions int
}

// ExactCurveSegmentHit owns exact coordinates. SegmentParameter runs from 0 to 1
// between the supplied segment ends. Transverse means a nonzero curve tangent
// not parallel to the segment; Tangent and Singular distinguish the alternatives.
// Multiplicity is the vanishing order of the GCD of the two line-incidence
// polynomials. At repeated knots, each incident span reports its own one-sided tangent.
type ExactCurveSegmentHit struct {
	// OtherParameter is populated by IntersectExactCurveAffineLine only.
	OtherParameter                                *big.Rat
	Parameter, SegmentParameter                   *big.Rat
	Point                                         [3]*big.Rat
	Span                                          ParameterSpan
	Multiplicity                                  int
	Transverse, Tangent, Singular, SharedEndpoint bool
}
type ExactCurveSegmentPending struct {
	Lower, Upper *big.Rat
	Span         ParameterSpan
	Multiplicity int
}

// ExactCurveSegmentIntersection is exhaustive only when Complete. Pending
// isolates unrecovered algebraic supporting-line roots that may meet the segment.
// Coincident records spans on the supporting line, without claiming their whole
// image lies inside the segment. Both cases force ErrConvergence.
type ExactCurveSegmentIntersection struct {
	// SupportingRootsComplete certifies exhaustive supporting-line isolation,
	// even when algebraic clipping or coincident spans remain unresolved.
	SupportingRootsComplete  bool
	Hits                     []ExactCurveSegmentHit
	Pending                  []ExactCurveSegmentPending
	Coincident               []ParameterSpan
	Complete                 bool
	Operations, Subdivisions int
}

// IntersectExactCurveSegment intersects an immutable spatial rational curve with
// a nondegenerate exact rational segment. Two independent homogeneous incidence
// polynomials share a root iff the curve lies on the supporting line. Their
// exact GCD proves separation without a coplanarity or floating residual test.
// Rational recovered roots are clipped and differentiated exactly. No general
// algebraic point representation or coincident overlap trimming is claimed.
func IntersectExactCurveSegment(ctx context.Context, c *ExactSplineCurve, start, end [3]*big.Rat, o ExactCurveSegmentOptions) (r ExactCurveSegmentIntersection, err error) {
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
		sort.SliceStable(r.Hits, func(i, j int) bool {
			cmp := r.Hits[i].Parameter.Cmp(r.Hits[j].Parameter)
			if cmp == 0 {
				return r.Hits[i].Span.Index < r.Hits[j].Span.Index
			}
			return cmp < 0
		})
	}()
	var direction [3]*big.Rat
	pivot := -1
	for axis := 0; axis < 3; axis++ {
		if start[axis] == nil || end[axis] == nil {
			return r, ErrInvalid
		}
		if e := budget.check(start[axis]); e != nil {
			return r, e
		}
		if e := budget.check(end[axis]); e != nil {
			return r, e
		}
		direction[axis] = new(big.Rat).Sub(end[axis], start[axis])
		if e := budget.check(direction[axis]); e != nil {
			return r, e
		}
		if direction[axis].Sign() != 0 && pivot < 0 {
			pivot = axis
		}
	}
	if pivot < 0 {
		return r, ErrDegenerate
	}
	outside := func(net []exactHomogeneous) (bool, error) {
		below, above := true, true
		for _, h := range net {
			s := new(big.Rat).Quo(new(big.Rat).Sub(h[pivot], new(big.Rat).Mul(start[pivot], h[3])), new(big.Rat).Mul(direction[pivot], h[3]))
			if e := budget.check(s); e != nil {
				return false, e
			}
			below = below && s.Sign() < 0
			above = above && s.Cmp(big.NewRat(1, 1)) > 0
		}
		return below || above, nil
	}
	stack := []*SurfacePatch{c.patch}
	index := 0
	domainLo, domainHi := c.Domain()
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if knot, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
			a, b, e := p.split(knot, true, budget)
			if e != nil {
				return r, e
			}
			stack = append(stack, b, a)
			continue
		}
		d := p.Domain()
		span := ParameterSpan{Start: d.UMin, End: d.UMax, Index: index}
		index++
		net := make([]exactHomogeneous, p.countU)
		for i := range net {
			net[i] = p.net[i*p.countV]
		}
		excluded, e := outside(net)
		if e != nil {
			return r, e
		}
		if excluded {
			continue
		}
		var common rationalPolynomial
		for axis := 0; axis < 3; axis++ {
			if axis == pivot {
				continue
			}
			coeff := make([]*big.Rat, len(net))
			for i, h := range net {
				delta := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(start[axis], h[3]))
				base := new(big.Rat).Sub(h[pivot], new(big.Rat).Mul(start[pivot], h[3]))
				runtime.KeepAlive(&h) // golang/go#82026: keeps h in the stack map. Remove once the Go fix is in use.
				coeff[i] = new(big.Rat).Sub(new(big.Rat).Mul(delta, direction[pivot]), new(big.Rat).Mul(base, direction[axis]))
				if e := budget.check(coeff[i]); e != nil {
					return r, e
				}
			}
			polynomial, e := bernsteinToPower(coeff, budget)
			if e != nil {
				return r, e
			}
			common, e = polynomialGCD(common, polynomial, budget)
			if e != nil {
				return r, e
			}
		}
		if len(common) == 0 {
			r.Coincident = append(r.Coincident, span)
			continue
		}
		if len(common) == 1 {
			continue
		}
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(common, new(big.Rat), big.NewRat(1, 1), budget, remaining, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			candidate, e := recoverRationalPolynomialRoot(common, lo, hi, budget)
			if e != nil {
				return false, e
			}
			if candidate == nil {
				restricted, e := restrictHomogeneousBezier(net, lo, hi, budget)
				if e != nil {
					return false, e
				}
				excluded, e := outside(restricted)
				if e != nil {
					return false, e
				}
				if excluded {
					return true, nil
				}
				lower, upper := parameterMap(span, lo), parameterMap(span, hi)
				if new(big.Rat).Sub(upper, lower).Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) > 0 {
					return false, nil
				}
				r.Pending = append(r.Pending, ExactCurveSegmentPending{lower, upper, span, m})
				return true, nil
			}
			parameter := parameterMap(span, candidate)
			point, jet, e := exactSurfaceJet(p, parameter, new(big.Rat), budget)
			if e != nil {
				return false, e
			}
			s := new(big.Rat).Quo(new(big.Rat).Sub(point[pivot], start[pivot]), direction[pivot])
			if e := budget.check(s); e != nil {
				return false, e
			}
			if s.Sign() < 0 || s.Cmp(big.NewRat(1, 1)) > 0 {
				return true, nil
			}
			hit := ExactCurveSegmentHit{Parameter: parameter, SegmentParameter: s, Point: point, Span: span, Multiplicity: m}
			nonzero, parallel := false, true
			for axis := 0; axis < 3; axis++ {
				expected := new(big.Rat).Add(start[axis], new(big.Rat).Mul(s, direction[axis]))
				if e := budget.check(expected); e != nil {
					return false, e
				}
				if expected.Cmp(point[axis]) != 0 {
					return false, ErrConvergence
				}
				nonzero = nonzero || jet[axis][0].Sign() != 0
				cross := new(big.Rat).Sub(new(big.Rat).Mul(jet[axis][0], direction[pivot]), new(big.Rat).Mul(jet[pivot][0], direction[axis]))
				if e := budget.check(cross); e != nil {
					return false, e
				}
				parallel = parallel && cross.Sign() == 0
			}
			hit.Singular = !nonzero
			hit.Tangent = nonzero && parallel
			hit.Transverse = nonzero && !parallel
			curveEnd := parameter.Cmp(new(big.Rat).SetFloat64(domainLo)) == 0 || parameter.Cmp(new(big.Rat).SetFloat64(domainHi)) == 0
			hit.SharedEndpoint = curveEnd && (s.Sign() == 0 || s.Cmp(big.NewRat(1, 1)) == 0)
			r.Hits = append(r.Hits, hit)
			return true, nil
		})
		r.Subdivisions += steps
		if e != nil {
			return r, e
		}
	}
	r.SupportingRootsComplete = true
	if len(r.Pending) > 0 || len(r.Coincident) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
