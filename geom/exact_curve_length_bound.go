package geom

import (
	"context"
	"math"
	"math/big"
)

// BoundExactCurveLengthContext bounds the complete immutable exact spline
// descriptor without exporting or rounding its homogeneous controls. It uses
// a tighter chord/control-polygon enclosure for positive rational quadratic
// spans, with the same conservative derivative fallback and refinement limits as
// BoundCurveLengthContext. Existing descriptor construction operations count
// against the requested budget; conservative partial bounds may survive errors.
func BoundExactCurveLengthContext(ctx context.Context, curve *ExactSplineCurve, o CurveLengthBoundOptions) (result CurveLengthBounds, err error) {
	result.Upper = math.Inf(1)
	if ctx == nil || curve == nil || curve.patch == nil {
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
	if o.MaxSpans == 0 {
		o.MaxSpans = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Tolerance) || o.Tolerance <= 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxSpans < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	p := curve.patch
	result.ExactOperations = p.operations
	b := &polynomialBudget{maxOperations: o.MaxOperations - p.operations, maxBits: o.MaxCoefficientBits}
	if b.maxOperations < 1 {
		return result, ErrConvergence
	}
	defer func() { result.ExactOperations = p.operations + b.operations }()
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
		t, ok := internalPatchKnot(n.p.knotsU, n.p.degreeU, n.p.countU)
		if ok {
			if len(stack)+len(leaves)+2 > o.MaxSpans {
				return result, ErrConvergence
			}
			left, right, e := n.p.split(t, true, b)
			if e != nil {
				return result, e
			}
			stack = append(stack, span{p: right}, span{p: left})
			continue
		}
		lo, hi, e := exactSpanLengthBounds(n.p, b)
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
		result.Spans = len(leaves)
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
		if len(leaves) >= o.MaxSpans {
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
		t := d.UMin + (d.UMax-d.UMin)/2
		if t == d.UMin || t == d.UMax {
			leaves[best].depth = o.MaxDepth
			continue
		}
		left, right, e := n.p.split(t, true, b)
		if e != nil {
			return result, e
		}
		ll, lh, e := exactSpanLengthBounds(left, b)
		if e != nil {
			return result, e
		}
		rl, rh, e := exactSpanLengthBounds(right, b)
		if e != nil {
			return result, e
		}
		leaves[best] = span{left, n.depth + 1, ll, lh}
		leaves = append(leaves, span{right, n.depth + 1, rl, rh})
	}
}
