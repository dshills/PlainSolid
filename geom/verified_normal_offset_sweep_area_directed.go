package geom

import (
	"container/heap"
	"context"
	"math/big"
	"math/bits"
)

// NormalOffsetSweepAreaEnclosureOptions bounds adaptive integration over
// the six original analytic faces. MaxDepth defaults to 32. MaxCells and
// the patch limits apply to one common ledger across the complete boundary.
type NormalOffsetSweepAreaEnclosureOptions struct {
	NormalOffsetSweepAreaOptions
	MaxDepth int
}

// EncloseBoundaryAreaRational integrates directed derivative enclosures on
// the six original analytic faces of an embedded verified sweep. It retains
// individually proved faces and, after full boundary coverage, a valid
// coarse total if available work cannot meet the requested width.
func (s *VerifiedNormalOffsetSweep) EncloseBoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaEnclosureOptions) (r NormalOffsetSweepBoundaryArea, err error) {
	if s == nil || s.source == nil || o.MaxDepth < 0 || o.MaxDepth > 64 {
		return r, ErrInvalid
	}
	options, err := normalizeSweepBoundaryAreaOptions(ctx, o.NormalOffsetSweepAreaOptions)
	if err != nil {
		return r, err
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	b := &polynomialBudget{maxOperations: options.Patch.MaxOperations, maxBits: options.Patch.MaxCoefficientBits}
	root := *s.source
	root.options.MaxPoles = min(root.options.MaxPoles, options.Patch.MaxPoles)
	w := directedSweepAreaWork{budget: b, maxDepth: o.MaxDepth, maxCells: options.MaxCells}
	defer func() { w.publish(&r) }()
	if len(root.net) > root.options.MaxPoles {
		return r, ErrConvergence
	}
	for _, q := range [3]*big.Rat{options.AbsoluteError, s.lower, s.upper} {
		if err := b.check(q); err != nil {
			return r, err
		}
	}
	// A shared dyadic denominator bounds the accumulation cost and
	// allocates a small fraction of the requested width to rounding.
	target := options.AbsoluteError
	if target.Sign() == 0 {
		target = big.NewRat(1, 100_000_000)
	}
	precision := max(0, target.Denom().BitLen()-target.Num().BitLen()) + bits.Len(uint(options.MaxCells)) + 4
	if precision >= b.maxBits {
		return r, ErrConvergence
	}
	w.factor = new(big.Int).Lsh(big.NewInt(1), uint(precision))
	for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
		t := exactInterval{s.lower, s.upper}
		switch side {
		case NormalOffsetSweepLower:
			t.hi = t.lo
		case NormalOffsetSweepUpper:
			t.lo = t.hi
		}
		bounds, cells, err := w.cover(ctx, directedSweepAreaCell{patch: &root, displacement: t, side: side})
		if err != nil {
			return r, err
		}
		if err := w.record(side, bounds); err != nil {
			return r, err
		}
		w.queue = append(w.queue, cells...)
	}
	heap.Init(&w.queue)
	r.Complete, err = w.refine(ctx, options.AbsoluteError)
	return r, err
}

type directedSweepAreaWork struct {
	budget                     *polynomialBudget
	factor                     *big.Int
	maxDepth, maxCells, visits int
	raw, best                  [6]exactInterval
	total                      exactInterval
	queue                      directedSweepAreaQueue
}

type directedSweepAreaCell struct {
	patch        *SurfacePatch
	displacement exactInterval
	side         NormalOffsetSweepSide
	depth        int
	area         exactInterval
	width        *big.Rat
	splitLayer   bool
	flatCapAxes  uint8
}

func (w *directedSweepAreaWork) publish(r *NormalOffsetSweepBoundaryArea) {
	r.VisitedCells, r.ExactOperations = w.visits, w.budget.operations
	for side, area := range w.best {
		if area.lo != nil {
			r.Faces = append(r.Faces, NormalOffsetSweepFaceArea{Side: NormalOffsetSweepSide(side), Bounds: copySweepInterval(area)})
		}
	}
	if len(r.Faces) == len(w.best) {
		r.Bounds = copySweepInterval(w.total)
	}
}

// cover returns a complete directed partition of start or no replacement.
// During refinement the previous cell's valid certificate is kept until
// both of its children and all their necessary subcells are covered.
func (w *directedSweepAreaWork) cover(ctx context.Context, start directedSweepAreaCell) (exactInterval, []directedSweepAreaCell, error) {
	stack := []directedSweepAreaCell{start}
	var leaves []directedSweepAreaCell
	var a = offsetIntervalArithmetic{b: w.budget}
	sum := offsetZeroInterval()
	for len(stack) != 0 {
		if err := ctx.Err(); err != nil {
			return exactInterval{}, nil, err
		}
		if w.visits >= w.maxCells {
			return exactInterval{}, nil, ErrConvergence
		}
		i := len(stack) - 1
		cell := stack[i]
		stack = stack[:i]
		w.visits++
		err := w.estimate(ctx, &cell)
		if err != nil {
			return exactInterval{}, nil, err
		}
		if cell.area.lo == nil {
			left, right, split, err := w.split(cell, true)
			if err != nil {
				return exactInterval{}, nil, err
			}
			if !split {
				return exactInterval{}, nil, ErrConvergence
			}
			stack = append(stack, right, left)
			continue
		}
		leaves = append(leaves, cell)
		sum = a.add(sum, cell.area)
		if a.err != nil {
			return exactInterval{}, nil, a.err
		}
	}
	return sum, leaves, nil
}

