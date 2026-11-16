package geom

import (
	"container/heap"
	"context"
	"math/big"
	"math/bits"
)

// NormalOffsetSweepMomentOptions bounds the interval widths in model
// units for the physical volume, each centroid coordinate, each second
// moment and each entry of the two inertia tensors. All four widths must
// be positive. First moments are also enclosed. Zero resource limits
// select 24 levels, 4095 visited cells and four million exact operations.
type NormalOffsetSweepMomentOptions struct {
	VolumeTolerance, CentroidTolerance, SecondMomentTolerance, InertiaTolerance *big.Rat
	Patch                                                                       SurfacePatchOptions
	MaxDepth, MaxCells                                                          int
}

// NormalOffsetSweepMassMoments encloses the unit-density material moments
// of the analytic sweep. FirstMoment is the integral of x; SecondMoment
// is the world-origin tensor integral of x*xᵀ. InertiaOrigin and
// InertiaCentroid are conventional inertia tensors. Bounds presented on
// ErrConvergence remain valid but may exceed the requested widths.
// Numerical bounds are only published after proving positive volume.
type NormalOffsetSweepMassMoments struct {
	Volume                      ExactParameterInterval
	FirstMoment, Centroid       [3]ExactParameterInterval
	SecondMoment, InertiaOrigin [3][3]ExactParameterInterval
	InertiaCentroid             [3][3]ExactParameterInterval
	Complete                    bool
	ActiveCells, VisitedCells   int
	ExactOperations             int
}

