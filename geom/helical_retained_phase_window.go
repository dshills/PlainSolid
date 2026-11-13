package geom

import (
	"context"
	"math/big"
	"sort"
)

// HelicalPhaseWindowComparison exhausts affine endpoint-order cells at a fixed
// radius. NoPositiveOverlap is universal over the closed phase window when
// Complete. A witness is an exact meridian overlap, not a rounded 3D point.
type HelicalPhaseWindowComparison struct {
	Complete, NoPositiveOverlap bool
	Events                      []*big.Rat
	Probes                      int
	WitnessPhase                *big.Rat
	Witness                     HelicalRetainedPair
	ExactOperations             int
}

// CompareHelicalRetainedPhaseWindow uses the same local axial stock/shift and
// regularized slice semantics as CompareHelicalRetainedQuarter. It supports
// 0<=phaseLo<=phaseHi<1. All cutter endpoints have common slope lead; therefore
// only source turn-window events and cutter/stock endpoint equalities can
// change retained overlap. Every event and every open cell midpoint is checked
// exactly. MaxIntervals bounds events/probes as well as each source enumeration.
func CompareHelicalRetainedPhaseWindow(ctx context.Context, a, c SeparatedHelicalStarts, radius, shift *big.Rat, phase [2]*big.Rat, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (out HelicalPhaseWindowComparison, err error) {
	if ctx == nil || a.count < 1 || c.count < 1 || radius == nil || shift == nil || phase[0] == nil || phase[1] == nil || phase[0].Sign() < 0 || phase[1].Cmp(big.NewRat(1, 1)) >= 0 || phase[0].Cmp(phase[1]) > 0 {
		return out, ErrInvalid
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
			out.NoPositiveOverlap = false
		}
	}()
	// Validate all recipe/band contracts through the shared exact evaluator.
	oo := o
	oo.MaxOperations = ops
	first, e := compareHelicalRetainedPhase(ctx, a, c, radius, phase[0], shift, bandA, bandB, oo)
	b.operations += first.ExactOperations
	if e != nil {
		return out, e
	}
	events := map[string]*big.Rat{}
	add := func(q *big.Rat) error {
		if e := b.check(q); e != nil {
			return e
		}
		if q.Cmp(phase[0]) >= 0 && q.Cmp(phase[1]) <= 0 {
			events[q.RatString()] = new(big.Rat).Set(q)
			if len(events) > max {
				return ErrConvergence
			}
		}
		return nil
	}
	if e = add(phase[0]); e != nil {
		return out, e
	}
	if e = add(phase[1]); e != nil {
		return out, e
	}
	floor := func(q *big.Rat) *big.Int {
		v, r := new(big.Int), new(big.Int)
		v.QuoRem(q.Num(), q.Denom(), r)
		if q.Sign() < 0 && r.Sign() != 0 {
			v.Sub(v, big.NewInt(1))
		}
		return v
	}
	stock := []*big.Rat{bandA[0], bandA[1], new(big.Rat).Add(bandB[0], shift), new(big.Rat).Add(bandB[1], shift)}
	for si, s := range []SeparatedHelicalStarts{a, c} {
		offset := new(big.Rat)
		if si == 1 {
			offset.Set(shift)
		}
		sl, e := exactHelicalPolygonSlice(ctx, s.base.profile, radius, b)
		if e != nil {
			return out, e
		}
		if !sl.Present || sl.Lower.Cmp(sl.Upper) == 0 {
			continue
		}
		low, high := new(big.Rat).SetFloat64(s.base.reference.domain.UMin), new(big.Rat).SetFloat64(s.base.reference.domain.UMax)
		n0 := new(big.Int).Neg(floor(new(big.Rat).Neg(new(big.Rat).Sub(low, phase[1]))))
		n1 := floor(new(big.Rat).Sub(high, phase[0]))
		if n0.Cmp(n1) > 0 {
			continue
		}
		count := new(big.Int).Add(new(big.Int).Sub(n1, n0), big.NewInt(1))
		count.Mul(count, big.NewInt(int64(s.count)))
		if !n0.IsInt64() || !n1.IsInt64() || !count.IsInt64() || count.Int64() > int64(max) {
			return out, ErrConvergence
		}
		lead := new(big.Rat).SetFloat64(s.base.reference.lead)
		for n := n0.Int64(); ; n++ {
			if e = ctx.Err(); e != nil {
				return out, e
			}
			nr := big.NewRat(n, 1)
			if e = add(new(big.Rat).Sub(low, nr)); e != nil {
				return out, e
			}
			if e = add(new(big.Rat).Sub(high, nr)); e != nil {
				return out, e
			}
			for j := 0; j < s.count; j++ {
				base := new(big.Rat).Add(offset, new(big.Rat).Add(new(big.Rat).Mul(lead, nr), new(big.Rat).Mul(s.spacing, big.NewRat(int64(j), 1))))
				for _, endpoint := range []*big.Rat{sl.Lower, sl.Upper} {
					z := new(big.Rat).Add(base, endpoint)
					for _, cap := range stock {
						if e = add(new(big.Rat).Quo(new(big.Rat).Sub(cap, z), lead)); e != nil {
							return out, e
						}
					}
				}
			}
			if n == n1.Int64() {
				break
			}
		}
	}
	for _, v := range events {
		out.Events = append(out.Events, v)
	}
	sort.Slice(out.Events, func(i, j int) bool { return out.Events[i].Cmp(out.Events[j]) < 0 })
	var probes []*big.Rat
	for i, v := range out.Events {
		probes = append(probes, v)
		if i+1 < len(out.Events) {
			probes = append(probes, new(big.Rat).Quo(new(big.Rat).Add(v, out.Events[i+1]), big.NewRat(2, 1)))
		}
	}
	if len(probes) > max {
		return out, ErrConvergence
	}
	for _, p := range probes {
		if b.operations >= ops {
			return out, ErrConvergence
		}
		oo.MaxOperations = ops - b.operations
		q, e := compareHelicalRetainedPhase(ctx, a, c, radius, p, shift, bandA, bandB, oo)
		b.operations += q.ExactOperations
		out.Probes++
		if e != nil {
			return out, e
		}
		if q.Overlap {
			out.WitnessPhase = new(big.Rat).Set(p)
			out.Witness = q
			out.Complete = true
			return out, nil
		}
	}
	out.Complete = true
	out.NoPositiveOverlap = true
	return out, nil
}
