package geom

import (
	"container/heap"
	"context"
	"math/big"
)

// NormalOffsetSweepDistanceOptions bounds the analytic boundary query.
// Nil SquaredTolerance selects 1e-8 in squared model units. An explicit
// zero asks for equality of both bounds. Default limits are 8192 visited
// cells, 32 partition levels, and four million exact operations.
type NormalOffsetSweepDistanceOptions struct {
	SquaredTolerance   *big.Rat
	Patch              SurfacePatchOptions
	MaxDepth, MaxCells int
}

// NormalOffsetSweepDistanceWitness retains one original chart, face and
// exact source U, V and signed displacement. Owner is zero-based.
// Position encloses the possibly irrational physical point.
type NormalOffsetSweepDistanceWitness struct {
	Side       NormalOffsetSweepSide
	Owner      int `json:",omitempty"`
	Parameters [3]*big.Rat
	Position   [3]ExactParameterInterval
}

// NormalOffsetSweepDistance is a directed squared-distance enclosure to
// the entire analytic boundary. Squared.Upper is supplied by Witness.
// A resource refusal retains any proved bounds. The upper bound and
// witness can be absent if exact work ends before evaluating a station.
// A joined search can have an upper witness before its full outer cover
// is available to establish a coarse lower bound.
type NormalOffsetSweepDistance struct {
	Squared                       ExactParameterInterval
	Witness                       *NormalOffsetSweepDistanceWitness
	Complete                      bool
	VisitedCells, ExactOperations int
}

// BoundPointBoundaryDistanceRational encloses the squared physical
// distance from an exact point to the six original analytic boundary
// faces. Complete means the requested squared interval width was met.
// The query applies to either interior or exterior points.
func (s *VerifiedNormalOffsetSweep) BoundPointBoundaryDistanceRational(ctx context.Context, point [3]*big.Rat,
	o NormalOffsetSweepDistanceOptions) (NormalOffsetSweepDistance, error) {
	if s == nil || s.source == nil {
		return NormalOffsetSweepDistance{}, ErrInvalid
	}
	var faces [6]VerifiedNormalOffsetSweepBoundaryFace
	for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
		faces[side] = VerifiedNormalOffsetSweepBoundaryFace{Side: side, Face: VerifiedNormalOffsetSweepFace{sweep: s, side: side}}
	}
	return boundPointSweepOuterDistance(ctx, point, o, []*VerifiedNormalOffsetSweep{s}, faces[:])
}

