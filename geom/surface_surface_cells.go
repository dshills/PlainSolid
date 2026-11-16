package geom

import "context"

// SurfaceSurfaceIsolationOptions bounds a four-parameter candidate covering.
// Zero fields select 0.05 parameter tolerance on all four axes, depth 32,
// 32768 pairs, 524288 queued poles, 2M exact operations and 16384 bits.
// MaxQueuedPoles counts referenced control nets per queued pair, including
// shared nets; it is a conservative allocation/work limit. Patch split limits
// continue to apply independently.
type SurfaceSurfaceIsolationOptions struct {
	ParameterTolerance                                                    float64
	MaxDepth, MaxPairs, MaxQueuedPoles, MaxOperations, MaxCoefficientBits int
	// SeparatingProjections uses exact projected hulls after AABB overlap.
	// The additional work counts against MaxOperations.
	SeparatingProjections bool
	// ConvexHullIterations enables an additional exact support search for
	// each overlapping pair. Zero keeps the prior search. Exact work also
	// counts against MaxOperations.
	ConvexHullIterations int
}

// SurfaceSurfaceCell covers possible contact between two parameter rectangles.
// Bounds is the intersection of outward control-hull AABBs, not proof of a
// common surface point. AtRequestedResolution applies to all four parameters.
type SurfaceSurfaceCell struct {
	DomainA, DomainB      ParameterRectangle
	Bounds                Box
	Depth                 int
	AtRequestedResolution bool
}

// SurfaceSurfaceIsolation is a conservative covering, not a curve intersection.
// Coincident contains pairs with exactly identical knot vectors and
// proportional homogeneous control nets, hence identical parameterizations.
// No geometric equality is inferred merely from overlapping or planar hulls.
// Complete means every possible contact is covered by resolved Candidates or
// proven Coincident pairs. Candidates may contain no intersection or whole
// overlapping surfaces. Pending preserves unprocessed contact on resource loss.
type SurfaceSurfaceIsolation struct {
	Candidates, Coincident, Pending []SurfaceSurfaceCell
	VisitedPairs, ExactOperations   int
	ProjectionExclusions            int
	ConvexHullExclusions            int
	Complete                        bool
}

func pairCell(a, b *SurfacePatch, depth int) (SurfaceSurfaceCell, bool) {
	bounds, ok := a.bounds.Intersect(b.bounds)
	return SurfaceSurfaceCell{DomainA: a.Domain(), DomainB: b.Domain(), Bounds: bounds, Depth: depth}, ok
}
func patchSplitChoice(p *SurfacePatch) (parameter float64, uDirection bool, knot bool) {
	if k, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
		return k, true, true
	}
	if k, ok := internalPatchKnot(p.knotsV, p.degreeV, p.countV); ok {
		return k, false, true
	}
	d := p.Domain()
	if d.UMax-d.UMin >= d.VMax-d.VMin {
		return parameterMidpoint(d.UMin, d.UMax), true, false
	}
	return parameterMidpoint(d.VMin, d.VMax), false, false
}

func parameterMidpoint(lo, hi float64) float64 {
	mid := lo + (hi-lo)/2
	if finite(mid) {
		return mid
	}
	// Finite opposite-signed endpoints can have a width that overflows.
	return lo/2 + hi/2
}
func patchParameterWidth(p *SurfacePatch) float64 {
	d := p.Domain()
	return max(d.UMax-d.UMin, d.VMax-d.VMin)
}

// BoundSurfaceSurface rejects pairs using strict hull-box separation and
// optionally exact separating projections. Touching faces, edges and points
// are retained, so tangent contacts are
// not lost. Existing knot lines are split before Bezier-cell bisection. A depth
// or work limit returns ErrConvergence with a covering in Candidates/Pending.
func BoundSurfaceSurface(a, b *SurfacePatch, options SurfaceSurfaceIsolationOptions) (result SurfaceSurfaceIsolation, err error) {
	return BoundSurfaceSurfaceContext(context.Background(), a, b, options)
}

