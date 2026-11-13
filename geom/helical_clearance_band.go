package geom

import (
	"context"
	"math/big"
	"sort"
)

type HelicalClearanceBandOptions struct {
	SurfacePatchEvaluationOptions
	MaxCells int
}

// HelicalClearanceEvent gives exact signed clearance at an original radius.
// Events include band ends, profile vertex radii, affine-gap switches and zeros.
type HelicalClearanceEvent struct{ Radius, Clearance *big.Rat }

// HelicalClearanceCell is a positive-width radial interval on which clearance
// is affine. InteriorSign is strict +1/-1 on its open interior, or zero when
// the entire cell is coincident in axial slice clearance. Endpoints can be zero.
type HelicalClearanceCell struct {
	Radius, Clearance [2]*big.Rat
	InteriorSign      int
}

// HelicalClearanceBand reports the full closed common radial intersection with
// the requested band. Minimum/Maximum are attained exact signed clearances.
// HasBand is false for no common radial/turn interval; a single common radius
// has one Event but no Cells. No conclusions apply outside the reported range.
type HelicalClearanceBand struct {
	Complete, HasBand bool
	CommonTurns       [2]*big.Rat
	RadialRange       [2]*big.Rat
	Events            []HelicalClearanceEvent
	Cells             []HelicalClearanceCell
	Minimum, Maximum  *big.Rat
	ExactOperations   int
}

// CompareHelicalRadialBand computes all signed-clearance regimes for aligned
// frame/lead/hand profiles at equal physical turn. TargetOffset is added to B.
// Convex polygon slices are affine between vertex radii. The maximum of the
// two directed affine gaps has only their crossing and zero roots as additional
// breakpoints; exact rational splitting therefore proves exhaustive coverage.
func CompareHelicalRadialBand(ctx context.Context, a, c HelicalPolygon, band [2]*big.Rat, targetOffset *big.Rat, o HelicalClearanceBandOptions) (out HelicalClearanceBand, err error) {
	if ctx == nil || len(a.profile) < 3 || len(c.profile) < 3 || band[0] == nil || band[1] == nil || band[0].Cmp(band[1]) >= 0 || targetOffset == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if a.reference.frame != c.reference.frame || a.reference.lead != c.reference.lead || a.reference.handedness != c.reference.handedness {
		return out, ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 512
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.HasBand = false
			out.Events = nil
			out.Cells = nil
			out.Minimum = nil
			out.Maximum = nil
		}
	}()
	for _, q := range []*big.Rat{band[0], band[1], targetOffset} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	low, high := new(big.Rat).Set(band[0]), new(big.Rat).Set(band[1])
	for _, p := range []HelicalPolygon{a, c} {
		lo, hi := p.reference.start.X, p.reference.end.X
		if q := new(big.Rat).SetFloat64(lo); q.Cmp(low) > 0 {
			low = q
		}
		if q := new(big.Rat).SetFloat64(hi); q.Cmp(high) < 0 {
			high = q
		}
	}
	if low.Cmp(high) > 0 {
		out.Complete = true
		return out, nil
	}
	out.RadialRange = [2]*big.Rat{low, high}
	cache := map[string]HelicalSliceClearance{}
	evaluate := func(radius *big.Rat) (HelicalSliceClearance, error) {
		if r, ok := cache[radius.RatString()]; ok {
			return r, nil
		}
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return HelicalSliceClearance{}, ErrConvergence
		}
		r, e := CompareHelicalMeridianSlices(ctx, a, c, radius, targetOffset, SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += r.ExactOperations
		if e == nil {
			cache[radius.RatString()] = r
		}
		return r, e
	}
	initial, e := evaluate(low)
	if e != nil {
		return out, e
	}
	if !initial.Comparable {
		out.Complete = true
		return out, nil
	}
	out.CommonTurns = initial.CommonTurns
	out.HasBand = true
	cuts := []*big.Rat{low, high}
	for _, p := range []HelicalPolygon{a, c} {
		for _, v := range p.profile {
			x := new(big.Rat).SetFloat64(v.X)
			if x.Cmp(low) > 0 && x.Cmp(high) < 0 {
				cuts = append(cuts, x)
			}
		}
	}
	unique := func(values []*big.Rat) []*big.Rat {
		sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
		r := values[:0]
		for _, q := range values {
			if len(r) == 0 || q.Cmp(r[len(r)-1]) != 0 {
				r = append(r, q)
			}
		}
		return r
	}
	cuts = unique(cuts)
	all := append([]*big.Rat(nil), cuts...)
	gaps := func(r HelicalSliceClearance) (*big.Rat, *big.Rat) {
		return new(big.Rat).Sub(r.B.Lower, r.A.Upper), new(big.Rat).Sub(r.A.Lower, r.B.Upper)
	}
	for i := 0; i+1 < len(cuts); i++ {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		left, e := evaluate(cuts[i])
		if e != nil {
			return out, e
		}
		right, e := evaluate(cuts[i+1])
		if e != nil {
			return out, e
		}
		if !left.Comparable || !right.Comparable {
			return out, ErrConvergence
		}
		f0, g0 := gaps(left)
		f1, g1 := gaps(right)
		for _, pair := range [][2]*big.Rat{{f0, f1}, {g0, g1}, {new(big.Rat).Sub(f0, g0), new(big.Rat).Sub(f1, g1)}} {
			if pair[0].Sign()*pair[1].Sign() < 0 {
				fraction := new(big.Rat).Quo(new(big.Rat).Neg(pair[0]), new(big.Rat).Sub(pair[1], pair[0]))
				root := new(big.Rat).Add(cuts[i], new(big.Rat).Mul(fraction, new(big.Rat).Sub(cuts[i+1], cuts[i])))
				if e := b.check(root); e != nil {
					return out, e
				}
				all = append(all, root)
			}
		}
		if len(all) > o.MaxCells+1+len(cuts) {
			return out, ErrConvergence
		}
	}
	all = unique(all)
	if len(all)-1 > o.MaxCells {
		return out, ErrConvergence
	}
	for _, radius := range all {
		r, e := evaluate(radius)
		if e != nil {
			return out, e
		}
		if !r.Comparable {
			return out, ErrConvergence
		}
		q := new(big.Rat).Set(r.SignedClearance)
		out.Events = append(out.Events, HelicalClearanceEvent{new(big.Rat).Set(radius), q})
		if out.Minimum == nil || q.Cmp(out.Minimum) < 0 {
			out.Minimum = new(big.Rat).Set(q)
		}
		if out.Maximum == nil || q.Cmp(out.Maximum) > 0 {
			out.Maximum = new(big.Rat).Set(q)
		}
	}
	for i := 0; i+1 < len(out.Events); i++ {
		l, r := out.Events[i], out.Events[i+1]
		sign := l.Clearance.Sign()
		if sign == 0 {
			sign = r.Clearance.Sign()
		}
		if l.Clearance.Sign()*r.Clearance.Sign() < 0 {
			return out, ErrConvergence
		}
		out.Cells = append(out.Cells, HelicalClearanceCell{Radius: [2]*big.Rat{new(big.Rat).Set(l.Radius), new(big.Rat).Set(r.Radius)}, Clearance: [2]*big.Rat{new(big.Rat).Set(l.Clearance), new(big.Rat).Set(r.Clearance)}, InteriorSign: sign})
	}
	out.Complete = true
	return out, nil
}
