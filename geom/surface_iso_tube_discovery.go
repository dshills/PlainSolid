package geom

import (
	"context"
	"errors"
	"math/big"
)

// DiscoverSurfaceIsoPlaneTubes proves local tubes covering an exact
// isoparametric section known (and rechecked) to lie in an affine plane patch.
// The requested represented free interval must lie in A's domain. B-free axes
// are proposed only after proving strict monotonicity of the projected iso
// coordinate, with exact represented endpoints. Consequently plane-side exits
// can be tube endpoints even when the global surface projection is nonlinear.
// Complete covers this iso section only, not other pair-intersection branches.
func DiscoverSurfaceIsoPlaneTubes(ctx context.Context, a, b *SurfacePatch, fixedAxis int, fixed, start, end float64, o SurfaceArcTubeDiscoveryOptions) (r SurfaceArcTubeDiscovery, err error) {
	if ctx == nil || a == nil || b == nil || fixedAxis < 0 || fixedAxis > 1 || !finite(fixed) || !finite(start) || !finite(end) || start >= end {
		return r, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 10
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 256
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Padding) || o.Padding < 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxIntervals < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	remaining := func() (int, error) {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		n := o.MaxOperations - budget.operations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	left, e := remaining()
	if e != nil {
		return r, e
	}
	curve, e := a.IsoCurve(fixedAxis, fixed, SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	budget.operations += curve.operations
	lo, hi := curve.Domain()
	if start < lo || end > hi {
		return r, ErrDomain
	}
	left, e = remaining()
	if e != nil {
		return r, e
	}
	chart, e := NewAffineSurfaceChart(ctx, b, AffineSurfaceChartOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	budget.operations += chart.operations
	projected, e := projectIsoCurveChart(ctx, curve, chart, budget)
	if e != nil {
		return r, e
	}
	r.OriginalA, r.OriginalB, r.ArcDomain = a.Domain(), b.Domain(), a.Domain()
	free := 1 - fixedAxis
	type node struct {
		lo, hi float64
		depth  int
	}
	stack := []node{{start, end, 0}}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		left, e := remaining()
		if e != nil {
			return r, e
		}
		if r.VisitedIntervals >= o.MaxIntervals {
			r.Pending = append(r.Pending, ParameterSpan{Start: n.lo, End: n.hi})
			for _, pending := range stack {
				r.Pending = append(r.Pending, ParameterSpan{Start: pending.lo, End: pending.hi})
			}
			return r, ErrConvergence
		}
		r.VisitedIntervals++
		sub, e := projected.Restrict(n.lo, n.hi, SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		budget.operations += sub.operations
		bounds, _ := sub.patch.Bounds()
		domains := pairParameterRanges(SurfacePairParameterBox{a.Domain(), b.Domain()})
		base := [4][2]float64{{fixed, fixed}, {fixed, fixed}, {bounds.Min.X, bounds.Max.X}, {bounds.Min.Y, bounds.Max.Y}}
		base[free] = [2]float64{n.lo, n.hi}
		if base[2][1] < domains[2][0] || base[2][0] > domains[2][1] || base[3][1] < domains[3][0] || base[3][0] > domains[3][1] {
			r.Excluded = append(r.Excluded, ParameterSpan{Start: n.lo, End: n.hi})
			continue
		}
		candidates := []int{free}
		intervals := map[int][2]float64{}
		left, e = remaining()
		if e != nil {
			return r, e
		}
		derivatives, e := sub.BoundDerivativesContext(ctx, CurveDerivativeBoundOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += derivatives.ExactOperations
		if e != nil {
			return r, e
		}
		first, e := exactCurveRationalPoint(sub.patch, new(big.Rat).SetFloat64(n.lo), budget)
		if e != nil {
			return r, e
		}
		last, e := exactCurveRationalPoint(sub.patch, new(big.Rat).SetFloat64(n.hi), budget)
		if e != nil {
			return r, e
		}
		for axis := 0; axis < 2; axis++ {
			positive, negative := true, true
			for _, cell := range derivatives.Cells {
				positive = positive && cell.First[axis].Min > 0
				negative = negative && cell.First[axis].Max < 0
			}
			if !positive && !negative {
				continue
			}
			l, e := exactRepresentableFloat(first[axis])
			if e != nil {
				continue
			}
			h, e := exactRepresentableFloat(last[axis])
			if e != nil {
				continue
			}
			if h < l {
				l, h = h, l
			}
			intervals[axis+2] = [2]float64{l, h}
			candidates = append(candidates, axis+2)
		}
		success := false
		for _, axis := range candidates {
			for _, scale := range []float64{1, .5, 2} {
				proposal := base
				if axis >= 2 {
					proposal[axis] = intervals[axis]
				}
				for i := 0; i < 4; i++ {
					if i == axis {
						continue
					}
					padding := o.Padding
					if padding == 0 {
						padding = min((domains[i][1]-domains[i][0])/16, max((base[i][1]-base[i][0])/4, (n.hi-n.lo)/4))
					}
					proposal[i] = [2]float64{max(domains[i][0], base[i][0]-scale*padding), min(domains[i][1], base[i][1]+scale*padding)}
				}
				box := rangesParameterPair(proposal)
				if !box.A.Valid() || !box.B.Valid() {
					continue
				}
				pa, e := trimPatchBudget(a, box.A, budget)
				if e != nil {
					return r, e
				}
				pb, e := trimPatchBudget(b, box.B, budget)
				if e != nil {
					return r, e
				}
				left, e = remaining()
				if e != nil {
					return r, e
				}
				r.Attempts++
				proof, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
				budget.operations += proof.Test.ExactOperations
				if errors.Is(e, ErrDomain) || errors.Is(e, ErrDegenerate) {
					continue
				}
				if e != nil {
					return r, e
				}
				if proof.ProvenBranch {
					r.Tubes = append(r.Tubes, SurfaceArcDiscoveredTube{FreeAxis: axis, Span: ParameterSpan{Start: n.lo, End: n.hi}, Box: box, Proof: VerifiedSurfaceTube{a: pa, b: pb, axis: axis, tangentSign: proof.FreeTangentSign}})
					success = true
					break
				}
			}
			if success {
				break
			}
		}
		if success {
			continue
		}
		mid := n.lo/2 + n.hi/2
		if n.depth >= o.MaxDepth || mid <= n.lo || mid >= n.hi {
			r.Pending = append(r.Pending, ParameterSpan{Start: n.lo, End: n.hi})
			continue
		}
		stack = append(stack, node{mid, n.hi, n.depth + 1}, node{n.lo, mid, n.depth + 1})
	}
	if len(r.Pending) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
