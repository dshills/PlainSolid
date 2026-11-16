package geom

import (
	"container/heap"
	"context"
	"math"
	"math/big"
)

// SurfacePairDistanceOptions bounds pairwise global distance search. Zero limits
// select squared gap 1e-6, depth 32, 8192 pairs, 524288 queued poles, 4M exact
// operations and 16384 bits. SquaredTolerance is not a linear distance tolerance.
type SurfacePairDistanceOptions struct {
	SquaredTolerance                                                      float64
	MaxDepth, MaxPairs, MaxQueuedPoles, MaxOperations, MaxCoefficientBits int
}

// SurfacePairDistanceBounds encloses the global squared distance between patches.
// Witness boxes enclose two actual surface points, when HasWitness. Complete
// means the exact internal bound gap meets tolerance; outward float conversion
// may widen the reported gap. On limits, valid bounds survive, possibly [0,+Inf].
type SurfacePairDistanceBounds struct {
	SquaredDistanceMin, SquaredDistanceMax float64
	UVA, UVB                               Vec2
	PointA, PointB                         Box
	HasWitness, Complete                   bool
	VisitedPairs, ExactOperations          int
}

// RationalSurfacePairDistanceBounds also retains the unrounded global
// squared distance interval and both exact model sample points. Lower is
// available after the first cell bound, and Upper after the first witness.
type RationalSurfacePairDistanceBounds struct {
	SurfacePairDistanceBounds
	Squared            ExactParameterInterval
	WitnessA, WitnessB [3]*big.Rat
}
type surfacePairDistanceNode struct {
	a, c  *SurfacePatch
	lower *big.Rat
	depth int
}
type surfacePairDistanceHeap []surfacePairDistanceNode

func (h surfacePairDistanceHeap) Len() int           { return len(h) }
func (h surfacePairDistanceHeap) Less(i, j int) bool { return h[i].lower.Cmp(h[j].lower) < 0 }
func (h surfacePairDistanceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *surfacePairDistanceHeap) Push(v any)        { *h = append(*h, v.(surfacePairDistanceNode)) }
func (h *surfacePairDistanceHeap) Pop() any          { a := *h; v := a[len(a)-1]; *h = a[:len(a)-1]; return v }
func exactBoxPairDistance(a, c Box, b *polynomialBudget) (*big.Rat, error) {
	sum := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		delta := new(big.Rat)
		alo, ahi, clo, chi := coordinate(a.Min, axis), coordinate(a.Max, axis), coordinate(c.Min, axis), coordinate(c.Max, axis)
		if ahi < clo {
			delta.Sub(new(big.Rat).SetFloat64(clo), new(big.Rat).SetFloat64(ahi))
		} else if chi < alo {
			delta.Sub(new(big.Rat).SetFloat64(alo), new(big.Rat).SetFloat64(chi))
		}
		sum.Add(sum, new(big.Rat).Mul(delta, delta))
		if e := b.check(sum); e != nil {
			return nil, e
		}
	}
	return sum, nil
}

type exactDistanceWitness struct {
	point [3]*big.Rat
	uv    Vec2
	box   Box
}