// BoundSurfaceSurfaceContext is BoundSurfaceSurface with cancellation between
// bounded exact pair operations. Pending still covers unprocessed contact.
func BoundSurfaceSurfaceContext(ctx context.Context, a, b *SurfacePatch, options SurfaceSurfaceIsolationOptions) (result SurfaceSurfaceIsolation, err error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 {
		return result, ErrInvalid
	}
	o := options
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = .05
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 32768
	}
	if o.MaxQueuedPoles == 0 {
		o.MaxQueuedPoles = 524288
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxPairs < 1 || o.MaxQueuedPoles < 8 ||
		o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.ConvexHullIterations < 0 || o.ConvexHullIterations > 1024 {
		return result, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	type node struct {
		a, b  *SurfacePatch
		depth int
	}
	stack := []node{{a, b, 0}}
	queued := len(a.net) + len(b.net)
	pending := func(current *node) {
		if current != nil {
			if c, ok := pairCell(current.a, current.b, current.depth); ok {
				result.Pending = append(result.Pending, c)
			}
		}
		for _, n := range stack {
			if c, ok := pairCell(n.a, n.b, n.depth); ok {
				result.Pending = append(result.Pending, c)
			}
		}
	}
	if queued > o.MaxQueuedPoles {
		pending(nil)
		return result, ErrConvergence
	}
	unresolved := false
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			pending(nil)
			return result, e
		}
		if result.VisitedPairs >= o.MaxPairs {
			pending(nil)
			return result, ErrConvergence
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		queued -= len(n.a.net) + len(n.b.net)
		result.VisitedPairs++
		cell, overlap := pairCell(n.a, n.b, n.depth)
		if !overlap {
			continue
		}
		same, e := identicalSurfacePatches(ctx, n.a, n.b, budget)
		if e != nil {
			pending(&n)
			return result, e
		}
		if same {
			result.Coincident = append(result.Coincident, cell)
			continue
		}
		if o.SeparatingProjections {
			left := budget.maxOperations - budget.operations
			if left <= 0 {
				pending(&n)
				return result, ErrConvergence
			}
			p, e := SeparateSurfacePatches(n.a, n.b, SurfaceSeparationOptions{MaxOperations: left, MaxCoefficientBits: budget.maxBits})
			budget.operations += p.ExactOperations
			if e != nil {
				pending(&n)
				return result, e
			}
			if p.Separated {
				result.ProjectionExclusions++
				continue
			}
		}
		if o.ConvexHullIterations > 0 {
			left := budget.maxOperations - budget.operations
			if left <= 0 {
				pending(&n)
				return result, ErrConvergence
			}
			p, err := SeparateSurfacePatchesConvexHull(ctx, n.a, n.b, SurfaceConvexHullSeparationOptions{
				MaxIterations: o.ConvexHullIterations, MaxOperations: left, MaxCoefficientBits: budget.maxBits,
			})
			budget.operations += p.ExactOperations
			if err != nil {
				pending(&n)
				return result, err
			}
			if p.Separated {
				result.ConvexHullExclusions++
				continue
			}
		}
		ta, ua, ka := patchSplitChoice(n.a)
		tb, ub, kb := patchSplitChoice(n.b)
		wa, wb := patchParameterWidth(n.a), patchParameterWidth(n.b)
		if !ka && !kb && wa <= o.ParameterTolerance && wb <= o.ParameterTolerance {
			cell.AtRequestedResolution = true
			result.Candidates = append(result.Candidates, cell)
			continue
		}
		splitA := ka || (!kb && wa >= wb)
		p, t, u := n.a, ta, ua
		if !splitA {
			p, t, u = n.b, tb, ub
		}
		d := p.Domain()
		if n.depth >= o.MaxDepth || (u && (t == d.UMin || t == d.UMax)) || (!u && (t == d.VMin || t == d.VMax)) {
			result.Candidates = append(result.Candidates, cell)
			unresolved = true
			continue
		}
		l, r, e := p.split(t, u, budget)
		if e != nil {
			pending(&n)
			return result, e
		}
		left, right := node{l, n.b, n.depth + 1}, node{r, n.b, n.depth + 1}
		if !splitA {
			left, right = node{n.a, l, n.depth + 1}, node{n.a, r, n.depth + 1}
		}
		needed := len(left.a.net) + len(left.b.net) + len(right.a.net) + len(right.b.net)
		if needed > o.MaxQueuedPoles-queued {
			pending(&n)
			return result, ErrConvergence
		}
		queued += needed
		stack = append(stack, right, left)
	}
	if unresolved {
		return result, ErrConvergence
	}
	result.Complete = true
	return result, nil
}
