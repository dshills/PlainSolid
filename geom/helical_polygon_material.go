package geom

import (
	"context"
	"math/big"
	"sort"
)

// HelicalPolygon is a finite sweep of a strictly convex meridian polygon.
// Vertices are (positive radius, axial offset); U is measured in turns. Either
// polygon orientation is accepted. The swept set is the union of all meridian
// copies, including overlaps of neighboring turns; this is not a B-rep solid.
type HelicalPolygon struct {
	reference HelicalProfileSurface
	profile   []Vec2
}

// NewHelicalPolygon copies 3..64 finite vertices and proves strict convexity
// with exact represented-coordinate determinants. Collinear edges, repeated
// vertices, and nonconvex or self-intersecting vertex orders are rejected.
func NewHelicalPolygon(frame Frame, profile []Vec2, lead float64, hand HelixHandedness, uMin, uMax float64) (HelicalPolygon, error) {
	if len(profile) < 3 || len(profile) > 64 {
		return HelicalPolygon{}, ErrInvalid
	}
	lo, hi := profile[0].X, profile[0].X
	for _, p := range profile {
		if !finite(p.X) || !finite(p.Y) || p.X <= 0 {
			return HelicalPolygon{}, ErrInvalid
		}
		if p.X < lo {
			lo = p.X
		}
		if p.X > hi {
			hi = p.X
		}
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sign := 0
	// Every other vertex must lie strictly on the same side of every edge.
	for i, a := range profile {
		b := profile[(i+1)%len(profile)]
		dx, dy := new(big.Rat).Sub(rat(b.X), rat(a.X)), new(big.Rat).Sub(rat(b.Y), rat(a.Y))
		for j, p := range profile {
			if j == i || j == (i+1)%len(profile) {
				continue
			}
			cross := new(big.Rat).Sub(new(big.Rat).Mul(dx, new(big.Rat).Sub(rat(p.Y), rat(a.Y))), new(big.Rat).Mul(dy, new(big.Rat).Sub(rat(p.X), rat(a.X))))
			if cross.Sign() == 0 {
				return HelicalPolygon{}, ErrDegenerate
			}
			if sign == 0 {
				sign = cross.Sign()
			}
			if cross.Sign() != sign {
				return HelicalPolygon{}, ErrInvalid
			}
		}
	}
	reference, e := NewHelicalProfileSurface(frame, Vec2{lo, 0}, Vec2{hi, 0}, lead, hand, uMin, uMax)
	if e != nil {
		return HelicalPolygon{}, e
	}
	return HelicalPolygon{reference, append([]Vec2(nil), profile...)}, nil
}

// HelicalPolygonLineOptions bounds turn enumeration and phase precision.
// Defaults match HelicalRayLineOptions; no finite line clipping is applied.
type HelicalPolygonLineOptions struct{ MaxTurns, MaxOperations, MaxCoefficientBits, PhaseBits int }

// HelicalMaterialContributor records an original turn and the profile edges
// meeting the lower/upper axial section endpoints. Vertex contacts retain all
// incident edge indices; a vertical edge can occur in both endpoint lists.
type HelicalMaterialContributor struct {
	Turn                   [2]*big.Rat
	LowerEdges, UpperEdges []int
}

// HelicalMaterialInterval is a closed retained line interval. Endpoints equal
// PhaseScale*phase+Offsets[0/1], with phase in report.Phase. Bounds enclose each
// endpoint separately; they are not an interval of uncertain material states.
// Contributors preserve each original swept meridian before exact union.
type HelicalMaterialInterval struct {
	Offsets      [2]*big.Rat
	Bounds       [2][2]*big.Rat
	Contributors []HelicalMaterialContributor
	Touch        bool
}
type HelicalPolygonLineResult struct {
	Complete                        bool
	Phase                           [2]*big.Rat
	PhaseScale, Radius              *big.Rat
	Intervals                       []HelicalMaterialInterval
	CandidateTurns, ExactOperations int
}

// IntersectAxisLine returns the ordered union of material intervals on
// origin+lambda*axisScale*Frame.ZAxis. The supplied rational unit ray (cs,sn)
// must match the line's positive radial ray. Cardinal phases are exact; other
// phases have certified rational bounds. Because every endpoint shares the
// same phase term, exact rational offsets prove interval order and merging.
// Failure discards partial intervals. Nonconvex profiles are not supported.
func (p HelicalPolygon) IntersectAxisLine(ctx context.Context, cs, sn *big.Rat, origin [3]*big.Rat, axisScale *big.Rat, o HelicalPolygonLineOptions) (out HelicalPolygonLineResult, err error) {
	if ctx == nil || len(p.profile) < 3 || cs == nil || sn == nil || axisScale == nil || axisScale.Sign() == 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxTurns < 0 || (o.PhaseBits != 0 && (o.PhaseBits < 16 || o.PhaseBits > 512)) {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Intervals = nil
		}
	}()

	for _, q := range append([]*big.Rat{cs, sn, axisScale}, origin[:]...) {
		if q == nil {
			return out, ErrInvalid
		}
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return out, ErrConvergence
	}
	options := HelicalAxisLineOptions{MaxContacts: o.MaxTurns, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits}
	var turns, lines [][2]*big.Rat
	var profileV *big.Rat
	if cs.Sign() == 0 || sn.Sign() == 0 {
		unit := new(big.Rat).Add(new(big.Rat).Mul(cs, cs), new(big.Rat).Mul(sn, sn))
		if unit.Cmp(big.NewRat(1, 1)) != 0 {
			return out, ErrInvalid
		}
		quarter := int64(0)
		if cs.Sign() < 0 {
			quarter = 2
		} else if sn.Sign() > 0 {
			quarter = 1
		} else if sn.Sign() < 0 {
			quarter = 3
		}
		quarter *= int64(p.reference.handedness)
		phase := new(big.Rat).SetFrac(new(big.Int).Mod(big.NewInt(quarter), big.NewInt(4)), big.NewInt(4))
		r, e := p.reference.IntersectCardinalAxisLine(ctx, phase, origin, axisScale, options)
		b.operations += r.ExactOperations
		if e != nil {
			return out, e
		}
		out.Phase = [2]*big.Rat{new(big.Rat).Set(phase), new(big.Rat).Set(phase)}
		out.CandidateTurns = r.CandidateTurns
		for _, h := range r.Hits {
			turns = append(turns, [2]*big.Rat{new(big.Rat).Set(h.Turn), new(big.Rat).Set(h.Turn)})
			lines = append(lines, [2]*big.Rat{new(big.Rat).Set(h.LineParameter), new(big.Rat).Set(h.LineParameter)})
			profileV = h.ProfileParameter
		}
	} else {
		r, e := p.reference.IntersectRationalRayAxisLine(ctx, cs, sn, origin, axisScale, HelicalRayLineOptions{HelicalAxisLineOptions: options, PhaseBits: o.PhaseBits})
		b.operations += r.ExactOperations
		if e != nil {
			return out, e
		}
		out.Phase = r.Phase
		out.CandidateTurns = r.CandidateTurns
		for _, h := range r.Hits {
			turns = append(turns, h.Turn)
			lines = append(lines, h.LineParameter)
			profileV = h.ProfileParameter
		}
	}
	if len(turns) == 0 {
		out.Complete = true
		return out, nil
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	radius := new(big.Rat).Add(rat(p.reference.start.X), new(big.Rat).Mul(profileV, new(big.Rat).Sub(rat(p.reference.end.X), rat(p.reference.start.X))))
	out.Radius = radius
	out.PhaseScale = new(big.Rat).Quo(rat(p.reference.lead), axisScale)
	var low, high *big.Rat
	var lowEdges, highEdges []int
	add := func(z *big.Rat, edge int) {
		if low == nil || z.Cmp(low) < 0 {
			low = new(big.Rat).Set(z)
			lowEdges = []int{edge}
		} else if z.Cmp(low) == 0 {
			lowEdges = append(lowEdges, edge)
		}
		if high == nil || z.Cmp(high) > 0 {
			high = new(big.Rat).Set(z)
			highEdges = []int{edge}
		} else if z.Cmp(high) == 0 {
			highEdges = append(highEdges, edge)
		}
	}
	for i, a := range p.profile {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		c := p.profile[(i+1)%len(p.profile)]
		x0, x1 := rat(a.X), rat(c.X)
		if x0.Cmp(x1) == 0 {
			if radius.Cmp(x0) == 0 {
				add(rat(a.Y), i)
				add(rat(c.Y), i)
			}
			continue
		}
		v := new(big.Rat).Quo(new(big.Rat).Sub(radius, x0), new(big.Rat).Sub(x1, x0))
		if e := b.check(v); e != nil {
			return out, e
		}
		if v.Sign() < 0 || v.Cmp(big.NewRat(1, 1)) > 0 {
			continue
		}
		z := new(big.Rat).Add(rat(a.Y), new(big.Rat).Mul(v, new(big.Rat).Sub(rat(c.Y), rat(a.Y))))
		if e := b.check(z); e != nil {
			return out, e
		}
		add(z, i)
	}
	if low == nil {
		return out, ErrConvergence
	}
	var intervals []HelicalMaterialInterval
	for i, u := range turns {
		phaseEnd := 0
		if out.PhaseScale.Sign() < 0 {
			phaseEnd = 1
		}
		base := new(big.Rat).Sub(lines[i][0], new(big.Rat).Mul(out.PhaseScale, out.Phase[phaseEnd]))
		offsets := [2]*big.Rat{new(big.Rat).Add(base, new(big.Rat).Quo(low, axisScale)), new(big.Rat).Add(base, new(big.Rat).Quo(high, axisScale))}
		if offsets[0].Cmp(offsets[1]) > 0 {
			offsets[0], offsets[1] = offsets[1], offsets[0]
		}
		for _, q := range offsets {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		intervals = append(intervals, HelicalMaterialInterval{Offsets: offsets, Contributors: []HelicalMaterialContributor{{Turn: u, LowerEdges: append([]int(nil), lowEdges...), UpperEdges: append([]int(nil), highEdges...)}}})
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].Offsets[0].Cmp(intervals[j].Offsets[0]) < 0 })
	for _, interval := range intervals {
		n := len(out.Intervals)
		if n == 0 || interval.Offsets[0].Cmp(out.Intervals[n-1].Offsets[1]) > 0 {
			out.Intervals = append(out.Intervals, interval)
		} else {
			last := &out.Intervals[n-1]
			if interval.Offsets[1].Cmp(last.Offsets[1]) > 0 {
				last.Offsets[1] = interval.Offsets[1]
			}
			last.Contributors = append(last.Contributors, interval.Contributors...)
		}
	}
	for i := range out.Intervals {
		interval := &out.Intervals[i]
		interval.Touch = interval.Offsets[0].Cmp(interval.Offsets[1]) == 0
		for endpoint := 0; endpoint < 2; endpoint++ {
			for bound := 0; bound < 2; bound++ {
				interval.Bounds[endpoint][bound] = new(big.Rat).Add(interval.Offsets[endpoint], new(big.Rat).Mul(out.PhaseScale, out.Phase[bound]))
				if e := b.check(interval.Bounds[endpoint][bound]); e != nil {
					return out, e
				}
			}
			if interval.Bounds[endpoint][0].Cmp(interval.Bounds[endpoint][1]) > 0 {
				interval.Bounds[endpoint][0], interval.Bounds[endpoint][1] = interval.Bounds[endpoint][1], interval.Bounds[endpoint][0]
			}
		}
	}
	out.Complete = true
	return out, nil
}
