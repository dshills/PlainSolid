//go:build research

package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedRationalStationDistanceOptions separates source authentication from
// all distance searches. Search.MaxCells and MaxOperations are cumulative;
// queue/depth limits apply to each serial search. At most16 original walls and
// two original cap rims are searched. Source also bounds each of at most two
// exact rim extractions and three point-query source reproofs. MaxPointOperations
// is shared by those three point queries, default24576, maximum1M.
type WrappedRationalStationDistanceOptions struct {
	Source             geom.SurfacePatchOptions
	Search             geom.SurfaceDistanceOptions
	MaxPointOperations int
}

// WrappedRationalStationFaceDistance encloses squared distance to one complete
// original retained face. Witness attains Upper but need not be its minimizer.
type WrappedRationalStationFaceDistance struct {
	Face         int
	Lower, Upper *big.Rat
	Witness      [3]*big.Rat
}

// WrappedRationalStationDistance encloses global boundary squared distance.
// Faces are all original owners of Witness, not a claim of all nearest faces.
// The upper witness belongs to the native boundary, including crease/rim ties.
// No artificial station disks participate. Returned rational data are owned.
type WrappedRationalStationDistance struct {
	Lower, Upper                  *big.Rat
	Witness                       [3]*big.Rat
	Faces                         []int
	PerFace                       []WrappedRationalStationFaceDistance
	VisitedCells, ExactOperations int
}