func boundPointSweepOuterDistance(ctx context.Context, point [3]*big.Rat, o NormalOffsetSweepDistanceOptions,
	sweeps []*VerifiedNormalOffsetSweep, faces []VerifiedNormalOffsetSweepBoundaryFace) (r NormalOffsetSweepDistance, err error) {
	if ctx == nil || o.MaxDepth < 0 || o.MaxDepth > 128 ||
		o.MaxCells < 0 || o.MaxCells > 1<<18 || (o.SquaredTolerance != nil && o.SquaredTolerance.Sign() < 0) {
		return r, ErrInvalid
	}
	for _, q := range point {
		if q == nil {
			return r, ErrInvalid
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if o.SquaredTolerance == nil {
		o.SquaredTolerance = big.NewRat(1, 100_000_000)
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 4_000_000
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if len(sweeps) == 1 {
		r.Squared.Lower = new(big.Rat)
	}
	roots := make([]sweepBoundaryDistanceRoot, len(sweeps))
	for i, sweep := range sweeps {
		root := &roots[i]
		root.patch = *sweep.source
		root.patch.options.MaxPoles = min(root.patch.options.MaxPoles, limits.MaxPoles)
		if len(root.patch.net) > root.patch.options.MaxPoles {
			return r, ErrConvergence
		}
		root.original = normalSweepParameterIntervals(root.patch.Domain(), exactInterval{sweep.lower, sweep.upper})
	}
	// The displacement interval is identical across independently
	// certified adjacent sweeps, so its endpoints need one budget charge.
	for _, q := range [6]*big.Rat{point[0], point[1], point[2], sweeps[0].lower, sweeps[0].upper, o.SquaredTolerance} {
		if err := b.check(q); err != nil {
			return r, err
		}
	}
	w := sweepBoundaryDistanceWork{point: point, options: o, budget: b, result: &r}
	err = w.search(ctx, roots, faces)
	return r, err
}

type sweepBoundaryDistanceWork struct {
	point   [3]*big.Rat
	options NormalOffsetSweepDistanceOptions
	budget  *polynomialBudget
	result  *NormalOffsetSweepDistance
}

type sweepBoundaryDistanceRoot struct {
	patch    SurfacePatch
	original [3]exactInterval
	prepared bool
	affine   *sweepBoundaryAffineFrame
}

type sweepBoundaryDistanceCell struct {
	patch        *SurfacePatch
	displacement exactInterval
	side         NormalOffsetSweepSide
	owner        int
	lower        *big.Rat
	depth        int
}

func (w *sweepBoundaryDistanceWork) search(ctx context.Context, roots []sweepBoundaryDistanceRoot,
	faces []VerifiedNormalOffsetSweepBoundaryFace) error {
	var queue sweepBoundaryDistanceQueue
	for _, outer := range faces {
		if err := ctx.Err(); err != nil {
			return err
		}
		if w.result.VisitedCells >= w.options.MaxCells {
			return ErrConvergence
		}
		root := &roots[outer.Owner]
		if !root.prepared {
			var err error
			root.affine, err = w.affine(ctx, &root.patch, root.original)
			if err != nil {
				return err
			}
			root.prepared = true
		}
		if root.affine != nil {
			w.result.VisitedCells++
			if err := w.affineFace(outer.Owner, outer.Side, root.affine); err != nil {
				return err
			}
		} else {
			cell := sweepBoundaryDistanceCell{
				patch: &root.patch, side: outer.Side, owner: outer.Owner, displacement: root.original[2], lower: new(big.Rat),
			}
			switch outer.Side {
			case NormalOffsetSweepLower:
				cell.displacement.hi = cell.displacement.lo
			case NormalOffsetSweepUpper:
				cell.displacement.lo = cell.displacement.hi
			}
			cell, err := w.estimate(ctx, cell)
			if err != nil {
				return err
			}
			heap.Push(&queue, cell)
		}
		if w.result.Squared.Upper.Sign() == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			w.finishExact()
			return nil
		}
	}
	if len(queue) == 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.finishExact()
		return nil
	}
	// Every retained outer face now has an initial cell or was
	// minimized exactly. Each affine minimum is at least the best
	// upper endpoint, so only the nonaffine cells need subdivision.
	var terminalLower *big.Rat
	for {
		lower := w.result.Squared.Upper
		if terminalLower != nil && terminalLower.Cmp(lower) < 0 {
			lower = terminalLower
		}
		if len(queue) != 0 && queue[0].lower.Cmp(lower) < 0 {
			lower = queue[0].lower
		}
		w.result.Squared.Lower = new(big.Rat).Set(lower)
		if err := ctx.Err(); err != nil {
			return err
		}
		width := new(big.Rat).Sub(w.result.Squared.Upper, lower)
		if err := w.budget.check(width); err != nil {
			return err
		}
		if width.Cmp(w.options.SquaredTolerance) <= 0 {
			w.result.Complete = true
			return nil
		}
		if len(queue) == 0 || w.result.VisitedCells > w.options.MaxCells-2 {
			return ErrConvergence
		}
		parent := heap.Pop(&queue).(sweepBoundaryDistanceCell)
		if parent.lower.Cmp(w.result.Squared.Upper) >= 0 {
			continue
		}
		left, right, split, err := w.split(parent)
		if err != nil {
			return err
		}
		if !split {
			if terminalLower == nil || parent.lower.Cmp(terminalLower) < 0 {
				terminalLower = parent.lower
			}
			continue
		}
		for _, child := range [2]sweepBoundaryDistanceCell{left, right} {
			child, err := w.estimate(ctx, child)
			if err != nil {
				return err
			}
			if w.result.Squared.Upper.Sign() == 0 {
				w.finishExact()
				return nil
			}
			if child.lower.Cmp(w.result.Squared.Upper) < 0 {
				heap.Push(&queue, child)
			}
		}
	}
}

func (w *sweepBoundaryDistanceWork) finishExact() {
	w.result.Squared.Lower = new(big.Rat).Set(w.result.Squared.Upper)
	w.result.Complete = true
}

func (w *sweepBoundaryDistanceWork) estimate(ctx context.Context, cell sweepBoundaryDistanceCell) (sweepBoundaryDistanceCell, error) {
	if err := ctx.Err(); err != nil {
		return cell, err
	}
	w.result.VisitedCells++
	// Establish a witness before computing the first lower cell bound,
	// so a short remainder can still return a genuine upper endpoint.
	first := w.result.Witness == nil
	if first {
		if err := w.sample(ctx, cell); err != nil {
			return cell, err
		}
		if w.result.Squared.Upper.Sign() == 0 {
			return cell, nil
		}
	}
	box, err := sweepBoundaryDistanceBox(ctx, cell.patch, cell.side, cell.displacement, w.budget)
	if err != nil {
		return cell, err
	}
	lower, err := w.boxDistance(box, false)
	if err != nil {
		return cell, err
	}
	if lower.Cmp(cell.lower) > 0 {
		cell.lower = lower
	}
	if !first && cell.lower.Cmp(w.result.Squared.Upper) < 0 {
		if err := w.sample(ctx, cell); err != nil {
			return cell, err
		}
	}
	return cell, nil
}

func (w *sweepBoundaryDistanceWork) sample(ctx context.Context, cell sweepBoundaryDistanceCell) error {
	q := normalSweepParameterIntervals(cell.patch.Domain(), cell.displacement)
	var uvw [3]*big.Rat
	for i, interval := range q {
		uvw[i] = exactSweepMidpoint(interval)
		if err := w.budget.check(uvw[i]); err != nil {
			return err
		}
	}
	axis := int(cell.side) / 2
	uvw[axis] = q[axis].lo
	if int(cell.side)%2 == 1 {
		uvw[axis] = q[axis].hi
	}
	point, partials, err := exactSurfaceJet(cell.patch, uvw[0], uvw[1], w.budget)
	if err != nil {
		return err
	}
	jet := RationalSurfacePatchJet{Point: point}
	for i, p := range partials {
		jet.Du[i], jet.Dv[i] = p[0], p[1]
	}
	box, err := w.evaluateOffset(ctx, jet, uvw[2])
	if err != nil {
		return err
	}
	return w.record(cell.owner, cell.side, uvw, box)
}

func (w *sweepBoundaryDistanceWork) evaluateOffset(ctx context.Context, jet RationalSurfacePatchJet, distance *big.Rat) (box [3]exactInterval, err error) {
	remaining := w.budget.maxOperations - w.budget.operations
	if remaining <= 0 {
		return box, ErrConvergence
	}
	p, err := jet.NormalOffsetRational(ctx, distance, SurfaceNormalOffsetEvaluationOptions{
		MaxOperations: remaining, MaxCoefficientBits: w.budget.maxBits,
	})
	w.budget.operations += p.ExactOperations
	if err != nil {
		return box, err
	}
	for i := range box {
		box[i] = exactInterval{p.Lower[i], p.Upper[i]}
	}
	return box, nil
}

func (w *sweepBoundaryDistanceWork) record(owner int, side NormalOffsetSweepSide, coordinates [3]*big.Rat, box [3]exactInterval) error {
	d, err := w.boxDistance(box, true)
	if err != nil {
		return err
	}
	if w.result.Squared.Upper != nil && w.result.Squared.Upper.Cmp(d) <= 0 {
		return nil
	}
	w.result.Squared.Upper = d
	witness := &NormalOffsetSweepDistanceWitness{Owner: owner, Side: side}
	for i := range coordinates {
		witness.Parameters[i] = new(big.Rat).Set(coordinates[i])
		witness.Position[i] = copySweepInterval(box[i])
	}
	w.result.Witness = witness
	return nil
}

func (w *sweepBoundaryDistanceWork) boxDistance(b [3]exactInterval, upper bool) (*big.Rat, error) {
	a := offsetIntervalArithmetic{b: w.budget}
	d := offsetZeroInterval()
	for i, x := range w.point {
		d = a.add(d, a.square(a.sub(b[i], exactInterval{x, x})))
	}
	if a.err != nil {
		return nil, a.err
	}
	if upper {
		return d.hi, nil
	}
	return d.lo, nil
}

func (w *sweepBoundaryDistanceWork) split(c sweepBoundaryDistanceCell) (left, right sweepBoundaryDistanceCell, split bool, err error) {
	if c.depth >= w.options.MaxDepth {
		return left, right, false, nil
	}
	left, right = c, c
	left.depth, right.depth = c.depth+1, c.depth+1
	fixed := int(c.side) / 2
	free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[fixed]
	d := c.patch.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for i := range free {
		axis := free[(c.depth+i)%2]
		if axis == 2 {
			middle := exactSweepMidpoint(c.displacement)
			if err := w.budget.check(middle); err != nil {
				return left, right, false, err
			}
			left.displacement.hi, right.displacement.lo = middle, middle
			return left, right, true, nil
		}
		lo, hi := ranges[axis][0], ranges[axis][1]
		middle := parameterMidpoint(lo, hi)
		if lo < middle && middle < hi {
			left.patch, right.patch, err = c.patch.split(middle, axis == 0, w.budget)
			return left, right, err == nil, err
		}
	}
	return left, right, false, nil
}

type sweepBoundaryDistanceQueue []sweepBoundaryDistanceCell

func (q sweepBoundaryDistanceQueue) Len() int           { return len(q) }
func (q sweepBoundaryDistanceQueue) Less(i, j int) bool { return q[i].lower.Cmp(q[j].lower) < 0 }
func (q sweepBoundaryDistanceQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *sweepBoundaryDistanceQueue) Push(v any)        { *q = append(*q, v.(sweepBoundaryDistanceCell)) }
func (q *sweepBoundaryDistanceQueue) Pop() any {
	i := len(*q) - 1
	v := (*q)[i]
	(*q)[i] = sweepBoundaryDistanceCell{}
	*q = (*q)[:i]
	return v
}
