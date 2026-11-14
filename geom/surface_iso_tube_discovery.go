package geom

import (
	"context"
	"errors"
	"math"
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
	if !finite(start) || !finite(end) {
		return r, ErrInvalid
	}
	return discoverSurfaceIsoPlaneRationalTubes(ctx, a, b, fixedAxis, fixed, new(big.Rat).SetFloat64(start), new(big.Rat).SetFloat64(end), o, false)
}

// DiscoverSurfaceIsoPlaneRationalTubes preserves exact original source endpoints.
// Represented enclosing windows are used only for conservative bounds. Every
// accepted free-axis endpoint is losslessly represented and the original patches
// reprove its tube. ExactSourceSpan records the requested source interval;
// Span is only its outward represented enclosure. Completeness concerns this
// iso only, never global pair coverage. The complete requested interval must
// pass exact target-chart clipping; coincident chart-boundary spans fail closed.
// Errors may retain diagnostic proposals, but never Complete.
func DiscoverSurfaceIsoPlaneRationalTubes(ctx context.Context, a, b *SurfacePatch, fixedAxis int, fixed float64, start, end *big.Rat, o SurfaceArcTubeDiscoveryOptions) (r SurfaceArcTubeDiscovery, err error) {
	return discoverSurfaceIsoPlaneRationalTubes(ctx, a, b, fixedAxis, fixed, start, end, o, true)
}
func discoverSurfaceIsoPlaneRationalTubes(ctx context.Context, a, b *SurfacePatch, fixedAxis int, fixed float64, start, end *big.Rat, o SurfaceArcTubeDiscoveryOptions, requireClip bool) (r SurfaceArcTubeDiscovery, err error) {
	if ctx == nil || a == nil || b == nil || fixedAxis < 0 || fixedAxis > 1 || !finite(fixed) || start == nil || end == nil || start.Cmp(end) >= 0 {
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
	if e := budget.check(start); e != nil {
		return r, e
	}
	if e := budget.check(end); e != nil {
		return r, e
	}
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
	if start.Cmp(new(big.Rat).SetFloat64(lo)) < 0 || end.Cmp(new(big.Rat).SetFloat64(hi)) > 0 {
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
	if requireClip {
		left, e = remaining()
		if e != nil {
			return r, e
		}
		clip, e := ClipIsoCurveToAffineChart(ctx, curve, chart, SplinePlaneOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += clip.Operations
		if e != nil {
			return r, e
		}
		if !clip.Complete || len(clip.CoincidentBoundary) > 0 {
			return r, ErrConvergence
		}
		cursor := new(big.Rat).Set(start)
		for _, span := range clip.Spans {
			if span.Start.Parameter.Cmp(cursor) > 0 {
				break
			}
			if span.End.Parameter.Cmp(cursor) > 0 {
				cursor.Set(span.End.Parameter)
			}
			if cursor.Cmp(end) >= 0 {
				break
			}
		}
		if cursor.Cmp(end) < 0 {
			return r, ErrDomain
		}
	}
	projected, e := projectIsoCurveChart(ctx, curve, chart, budget)
	if e != nil {
		return r, e
	}
	r.OriginalA, r.OriginalB, r.ArcDomain = a.Domain(), b.Domain(), a.Domain()
	free := 1 - fixedAxis
	type node struct {
		lo, hi *big.Rat
		depth  int
	}
	stack := []node{{new(big.Rat).Set(start), new(big.Rat).Set(end), 0}}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		left, e := remaining()
		if e != nil {
			return r, e
		}
		if r.VisitedIntervals >= o.MaxIntervals {
			r.Pending = append(r.Pending, isoRationalEnclosure(n.lo, n.hi))
			for _, pending := range stack {
				r.Pending = append(r.Pending, isoRationalEnclosure(pending.lo, pending.hi))
			}
			return r, ErrConvergence
		}
		r.VisitedIntervals++
		window := isoRationalEnclosure(n.lo, n.hi)
		sub, e := projected.Restrict(window.Start, window.End, SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		budget.operations += sub.operations
		bounds, _ := sub.patch.Bounds()
		domains := pairParameterRanges(SurfacePairParameterBox{a.Domain(), b.Domain()})
		base := [4][2]float64{{fixed, fixed}, {fixed, fixed}, {bounds.Min.X, bounds.Max.X}, {bounds.Min.Y, bounds.Max.Y}}
		base[free] = [2]float64{window.Start, window.End}
		if base[2][1] < domains[2][0] || base[2][0] > domains[2][1] || base[3][1] < domains[3][0] || base[3][0] > domains[3][1] {
			r.Excluded = append(r.Excluded, isoRationalEnclosure(n.lo, n.hi))
			continue
		}
		candidates := []int{}
		if _, e := exactRepresentableFloat(n.lo); e == nil {
			if _, e := exactRepresentableFloat(n.hi); e == nil {
				candidates = append(candidates, free)
			}
		}
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
		first, e := exactCurveRationalPoint(projected.patch, n.lo, budget)
		if e != nil {
			return r, e
		}
		last, e := exactCurveRationalPoint(projected.patch, n.hi, budget)
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
						padding = min((domains[i][1]-domains[i][0])/16, max((base[i][1]-base[i][0])/4, (window.End-window.Start)/4))
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
					r.Tubes = append(r.Tubes, SurfaceArcDiscoveredTube{FreeAxis: axis, Span: isoRationalEnclosure(n.lo, n.hi), ExactSourceSpan: [2]*big.Rat{new(big.Rat).Set(n.lo), new(big.Rat).Set(n.hi)}, Box: box, Proof: VerifiedSurfaceTube{a: pa, b: pb, axis: axis, tangentSign: proof.FreeTangentSign}})
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
		mid := new(big.Rat).SetFloat64(window.Start/2 + window.End/2)
		if n.depth >= o.MaxDepth || mid.Cmp(n.lo) <= 0 || mid.Cmp(n.hi) >= 0 {
			r.Pending = append(r.Pending, isoRationalEnclosure(n.lo, n.hi))
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

// Bounds only: never used as an exact parameter identity.
func isoRationalEnclosure(lo, hi *big.Rat) ParameterSpan {
	l, _ := lo.Float64()
	h, _ := hi.Float64()
	if new(big.Rat).SetFloat64(l).Cmp(lo) > 0 {
		l = math.Nextafter(l, math.Inf(-1))
	}
	if new(big.Rat).SetFloat64(h).Cmp(hi) < 0 {
		h = math.Nextafter(h, math.Inf(1))
	}
	return ParameterSpan{Start: l, End: h}
}
