package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAnisotropicBoreDistanceOptions bounds original surface/rim searches.
// Search cells and operations are shared across every face and result arithmetic.
// MaxPointOperations additionally caps all prepared annular-projection queries
// together (default24576,max1M); their work also counts against Search operations.
// Source reauthentication and at most two exact rim extractions keep their
// separately bounded source passes. Search depth/queue limits apply per search.
type WrappedAnisotropicBoreDistanceOptions struct {
	Source             geom.SurfacePatchOptions
	Search             geom.SurfaceDistanceOptions
	MaxPointOperations int
}

// WrappedAnisotropicBoreDistance encloses global original-boundary squared
// distance. Witness is an attained rational boundary point on WitnessFace;
// it need not be the exact minimizer. This is one authenticated owner, not a
// complete tie/nearest-face report. PerFace may be wider where an exact height
// gap already excludes that wall. All rational output data are owned.
type WrappedAnisotropicBoreDistance struct {
	Lower, Upper                                   *big.Rat
	Witness                                        [3]*big.Rat
	WitnessFace                                    int
	PerFace                                        []WrappedRationalStationFaceDistance
	VisitedCells, ExactOperations, PointOperations int
}

// BoundWrappedAnisotropicStationBoreDistanceRational searches actual retained
// wall patches and exposed annular caps without circular substitution. An
// orthogonal cap projection is accepted only after exact annular classification.
// Otherwise the closest cap point lies on its inner or outer original rim;
// strict nesting selects the appropriate rim without an artificial filled disk.
// The global squared interval meets Search.SquaredTolerance or returns empty.
func (s Solid) BoundWrappedAnisotropicStationBoreDistanceRational(ctx context.Context, p [3]*big.Rat, o WrappedAnisotropicBoreDistanceOptions) (WrappedAnisotropicBoreDistance, error) {
	var empty WrappedAnisotropicBoreDistance
	if o.MaxPointOperations == 0 {
		o.MaxPointOperations = 24576
	}
	if o.Search.MaxCells == 0 {
		o.Search.MaxCells = 8192
	}
	if o.Search.MaxOperations == 0 {
		o.Search.MaxOperations = 4000000
	}
	if o.Search.MaxCoefficientBits == 0 {
		o.Search.MaxCoefficientBits = 16384
	}
	if o.Search.SquaredTolerance == 0 {
		o.Search.SquaredTolerance = 1e-8
	}
	if o.Search.MaxDepth == 0 {
		o.Search.MaxDepth = 32
	}
	if o.Search.MaxQueuedPoles == 0 {
		o.Search.MaxQueuedPoles = 262144
	}
	if ctx == nil || o.MaxPointOperations < 1 || o.MaxPointOperations > 1000000 || o.Search.MaxCells < 1 || o.Search.MaxCells > 1000000 || o.Search.MaxOperations < 1 || o.Search.MaxOperations > 64000000 || o.Search.MaxCoefficientBits < 64 || o.Search.MaxCoefficientBits > 65536 || !finite(o.Search.SquaredTolerance) || o.Search.SquaredTolerance <= 0 || o.Search.MaxDepth < 1 || o.Search.MaxDepth > 128 || o.Search.MaxQueuedPoles < 4 || o.Search.MaxQueuedPoles > 4194304 {
		return empty, geom.ErrInvalid
	}
	b := &stationMassBudget{ctx: ctx, limit: o.Search.MaxOperations, bits: o.Search.MaxCoefficientBits}
	for _, v := range p {
		if v == nil {
			return empty, geom.ErrInvalid
		}
		b.check(v)
	}
	if b.err != nil {
		return empty, b.err
	}
	proof, sources, e := s.validateAnisotropicStationBore(ctx, o.Source)
	if e != nil {
		return empty, e
	}
	proofs := [2]WrappedAnisotropicStationProof{proof.Outer, proof.Inner}
	walls := [2][]int{proof.OuterWalls, proof.InnerWalls}
	var bands [2][]*geom.WrappedSurfacePatch
	for side, ids := range walls {
		for _, id := range ids {
			bands[side] = append(bands[side], s.faces[id].Surface.(*geom.WrappedSurfacePatch))
		}
	}
	out := WrappedAnisotropicBoreDistance{WitnessFace: -1}
	copyPoint := func(q [3]*big.Rat) (c [3]*big.Rat) {
		for i, v := range q {
			c[i] = new(big.Rat).Set(v)
		}
		return
	}
	squared := func(q [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range p {
			d := b.sub(p[i], q[i])
			v = b.add(v, b.mul(d, d))
		}
		return v
	}
	add := func(face int, lo, hi *big.Rat, q [3]*big.Rat) error {
		b.check(lo)
		b.check(hi)
		for _, v := range q {
			b.check(v)
		}
		if b.err != nil {
			return b.err
		}
		if lo.Sign() < 0 || lo.Cmp(hi) > 0 {
			return geom.ErrConvergence
		}
		// Recompute the attained witness distance independently of the search's
		// reported bound; it must equal Upper exactly, never merely lie near it.
		actual := squared(q)
		if b.err != nil {
			return b.err
		}
		if actual.Cmp(hi) != 0 {
			return geom.ErrConvergence
		}
		out.PerFace = append(out.PerFace, WrappedRationalStationFaceDistance{Face: face, Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi), Witness: copyPoint(q)})
		if out.Lower == nil || lo.Cmp(out.Lower) < 0 {
			out.Lower = new(big.Rat).Set(lo)
		}
		if out.Upper == nil || hi.Cmp(out.Upper) < 0 {
			out.Upper = new(big.Rat).Set(hi)
			out.Witness = copyPoint(q)
			out.WitnessFace = face
		}
		return nil
	}
	remaining := func() (geom.SurfaceDistanceOptions, error) {
		q := o.Search
		q.MaxCells -= out.VisitedCells
		q.MaxOperations -= b.used
		if q.MaxCells < 1 || q.MaxOperations < 1 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	for cap, face := range proof.Caps {
		z := proof.Outer.Heights[0]
		if cap == 1 {
			z = proof.Outer.Heights[len(proof.Outer.Heights)-1]
		}
		q := copyPoint(p)
		q[2] = new(big.Rat).Set(z)
		var locations [2]WrappedAnisotropicStationPoint
		for side := range locations {
			allowance := min(o.MaxPointOperations-out.PointOperations, b.limit-b.used)
			if allowance < 1 {
				return empty, geom.ErrConvergence
			}
			v, e := sources[side].locatePreparedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{Source: o.Source, MaxPointOperations: allowance, MaxCoefficientBits: b.bits}, &proofs[side], bands[side])
			if e != nil {
				return empty, e
			}
			locations[side] = v
			b.used += v.ExactOperations
			out.PointOperations += v.ExactOperations
		}
		innerWall := false
		for _, id := range locations[1].Faces {
			for _, wall := range proofs[1].History.WallFaces {
				if id == wall {
					innerWall = true
				}
			}
		}
		if locations[0].Location != parametric.Outside && (locations[1].Location == parametric.Outside || innerWall) {
			d := squared(q)
			if e = add(face, d, d, q); e != nil {
				return empty, e
			}
			continue
		}
		// Outside the outer disk: every route to the nested inner loop crosses
		// the outer rim first. Inside the hole: every route to the outer loop
		// crosses the inner rim first. The perpendicular height term is identical.
		side := 1
		if locations[0].Location == parametric.Outside {
			side = 0
		}
		index := 0
		if cap == 1 {
			index = len(bands[side]) - 1
		}
		patch := bands[side][index].FundamentalPatch()
		domain := patch.Domain()
		v := domain.VMin
		if cap == 1 {
			v = domain.VMax
		}
		curve, e := patch.IsoCurve(1, v, o.Source)
		if e != nil {
			return empty, e
		}
		limit, e := remaining()
		if e != nil {
			return empty, e
		}
		bound, e := geom.BoundPointExactCurveDistanceRational(ctx, p, curve, geom.CurveDistanceOptions(limit))
		if e != nil {
			return empty, e
		}
		if !bound.Complete || !bound.HasWitness {
			return empty, geom.ErrConvergence
		}
		out.VisitedCells += bound.VisitedCells
		b.used += bound.ExactOperations
		if e = add(face, bound.Squared.Lower, bound.Squared.Upper, bound.Witness); e != nil {
			return empty, e
		}
	}
	for side, ids := range walls {
		source := proofs[side]
		for i, face := range ids {
			gap := new(big.Rat)
			if p[2].Cmp(source.Heights[i]) < 0 {
				gap = b.sub(source.Heights[i], p[2])
			}
			if p[2].Cmp(source.Heights[i+1]) > 0 {
				gap = b.sub(p[2], source.Heights[i+1])
			}
			lo := b.mul(gap, gap)
			if b.err != nil {
				return empty, b.err
			}
			if lo.Cmp(out.Upper) >= 0 {
				witness := [3]*big.Rat{b.add(source.Center[0], source.Scales[i][0]), new(big.Rat).Set(source.Center[1]), new(big.Rat).Set(source.Heights[i])}
				if e = add(face, lo, squared(witness), witness); e != nil {
					return empty, e
				}
				continue
			}
			limit, e := remaining()
			if e != nil {
				return empty, e
			}
			bound, e := geom.BoundPointSurfaceDistanceRational(ctx, p, bands[side][i].FundamentalPatch(), limit)
			if e != nil {
				return empty, e
			}
			if !bound.Complete || !bound.HasUpperPoint {
				return empty, geom.ErrConvergence
			}
			out.VisitedCells += bound.VisitedCells
			b.used += bound.ExactOperations
			if e = add(face, bound.Squared.Lower, bound.Squared.Upper, bound.Witness); e != nil {
				return empty, e
			}
		}
	}
	width := b.sub(out.Upper, out.Lower)
	if b.err != nil {
		return empty, b.err
	}
	if width.Cmp(ratInset(o.Search.SquaredTolerance)) > 0 {
		return empty, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	out.ExactOperations = b.used
	return out, nil
}
