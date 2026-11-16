package geom

import (
	"container/heap"
	"context"
	"math/big"
	"math/bits"
)

// NormalOffsetSweepVolumeOptions bounds analytic volume integration.
// AbsoluteError is the required positive width of the volume enclosure.
// Zero limits allow 24 subdivision levels, 4095 visited cells and four
// million exact operations. The ordinary patch coefficient limits apply.
type NormalOffsetSweepVolumeOptions struct {
	AbsoluteError      *big.Rat
	Patch              SurfacePatchOptions
	MaxDepth, MaxCells int
}

// NormalOffsetSweepVolume encloses the positive physical volume. Bounds
// remain valid when refinement exhausts its limits after proving a coarse
// enclosure. Complete means the requested width was reached.
type NormalOffsetSweepVolume struct {
	Bounds                    ExactParameterInterval
	Complete                  bool
	ActiveCells, VisitedCells int
	ExactOperations           int
}

// VolumeRational integrates the analytic normal sweep, using the exact
// displacement moments of its quadratic Jacobian and directed UV boxes.
// It does not integrate a tessellation or use a sampled Jacobian sign.
func (s *VerifiedNormalOffsetSweep) VolumeRational(ctx context.Context, o NormalOffsetSweepVolumeOptions) (r NormalOffsetSweepVolume, err error) {
	if ctx == nil || s == nil || s.source == nil || o.AbsoluteError == nil || o.AbsoluteError.Sign() <= 0 ||
		o.MaxDepth < 0 || o.MaxDepth > 64 || o.MaxCells < 0 || o.MaxCells > 1<<18 {
		return r, ErrInvalid
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
	if err := ctx.Err(); err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if err := b.check(o.AbsoluteError); err != nil {
		return r, err
	}
	root := *s.source
	root.options.MaxPoles = min(root.options.MaxPoles, limits.MaxPoles)
	if len(root.net) > root.options.MaxPoles {
		return r, ErrConvergence
	}
	a := boundedRatArithmetic{b: b}
	l, h := s.lower, s.upper
	l2, h2 := a.mul(l, l), a.mul(h, h)
	moments := [3]*big.Rat{a.sub(h, l), a.quo(a.sub(h2, l2), big.NewRat(2, 1)),
		a.quo(a.sub(a.mul(h2, h), a.mul(l2, l)), big.NewRat(3, 1))}
	extension := new(big.Rat).Abs(l)
	if q := new(big.Rat).Abs(h); q.Cmp(extension) > 0 {
		extension = q
	}
	if a.err != nil {
		return r, a.err
	}
	// A common dyadic grid keeps the sum of many directed nonexact
	// contributions small. Its total padding is below the target width.
	precision := max(0, o.AbsoluteError.Denom().BitLen()-o.AbsoluteError.Num().BitLen()) + bits.Len(uint(o.MaxCells)) + 4
	if precision >= b.maxBits {
		return r, ErrConvergence
	}
	factor := new(big.Int).Lsh(big.NewInt(1), uint(precision))
	estimate := func(p *SurfacePatch, depth int) (sweepVolumeCell, error) {
		r.VisitedCells++
		q, err := normalSweepVolumeCell(ctx, p, moments, extension, s.orientation, b)
		if err != nil {
			return sweepVolumeCell{}, err
		}
		if q.lo.Cmp(q.hi) != 0 || depth != 0 {
			q, err = roundSweepVolumeBounds(q, factor, b)
			if err != nil {
				return sweepVolumeCell{}, err
			}
		}
		size := new(big.Rat).Sub(q.hi, q.lo)
		return sweepVolumeCell{p, q, size, depth}, b.check(size)
	}
	first, err := estimate(&root, 0)
	if err != nil {
		return r, err
	}
	q, best := first.volume, first.volume
	r.ActiveCells = 1
	defer func() { r.Bounds = copySweepInterval(best) }()
	queue := sweepVolumeQueue{first}
	for {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if new(big.Rat).Sub(best.hi, best.lo).Cmp(o.AbsoluteError) <= 0 {
			r.Complete = true
			return r, nil
		}
		if len(queue) == 0 || r.VisitedCells > o.MaxCells-2 {
			return r, ErrConvergence
		}
		parent := heap.Pop(&queue).(sweepVolumeCell)
		if parent.depth >= o.MaxDepth || parent.width.Sign() == 0 {
			continue
		}
		d := parent.patch.Domain()
		u := parent.depth%2 == 0
		mu, mv := parameterMidpoint(d.UMin, d.UMax), parameterMidpoint(d.VMin, d.VMax)
		canU, canV := d.UMin < mu && mu < d.UMax, d.VMin < mv && mv < d.VMax
		if !canU && !canV {
			continue
		}
		u = canU && (u || !canV)
		mid := mv
		if u {
			mid = mu
		}
		left, right, err := parent.patch.split(mid, u, b)
		if err != nil {
			return r, err
		}
		lc, err := estimate(left, parent.depth+1)
		if err != nil {
			return r, err
		}
		rc, err := estimate(right, parent.depth+1)
		if err != nil {
			return r, err
		}
		// Remove like endpoints because this is an accumulated sum of
		// independent cell bounds, not interval subtraction of volumes.
		q, err = boundedExactInterval(a.add(a.sub(q.lo, parent.volume.lo), a.add(lc.volume.lo, rc.volume.lo)),
			a.add(a.sub(q.hi, parent.volume.hi), a.add(lc.volume.hi, rc.volume.hi)), b)
		if a.err != nil {
			return r, a.err
		}
		if err != nil {
			return r, err
		}
		newBest := best
		if q.lo.Cmp(newBest.lo) > 0 {
			newBest.lo = q.lo
		}
		if q.hi.Cmp(newBest.hi) < 0 {
			newBest.hi = q.hi
		}
		if newBest.lo.Cmp(newBest.hi) > 0 {
			return r, ErrConvergence
		}
		best = newBest
		r.ActiveCells++
		heap.Push(&queue, lc)
		heap.Push(&queue, rc)
	}
}

func normalSweepVolumeCell(ctx context.Context, p *SurfacePatch, moment [3]*big.Rat, extension *big.Rat,
	orientation int, b *polynomialBudget) (exactInterval, error) {
	d, err := offsetCellJacobianBounds(ctx, p, b)
	if err != nil {
		return exactInterval{}, err
	}
	if d == nil {
		// The verified volume is embedded, so its part over this chart
		// sits inside the source hull expanded by the greatest |shift|.
		// A coarse cell can use this fallback until normals separate.
		a := boundedRatArithmetic{b: b}
		twice := a.add(extension, extension)
		v := big.NewRat(1, 1)
		for axis := range 3 {
			lo := new(big.Rat).SetFloat64(coordinate(p.bounds.Min, axis))
			hi := new(big.Rat).SetFloat64(coordinate(p.bounds.Max, axis))
			v = a.mul(v, a.add(a.sub(hi, lo), twice))
		}
		return exactInterval{new(big.Rat), v}, a.err
	}
	a := &d.arithmetic
	var n, nu, nv [3]exactInterval
	for i := range n {
		n[i] = a.div(d.normal[i], d.length)
		nu[i], nv[i] = d.normalDerivative(i, 0), d.normalDerivative(i, 1)
	}
	// det(Su+t Nu, Sv+t Nv, N) is quadratic in t. Its constant
	// term is |Su×Sv|. Integrate that polynomial exactly in t while
	// retaining UV dependence as directed coefficient intervals.
	c0 := a.mul(exactInterval{d.scale, d.scale}, d.length)
	q := a.mul(exactInterval{moment[0], moment[0]}, c0)
	if moment[1].Sign() != 0 {
		x, y := a.cross(nu, d.tangent[1]), a.cross(d.tangent[0], nv)
		for i := range x {
			x[i] = a.add(x[i], y[i])
		}
		q = a.add(q, a.mul(exactInterval{moment[1], moment[1]}, a.dot(x, n)))
	}
	q = a.add(q, a.mul(exactInterval{moment[2], moment[2]}, a.dot(a.cross(nu, nv), n)))
	if orientation < 0 {
		q = a.sub(offsetZeroInterval(), q)
	}
	x := normalSweepParameterIntervals(p.Domain(), exactInterval{})
	width := func(axis int) exactInterval {
		return a.sub(exactInterval{x[axis].hi, x[axis].hi}, exactInterval{x[axis].lo, x[axis].lo})
	}
	q = a.mul(q, a.mul(width(0), width(1)))
	if a.err != nil || q.hi.Sign() < 0 {
		if a.err != nil {
			return exactInterval{}, a.err
		}
		return exactInterval{}, ErrConvergence
	}
	if q.lo.Sign() < 0 {
		q.lo = new(big.Rat)
	}
	return q, nil
}

func roundSweepVolumeBounds(q exactInterval, factor *big.Int, b *polynomialBudget) (exactInterval, error) {
	var n, lo, hi, rem big.Int
	for _, x := range [2]*big.Rat{q.lo, q.hi} {
		if err := b.checkBits(x.Num().BitLen()+factor.BitLen(), x.Denom().BitLen()); err != nil {
			return exactInterval{}, err
		}
	}
	lo.Div(n.Mul(q.lo.Num(), factor), q.lo.Denom())
	hi.DivMod(n.Mul(q.hi.Num(), factor), q.hi.Denom(), &rem)
	if rem.Sign() != 0 {
		hi.Add(&hi, big.NewInt(1))
	}
	return boundedExactInterval(new(big.Rat).SetFrac(&lo, factor), new(big.Rat).SetFrac(&hi, factor), b)
}

type sweepVolumeCell struct {
	patch  *SurfacePatch
	volume exactInterval
	width  *big.Rat
	depth  int
}

type sweepVolumeQueue []sweepVolumeCell

func (q sweepVolumeQueue) Len() int           { return len(q) }
func (q sweepVolumeQueue) Less(i, j int) bool { return q[i].width.Cmp(q[j].width) > 0 }
func (q sweepVolumeQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *sweepVolumeQueue) Push(v any)        { *q = append(*q, v.(sweepVolumeCell)) }
func (q *sweepVolumeQueue) Pop() any {
	i := len(*q) - 1
	x := (*q)[i]
	(*q)[i] = sweepVolumeCell{}
	*q = (*q)[:i]
	return x
}
