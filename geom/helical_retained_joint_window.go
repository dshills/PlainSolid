package geom

import (
	"context"
	"math/big"
	"sort"
)

// HelicalJointWindowComparison is an exhaustive selected meridian-coordinate
// arrangement proof, not a Euclidean clearance bound. Complete clear results
// exclude positive axial overlap at every radius and phase in the closed box.
type HelicalJointWindowComparison struct {
	Complete, NoPositiveOverlap bool
	RadiusEvents                []*big.Rat
	Probes                      int
	WitnessRadius, WitnessPhase *big.Rat
	Witness                     HelicalRetainedPair
	ExactOperations             int
}

// CompareHelicalRetainedJointWindow vertically decomposes affine endpoint
// events on each polygon vertex-radius slab. It includes source finite-turn
// events, cutter/cutter equality radii, phase-bound crossings, and phase-event
// line intersections. Exact phase-window checks at every resulting radial
// event and open-cell midpoint then exhaust the closed box. MaxIntervals caps
// coefficients, arrangement lines, radial events and probes; MaxOperations is
// cumulative across construction and all subordinate proofs.
func CompareHelicalRetainedJointWindow(ctx context.Context, a, c SeparatedHelicalStarts, shift *big.Rat, radii, phases, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (out HelicalJointWindowComparison, err error) {
	if ctx == nil || radii[0] == nil || radii[1] == nil || radii[0].Sign() <= 0 || radii[0].Cmp(radii[1]) > 0 {
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
	initial, e := CompareHelicalRetainedPhaseWindow(ctx, a, c, radii[0], shift, phases, bandA, bandB, oo)
	b.operations += initial.ExactOperations
	if e != nil {
		return out, e
	}
	events := map[string]*big.Rat{}
	add := func(v *big.Rat) error {
		if e := b.check(v); e != nil {
			return e
		}
		if v.Cmp(radii[0]) >= 0 && v.Cmp(radii[1]) <= 0 {
			events[v.RatString()] = new(big.Rat).Set(v)
			if len(events) > max {
				return ErrConvergence
			}
		}
		return nil
	}
	for _, v := range radii {
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
		n, r := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), r)
		if q.Sign() < 0 && r.Sign() != 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	type affine struct{ r, p, k *big.Rat }
	type line struct{ r, k *big.Rat }
	zero := func() *big.Rat { return new(big.Rat) }
	vertices := sorted()
	for cell := 0; cell+1 < len(vertices); cell++ {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		lo, hi := vertices[cell], vertices[cell+1]
		span := new(big.Rat).Sub(hi, lo)
		x := new(big.Rat).Add(lo, new(big.Rat).Mul(span, big.NewRat(1, 4)))
		y := new(big.Rat).Add(lo, new(big.Rat).Mul(span, big.NewRat(3, 4)))
		var functions []affine
		for _, v := range []*big.Rat{bandA[0], bandA[1], new(big.Rat).Add(bandB[0], shift), new(big.Rat).Add(bandB[1], shift)} {
			functions = append(functions, affine{zero(), zero(), new(big.Rat).Set(v)})
		}
		lines := []line{{zero(), new(big.Rat).Set(phases[0])}, {zero(), new(big.Rat).Set(phases[1])}}
		for si, s := range []SeparatedHelicalStarts{a, c} {
			sx, e := exactHelicalPolygonSlice(ctx, s.base.profile, x, b)
			if e != nil {
				return out, e
			}
			sy, e := exactHelicalPolygonSlice(ctx, s.base.profile, y, b)
			if e != nil {
				return out, e
			}
			if sx.Present != sy.Present {
				return out, ErrConvergence
			}
			if !sx.Present {
				continue
			}
			lower, upper := new(big.Rat).SetFloat64(s.base.reference.domain.UMin), new(big.Rat).SetFloat64(s.base.reference.domain.UMax)
			n0 := new(big.Int).Neg(floor(new(big.Rat).Neg(new(big.Rat).Sub(lower, phases[1]))))
			n1 := floor(new(big.Rat).Sub(upper, phases[0]))
			if n0.Cmp(n1) > 0 {
				continue
			}
			count := new(big.Int).Mul(new(big.Int).Add(new(big.Int).Sub(n1, n0), big.NewInt(1)), big.NewInt(int64(s.count)))
			if !n0.IsInt64() || !n1.IsInt64() || !count.IsInt64() || count.Int64() > int64(max) {
				return out, ErrConvergence
			}
			lead := new(big.Rat).SetFloat64(s.base.reference.lead)
			off := zero()
			if si == 1 {
				off.Set(shift)
			}
			for n := n0.Int64(); ; n++ {
				nr := big.NewRat(n, 1)
				lines = append(lines, line{zero(), new(big.Rat).Sub(lower, nr)}, line{zero(), new(big.Rat).Sub(upper, nr)})
				for j := 0; j < s.count; j++ {
					offset := new(big.Rat).Add(off, new(big.Rat).Add(new(big.Rat).Mul(lead, nr), new(big.Rat).Mul(s.spacing, big.NewRat(int64(j), 1))))
					for side, vx := range []*big.Rat{sx.Lower, sx.Upper} {
						vy := []*big.Rat{sy.Lower, sy.Upper}[side]
						slope := new(big.Rat).Quo(new(big.Rat).Sub(vy, vx), new(big.Rat).Sub(y, x))
						constant := new(big.Rat).Add(offset, new(big.Rat).Sub(vx, new(big.Rat).Mul(slope, x)))
						functions = append(functions, affine{slope, new(big.Rat).Set(lead), constant})
						if len(functions) > max {
							return out, ErrConvergence
						}
					}
				}
				if n == n1.Int64() {
					break
				}
			}
		}
		localAdd := func(v *big.Rat) error {
			if v.Cmp(lo) > 0 && v.Cmp(hi) < 0 {
				return add(v)
			}
			return b.check(v)
		}
		for i, f := range functions {
			for _, g := range functions[i+1:] {
				dr, dp, dk := new(big.Rat).Sub(f.r, g.r), new(big.Rat).Sub(f.p, g.p), new(big.Rat).Sub(f.k, g.k)
				if e = b.check(dk); e != nil {
					return out, e
				}
				if dp.Sign() == 0 {
					if dr.Sign() != 0 {
						if e = localAdd(new(big.Rat).Quo(new(big.Rat).Neg(dk), dr)); e != nil {
							return out, e
						}
					}
					continue
				}
				lines = append(lines, line{new(big.Rat).Quo(new(big.Rat).Neg(dr), dp), new(big.Rat).Quo(new(big.Rat).Neg(dk), dp)})
				if len(lines) > max {
					return out, ErrConvergence
				}
			}
		}
		for i, f := range lines {
			for _, g := range lines[i+1:] {
				den := new(big.Rat).Sub(f.r, g.r)
				if e = b.check(den); e != nil {
					return out, e
				}
				if den.Sign() != 0 {
					if e = localAdd(new(big.Rat).Quo(new(big.Rat).Sub(g.k, f.k), den)); e != nil {
						return out, e
					}
				}
			}
		}
	}
	out.RadiusEvents = sorted()
	var probes []*big.Rat
	for i, v := range out.RadiusEvents {
		probes = append(probes, v)
		if i+1 < len(out.RadiusEvents) {
			probes = append(probes, new(big.Rat).Mul(new(big.Rat).Add(v, out.RadiusEvents[i+1]), big.NewRat(1, 2)))
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
		q, e := CompareHelicalRetainedPhaseWindow(ctx, a, c, r, shift, phases, bandA, bandB, oo)
		b.operations += q.ExactOperations
		out.Probes++
		if e != nil {
			return out, e
		}
		if !q.NoPositiveOverlap {
			out.Complete = true
			out.WitnessRadius = new(big.Rat).Set(r)
			out.WitnessPhase = q.WitnessPhase
			out.Witness = q.Witness
			return out, nil
		}
	}
	out.Complete = true
	out.NoPositiveOverlap = true
	return out, nil
}
