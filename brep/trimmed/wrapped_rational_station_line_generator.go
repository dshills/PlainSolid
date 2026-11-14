package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ClipWrappedRationalStationLoftGeneratorRational proves a positive-length
// straight generator on one original wall and clips it to that wall and the
// requested parameter interval. It makes no material claim about the rest of
// the line. Faces owns the span interior; endpoint owners include the original
// neighboring wall or terminal cap, never an artificial internal station disk.
// A secant, horizontal line, empty/singleton overlap or wrong face is rejected.
// Geometry quotas remain per original proof stage (at most16 bands); the two
// endpoint queries share MaxPointOperations, default8192, maximum1M. Their
// source authentication quotas are separate. All failures are atomic.
func (s Solid) ClipWrappedRationalStationLoftGeneratorRational(ctx context.Context, face int, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalBoundarySpan, error) {
	empty := WrappedRationalBoundarySpan{}
	if o.MaxEvents == 0 {
		o.MaxEvents = 128
	}
	if o.MaxPointOperations == 0 {
		o.MaxPointOperations = 8192
	}
	bits := o.Geometry.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if ctx == nil || o.MaxEvents < 1 || o.MaxEvents > 256 || o.MaxPointOperations < 2 || o.MaxPointOperations > 1000000 || bits < 64 || bits > 65536 || !finite(o.Geometry.ParameterTolerance) || o.Geometry.ParameterTolerance <= 0 || o.Geometry.MaxSubdivisions < 0 || o.Geometry.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	valid := func(q *big.Rat) bool { return q != nil && q.Num().BitLen() <= bits && q.Denom().BitLen() <= bits }
	if !valid(minT) || !valid(maxT) || minT.Cmp(maxT) >= 0 {
		return empty, geom.ErrInvalid
	}
	for i := 0; i < 3; i++ {
		if !valid(origin[i]) || !valid(direction[i]) {
			return empty, geom.ErrInvalid
		}
	}
	if direction[2].Sign() == 0 {
		return empty, geom.ErrDomain
	}
	proof, bands, e := s.validateWrappedProfileStationLoft(ctx, o.Geometry.Surface, false)
	if e != nil {
		return empty, e
	}
	band := slices.Index(proof.History.WallFaces, face)
	if band < 0 {
		return empty, geom.ErrDomain
	}
	report, e := geom.IntersectWrappedFrustumLineRational(ctx, bands[band], origin, direction, minT, maxT, o.Geometry)
	if e != nil {
		return empty, e
	}
	if len(report.Overlaps) != 1 {
		return empty, geom.ErrDomain
	}
	if len(report.Hits)+len(report.Overlaps) > o.MaxEvents {
		return empty, geom.ErrConvergence
	}
	span := report.Overlaps[0]
	exact := func(q geom.ToroidLineRoot) bool {
		return valid(q.Min) && valid(q.Max) && q.Min.Cmp(q.Max) == 0 && q.Min.Cmp(minT) >= 0 && q.Max.Cmp(maxT) <= 0
	}
	if span.Part != geom.CylinderLateral || !exact(span.Start) || !exact(span.End) || span.Start.Max.Cmp(span.End.Min) >= 0 {
		return empty, geom.ErrConvergence
	}
	owners := func(t *big.Rat) ([]int, error) {
		var p [3]*big.Rat
		for i := range p {
			product := new(big.Rat).Mul(t, direction[i])
			if !valid(product) {
				return nil, geom.ErrConvergence
			}
			p[i] = new(big.Rat).Add(origin[i], product)
			if !valid(p[i]) {
				return nil, geom.ErrConvergence
			}
		}
		q, e := s.LocateWrappedRationalStationLoftRational(ctx, p, WrappedRationalStationPointOptions{Source: o.Geometry.Surface, MaxPointOperations: o.MaxPointOperations / 2, MaxCoefficientBits: bits})
		if e != nil {
			return nil, e
		}
		if q.Location != parametric.Boundary || !slices.Contains(q.Faces, face) {
			return nil, geom.ErrConvergence
		}
		return append([]int(nil), q.Faces...), nil
	}
	first, e := owners(span.Start.Min)
	if e != nil {
		return empty, e
	}
	last, e := owners(span.End.Min)
	if e != nil {
		return empty, e
	}
	clone := func(q geom.ToroidLineRoot) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(q.Min), Max: new(big.Rat).Set(q.Max), Multiplicity: q.Multiplicity}
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return WrappedRationalBoundarySpan{Start: clone(span.Start), End: clone(span.End), Faces: []int{face}, StartFaces: first, EndFaces: last}, nil
}