// MassMomentsRational integrates the original analytic normal sweep at
// unit density. It integrates displacement algebraically through degree
// four and encloses the remaining UV integrals by directed subdivision.
// Centroid and centroidal inertia use a local reference before conversion
// to model coordinates. Cancellation returns no numerical bounds.
func (s *VerifiedNormalOffsetSweep) MassMomentsRational(ctx context.Context, o NormalOffsetSweepMomentOptions) (r NormalOffsetSweepMassMoments, err error) {
	if ctx == nil || s == nil || s.source == nil ||
		o.MaxDepth < 0 || o.MaxDepth > 64 || o.MaxCells < 0 || o.MaxCells > 1<<18 {
		return r, ErrInvalid
	}
	tolerances := [4]*big.Rat{o.VolumeTolerance, o.CentroidTolerance, o.SecondMomentTolerance, o.InertiaTolerance}
	for _, q := range tolerances {
		if q == nil || q.Sign() <= 0 {
			return r, ErrInvalid
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4095
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 4_000_000
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return r, err
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	precision := 64
	for _, q := range tolerances {
		if err := budget.check(q); err != nil {
			return r, err
		}
		precision = max(precision, q.Denom().BitLen()-q.Num().BitLen()+bits.Len(uint(o.MaxCells))+16)
	}
	root := *s.source
	root.options.MaxPoles = min(root.options.MaxPoles, limits.MaxPoles)
	if len(root.net) > root.options.MaxPoles {
		return r, ErrConvergence
	}
	w := sweepMomentWork{
		options: o, budget: budget, sweep: s, precision: precision,
	}
	if err := w.setup(ctx, &root); err != nil {
		return r, err
	}
	return w.integrate(ctx, &root)
}

const sweepMomentCount = 10

type sweepRawMoments [sweepMomentCount]exactInterval

var sweepMomentTensorAxes = [6][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 1}, {1, 2}, {2, 2}}

type sweepMomentCell struct {
	patch    *SurfacePatch
	value    sweepRawMoments
	priority *big.Rat
	depth    int
}

type sweepMomentWork struct {
	options         NormalOffsetSweepMomentOptions
	budget          *polynomialBudget
	sweep           *VerifiedNormalOffsetSweep
	factor          *big.Int
	precision       int
	reference       [3]*big.Rat
	displacement    [5]*big.Rat
	extension       *big.Rat
	visited, active int
}

func (w *sweepMomentWork) setup(ctx context.Context, root *SurfacePatch) error {
	a := boundedRatArithmetic{b: w.budget}
	two := big.NewRat(2, 1)
	var hull [3]exactInterval
	for _, h := range root.net {
		if err := ctx.Err(); err != nil {
			return err
		}
		for i := range hull {
			coordinate := a.quo(h[i], h[3])
			if a.err != nil {
				return a.err
			}
			if hull[i].lo == nil || coordinate.Cmp(hull[i].lo) < 0 {
				hull[i].lo = coordinate
			}
			if hull[i].hi == nil || coordinate.Cmp(hull[i].hi) > 0 {
				hull[i].hi = coordinate
			}
		}
	}
	for i, q := range hull {
		w.reference[i] = a.quo(a.add(q.lo, q.hi), two)
	}
	l, h := w.sweep.lower, w.sweep.upper
	if err := w.budget.check(l); err != nil {
		return err
	}
	if err := w.budget.check(h); err != nil {
		return err
	}
	w.extension = new(big.Rat).Abs(l)
	if x := new(big.Rat).Abs(h); x.Cmp(w.extension) > 0 {
		w.extension = x
	}
	lp, hp := l, h
	for i := range w.displacement {
		w.displacement[i] = a.quo(a.sub(hp, lp), big.NewRat(int64(i+1), 1))
		if i+1 < len(w.displacement) {
			lp, hp = a.mul(lp, l), a.mul(hp, h)
		}
	}
	return a.err
}

func (w *sweepMomentWork) integrate(ctx context.Context, root *SurfacePatch) (r NormalOffsetSweepMassMoments, err error) {
	defer func() {
		r.ActiveCells, r.VisitedCells = w.active, w.visited
		if ctx.Err() != nil {
			r = NormalOffsetSweepMassMoments{ActiveCells: w.active, VisitedCells: w.visited}
			err = ctx.Err()
		}
	}()
	exact, affine, err := w.affineCell(ctx, root)
	if err != nil {
		return r, err
	}
	if affine {
		w.active, w.visited = 1, 1
		r, err = w.derive(exact)
		if err != nil {
			return r, err
		}
		complete, err := w.complete(r)
		if err != nil {
			return r, err
		}
		if !complete {
			return r, ErrConvergence
		}
		r.Complete = true
		return r, nil
	}
	if w.precision >= w.budget.maxBits {
		return r, ErrConvergence
	}
	w.factor = new(big.Int).Lsh(big.NewInt(1), uint(w.precision))
	first, err := w.estimate(ctx, root, 0)
	if err != nil {
		return r, err
	}
	sum, best := first.value, first.value
	w.active = 1
	queue := sweepMomentQueue{first}
	for {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if best[0].lo.Sign() > 0 {
			candidate, err := w.derive(best)
			if err != nil {
				return r, err
			}
			r = candidate
			complete, err := w.complete(r)
			if err != nil {
				return r, err
			}
			if complete {
				r.Complete = true
				return r, nil
			}
		}
		if len(queue) == 0 || w.visited > w.options.MaxCells-2 {
			return r, ErrConvergence
		}
		parent := heap.Pop(&queue).(sweepMomentCell)
		if parent.depth >= w.options.MaxDepth || parent.priority.Sign() == 0 {
			continue
		}
		mid, u, ok := sweepMomentSplit(parent)
		if !ok {
			continue
		}
		left, right, err := parent.patch.split(mid, u, w.budget)
		if err != nil {
			return r, err
		}
		lc, err := w.estimate(ctx, left, parent.depth+1)
		if err != nil {
			return r, err
		}
		rc, err := w.estimate(ctx, right, parent.depth+1)
		if err != nil {
			return r, err
		}
		sum, best, err = w.refined(sum, best, parent.value, lc.value, rc.value)
		if err != nil {
			return r, err
		}
		w.active++
		heap.Push(&queue, lc)
		heap.Push(&queue, rc)
	}
}

func (w *sweepMomentWork) estimate(ctx context.Context, p *SurfacePatch, depth int) (sweepMomentCell, error) {
	w.visited++
	q, err := w.curvedCell(ctx, p)
	if err != nil {
		return sweepMomentCell{}, err
	}
	o := w.options
	second := o.SecondMomentTolerance
	if o.InertiaTolerance.Cmp(second) < 0 {
		second = o.InertiaTolerance
	}
	a := boundedRatArithmetic{b: w.budget}
	priority := new(big.Rat)
	for i, v := range q {
		if v.lo.Cmp(v.hi) != 0 || depth != 0 {
			q[i], err = roundSweepVolumeBounds(v, w.factor, w.budget)
			if err != nil {
				return sweepMomentCell{}, err
			}
		}
		tolerance := second
		switch {
		case i == 0:
			tolerance = o.VolumeTolerance
			q[i].lo = maxSweepMomentZero(q[i].lo)
		case i < 4:
			tolerance = o.CentroidTolerance
		}
		width := a.quo(a.sub(q[i].hi, q[i].lo), tolerance)
		if a.err != nil {
			return sweepMomentCell{}, a.err
		}
		if width.Cmp(priority) > 0 {
			priority = width
		}
	}
	return sweepMomentCell{patch: p, value: q, priority: priority, depth: depth}, nil
}

func (w *sweepMomentWork) refined(sum, best, parent, left, right sweepRawMoments) (sweepRawMoments, sweepRawMoments, error) {
	a := boundedRatArithmetic{b: w.budget}
	for i := range sum {
		// Replace the contribution by subtracting like bounds of the
		// parent from the accumulated independent cell bounds.
		lo := a.add(a.sub(sum[i].lo, parent[i].lo), a.add(left[i].lo, right[i].lo))
		hi := a.add(a.sub(sum[i].hi, parent[i].hi), a.add(left[i].hi, right[i].hi))
		if a.err != nil {
			return sum, best, a.err
		}
		sum[i] = exactInterval{lo, hi}
		if lo.Cmp(best[i].lo) > 0 {
			best[i].lo = lo
		}
		if hi.Cmp(best[i].hi) < 0 {
			best[i].hi = hi
		}
		if best[i].lo.Cmp(best[i].hi) > 0 {
			return sum, best, ErrConvergence
		}
	}
	return sum, best, nil
}

func sweepMomentSplit(c sweepMomentCell) (mid float64, u, ok bool) {
	d := c.patch.Domain()
	mu, mv := parameterMidpoint(d.UMin, d.UMax), parameterMidpoint(d.VMin, d.VMax)
	canU, canV := d.UMin < mu && mu < d.UMax, d.VMin < mv && mv < d.VMax
	if canU && (c.depth%2 == 0 || !canV) {
		return mu, true, true
	}
	return mv, false, canV
}

func maxSweepMomentZero(q *big.Rat) *big.Rat {
	if q.Sign() < 0 {
		return new(big.Rat)
	}
	return q
}

type sweepMomentQueue []sweepMomentCell

func (q sweepMomentQueue) Len() int           { return len(q) }
func (q sweepMomentQueue) Less(i, j int) bool { return q[i].priority.Cmp(q[j].priority) > 0 }
func (q sweepMomentQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *sweepMomentQueue) Push(v any)        { *q = append(*q, v.(sweepMomentCell)) }
func (q *sweepMomentQueue) Pop() any {
	i := len(*q) - 1
	x := (*q)[i]
	(*q)[i] = sweepMomentCell{}
	*q = (*q)[:i]
	return x
}
