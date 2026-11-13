package geom

import (
	"context"
	"math/big"
	"sort"
)

// HelicalRadialBandComparison exhausts exact endpoint-order cells at a fixed
// rational physical-turn phase. NoPositiveOverlap is universal on RadiusBand
// when Complete; otherwise WitnessRadius binds a positive axial overlap.
type HelicalRadialBandComparison struct {
	Complete, NoPositiveOverlap bool
	Events                      []*big.Rat
	Probes                      int
	WitnessRadius               *big.Rat
	Witness                     HelicalRetainedPair
	ExactOperations             int
}

// CompareHelicalRetainedRadialBand fixes 0<=phase<1 and partitions a positive
// rational radius band at all profile vertex radii and all affine endpoint
// equality events. Convex polygon slices have affine endpoints on each open
// vertex slab. Stock/cutter and cutter/cutter order therefore cannot change
// between the enumerated events. Exact events and rational midpoints suffice
// to prove absence of positive-length retained axial overlap everywhere.
// This is a radial-meridian proof at one phase, not a complete 3D collision test.
func CompareHelicalRetainedRadialBand(ctx context.Context, a, c SeparatedHelicalStarts, phase, shift *big.Rat, radiusBand, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (out HelicalRadialBandComparison, err error) {
	if ctx == nil || radiusBand[0] == nil || radiusBand[1] == nil || radiusBand[0].Sign() <= 0 || radiusBand[0].Cmp(radiusBand[1]) > 0 {
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
	oo := o
	oo.MaxOperations = ops
	first, e := compareHelicalRetainedPhase(ctx, a, c, radiusBand[0], phase, shift, bandA, bandB, oo)
	b.operations += first.ExactOperations
	if e != nil {
		return out, e
	}
	events := map[string]*big.Rat{}
	add := func(q *big.Rat) error {
		if e := b.check(q); e != nil {
			return e
		}
		if q.Cmp(radiusBand[0]) >= 0 && q.Cmp(radiusBand[1]) <= 0 {
			events[q.RatString()] = new(big.Rat).Set(q)
			if len(events) > max {
				return ErrConvergence
			}
		}
		return nil
	}
	for _, v := range radiusBand {
		if e = add(v); e != nil {
			return out, e
		}
	}
	for _, s := range []SeparatedHelicalStarts{a, c} {
		for _, v := range s.base.profile {
			if e = add(new(big.Rat).SetFloat64(v.X)); e != nil {
				return out, e
			}
		}
	}
	sorted := func() []*big.Rat {
		var v []*big.Rat
		for _, q := range events {
			v = append(v, q)
		}
		sort.Slice(v, func(i, j int) bool { return v[i].Cmp(v[j]) < 0 })
		return v
	}
	floor := func(q *big.Rat) *big.Int {
		v, r := new(big.Int), new(big.Int)
		v.QuoRem(q.Num(), q.Denom(), r)
		if q.Sign() < 0 && r.Sign() != 0 {
			v.Sub(v, big.NewInt(1))
		}
		return v
	}
	// Return endpoints in stable recipe/turn/start/lower-upper order, regardless
	// of whether the endpoint currently lies inside stock. Clipping changes are
	// precisely among the subsequent pairwise equality events.
	endpoints := func(radius *big.Rat) ([]*big.Rat, error) {
		result := []*big.Rat{new(big.Rat).Set(bandA[0]), new(big.Rat).Set(bandA[1]), new(big.Rat).Add(bandB[0], shift), new(big.Rat).Add(bandB[1], shift)}
		for si, s := range []SeparatedHelicalStarts{a, c} {
			sl, e := exactHelicalPolygonSlice(ctx, s.base.profile, radius, b)
			if e != nil {
				return nil, e
			}
			if !sl.Present {
				continue
			}
			offset := new(big.Rat)
			if si == 1 {
				offset.Set(shift)
			}
			lo := new(big.Rat).Sub(new(big.Rat).SetFloat64(s.base.reference.domain.UMin), phase)
			hi := new(big.Rat).Sub(new(big.Rat).SetFloat64(s.base.reference.domain.UMax), phase)
			n0 := new(big.Int).Neg(floor(new(big.Rat).Neg(lo)))
			n1 := floor(hi)
			if n0.Cmp(n1) > 0 {
				continue
			}
			count := new(big.Int).Mul(new(big.Int).Add(new(big.Int).Sub(n1, n0), big.NewInt(1)), big.NewInt(int64(s.count)))
			if !n0.IsInt64() || !n1.IsInt64() || !count.IsInt64() || count.Int64() > int64(max) {
				return nil, ErrConvergence
			}
			lead := new(big.Rat).SetFloat64(s.base.reference.lead)
			for n := n0.Int64(); ; n++ {
				for j := 0; j < s.count; j++ {
					z := new(big.Rat).Add(offset, new(big.Rat).Add(new(big.Rat).Mul(lead, new(big.Rat).Add(phase, big.NewRat(n, 1))), new(big.Rat).Mul(s.spacing, big.NewRat(int64(j), 1))))
					for _, v := range []*big.Rat{sl.Lower, sl.Upper} {
						q := new(big.Rat).Add(z, v)
						if e = b.check(q); e != nil {
							return nil, e
						}
						result = append(result, q)
						if len(result) > max {
							return nil, ErrConvergence
						}
					}
				}
				if n == n1.Int64() {
					break
				}
			}
		}
		return result, nil
	}
	vertex := sorted()
	for i := 0; i+1 < len(vertex); i++ {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		lo, hi := vertex[i], vertex[i+1]
		delta := new(big.Rat).Sub(hi, lo)
		x := new(big.Rat).Add(lo, new(big.Rat).Quo(delta, big.NewRat(4, 1)))
		y := new(big.Rat).Add(lo, new(big.Rat).Mul(delta, big.NewRat(3, 4)))
		vx, e := endpoints(x)
		if e != nil {
			return out, e
		}
		vy, e := endpoints(y)
		if e != nil {
			return out, e
		}
		if len(vx) != len(vy) {
			return out, ErrConvergence
		}
		for j := 0; j < len(vx); j++ {
			for k := j + 1; k < len(vx); k++ {
				dx := new(big.Rat).Sub(vx[j], vx[k])
				dy := new(big.Rat).Sub(vy[j], vy[k])
				den := new(big.Rat).Sub(dy, dx)
				if e = b.check(den); e != nil {
					return out, e
				}
				if den.Sign() == 0 {
					continue
				}
				root := new(big.Rat).Sub(x, new(big.Rat).Quo(new(big.Rat).Mul(dx, new(big.Rat).Sub(y, x)), den))
				if root.Cmp(lo) > 0 && root.Cmp(hi) < 0 {
					if e = add(root); e != nil {
						return out, e
					}
				}
			}
		}
	}
	out.Events = sorted()
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
	for _, r := range probes {
		if b.operations >= ops {
			return out, ErrConvergence
		}
		oo.MaxOperations = ops - b.operations
		q, e := compareHelicalRetainedPhase(ctx, a, c, r, phase, shift, bandA, bandB, oo)
		b.operations += q.ExactOperations
		out.Probes++
		if e != nil {
			return out, e
		}
		if q.Overlap {
			out.WitnessRadius = new(big.Rat).Set(r)
			out.Witness = q
			out.Complete = true
			return out, nil
		}
	}
	out.Complete = true
	out.NoPositiveOverlap = true
	return out, nil
}
