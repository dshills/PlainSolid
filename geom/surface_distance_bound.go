package geom

import (
	"container/heap"
	"context"
	"math"
	"math/big"
)

// SurfaceDistanceOptions bounds certified global point/surface distance search.
// SquaredTolerance is the allowed upper-minus-lower squared-distance gap (not
// a linear distance tolerance); zero selects 1e-8. Zero limits select depth 32,
// 8192 cells, 262144 queued poles, 4M operations and 16384 coefficient bits.
type SurfaceDistanceOptions struct {
	SquaredTolerance                                                      float64
	MaxDepth, MaxCells, MaxQueuedPoles, MaxOperations, MaxCoefficientBits int
}

// SurfaceDistanceBounds encloses the exact global squared distance to a bounded
// patch. When HasUpperPoint, UpperPoint encloses an actual sampled point at
// UpperUV, whose exact
// squared distance supplies the upper bound; it need not be the minimizer.
// Complete means the exact internal squared gap meets the requested tolerance;
// final float64 outward conversion can widen the displayed gap by rounding. On
// resource exhaustion the returned bounds remain valid but may be wider.
type SurfaceDistanceBounds struct {
	HasUpperPoint                          bool
	SquaredDistanceMin, SquaredDistanceMax float64
	UpperUV                                Vec2
	UpperPoint                             Box
	Complete                               bool
	VisitedCells, ExactOperations          int
}
type distanceBoundNode struct {
	patch *SurfacePatch
	lower *big.Rat
	depth int
}
type distanceBoundHeap []distanceBoundNode

func (h distanceBoundHeap) Len() int           { return len(h) }
func (h distanceBoundHeap) Less(i, j int) bool { return h[i].lower.Cmp(h[j].lower) < 0 }
func (h distanceBoundHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *distanceBoundHeap) Push(x any)        { *h = append(*h, x.(distanceBoundNode)) }
func (h *distanceBoundHeap) Pop() any          { a := *h; n := a[len(a)-1]; *h = a[:len(a)-1]; return n }
func exactPointBoxDistance(query Vec3, box Box, b *polynomialBudget) (*big.Rat, error) {
	sum := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		q := coordinate(query, axis)
		lo, hi := coordinate(box.Min, axis), coordinate(box.Max, axis)
		difference := new(big.Rat)
		if q < lo {
			difference.Sub(new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(q))
		} else if q > hi {
			difference.Sub(new(big.Rat).SetFloat64(q), new(big.Rat).SetFloat64(hi))
		}
		sum.Add(sum, new(big.Rat).Mul(difference, difference))
		if e := b.check(sum); e != nil {
			return nil, e
		}
	}
	return sum, nil
}
func distancePatchSample(p *SurfacePatch, query Vec3, b *polynomialBudget) (value *big.Rat, uv Vec2, box Box, err error) {
	d := p.Domain()
	uv = Vec2{d.UMin + (d.UMax-d.UMin)/2, d.VMin + (d.VMax-d.VMin)/2}
	current := p
	for {
		t, u, knot := patchSplitChoice(current)
		if !knot {
			break
		}
		left, right, e := current.split(t, u, b)
		if e != nil {
			return nil, uv, box, e
		}
		parameter := uv.X
		if !u {
			parameter = uv.Y
		}
		if parameter < t {
			current = left
		} else {
			current = right
		}
	}
	point, _, e := exactSurfaceJet(current, new(big.Rat).SetFloat64(uv.X), new(big.Rat).SetFloat64(uv.Y), b)
	if e != nil {
		return nil, uv, box, e
	}
	value = new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		delta := new(big.Rat).Sub(point[axis], new(big.Rat).SetFloat64(coordinate(query, axis)))
		value.Add(value, new(big.Rat).Mul(delta, delta))
		if e := b.check(value); e != nil {
			return nil, uv, box, e
		}
		lo, hi := rationalFloatBound(point[axis], true), rationalFloatBound(point[axis], false)
		switch axis {
		case 0:
			box.Min.X, box.Max.X = lo, hi
		case 1:
			box.Min.Y, box.Max.Y = lo, hi
		case 2:
			box.Min.Z, box.Max.Z = lo, hi
		}
	}
	// Endpoint control points of clamped patches are exact surface corners.
	for _, corner := range []struct {
		index int
		uv    Vec2
	}{{0, Vec2{d.UMin, d.VMin}}, {p.countV - 1, Vec2{d.UMin, d.VMax}}, {(p.countU - 1) * p.countV, Vec2{d.UMax, d.VMin}}, {len(p.net) - 1, Vec2{d.UMax, d.VMax}}} {
		h := p.net[corner.index]
		candidate := new(big.Rat)
		var candidateBox Box
		for axis := 0; axis < 3; axis++ {
			q := new(big.Rat).Quo(h[axis], h[3])
			delta := new(big.Rat).Sub(q, new(big.Rat).SetFloat64(coordinate(query, axis)))
			candidate.Add(candidate, new(big.Rat).Mul(delta, delta))
			if e := b.check(candidate); e != nil {
				return nil, uv, box, e
			}
			lo, hi := rationalFloatBound(q, true), rationalFloatBound(q, false)
			switch axis {
			case 0:
				candidateBox.Min.X, candidateBox.Max.X = lo, hi
			case 1:
				candidateBox.Min.Y, candidateBox.Max.Y = lo, hi
			case 2:
				candidateBox.Min.Z, candidateBox.Max.Z = lo, hi
			}
		}
		if candidate.Cmp(value) < 0 {
			value, uv, box = candidate, corner.uv, candidateBox
		}
	}
	return value, uv, box, nil
}

