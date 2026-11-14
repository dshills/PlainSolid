package geom

import (
	"context"
	"math/big"
)

// WrappedRectangleOverlapOptions limits enumerated lifted occurrences before
// allocation. Defaults are 256 components, 1M arithmetic operations,16384 bits.
type WrappedRectangleOverlapOptions struct{ MaxComponents, MaxOperations, MaxCoefficientBits int }

// WrappedRectangleOverlap retains original owner intervals, in their original
// axis order. Mapped A equals B+TargetPeriodShift*target period. Dimension counts
// nonzero overlap axes: two is finite area, one an edge, zero a point. Separate
// lifted occurrences may describe the same model points on a periodic or
// self-intersecting support; this is not a material-union or embedding report.
type WrappedRectangleOverlap struct {
	A, B              [2][2]*big.Rat
	TargetPeriodShift [2]*big.Int
	Dimension         int
}
type WrappedRectangleOverlaps struct {
	Complete        bool
	Components      []WrappedRectangleOverlap
	ExactOperations int
}

// OverlapRectanglesRational enumerates all exact quotient-chart matches of two
// finite closed lifted rectangles using the retained full surface identity.
// Input intervals must have positive width. Unwrapped axes must remain within
// the fundamental domain; wrapped axes may include any finite number of turns.
// Error/cancellation returns no partial result. All result numbers are copies.
func (r *WrappedSurfaceChartRelation) OverlapRectanglesRational(ctx context.Context, a, b [2][2]*big.Rat, o WrappedRectangleOverlapOptions) (WrappedRectangleOverlaps, error) {
	var zero WrappedRectangleOverlaps
	if ctx == nil || r == nil || r.source == nil || r.target == nil {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxComponents == 0 {
		o.MaxComponents = 256
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxComponents < 1 || o.MaxComponents > 65536 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return zero, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for owner, w := range []*WrappedSurfacePatch{r.source, r.target} {
		rect := a
		if owner == 1 {
			rect = b
		}
		d := w.Domain()
		bounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		for axis, span := range rect {
			for _, v := range span {
				if v == nil {
					return zero, ErrInvalid
				}
				if e := budget.check(v); e != nil {
					return zero, e
				}
			}
			if span[0].Cmp(span[1]) >= 0 {
				return zero, ErrDomain
			}
			if w.period[axis] == nil && (span[0].Cmp(new(big.Rat).SetFloat64(bounds[axis][0])) < 0 || span[1].Cmp(new(big.Rat).SetFloat64(bounds[axis][1])) > 0) {
				return zero, ErrDomain
			}
		}
	}
	var mapped [2][2]*big.Rat
	for i, j := range r.axis {
		for end := 0; end < 2; end++ {
			mapped[j][end] = new(big.Rat).Add(r.offset[i], new(big.Rat).Mul(r.scale[i], a[i][end]))
			if e := budget.check(mapped[j][end]); e != nil {
				return zero, e
			}
		}
		if mapped[j][0].Cmp(mapped[j][1]) > 0 {
			mapped[j][0], mapped[j][1] = mapped[j][1], mapped[j][0]
		}
	}
	floor := func(q *big.Rat) *big.Int {
		n, rem := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), rem)
		if rem.Sign() < 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	ceil := func(q *big.Rat) *big.Int { return new(big.Int).Neg(floor(new(big.Rat).Neg(q))) }
	var first, last [2]*big.Int
	total := big.NewInt(1)
	for j := 0; j < 2; j++ {
		if p := r.target.period[j]; p != nil {
			lo := new(big.Rat).Quo(new(big.Rat).Sub(mapped[j][0], b[j][1]), p)
			hi := new(big.Rat).Quo(new(big.Rat).Sub(mapped[j][1], b[j][0]), p)
			if e := budget.check(lo); e != nil {
				return zero, e
			}
			if e := budget.check(hi); e != nil {
				return zero, e
			}
			first[j], last[j] = ceil(lo), floor(hi)
		} else {
			first[j], last[j] = new(big.Int), new(big.Int)
			if mapped[j][1].Cmp(b[j][0]) < 0 || b[j][1].Cmp(mapped[j][0]) < 0 {
				return WrappedRectangleOverlaps{Complete: true, ExactOperations: budget.operations}, nil
			}
		}
		count := new(big.Int).Add(new(big.Int).Sub(last[j], first[j]), big.NewInt(1))
		if count.Sign() <= 0 {
			return WrappedRectangleOverlaps{Complete: true, ExactOperations: budget.operations}, nil
		}
		total.Mul(total, count)
	}
	if total.Cmp(big.NewInt(int64(o.MaxComponents))) > 0 {
		return zero, ErrConvergence
	}
	out := WrappedRectangleOverlaps{Components: make([]WrappedRectangleOverlap, 0, int(total.Int64()))}
	for k0 := new(big.Int).Set(first[0]); k0.Cmp(last[0]) <= 0; k0.Add(k0, big.NewInt(1)) {
		for k1 := new(big.Int).Set(first[1]); k1.Cmp(last[1]) <= 0; k1.Add(k1, big.NewInt(1)) {
			if e := ctx.Err(); e != nil {
				return zero, e
			}
			item := WrappedRectangleOverlap{TargetPeriodShift: [2]*big.Int{new(big.Int).Set(k0), new(big.Int).Set(k1)}}
			for i, j := range r.axis {
				shift := new(big.Rat)
				if p := r.target.period[j]; p != nil {
					shift.Mul(new(big.Rat).SetInt(item.TargetPeriodShift[j]), p)
				}
				lo := new(big.Rat).Add(b[j][0], shift)
				hi := new(big.Rat).Add(b[j][1], shift)
				if lo.Cmp(mapped[j][0]) < 0 {
					lo.Set(mapped[j][0])
				}
				if hi.Cmp(mapped[j][1]) > 0 {
					hi.Set(mapped[j][1])
				}
				if lo.Cmp(hi) > 0 {
					return zero, ErrConvergence
				}
				if lo.Cmp(hi) < 0 {
					item.Dimension++
				}
				item.B[j] = [2]*big.Rat{new(big.Rat).Sub(lo, shift), new(big.Rat).Sub(hi, shift)}
				item.A[i] = [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Sub(lo, r.offset[i]), r.scale[i]), new(big.Rat).Quo(new(big.Rat).Sub(hi, r.offset[i]), r.scale[i])}
				if item.A[i][0].Cmp(item.A[i][1]) > 0 {
					item.A[i][0], item.A[i][1] = item.A[i][1], item.A[i][0]
				}
				for _, q := range []*big.Rat{shift, item.B[j][0], item.B[j][1], item.A[i][0], item.A[i][1]} {
					if e := budget.check(q); e != nil {
						return zero, e
					}
				}
			}
			out.Components = append(out.Components, item)
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Complete = true
	out.ExactOperations = budget.operations
	return out, nil
}
