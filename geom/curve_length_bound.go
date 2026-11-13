package geom

import (
	"context"
	"math"
	"math/big"
)

// CurveLengthBoundOptions requests a certified absolute length gap. Zero fields
// select tolerance 1e-5, depth 24, 8192 spans, 4M exact operations and 16384 bits.
type CurveLengthBoundOptions struct {
	Tolerance                                             float64
	MaxDepth, MaxSpans, MaxOperations, MaxCoefficientBits int
}

// CurveLengthBounds encloses exact arc length of the stored rational spline.
// Complete means Upper-Lower <= Tolerance using outward float endpoints. On
// cancellation/limits, completed span bounds survive and remaining spans are
// conservatively covered (Upper may be +Inf before initial classification).
type CurveLengthBounds struct {
	Lower, Upper           float64
	Complete               bool
	Spans, ExactOperations int
}

func outwardSqrtRat(x *big.Rat, lower bool) float64 {
	if x.Sign() == 0 {
		return 0
	}
	f, _ := new(big.Float).SetPrec(256).SetRat(x).Sqrt(new(big.Float).SetPrec(256).SetRat(x)).Float64()
	if math.IsInf(f, 1) {
		if lower {
			return math.MaxFloat64
		}
		return f
	}
	for {
		r := new(big.Rat).SetFloat64(f)
		square := new(big.Rat).Mul(r, r)
		if lower && square.Cmp(x) > 0 {
			f = math.Nextafter(f, math.Inf(-1))
			continue
		}
		if !lower && square.Cmp(x) < 0 {
			f = math.Nextafter(f, math.Inf(1))
			if math.IsInf(f, 1) {
				return f
			}
			continue
		}
		break
	}

	return f
}

// spanLengthBounds uses a chord lower bound and a derivative-enclosure upper
// bound. No polynomial control-polygon theorem is assumed for rational weights.
func spanLengthBounds(p *SurfacePatch, b *polynomialBudget) (float64, float64, error) {
	first, last := p.net[0], p.net[(p.countU-1)*p.countV]
	chord := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		delta := new(big.Rat).Sub(new(big.Rat).Quo(last[axis], last[3]), new(big.Rat).Quo(first[axis], first[3]))
		chord.Add(chord, new(big.Rat).Mul(delta, delta))
		if e := b.check(chord); e != nil {
			return 0, 0, e
		}
	}
	_, derivative, e := surfaceDifferentialIntervals(p, -1, 0, b)
	if e != nil {
		return 0, 0, e
	}
	speedSquared := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		v := derivative[axis][0]
		maximum := new(big.Rat).Abs(v.lo)
		other := new(big.Rat).Abs(v.hi)
		if other.Cmp(maximum) > 0 {
			maximum = other
		}
		speedSquared.Add(speedSquared, new(big.Rat).Mul(maximum, maximum))
		if e := b.check(speedSquared); e != nil {
			return 0, 0, e
		}
	}
	d := p.Domain()
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin))
	upperSquared := new(big.Rat).Mul(speedSquared, new(big.Rat).Mul(width, width))
	if e := b.check(upperSquared); e != nil {
		return 0, 0, e
	}
	return outwardSqrtRat(chord, true), outwardSqrtRat(upperSquared, false), nil
}

// BoundCurveLength certifies continuous clamped positive-weight rational length.
// It is separate from numerical quadrature. Exact homogeneous subdivision and
// derivative hulls provide bounds, with exact squared comparisons directing
// outward square-root rounding. Discontinuous/unclamped inputs are unsupported.
func BoundCurveLength(curve *BSpline, o CurveLengthBoundOptions) (CurveLengthBounds, error) {
	return BoundCurveLengthContext(context.Background(), curve, o)
}

// BoundCurveLengthContext checks cancellation between bounded span operations.
func BoundCurveLengthContext(ctx context.Context, curve *BSpline, o CurveLengthBoundOptions) (result CurveLengthBounds, err error) {
	result.Upper = math.Inf(1)
	if ctx == nil || curve == nil || len(curve.poles) == 0 {
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
	p, e := curveDistancePatch(curve, o.MaxOperations, o.MaxCoefficientBits)
	if e != nil {
		return result, e
	}
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
		lo, hi, e := spanLengthBounds(n.p, b)
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
		ll, lh, e := spanLengthBounds(left, b)
		if e != nil {
			return result, e
		}
		rl, rh, e := spanLengthBounds(right, b)
		if e != nil {
			return result, e
		}
		leaves[best] = span{left, n.depth + 1, ll, lh}
		leaves = append(leaves, span{right, n.depth + 1, rl, rh})
	}
}