// BoundPointSurfaceDistance uses exact point-to-outward-control-box lower bounds
// and exact rational surface-sample upper bounds. A best-first knot-aware search
// never replaces a certified bound with a floating residual estimate. Positive
// weights and bounded clamped patch domains are inherited from SurfacePatch.
func BoundPointSurfaceDistance(query Vec3, p *SurfacePatch, o SurfaceDistanceOptions) (result SurfaceDistanceBounds, err error) {
	return boundPointSurfaceDistance(query, p, o, false)
}

func boundPointSurfaceDistance(query Vec3, p *SurfacePatch, o SurfaceDistanceOptions, curveOnly bool) (result SurfaceDistanceBounds, err error) {
	return boundPointSurfaceDistanceContext(context.Background(), query, p, o, curveOnly)
}

// BoundPointSurfaceDistanceContext cooperatively cancels between bounded exact
// search steps, preserving established global bounds and any sample witness.
func BoundPointSurfaceDistanceContext(ctx context.Context, query Vec3, p *SurfacePatch, o SurfaceDistanceOptions) (SurfaceDistanceBounds, error) {
	return boundPointSurfaceDistanceContext(ctx, query, p, o, false)
}

func boundPointSurfaceDistanceContext(ctx context.Context, query Vec3, p *SurfacePatch, o SurfaceDistanceOptions, curveOnly bool) (result SurfaceDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	if ctx == nil {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if p == nil || len(p.net) == 0 || !query.IsFinite() {
		return result, ErrInvalid
	}
	if o.SquaredTolerance == 0 {
		o.SquaredTolerance = 1e-8
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxQueuedPoles == 0 {
		o.MaxQueuedPoles = 262144
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.SquaredTolerance) || o.SquaredTolerance <= 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxQueuedPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	result.SquaredDistanceMax = math.Inf(1)
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	lower, e := exactPointBoxDistance(query, p.bounds, b)
	if e != nil {
		return result, e
	}
	result.SquaredDistanceMin = rationalFloatBound(lower, true)
	if e := ctx.Err(); e != nil {
		return result, e
	}
	upper, uv, point, e := distancePatchSample(p, query, b)
	if e != nil {
		return result, e
	}
	result.HasUpperPoint = true
	result.UpperUV, result.UpperPoint = uv, point
	result.SquaredDistanceMax = rationalFloatBound(upper, false)
	queue := &distanceBoundHeap{{p, lower, 0}}
	heap.Init(queue)
	queued := len(p.net)
	tolerance := new(big.Rat).SetFloat64(o.SquaredTolerance)
	var terminal *big.Rat
	currentLower := func() *big.Rat {
		minimum := upper
		if queue.Len() > 0 && (*queue)[0].lower.Cmp(minimum) < 0 {
			minimum = (*queue)[0].lower
		}
		if terminal != nil && terminal.Cmp(minimum) < 0 {
			minimum = terminal
		}
		return minimum
	}
	finish := func() {
		result.SquaredDistanceMin = rationalFloatBound(currentLower(), true)
		result.SquaredDistanceMax = rationalFloatBound(upper, false)
	}
	if queued > o.MaxQueuedPoles {
		finish()
		return result, ErrConvergence
	}
	for queue.Len() > 0 {
		if e := ctx.Err(); e != nil {
			finish()
			return result, e
		}
		minimum := currentLower()
		gap := new(big.Rat).Sub(upper, minimum)
		if gap.Cmp(tolerance) <= 0 {
			finish()
			result.Complete = true
			return result, nil
		}
		if result.VisitedCells >= o.MaxCells {
			finish()
			return result, ErrConvergence
		}
		n := heap.Pop(queue).(distanceBoundNode)
		queued -= len(n.patch.net)
		result.VisitedCells++
		restore := func() { heap.Push(queue, n); queued += len(n.patch.net); finish() }
		if n.lower.Cmp(upper) >= 0 {
			continue
		}
		sample, uv, box, e := distancePatchSample(n.patch, query, b)
		if e != nil {
			restore()
			return result, e
		}
		if sample.Cmp(upper) < 0 {
			upper = sample
			result.UpperUV, result.UpperPoint = uv, box
		}
		if e := ctx.Err(); e != nil {
			restore()
			return result, e
		}
		t, u, _ := patchSplitChoice(n.patch)
		d := n.patch.Domain()
		if curveOnly {
			u = true
			if knot, ok := internalPatchKnot(n.patch.knotsU, n.patch.degreeU, n.patch.countU); ok {
				t = knot
			} else {
				t = d.UMin + (d.UMax-d.UMin)/2
			}
		}
		if n.depth >= o.MaxDepth || (u && (t == d.UMin || t == d.UMax)) || (!u && (t == d.VMin || t == d.VMax)) {
			if terminal == nil || n.lower.Cmp(terminal) < 0 {
				terminal = n.lower
			}
			continue
		}
		left, right, e := n.patch.split(t, u, b)
		if e != nil {
			restore()
			return result, e
		}
		if len(left.net)+len(right.net) > o.MaxQueuedPoles-queued {
			restore()
			return result, ErrConvergence
		}
		children := []distanceBoundNode{}
		for _, child := range []*SurfacePatch{left, right} {
			bound, e := exactPointBoxDistance(query, child.bounds, b)
			if e != nil {
				restore()
				return result, e
			}
			children = append(children, distanceBoundNode{child, bound, n.depth + 1})
		}
		for _, child := range children {
			if child.lower.Cmp(upper) < 0 {
				heap.Push(queue, child)
				queued += len(child.patch.net)
			}
		}
	}
	finish()
	if new(big.Rat).Sub(upper, currentLower()).Cmp(tolerance) <= 0 {
		result.Complete = true
		return result, nil
	}
	return result, ErrConvergence
}
