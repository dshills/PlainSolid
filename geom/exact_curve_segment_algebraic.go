package geom

import (
	"context"
	"errors"
	"math/big"
)

// ExactCurveSegmentAlgebraicHit proves a bounded-segment contact at the unique
// algebraic parameter in Lower/Upper. AtStart/AtEnd are exact model incidence,
// not interval overlap. Differential diagnostics are intentionally not inferred.
type ExactCurveSegmentAlgebraicHit struct {
	ExactCurveSegmentPending
	AtStart, AtEnd bool
	// SegmentLower/Upper bound the affine segment parameter in [0,1].
	SegmentLower, SegmentUpper *big.Rat
	// OtherLower/Upper are original affine-curve units when populated by
	// IntersectExactCurveAffineLineAlgebraic.
	OtherLower, OtherUpper *big.Rat
}

// ExactCurveSegmentAlgebraicIntersection extends rational Hits with proved
// Algebraic contacts. Pending contains only unresolved events. Coincident spans
// retain the original supporting-line meaning and prevent Complete.
type ExactCurveSegmentAlgebraicIntersection struct {
	ExactCurveSegmentIntersection
	Algebraic []ExactCurveSegmentAlgebraicHit
}

// IntersectExactCurveSegmentAlgebraic resolves algebraic supporting-line events
// using exact raw-plane sign at the root, including segment endpoint equality.
// A single incidence-plane root must be isolatable in each retained bracket;
// unsupported multiple roots stay Pending. Arithmetic budgets are cumulative.
func IntersectExactCurveSegmentAlgebraic(ctx context.Context, c *ExactSplineCurve, start, end [3]*big.Rat, o ExactCurveSegmentOptions) (r ExactCurveSegmentAlgebraicIntersection, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	initial, e := IntersectExactCurveSegment(ctx, c, start, end, o)
	r.ExactCurveSegmentIntersection = initial
	if e != nil && !errors.Is(e, ErrConvergence) {
		return r, e
	}
	if !initial.SupportingRootsComplete || len(initial.Pending) == 0 {
		return r, e
	}
	r.Pending = nil
	r.Complete = false
	var direction [3]*big.Rat
	pivot := -1
	for i := range direction {
		direction[i] = new(big.Rat).Sub(end[i], start[i])
		if pivot < 0 && direction[i].Sign() != 0 {
			pivot = i
		}
	}
	if pivot < 0 {
		return r, ErrDegenerate
	}
	var planes [][4]*big.Rat
	for axis := 0; axis < 3; axis++ {
		if axis == pivot {
			continue
		}
		var plane [4]*big.Rat
		for i := range plane {
			plane[i] = new(big.Rat)
		}
		plane[axis].Set(direction[pivot])
		plane[pivot].Neg(direction[axis])
		plane[3].Sub(new(big.Rat).Mul(direction[axis], start[pivot]), new(big.Rat).Mul(direction[pivot], start[axis]))
		planes = append(planes, plane)
	}
	var clips [2][4]*big.Rat
	for i, point := range [2][3]*big.Rat{start, end} {
		for j := range clips[i] {
			clips[i][j] = new(big.Rat)
		}
		clips[i][pivot].SetInt64(int64(direction[pivot].Sign()))
		clips[i][3].Neg(new(big.Rat).Mul(clips[i][pivot], point[pivot]))
	}
	for index, pending := range initial.Pending {
		if e := ctx.Err(); e != nil {
			r.Pending = append(r.Pending, initial.Pending[index:]...)
			return r, e
		}
		budget := &polynomialBudget{maxOperations: o.MaxOperations - r.Operations, maxBits: o.MaxCoefficientBits}
		if budget.maxOperations < 1 {
			r.Pending = append(r.Pending, initial.Pending[index:]...)
			return r, ErrConvergence
		}
		domain := c.patch.Domain()
		domain.UMin = pending.Span.Start
		domain.UMax = pending.Span.End
		p, e := trimPatchBudget(c.patch, domain, budget)
		r.Operations += budget.operations
		if e != nil {
			r.Pending = append(r.Pending, initial.Pending[index:]...)
			return r, e
		}
		local := &SurfaceIsoCurve{patch: p}
		resolved := false
		for _, plane := range planes {
			var proof [2]ExactCurvePlaneEvent
			valid := true
			for i := range proof {
				remaining, subdivisions := o.MaxOperations-r.Operations, o.MaxSubdivisions-r.Subdivisions
				if remaining < 1 || subdivisions < 1 {
					valid = false
					break
				}
				proof[i], e = ClassifyExactCurvePlaneEvent(ctx, local, plane, clips[i], pending.Lower, pending.Upper, ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: subdivisions})
				r.Operations += proof[i].Operations
				r.Subdivisions += proof[i].Subdivisions
				if e != nil {
					if !errors.Is(e, ErrConvergence) {
						r.Pending = append(r.Pending, initial.Pending[index:]...)
						return r, e
					}
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			resolved = true
			if proof[0].Side < 0 || proof[1].Side > 0 {
				break
			}
			lower, upper := pending.Lower, pending.Upper
			for _, q := range proof {
				if q.Lower.Cmp(lower) > 0 {
					lower = q.Lower
				}
				if q.Upper.Cmp(upper) < 0 {
					upper = q.Upper
				}
			}
			hit := pending
			hit.Lower = new(big.Rat).Set(lower)
			hit.Upper = new(big.Rat).Set(upper)
			boundBudget := &polynomialBudget{maxOperations: o.MaxOperations - r.Operations, maxBits: o.MaxCoefficientBits}
			if boundBudget.maxOperations < 1 {
				r.Pending = append(r.Pending, initial.Pending[index:]...)
				return r, ErrConvergence
			}
			spanLo := new(big.Rat).SetFloat64(pending.Span.Start)
			spanWidth := new(big.Rat).Sub(new(big.Rat).SetFloat64(pending.Span.End), spanLo)
			normalizedLo := new(big.Rat).Quo(new(big.Rat).Sub(lower, spanLo), spanWidth)
			normalizedHi := new(big.Rat).Quo(new(big.Rat).Sub(upper, spanLo), spanWidth)
			net := make([]exactHomogeneous, p.countU)
			for i := range net {
				net[i] = p.net[i*p.countV]
			}
			restricted, e := restrictHomogeneousBezier(net, normalizedLo, normalizedHi, boundBudget)
			var segmentLo, segmentHi *big.Rat
			if e == nil {
				for _, h := range restricted {
					v := new(big.Rat).Quo(new(big.Rat).Sub(h[pivot], new(big.Rat).Mul(start[pivot], h[3])), new(big.Rat).Mul(direction[pivot], h[3]))
					if e = boundBudget.check(v); e != nil {
						break
					}
					if segmentLo == nil || v.Cmp(segmentLo) < 0 {
						segmentLo = new(big.Rat).Set(v)
					}
					if segmentHi == nil || v.Cmp(segmentHi) > 0 {
						segmentHi = new(big.Rat).Set(v)
					}
				}
			}
			r.Operations += boundBudget.operations
			if e != nil {
				r.Pending = append(r.Pending, initial.Pending[index:]...)
				return r, e
			}
			if segmentLo.Sign() < 0 {
				segmentLo.SetInt64(0)
			}
			if segmentHi.Cmp(big.NewRat(1, 1)) > 0 {
				segmentHi.SetInt64(1)
			}
			if proof[0].Side == 0 {
				segmentLo.SetInt64(0)
				segmentHi.SetInt64(0)
			}
			if proof[1].Side == 0 {
				segmentLo.SetInt64(1)
				segmentHi.SetInt64(1)
			}
			r.Algebraic = append(r.Algebraic, ExactCurveSegmentAlgebraicHit{ExactCurveSegmentPending: hit, AtStart: proof[0].Side == 0, AtEnd: proof[1].Side == 0, SegmentLower: segmentLo, SegmentUpper: segmentHi})
			break
		}
		if !resolved {
			r.Pending = append(r.Pending, pending)
		}
	}
	if len(r.Pending) > 0 || len(r.Coincident) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