func patchDistanceWitnesses(p *SurfacePatch, b *polynomialBudget) ([]exactDistanceWitness, error) {
	d := p.Domain()
	uv := Vec2{parameterMidpoint(d.UMin, d.UMax), parameterMidpoint(d.VMin, d.VMax)}
	current := p
	for {
		t, u, knot := patchSplitChoice(current)
		if !knot {
			break
		}
		left, right, e := current.split(t, u, b)
		if e != nil {
			return nil, e
		}
		x := uv.X
		if !u {
			x = uv.Y
		}
		if x < t {
			current = left
		} else {
			current = right
		}
	}
	point, _, e := exactSurfaceJet(current, new(big.Rat).SetFloat64(uv.X), new(big.Rat).SetFloat64(uv.Y), b)
	if e != nil {
		return nil, e
	}
	result := []exactDistanceWitness{{point: point, uv: uv}}
	for _, corner := range []struct {
		index int
		uv    Vec2
	}{{0, Vec2{d.UMin, d.VMin}}, {p.countV - 1, Vec2{d.UMin, d.VMax}}, {(p.countU - 1) * p.countV, Vec2{d.UMax, d.VMin}}, {len(p.net) - 1, Vec2{d.UMax, d.VMax}}} {
		h := p.net[corner.index]
		var point [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			point[axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := b.check(point[axis]); e != nil {
				return nil, e
			}
		}
		result = append(result, exactDistanceWitness{point: point, uv: corner.uv})
	}
	for i := range result {
		for axis, p := range result[i].point {
			lo, hi := rationalFloatBound(p, true), rationalFloatBound(p, false)
			switch axis {
			case 0:
				result[i].box.Min.X, result[i].box.Max.X = lo, hi
			case 1:
				result[i].box.Min.Y, result[i].box.Max.Y = lo, hi
			case 2:
				result[i].box.Min.Z, result[i].box.Max.Z = lo, hi
			}
		}
	}
	return result, nil
}

// BoundSurfacePairDistance maintains exact rational lower and witness upper
// bounds over the entire Cartesian product of parameter domains. Box distance
// and exact separating-projection gaps provide conservative lower bounds. Rejection
// is conservative even at contact. A best-first queue subdivides existing knot
// lines before parameter bisection. It does not infer intersection from zero
// lower bounds, nor require that the closest points are unique or interior.
func BoundSurfacePairDistance(a, c *SurfacePatch, o SurfacePairDistanceOptions) (result SurfacePairDistanceBounds, err error) {
	return boundSurfacePairDistanceMixed(a, c, o, false, false)
}

func boundSurfacePairDistance(a, c *SurfacePatch, o SurfacePairDistanceOptions, curveOnly bool) (result SurfacePairDistanceBounds, err error) {
	return boundSurfacePairDistanceMixed(a, c, o, curveOnly, curveOnly)
}

func boundSurfacePairDistanceMixed(a, c *SurfacePatch, o SurfacePairDistanceOptions, curveA, curveC bool) (result SurfacePairDistanceBounds, err error) {
	return boundSurfacePairDistanceContext(context.Background(), a, c, o, curveA, curveC)
}

// BoundSurfacePairDistanceContext cancels between bounded search steps and
// retains established global bounds and exact sample witnesses.
func BoundSurfacePairDistanceContext(ctx context.Context, a, c *SurfacePatch, o SurfacePairDistanceOptions) (SurfacePairDistanceBounds, error) {
	return boundSurfacePairDistanceContext(ctx, a, c, o, false, false)
}

func boundSurfacePairDistanceContext(ctx context.Context, a, c *SurfacePatch, o SurfacePairDistanceOptions, curveA, curveC bool) (result SurfacePairDistanceBounds, err error) {
	r, err := boundSurfacePairDistanceRational(ctx, a, c, nil, o, curveA, curveC)
	return r.SurfacePairDistanceBounds, err
}

// BoundSurfacePairDistanceRational uses exact control hulls and retains exact
// bounds even when the squared distance underflows in float64. The positive
// squaredTolerance replaces the float tolerance in options. On exhaustion,
// the returned enclosure and sample witnesses remain valid when available.
func BoundSurfacePairDistanceRational(ctx context.Context, a, c *SurfacePatch, squaredTolerance *big.Rat,
	o SurfacePairDistanceOptions) (RationalSurfacePairDistanceBounds, error) {
	if squaredTolerance == nil || squaredTolerance.Sign() <= 0 {
		return RationalSurfacePairDistanceBounds{SurfacePairDistanceBounds: SurfacePairDistanceBounds{SquaredDistanceMax: math.Inf(1)}}, ErrInvalid
	}
	return boundSurfacePairDistanceRational(ctx, a, c, squaredTolerance, o, false, false)
}

func boundSurfacePairDistanceRational(ctx context.Context, a, c *SurfacePatch, tolerance *big.Rat, o SurfacePairDistanceOptions,
	curveA, curveC bool) (result RationalSurfacePairDistanceBounds, err error) {
	result.SquaredDistanceMax = math.Inf(1)
	if ctx == nil {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return result, ErrInvalid
	}
	exactHull := tolerance != nil
	if !exactHull && o.SquaredTolerance == 0 {
		o.SquaredTolerance = 1e-6
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 8192
	}
	if o.MaxQueuedPoles == 0 {
		o.MaxQueuedPoles = 524288
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if (!exactHull && (!finite(o.SquaredTolerance) || o.SquaredTolerance <= 0)) || o.MaxDepth < 1 || o.MaxDepth > 128 ||
		o.MaxPairs < 1 || o.MaxQueuedPoles < 8 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	result.SquaredDistanceMax = math.Inf(1)
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	lowerBound := surfacePairDistanceLower
	if exactHull {
		lowerBound = exactSurfacePairDistanceLower
		if err := b.check(tolerance); err != nil {
			return result, err
		}
	} else {
		tolerance = new(big.Rat).SetFloat64(o.SquaredTolerance)
	}
	lower, e := lowerBound(a, c, b)
	if e != nil {
		return result, e
	}
	result.SquaredDistanceMin = rationalFloatBound(lower, true)
	if exactHull {
		result.Squared.Lower = lower
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	var upper *big.Rat
	sample := func(pa, pc *SurfacePatch) error {
		aw, e := patchDistanceWitnesses(pa, b)
		if e != nil {
			return e
		}
		cw, e := patchDistanceWitnesses(pc, b)
		if e != nil {
			return e
		}
		for _, x := range aw {
			for _, y := range cw {
				value := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					delta := new(big.Rat).Sub(x.point[axis], y.point[axis])
					value.Add(value, new(big.Rat).Mul(delta, delta))
					if e := b.check(value); e != nil {
						return e
					}
				}
				if upper == nil || value.Cmp(upper) < 0 {
					upper = value
					result.HasWitness = true
					result.UVA, result.UVB = x.uv, y.uv
					result.PointA, result.PointB = x.box, y.box
					result.SquaredDistanceMax = rationalFloatBound(upper, false)
					if exactHull {
						result.Squared.Upper = upper
						result.WitnessA, result.WitnessB = x.point, y.point
					}
				}
			}
		}
		return nil
	}
	if e := sample(a, c); e != nil {
		return result, e
	}
	queue := &surfacePairDistanceHeap{{a, c, lower, 0}}
	heap.Init(queue)
	queued := len(a.net) + len(c.net)
	var terminal *big.Rat
	minimum := func() *big.Rat {
		v := upper
		if queue.Len() > 0 && (*queue)[0].lower.Cmp(v) < 0 {
			v = (*queue)[0].lower
		}
		if terminal != nil && terminal.Cmp(v) < 0 {
			v = terminal
		}
		return v
	}
	finish := func() {
		lower := minimum()
		result.SquaredDistanceMin = rationalFloatBound(lower, true)
		result.SquaredDistanceMax = rationalFloatBound(upper, false)
		if exactHull {
			result.Squared = ExactParameterInterval{Lower: new(big.Rat).Set(lower), Upper: new(big.Rat).Set(upper)}
		}
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
		if new(big.Rat).Sub(upper, minimum()).Cmp(tolerance) <= 0 {
			finish()
			result.Complete = true
			return result, nil
		}
		if result.VisitedPairs >= o.MaxPairs {
			finish()
			return result, ErrConvergence
		}
		n := heap.Pop(queue).(surfacePairDistanceNode)
		queued -= len(n.a.net) + len(n.c.net)
		result.VisitedPairs++
		restore := func() { heap.Push(queue, n); queued += len(n.a.net) + len(n.c.net); finish() }
		if n.lower.Cmp(upper) >= 0 {
			continue
		}
		if e := sample(n.a, n.c); e != nil {
			restore()
			return result, e
		}
		if e := ctx.Err(); e != nil {
			restore()
			return result, e
		}
		ta, ua, ka := patchSplitChoice(n.a)
		tc, uc, kc := patchSplitChoice(n.c)
		if curveA {
			ua = true
			da := n.a.Domain()
			ta, ka = internalPatchKnot(n.a.knotsU, n.a.degreeU, n.a.countU)
			if !ka {
				ta = parameterMidpoint(da.UMin, da.UMax)
			}
		}
		if curveC {
			uc = true
			dc := n.c.Domain()
			tc, kc = internalPatchKnot(n.c.knotsU, n.c.degreeU, n.c.countU)
			if !kc {
				tc = parameterMidpoint(dc.UMin, dc.UMax)
			}
		}
		widthA, widthC := patchParameterWidth(n.a), patchParameterWidth(n.c)
		if curveA {
			da := n.a.Domain()
			widthA = da.UMax - da.UMin
		}
		if curveC {
			dc := n.c.Domain()
			widthC = dc.UMax - dc.UMin
		}
		splitA := ka || (!kc && widthA >= widthC)
		p, t, u := n.a, ta, ua
		if !splitA {
			p, t, u = n.c, tc, uc
		}
		d := p.Domain()
		if n.depth >= o.MaxDepth || (u && (t == d.UMin || t == d.UMax)) || (!u && (t == d.VMin || t == d.VMax)) {
			if terminal == nil || n.lower.Cmp(terminal) < 0 {
				terminal = n.lower
			}
			continue
		}
		left, right, e := p.split(t, u, b)
		if e != nil {
			restore()
			return result, e
		}
		children := []surfacePairDistanceNode{{left, n.c, nil, n.depth + 1}, {right, n.c, nil, n.depth + 1}}
		if !splitA {
			children = []surfacePairDistanceNode{{n.a, left, nil, n.depth + 1}, {n.a, right, nil, n.depth + 1}}
		}
		needed := 0
		for i := range children {
			needed += len(children[i].a.net) + len(children[i].c.net)
			children[i].lower, e = lowerBound(children[i].a, children[i].c, b)
			if e != nil {
				restore()
				return result, e
			}
		}
		if needed > o.MaxQueuedPoles-queued {
			restore()
			return result, ErrConvergence
		}
		for _, child := range children {
			if child.lower.Cmp(upper) < 0 {
				heap.Push(queue, child)
				queued += len(child.a.net) + len(child.c.net)
			}
		}
	}
	finish()
	if new(big.Rat).Sub(upper, minimum()).Cmp(tolerance) <= 0 {
		result.Complete = true
		return result, nil
	}
	return result, ErrConvergence
}

// A separating projection supplies gap²/||direction||² as a further exact
// Euclidean lower bound; its direction is chosen from control-box centers.
func surfacePairDistanceLower(a, c *SurfacePatch, b *polynomialBudget) (*big.Rat, error) {
	lower, e := exactBoxPairDistance(a.bounds, c.bounds, b)
	if e != nil {
		return nil, e
	}
	var direction [3]*big.Rat
	norm := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		as := new(big.Rat).Add(new(big.Rat).SetFloat64(coordinate(a.bounds.Min, axis)), new(big.Rat).SetFloat64(coordinate(a.bounds.Max, axis)))
		cs := new(big.Rat).Add(new(big.Rat).SetFloat64(coordinate(c.bounds.Min, axis)), new(big.Rat).SetFloat64(coordinate(c.bounds.Max, axis)))
		direction[axis] = new(big.Rat).Sub(cs, as)
		norm.Add(norm, new(big.Rat).Mul(direction[axis], direction[axis]))
	}
	if e := b.check(norm); e != nil {
		return nil, e
	}
	if norm.Sign() == 0 {
		return lower, nil
	}
	pa, e := exactProjectionHull(a, a.bounds.Min, direction, b)
	if e != nil {
		return nil, e
	}
	pc, e := exactProjectionHull(c, a.bounds.Min, direction, b)
	if e != nil {
		return nil, e
	}
	gap := new(big.Rat).Sub(pc.lo, pa.hi)
	other := new(big.Rat).Sub(pa.lo, pc.hi)
	if other.Cmp(gap) > 0 {
		gap = other
	}
	if gap.Sign() > 0 {
		projected := new(big.Rat).Quo(new(big.Rat).Mul(gap, gap), norm)
		if e := b.check(projected); e != nil {
			return nil, e
		}
		if projected.Cmp(lower) > 0 {
			lower = projected
		}
	}
	return lower, nil
}

func exactSurfacePairDistanceLower(a, c *SurfacePatch, b *polynomialBudget) (*big.Rat, error) {
	axisRange := func(p *SurfacePatch, axis int) (exactInterval, error) {
		var out exactFractionRange
		var numerator, denominator big.Int
		for _, h := range p.net {
			numerator.Mul(h[axis].Num(), h[3].Denom())
			denominator.Mul(h[axis].Denom(), h[3].Num())
			if err := out.extend(&numerator, &denominator, b); err != nil {
				return exactInterval{}, err
			}
		}
		return out.interval(), nil
	}
	gapBetween := func(x, y exactInterval) *big.Rat {
		if x.hi.Cmp(y.lo) < 0 {
			return new(big.Rat).Sub(y.lo, x.hi)
		}
		if y.hi.Cmp(x.lo) < 0 {
			return new(big.Rat).Sub(x.lo, y.hi)
		}
		return new(big.Rat)
	}
	calc := boundedRatArithmetic{b: b}
	lower, norm := new(big.Rat), new(big.Rat)
	var direction [3]*big.Rat
	for axis := range direction {
		pa, err := axisRange(a, axis)
		if err != nil {
			return nil, err
		}
		pc, err := axisRange(c, axis)
		if err != nil {
			return nil, err
		}
		gap := gapBetween(pa, pc)
		lower = calc.add(lower, calc.mul(gap, gap))
		// The difference of the exact hull centers, scaled by two,
		// preserves even a sub-ULP separation in an oblique direction.
		direction[axis] = calc.sub(calc.add(pc.lo, pc.hi), calc.add(pa.lo, pa.hi))
		norm = calc.add(norm, calc.mul(direction[axis], direction[axis]))
	}
	if calc.err != nil || norm.Sign() == 0 {
		return lower, calc.err
	}
	pa, err := exactProjectionHull(a, Vec3{}, direction, b)
	if err != nil {
		return nil, err
	}
	pc, err := exactProjectionHull(c, Vec3{}, direction, b)
	if err != nil {
		return nil, err
	}
	gap := gapBetween(pa, pc)
	projected := calc.quo(calc.mul(gap, gap), norm)
	if calc.err != nil {
		return nil, calc.err
	}
	if projected.Cmp(lower) > 0 {
		return projected, nil
	}
	return lower, nil
}