func (w *directedSweepAreaWork) record(side NormalOffsetSweepSide, current exactInterval) error {
	best := current
	old := w.best[side]
	total := w.total
	if total.lo == nil {
		total = offsetZeroInterval()
	}
	a := boundedRatArithmetic{b: w.budget}
	if old.lo != nil {
		if old.lo.Cmp(best.lo) > 0 {
			best.lo = old.lo
		}
		if old.hi.Cmp(best.hi) < 0 {
			best.hi = old.hi
		}
		if best.lo.Cmp(best.hi) > 0 {
			return ErrConvergence
		}
		total = exactInterval{a.sub(total.lo, old.lo), a.sub(total.hi, old.hi)}
	}
	total = exactInterval{a.add(total.lo, best.lo), a.add(total.hi, best.hi)}
	if a.err != nil {
		return a.err
	}
	w.raw[side], w.best[side], w.total = current, best, total
	return nil
}

func (w *directedSweepAreaWork) refine(ctx context.Context, tolerance *big.Rat) (bool, error) {
	a := boundedRatArithmetic{b: w.budget}
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		width := a.sub(w.total.hi, w.total.lo)
		if a.err != nil {
			return false, a.err
		}
		if width.Cmp(tolerance) <= 0 {
			return true, nil
		}
		if len(w.queue) == 0 || w.visits > w.maxCells-2 {
			return false, ErrConvergence
		}
		parent := heap.Pop(&w.queue).(directedSweepAreaCell)
		if parent.width.Sign() == 0 {
			continue
		}
		left, right, split, err := w.split(parent, false)
		if err != nil {
			return false, err
		}
		if !split {
			continue
		}
		l, lCells, err := w.cover(ctx, left)
		if err != nil {
			return false, err
		}
		h, hCells, err := w.cover(ctx, right)
		if err != nil {
			return false, err
		}
		old := w.raw[parent.side]
		current := exactInterval{a.add(a.sub(old.lo, parent.area.lo), a.add(l.lo, h.lo)),
			a.add(a.sub(old.hi, parent.area.hi), a.add(l.hi, h.hi))}
		if a.err != nil {
			return false, a.err
		}
		if err := w.record(parent.side, current); err != nil {
			return false, err
		}
		for _, cell := range append(lCells, hCells...) {
			heap.Push(&w.queue, cell)
		}
	}
}

func (w *directedSweepAreaWork) split(cell directedSweepAreaCell, needSpatial bool) (
	left, right directedSweepAreaCell, split bool, err error) {
	left, right = cell, cell
	left.area, right.area = exactInterval{}, exactInterval{}
	left.depth, right.depth = cell.depth+1, cell.depth+1
	if cell.depth >= w.maxDepth {
		return left, right, false, nil
	}
	d := cell.patch.Domain()
	mu, mv := parameterMidpoint(d.UMin, d.UMax), parameterMidpoint(d.VMin, d.VMax)
	canU, canV := d.UMin < mu && mu < d.UMax, d.VMin < mv && mv < d.VMax
	wall := cell.side < NormalOffsetSweepLower
	if wall {
		canU = canU && cell.side >= NormalOffsetSweepVMin
		canV = canV && cell.side <= NormalOffsetSweepUMax
		if !needSpatial && (cell.splitLayer || !canU && !canV) {
			mid := exactSweepMidpoint(cell.displacement)
			if err := w.budget.check(mid); err != nil {
				return left, right, false, err
			}
			left.displacement.hi, right.displacement.lo = mid, mid
			return left, right, true, nil
		}
	}
	if !canU && !canV {
		return left, right, false, nil
	}
	u := canU && (!canV || cell.depth%2 == 0)
	if !wall && canU && canV {
		switch cell.flatCapAxes {
		case 1:
			u = false
		case 2:
			u = true
		}
	}
	mid := mv
	if u {
		mid = mu
	}
	left.patch, right.patch, err = cell.patch.split(mid, u, w.budget)
	return left, right, err == nil, err
}

type directedSweepAreaQueue []directedSweepAreaCell

func (q directedSweepAreaQueue) Len() int           { return len(q) }
func (q directedSweepAreaQueue) Less(i, j int) bool { return q[i].width.Cmp(q[j].width) > 0 }
func (q directedSweepAreaQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *directedSweepAreaQueue) Push(v any)        { *q = append(*q, v.(directedSweepAreaCell)) }
func (q *directedSweepAreaQueue) Pop() any {
	i := len(*q) - 1
	v := (*q)[i]
	(*q)[i] = directedSweepAreaCell{}
	*q = (*q)[:i]
	return v
}