// BoundWrappedRationalStationBoundaryDistanceRational searches original walls
// and exposed terminal caps of a source-authenticated weighted-profile loft.
// The global interval meets Search.SquaredTolerance; a wall excluded by its
// exact axial gap may retain a wider per-face interval. Exhausted work,
// unsupported source or cancellation returns no partial result.
func (s Solid) BoundWrappedRationalStationBoundaryDistanceRational(ctx context.Context, p [3]*big.Rat, o WrappedRationalStationDistanceOptions) (WrappedRationalStationDistance, error) {
	empty := WrappedRationalStationDistance{}
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
	if ctx == nil || o.MaxPointOperations < 3 || o.MaxPointOperations > 1000000 || o.Search.MaxCells < 1 || o.Search.MaxCells > 1000000 || o.Search.MaxOperations < 1 || o.Search.MaxOperations > 64000000 || o.Search.MaxCoefficientBits < 64 || o.Search.MaxCoefficientBits > 65536 || !finite(o.Search.SquaredTolerance) || o.Search.SquaredTolerance <= 0 || o.Search.MaxDepth < 1 || o.Search.MaxDepth > 128 || o.Search.MaxQueuedPoles < 4 || o.Search.MaxQueuedPoles > 4194304 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	valid := func(q *big.Rat) bool {
		return q != nil && q.Num().BitLen() <= o.Search.MaxCoefficientBits && q.Denom().BitLen() <= o.Search.MaxCoefficientBits
	}
	for _, v := range p {
		if !valid(v) {
			return empty, geom.ErrInvalid
		}
	}
	proof, bands, e := s.validateWrappedProfileStationLoft(ctx, o.Source, false)
	if e != nil {
		return empty, e
	}
	result := WrappedRationalStationDistance{}
	charge := func(values ...*big.Rat) error {
		result.ExactOperations += len(values)
		if result.ExactOperations > o.Search.MaxOperations {
			return geom.ErrConvergence
		}
		for _, v := range values {
			if !valid(v) {
				return geom.ErrConvergence
			}
		}
		return nil
	}
	clonePoint := func(p [3]*big.Rat) (q [3]*big.Rat) {
		for i := range q {
			q[i] = new(big.Rat).Set(p[i])
		}
		return
	}
	add := func(face int, lo, hi *big.Rat, w [3]*big.Rat) error {
		if !valid(lo) || !valid(hi) || lo.Sign() < 0 || lo.Cmp(hi) > 0 {
			return geom.ErrConvergence
		}
		for _, v := range w {
			if !valid(v) {
				return geom.ErrConvergence
			}
		}
		result.PerFace = append(result.PerFace, WrappedRationalStationFaceDistance{Face: face, Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi), Witness: clonePoint(w)})
		if result.Lower == nil || lo.Cmp(result.Lower) < 0 {
			result.Lower = new(big.Rat).Set(lo)
		}
		if result.Upper == nil || hi.Cmp(result.Upper) < 0 {
			result.Upper = new(big.Rat).Set(hi)
			result.Witness = clonePoint(w)
		}
		return nil
	}
	remaining := func() (geom.SurfaceDistanceOptions, error) {
		q := o.Search
		q.MaxCells -= result.VisitedCells
		q.MaxOperations -= result.ExactOperations
		if q.MaxCells < 1 || q.MaxOperations < 1 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	pointOptions := WrappedRationalStationPointOptions{Source: o.Source, MaxPointOperations: o.MaxPointOperations / 3, MaxCoefficientBits: o.Search.MaxCoefficientBits}
	// A cap projection is admitted only by original native face ownership.
	for capIndex, bandIndex := range []int{0, len(bands) - 1} {
		face := proof.History.CapFaces[capIndex]
		z := proof.Heights[0]
		if capIndex == 1 {
			z = proof.Heights[len(proof.Heights)-1]
		}
		w := clonePoint(p)
		w[2] = new(big.Rat).Set(z)
		loc, e := s.LocateWrappedRationalStationLoftRational(ctx, w, pointOptions)
		if e != nil {
			return empty, e
		}
		if loc.Location == parametric.Boundary && slices.Contains(loc.Faces, face) {
			d := new(big.Rat).Sub(p[2], z)
			if !valid(d) {
				return empty, geom.ErrConvergence
			}
			d.Mul(d, d)
			if e = charge(d, d); e != nil {
				return empty, e
			}
			if e = add(face, d, d, w); e != nil {
				return empty, e
			}
			continue
		}
		patch := bands[bandIndex].FundamentalPatch()
		domain := patch.Domain()
		v := domain.VMin
		if capIndex == 1 {
			v = domain.VMax
		}
		rim, e := patch.IsoCurve(1, v, o.Source)
		if e != nil {
			return empty, e
		}
		limit, e := remaining()
		if e != nil {
			return empty, e
		}
		bound, e := geom.BoundPointExactCurveDistanceRational(ctx, p, rim, geom.CurveDistanceOptions(limit))
		if e != nil {
			return empty, e
		}
		if !bound.Complete || !bound.HasWitness {
			return empty, geom.ErrConvergence
		}
		result.VisitedCells += bound.VisitedCells
		result.ExactOperations += bound.ExactOperations
		if e = add(face, bound.Squared.Lower, bound.Squared.Upper, bound.Witness); e != nil {
			return empty, e
		}
	}
	for i, band := range bands {
		// Every point on this original band has height in the proved station
		// interval. If that exact axial gap already excludes it, avoid a costly
		// unnecessary support minimization. Its cardinal corner is an attained
		// original wall witness, not a point on an artificial disk.
		gap := new(big.Rat)
		if p[2].Cmp(proof.Heights[i]) < 0 {
			gap.Sub(proof.Heights[i], p[2])
		}
		if p[2].Cmp(proof.Heights[i+1]) > 0 {
			gap.Sub(p[2], proof.Heights[i+1])
		}
		lower := new(big.Rat).Mul(gap, gap)
		if e := charge(gap, lower); e != nil {
			return empty, e
		}
		if lower.Cmp(result.Upper) >= 0 {
			witness := [3]*big.Rat{new(big.Rat).Add(proof.Center[0], proof.Radii[i]), new(big.Rat).Set(proof.Center[1]), new(big.Rat).Set(proof.Heights[i])}
			if e := charge(witness[0]); e != nil {
				return empty, e
			}
			upper := new(big.Rat)
			for k := range p {
				d := new(big.Rat).Sub(p[k], witness[k])
				sq := new(big.Rat).Mul(d, d)
				upper.Add(upper, sq)
				if e := charge(d, sq, upper); e != nil {
					return empty, e
				}
			}
			if e := add(proof.History.WallFaces[i], lower, upper, witness); e != nil {
				return empty, e
			}
			continue
		}
		limit, e := remaining()
		if e != nil {
			return empty, e
		}
		bound, e := geom.BoundPointSurfaceDistanceRational(ctx, p, band.FundamentalPatch(), limit)
		if e != nil {
			return empty, e
		}
		if !bound.Complete || !bound.HasUpperPoint {
			return empty, geom.ErrConvergence
		}
		result.VisitedCells += bound.VisitedCells
		result.ExactOperations += bound.ExactOperations
		if e = add(proof.History.WallFaces[i], bound.Squared.Lower, bound.Squared.Upper, bound.Witness); e != nil {
			return empty, e
		}
	}
	if new(big.Rat).Sub(result.Upper, result.Lower).Cmp(new(big.Rat).SetFloat64(o.Search.SquaredTolerance)) > 0 {
		return empty, geom.ErrConvergence
	}
	loc, e := s.LocateWrappedRationalStationLoftRational(ctx, result.Witness, pointOptions)
	if e != nil {
		return empty, e
	}
	if loc.Location != parametric.Boundary || len(loc.Faces) == 0 {
		return empty, geom.ErrConvergence
	}
	result.Faces = append([]int(nil), loc.Faces...)
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
