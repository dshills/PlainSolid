package geom

import (
	"context"
	"errors"
	"math/big"
)

// DiscoverAffineXYGraphArcTubes extends common-chart proposal discovery to
// independently scaled/reversed/permuted rectangular graph charts. Every
// returned tube is re-proved on original supports. Span stays in source arc
// units, while FreeAxis and Box use original ordered chart coordinates.
// Complete concerns only the requested arc slab; independent pair coverage and
// endpoint completeness remain required. Nonrepresented B-free endpoints stay
// pending, rather than rounding a requested terminal.
func discoverAffineXYGraphArcTubesOnce(ctx context.Context, arc SurfacePlaneArc, a, c *SurfacePatch, start, end float64, o SurfaceArcTubeDiscoveryOptions) (r SurfaceArcTubeDiscovery, err error) {
	processed := false
	defer func() {
		if err != nil && !processed {
			r.Tubes = nil
			r.Excluded = nil
			r.Pending = []ParameterSpan{{Start: start, End: end}}
			r.Complete = false
		}
	}()
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	alignment, e := AlignAffineXYCharts(ctx, a, c, SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	used := alignment.ExactOperations()
	defer func() { r.ExactOperations = used }()
	child := o
	child.MaxOperations -= used
	if child.MaxOperations < 1 {
		return r, ErrConvergence
	}
	proposals, e := DiscoverCommonXYGraphArcTubes(ctx, arc, a, alignment.target, start, end, child)
	used += proposals.ExactOperations
	r = proposals
	r.Tubes = nil
	r.Complete = false
	r.OriginalB = c.Domain()
	if e != nil && !errors.Is(e, ErrConvergence) {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: used}
	defer func() { used = budget.operations }()
	for _, candidate := range proposals.Tubes {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		left := o.MaxOperations - budget.operations
		if left < 1 {
			r.Pending = append(r.Pending, candidate.Span)
			return r, ErrConvergence
		}
		mapped, _, ops, e := alignment.MapSourceBox(ctx, candidate.Box.B, SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += ops
		if e != nil {
			r.Pending = append(r.Pending, candidate.Span)
			return r, e
		}
		free := candidate.FreeAxis
		if free >= 2 {
			sourceAxis := free - 2
			targetAxis := 0
			if alignment.mapping[0][sourceAxis].Sign() == 0 {
				targetAxis = 1
			}
			free = 2 + targetAxis
			valid := true
			sourceRange := [2]float64{candidate.Box.B.UMin, candidate.Box.B.UMax}
			if sourceAxis == 1 {
				sourceRange = [2]float64{candidate.Box.B.VMin, candidate.Box.B.VMax}
			}
			for _, x := range sourceRange {
				q := new(big.Rat).Add(new(big.Rat).Mul(alignment.mapping[targetAxis][sourceAxis], new(big.Rat).SetFloat64(x)), alignment.mapping[targetAxis][2])
				if e := budget.check(q); e != nil {
					return r, e
				}
				if _, e := RepresentedExactScalar(q); e != nil {
					valid = false
				}
			}
			if !valid {
				r.Pending = append(r.Pending, candidate.Span)
				continue
			}
		}
		d := c.Domain()
		mapped.UMin = max(mapped.UMin, d.UMin)
		mapped.UMax = min(mapped.UMax, d.UMax)
		mapped.VMin = max(mapped.VMin, d.VMin)
		mapped.VMax = min(mapped.VMax, d.VMax)
		pa, e := trimPatchBudget(a, candidate.Box.A, budget)
		if e != nil {
			return r, e
		}
		pb, e := trimPatchBudget(c, mapped, budget)
		if e != nil {
			return r, e
		}
		left = o.MaxOperations - budget.operations
		if left < 1 {
			return r, ErrConvergence
		}
		proof, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: free, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += proof.Test.ExactOperations
		r.Attempts++
		if e != nil && !errors.Is(e, ErrDomain) && !errors.Is(e, ErrDegenerate) && !errors.Is(e, ErrConvergence) {
			return r, e
		}
		if e != nil || !proof.ProvenBranch {
			r.Pending = append(r.Pending, candidate.Span)
			continue
		}
		candidate.Boundary = nil
		candidate.Box.B = mapped
		candidate.FreeAxis = free
		candidate.Proof = VerifiedSurfaceTube{a: pa, b: pb, axis: free, tangentSign: proof.FreeTangentSign}
		r.Tubes = append(r.Tubes, candidate)
	}
	processed = true
	r.Complete = len(r.Pending) == 0
	if !r.Complete {
		return r, ErrConvergence
	}
	return r, nil
}

// DiscoverAffineXYGraphArcTubes adaptively retries only unproved original-chart
// slabs. Accepted tubes are retained. All attempts share the operation and
// visited-interval budgets, with an additional depth bound for retries.
func DiscoverAffineXYGraphArcTubes(ctx context.Context, arc SurfacePlaneArc, a, c *SurfacePatch, start, end float64, o SurfaceArcTubeDiscoveryOptions) (r SurfaceArcTubeDiscovery, err error) {
	if ctx == nil || a == nil || c == nil || !finite(start) || !finite(end) || start >= end {
		return r, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 10
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 256
	}
	if o.MaxOperations < 1 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxIntervals < 1 {
		return r, ErrInvalid
	}
	type slab struct {
		span  ParameterSpan
		depth int
	}
	stack := []slab{{ParameterSpan{Start: start, End: end}, 0}}
	r.OriginalA, r.OriginalB = a.Domain(), c.Domain()
	if arc.source != nil {
		r.ArcDomain = arc.source.Domain()
	}
	preserve := func(n slab) {
		r.Pending = append(r.Pending, n.span)
		for _, pending := range stack {
			r.Pending = append(r.Pending, pending.span)
		}
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if e := ctx.Err(); e != nil {
			preserve(n)
			return r, e
		}
		child := o
		child.MaxOperations -= r.ExactOperations
		child.MaxIntervals -= r.VisitedIntervals
		child.MaxDepth -= n.depth
		if child.MaxOperations < 1 || child.MaxIntervals < 1 || child.MaxDepth < 1 {
			preserve(n)
			return r, ErrConvergence
		}
		report, e := discoverAffineXYGraphArcTubesOnce(ctx, arc, a, c, n.span.Start, n.span.End, child)
		r.ExactOperations += report.ExactOperations
		r.VisitedIntervals += report.VisitedIntervals
		r.Attempts += report.Attempts
		for _, tube := range report.Tubes {
			tube.Span.Index = len(r.Tubes)
			r.Tubes = append(r.Tubes, tube)
		}
		r.Excluded = append(r.Excluded, report.Excluded...)
		if e != nil && !errors.Is(e, ErrConvergence) {
			preserve(n)
			return r, e
		}
		if e != nil && report.VisitedIntervals == 0 {
			preserve(n)
			return r, e
		}
		if report.Complete {
			continue
		}
		if len(report.Pending) == 0 {
			preserve(n)
			return r, ErrConvergence
		}
		for _, pending := range report.Pending {
			mid := pending.Start/2 + pending.End/2
			if n.depth+1 >= o.MaxDepth || mid <= pending.Start || mid >= pending.End {
				r.Pending = append(r.Pending, pending)
				continue
			}
			stack = append(stack, slab{ParameterSpan{Start: mid, End: pending.End}, n.depth + 1}, slab{ParameterSpan{Start: pending.Start, End: mid}, n.depth + 1})
		}
	}
	r.Complete = len(r.Pending) == 0
	if !r.Complete {
		return r, ErrConvergence
	}
	return r, nil
}
