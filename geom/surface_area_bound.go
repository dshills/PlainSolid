package geom

import (
	"context"
	"math"
	"math/big"
)

// SurfaceAreaBoundOptions requests a certified absolute area gap. Zero fields
// select tolerance 1e-5, depth 24, 8192 cells, 4M exact operations and 16384 bits.
type SurfaceAreaBoundOptions struct {
	Tolerance                                             float64
	MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int
}

// SurfaceAreaBounds encloses the parametric area integral of the exact rational patch.
// Complete means Upper-Lower <= Tolerance using outward float endpoints. On
// cancellation/limits, completed cell bounds survive and remaining cells are
// conservatively covered (Upper may be +Inf before initial classification).
type SurfaceAreaBounds struct {
	Lower, Upper           float64
	Complete               bool
	Cells, ExactOperations int
}

// EncloseSurfaceCellArea bounds the area of one knot cell with one exact
// derivative enclosure. Its upper bound also bounds the area of every
// measurable subregion of the cell. The context is checked before the
// bounded polynomial operations. A patch spanning an internal knot is
// outside this method's domain.
func EncloseSurfaceCellArea(ctx context.Context, p *SurfacePatch, o SurfacePatchEvaluationOptions) (lower, upper float64, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 {
		return 0, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return 0, 0, e
	}
	if _, _, split := patchSplitChoice(p); split {
		return 0, 0, ErrDomain
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return 0, 0, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	return surfaceAreaCellBounds(p, b)
}

// surfaceAreaCellBounds integrates a box enclosure of Su cross Sv.
func surfaceAreaCellBounds(p *SurfacePatch, b *polynomialBudget) (float64, float64, error) {
	_, derivative, e := surfaceDifferentialIntervals(p, -1, 0, b)
	if e != nil {
		return 0, 0, e
	}
	lower, upper := new(big.Rat), new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		j, k := (axis+1)%3, (axis+2)%3
		x, e := intervalMultiply(derivative[j][0], derivative[k][1], b)
		if e != nil {
			return 0, 0, e
		}
		y, e := intervalMultiply(derivative[k][0], derivative[j][1], b)
		if e != nil {
			return 0, 0, e
		}
		z, e := intervalSubtract(x, y, b)
		if e != nil {
			return 0, 0, e
		}
		lo, hi := new(big.Rat).Abs(z.lo), new(big.Rat).Abs(z.hi)
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if z.lo.Sign() <= 0 && z.hi.Sign() >= 0 {
			lo = new(big.Rat)
		}
		lower.Add(lower, new(big.Rat).Mul(lo, lo))
		upper.Add(upper, new(big.Rat).Mul(hi, hi))
	}
	d := p.Domain()
	u := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin))
	v := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), new(big.Rat).SetFloat64(d.VMin))
	area := new(big.Rat).Mul(u, v)
	area.Mul(area, area)
	lower.Mul(lower, area)
	upper.Mul(upper, area)
	if e := b.check(lower); e != nil {
		return 0, 0, e
	}
	if e := b.check(upper); e != nil {
		return 0, 0, e
	}
	return outwardSqrtRat(lower, true), outwardSqrtRat(upper, false), nil
}

// BoundSurfaceArea encloses the integral of |Su cross Sv| over the patch.
// Self-overlap is counted with parameter multiplicity. Exact rational derivative
// intervals give bounds on every knot-aligned cell; knot derivative jumps have
// measure zero. Tight tolerances may converge slowly, especially near singular
// points or with disparate parameter scales. Limits return conservative bounds.
func BoundSurfaceArea(p *SurfacePatch, o SurfaceAreaBoundOptions) (SurfaceAreaBounds, error) {
	return BoundSurfaceAreaContext(context.Background(), p, o)
}

// BoundSurfaceAreaContext checks cancellation between bounded cell operations.
func BoundSurfaceAreaContext(ctx context.Context, p *SurfacePatch, o SurfaceAreaBoundOptions) (result SurfaceAreaBounds, err error) {
	result.Upper = math.Inf(1)
	if ctx == nil || p == nil || len(p.net) == 0 {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-5
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Tolerance) || o.Tolerance <= 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	type span struct {
		p      *SurfacePatch
		depth  int
		lo, hi float64
	}
	stack := []span{{p: p}}
	leaves := []span{}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		t, u, ok := patchSplitChoice(n.p)
		if ok {
			if len(stack)+len(leaves)+2 > o.MaxCells {
				return result, ErrConvergence
			}
			left, right, e := n.p.split(t, u, b)
			if e != nil {
				return result, e
			}
			stack = append(stack, span{p: right}, span{p: left})
			continue
		}
		lo, hi, e := surfaceAreaCellBounds(n.p, b)
		if e != nil {
			return result, e
		}
		n.lo, n.hi = lo, hi
		leaves = append(leaves, n)
	}
	totals := func() error {
		lower, upper := new(big.Rat), new(big.Rat)
		infinite := false
		for _, n := range leaves {
			lower.Add(lower, new(big.Rat).SetFloat64(n.lo))
			if math.IsInf(n.hi, 1) {
				infinite = true
			} else {
				upper.Add(upper, new(big.Rat).SetFloat64(n.hi))
			}
			if e := b.check(lower); e != nil {
				return e
			}
			if e := b.check(upper); e != nil {
				return e
			}
		}
		result.Lower = rationalFloatBound(lower, true)
		result.Upper = math.Inf(1)
		if !infinite {
			result.Upper = rationalFloatBound(upper, false)
		}
		result.Cells = len(leaves)
		return nil
	}
	for {
		if e := totals(); e != nil {
			return result, e
		}
		if e := ctx.Err(); e != nil {
			return result, e
		}
		if finite(result.Upper) && new(big.Rat).Sub(new(big.Rat).SetFloat64(result.Upper), new(big.Rat).SetFloat64(result.Lower)).Cmp(new(big.Rat).SetFloat64(o.Tolerance)) <= 0 {
			result.Complete = true
			return result, nil
		}
		if len(leaves) >= o.MaxCells {
			return result, ErrConvergence
		}
		best := -1
		gap := -1.0
		for i, n := range leaves {
			if n.depth < o.MaxDepth && n.hi-n.lo > gap {
				best, gap = i, n.hi-n.lo
			}
		}
		if best < 0 {
			return result, ErrConvergence
		}
		n := leaves[best]
		d := n.p.Domain()
		t, u, _ := patchSplitChoice(n.p)
		if (u && (t == d.UMin || t == d.UMax)) || (!u && (t == d.VMin || t == d.VMax)) {
			leaves[best].depth = o.MaxDepth
			continue
		}
		left, right, e := n.p.split(t, u, b)
		if e != nil {
			return result, e
		}
		ll, lh, e := surfaceAreaCellBounds(left, b)
		if e != nil {
			return result, e
		}
		rl, rh, e := surfaceAreaCellBounds(right, b)
		if e != nil {
			return result, e
		}
		leaves[best] = span{left, n.depth + 1, ll, lh}
		leaves = append(leaves, span{right, n.depth + 1, rl, rh})
	}
}
