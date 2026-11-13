package geom

import (
	"context"
	"math/big"
	"sort"
)

// HelicalRetainedBoundary identifies a stock cap or an original cutter slice
// endpoint. CutterUpper means the upper axial side of that cutter.
type HelicalRetainedBoundary struct {
	StockCap    bool
	Upper       bool
	Start, Turn int
	CutterUpper bool
}
type HelicalRetainedInterval struct {
	Bounds   [2]*big.Rat
	Boundary [2]HelicalRetainedBoundary
}
type HelicalRetainedContact struct {
	A, B   int
	Bounds [2]*big.Rat
	Touch  bool
}
type HelicalRetainedPair struct {
	Complete        bool
	A, B            []HelicalRetainedInterval
	Contacts        []HelicalRetainedContact
	Overlap, Touch  bool
	ExactOperations int
}

// CompareHelicalRetainedQuarter compares regularized one-dimensional stock
// complements of two finite cutter recipes at one exact quarter-turn phase.
// Bands are local axial frame coordinates; shift translates B's stock AND
// cutters. Positive-length cutter slices remove their open interiors; their
// endpoints remain boundary. A zero-width cutter slice removes no interval.
// This is an exact selected-ray witness, not exhaustive 3D collision testing.
// Frame, lead and handedness must match. MaxIntervals bounds each enumeration
// and the final pair tests. Source turn windows retain their closed semantics.
func CompareHelicalRetainedQuarter(ctx context.Context, a, c SeparatedHelicalStarts, radius, phase, shift *big.Rat, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (out HelicalRetainedPair, err error) {
	if phase == nil || !new(big.Rat).Mul(phase, big.NewRat(4, 1)).IsInt() {
		return out, ErrDomain
	}
	if phase.Cmp(big.NewRat(1, 1)) >= 0 {
		return out, ErrInvalid
	}
	return compareHelicalRetainedPhase(ctx, a, c, radius, phase, shift, bandA, bandB, o)
}
func compareHelicalRetainedPhase(ctx context.Context, a, c SeparatedHelicalStarts, radius, phase, shift *big.Rat, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (out HelicalRetainedPair, err error) {
	if ctx == nil || a.count < 1 || c.count < 1 || radius == nil || phase == nil || shift == nil || phase.Sign() < 0 || phase.Cmp(big.NewRat(1, 1)) > 0 {
		return out, ErrInvalid
	}
	if a.base.reference.frame != c.base.reference.frame || a.base.reference.lead != c.base.reference.lead || a.base.reference.handedness != c.base.reference.handedness {
		return out, ErrDomain
	}
	for _, v := range [][2]*big.Rat{bandA, bandB} {
		if v[0] == nil || v[1] == nil || v[0].Cmp(v[1]) >= 0 {
			return out, ErrInvalid
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	max := o.MaxIntervals
	if max == 0 {
		max = 8192
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 1000000
	}
	if bits == 0 {
		bits = 16384
	}
	if max < 1 || ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
		}
	}()
	for _, v := range []*big.Rat{radius, phase, shift, bandA[0], bandA[1], bandB[0], bandB[1]} {
		if e := b.check(v); e != nil {
			return out, e
		}
	}
	floor := func(q *big.Rat) *big.Int {
		v, rem := new(big.Int), new(big.Int)
		v.QuoRem(q.Num(), q.Denom(), rem)
		if q.Sign() < 0 && rem.Sign() != 0 {
			v.Sub(v, big.NewInt(1))
		}
		return v
	}
	enumerate := func(s SeparatedHelicalStarts, band [2]*big.Rat, offset *big.Rat) ([]HelicalRetainedInterval, error) {
		sl, e := exactHelicalPolygonSlice(ctx, s.base.profile, radius, b)
		if e != nil {
			return nil, e
		}
		low, high := new(big.Rat).Add(band[0], offset), new(big.Rat).Add(band[1], offset)
		whole := HelicalRetainedInterval{[2]*big.Rat{low, high}, [2]HelicalRetainedBoundary{{StockCap: true}, {StockCap: true, Upper: true}}}
		if !sl.Present || sl.Lower.Cmp(sl.Upper) == 0 {
			return []HelicalRetainedInterval{whole}, nil
		}
		lo := new(big.Rat).Sub(new(big.Rat).SetFloat64(s.base.reference.domain.UMin), phase)
		hi := new(big.Rat).Sub(new(big.Rat).SetFloat64(s.base.reference.domain.UMax), phase)
		first := new(big.Int).Neg(floor(new(big.Rat).Neg(lo)))
		last := floor(hi)
		if first.Cmp(last) > 0 {
			return []HelicalRetainedInterval{whole}, nil
		}
		count := new(big.Int).Add(new(big.Int).Sub(last, first), big.NewInt(1))
		count.Mul(count, big.NewInt(int64(s.count)))
		if !first.IsInt64() || !last.IsInt64() || !count.IsInt64() || count.Int64() > int64(max) {
			return nil, ErrConvergence
		}
		type cut struct {
			lo, hi      *big.Rat
			start, turn int
		}
		var cuts []cut
		lead := new(big.Rat).SetFloat64(s.base.reference.lead)
		for n := first.Int64(); ; n++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			for j := 0; j < s.count; j++ {
				z := new(big.Rat).Add(offset, new(big.Rat).Add(new(big.Rat).Mul(lead, new(big.Rat).Add(phase, big.NewRat(n, 1))), new(big.Rat).Mul(s.spacing, big.NewRat(int64(j), 1))))
				l, h := new(big.Rat).Add(z, sl.Lower), new(big.Rat).Add(z, sl.Upper)
				if e := b.check(l); e != nil {
					return nil, e
				}
				if e := b.check(h); e != nil {
					return nil, e
				}
				if h.Cmp(low) > 0 && l.Cmp(high) < 0 {
					cuts = append(cuts, cut{l, h, j, int(n)})
				}
			}
			if n == last.Int64() {
				break
			}
		}
		sort.Slice(cuts, func(i, j int) bool { return cuts[i].lo.Cmp(cuts[j].lo) < 0 })
		cursor := low
		ancestry := whole.Boundary[0]
		var result []HelicalRetainedInterval
		for _, v := range cuts {
			if v.lo.Cmp(cursor) > 0 {
				result = append(result, HelicalRetainedInterval{[2]*big.Rat{new(big.Rat).Set(cursor), new(big.Rat).Set(v.lo)}, [2]HelicalRetainedBoundary{ancestry, {Start: v.start, Turn: v.turn}}})
			}
			if v.hi.Cmp(cursor) > 0 {
				cursor = v.hi
				ancestry = HelicalRetainedBoundary{Start: v.start, Turn: v.turn, CutterUpper: true}
			}
			if cursor.Cmp(high) >= 0 {
				break
			}
		}
		if cursor.Cmp(high) < 0 {
			result = append(result, HelicalRetainedInterval{[2]*big.Rat{new(big.Rat).Set(cursor), high}, [2]HelicalRetainedBoundary{ancestry, whole.Boundary[1]}})
		}
		return result, nil
	}
	out.A, err = enumerate(a, bandA, new(big.Rat))
	if err != nil {
		return out, err
	}
	out.B, err = enumerate(c, bandB, shift)
	if err != nil {
		return out, err
	}
	i, j, tests := 0, 0, 0
	for i < len(out.A) && j < len(out.B) {
		tests++
		if tests > max {
			return out, ErrConvergence
		}
		x, y := out.A[i], out.B[j]
		lo, hi := x.Bounds[0], x.Bounds[1]
		if y.Bounds[0].Cmp(lo) > 0 {
			lo = y.Bounds[0]
		}
		if y.Bounds[1].Cmp(hi) < 0 {
			hi = y.Bounds[1]
		}
		if lo.Cmp(hi) <= 0 {
			touch := lo.Cmp(hi) == 0
			out.Contacts = append(out.Contacts, HelicalRetainedContact{i, j, [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}, touch})
			if touch {
				out.Touch = true
			} else {
				out.Overlap = true
			}
		}
		if x.Bounds[1].Cmp(y.Bounds[1]) < 0 {
			i++
		} else {
			j++
		}
	}
	out.Complete = true
	return out, nil
}
