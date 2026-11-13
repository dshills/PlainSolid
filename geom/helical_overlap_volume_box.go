package geom

import (
	"context"
	"math/big"
	"sort"
)

// HelicalOverlapVolumeBox proves an annular-sector axial box is contained in
// both retained stocks. Volume is exactly PiCoefficient*pi in model units;
// the represented frame determinant is included. No transcendental floating
// volume is used as evidence. Bounds and coefficients belong to the caller.
type HelicalOverlapVolumeBox struct {
	Proven                                                      bool
	Radius, Phase, Axial                                        [2]*big.Rat
	FrameAbsoluteDeterminant, LocalPiCoefficient, PiCoefficient *big.Rat
	CutterCopies, ExactOperations                               int
}

// CertifyHelicalOverlapVolumeBox proves strict separation from every cutter
// over a supplied positive radius/phase/axial box. It uses conservative exact
// axial hulls of convex-profile slices (including all vertex radii) and each
// finite turn's clipped phase range. Overlapping hulls return ErrConvergence,
// never a collision/absence claim. Same represented frame/lead/hand required.
// 0<=phaseLo<phaseHi<=1 prevents angular double coverage except measure-zero
// boundary identifications. The open box has the reported positive volume.
func CertifyHelicalOverlapVolumeBox(ctx context.Context, a, c SeparatedHelicalStarts, shift *big.Rat, radii, phases, axial, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (out HelicalOverlapVolumeBox, err error) {
	if ctx == nil || shift == nil || a.count < 1 || c.count < 1 {
		return out, ErrInvalid
	}
	for _, v := range [][2]*big.Rat{radii, phases, axial, bandA, bandB} {
		if v[0] == nil || v[1] == nil || v[0].Cmp(v[1]) >= 0 {
			return out, ErrInvalid
		}
	}
	if radii[0].Sign() <= 0 || phases[0].Sign() < 0 || phases[1].Cmp(big.NewRat(1, 1)) > 0 {
		return out, ErrDomain
	}
	if a.base.reference.frame != c.base.reference.frame || a.base.reference.lead != c.base.reference.lead || a.base.reference.handedness != c.base.reference.handedness {
		return out, ErrDomain
	}
	if axial[0].Cmp(bandA[0]) < 0 || axial[1].Cmp(bandA[1]) > 0 || axial[0].Cmp(new(big.Rat).Add(bandB[0], shift)) < 0 || axial[1].Cmp(new(big.Rat).Add(bandB[1], shift)) > 0 {
		return out, ErrDomain
	}
	ops, bits, max := o.MaxOperations, o.MaxCoefficientBits, o.MaxIntervals
	if ops == 0 {
		ops = 1000000
	}
	if bits == 0 {
		bits = 16384
	}
	if max == 0 {
		max = 8192
	}
	if ops < 1 || bits < 64 || max < 1 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Proven = false
		}
	}()
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, v := range [][2]*big.Rat{radii, phases, axial, bandA, bandB} {
		for _, q := range v {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	if e := b.check(shift); e != nil {
		return out, e
	}
	floor := func(q *big.Rat) *big.Int {
		n, r := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), r)
		if q.Sign() < 0 && r.Sign() != 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	for si, s := range []SeparatedHelicalStarts{a, c} {
		rs := []*big.Rat{new(big.Rat).Set(radii[0]), new(big.Rat).Set(radii[1])}
		for _, p := range s.base.profile {
			r := new(big.Rat).SetFloat64(p.X)
			if r.Cmp(radii[0]) > 0 && r.Cmp(radii[1]) < 0 {
				rs = append(rs, r)
			}
		}
		sort.Slice(rs, func(i, j int) bool { return rs[i].Cmp(rs[j]) < 0 })
		var low, high *big.Rat
		for _, r := range rs {
			sl, e := exactHelicalPolygonSlice(ctx, s.base.profile, r, b)
			if e != nil {
				return out, e
			}
			if sl.Present {
				if low == nil || sl.Lower.Cmp(low) < 0 {
					low = sl.Lower
				}
				if high == nil || sl.Upper.Cmp(high) > 0 {
					high = sl.Upper
				}
			}
		}
		if low == nil {
			continue
		}
		u0, u1 := new(big.Rat).SetFloat64(s.base.reference.domain.UMin), new(big.Rat).SetFloat64(s.base.reference.domain.UMax)
		n0 := new(big.Int).Neg(floor(new(big.Rat).Neg(new(big.Rat).Sub(u0, phases[1]))))
		n1 := floor(new(big.Rat).Sub(u1, phases[0]))
		if n0.Cmp(n1) > 0 {
			continue
		}
		count := new(big.Int).Mul(new(big.Int).Add(new(big.Int).Sub(n1, n0), big.NewInt(1)), big.NewInt(int64(s.count)))
		if !n0.IsInt64() || !n1.IsInt64() || !count.IsInt64() || count.Int64() > int64(max-out.CutterCopies) {
			return out, ErrConvergence
		}
		lead := new(big.Rat).SetFloat64(s.base.reference.lead)
		offset := new(big.Rat)
		if si == 1 {
			offset.Set(shift)
		}
		for n := n0.Int64(); ; n++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			nr := big.NewRat(n, 1)
			lo, hi := new(big.Rat).Add(phases[0], nr), new(big.Rat).Add(phases[1], nr)
			if lo.Cmp(u0) < 0 {
				lo.Set(u0)
			}
			if hi.Cmp(u1) > 0 {
				hi.Set(u1)
			}
			for j := 0; j < s.count; j++ {
				out.CutterCopies++
				z := new(big.Rat).Add(offset, new(big.Rat).Mul(s.spacing, big.NewRat(int64(j), 1)))
				l, h := new(big.Rat).Add(z, new(big.Rat).Add(low, new(big.Rat).Mul(lead, lo))), new(big.Rat).Add(z, new(big.Rat).Add(high, new(big.Rat).Mul(lead, hi)))
				if e := b.check(l); e != nil {
					return out, e
				}
				if e := b.check(h); e != nil {
					return out, e
				}
				if h.Cmp(axial[0]) >= 0 && l.Cmp(axial[1]) <= 0 {
					return out, ErrConvergence
				}
			}
			if n == n1.Int64() {
				break
			}
		}
	}
	f := a.base.reference.frame
	v := [3]Vec3{f.XAxis(), f.YAxis(), f.ZAxis()}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		term := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(v[1], j)), new(big.Rat).SetFloat64(coordinate(v[2], k))), new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(v[1], k)), new(big.Rat).SetFloat64(coordinate(v[2], j))))
		det.Add(det, new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(v[0], i)), term))
	}
	det.Abs(det)
	if det.Sign() == 0 {
		return out, ErrDegenerate
	}
	coeff := new(big.Rat).Mul(new(big.Rat).Sub(new(big.Rat).Mul(radii[1], radii[1]), new(big.Rat).Mul(radii[0], radii[0])), new(big.Rat).Mul(new(big.Rat).Sub(phases[1], phases[0]), new(big.Rat).Sub(axial[1], axial[0])))
	world := new(big.Rat).Mul(coeff, det)
	for _, q := range []*big.Rat{det, coeff, world} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.FrameAbsoluteDeterminant = det
	out.LocalPiCoefficient = coeff
	out.PiCoefficient = world
	for i := 0; i < 2; i++ {
		out.Radius[i] = new(big.Rat).Set(radii[i])
		out.Phase[i] = new(big.Rat).Set(phases[i])
		out.Axial[i] = new(big.Rat).Set(axial[i])
	}
	out.Proven = true
	return out, nil
}
