//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ClipWrappedRationalStationBoreLineRational returns isolated original
// wall/rim/cap events and open material spans of a weighted through-bore.
// Coincident wall intervals and lines lying in an annular cap plane are
// outside this endpoint-isolation API. The query budget is shared across
// both original profiles and all retained parameter intervals.
func (s Solid) ClipWrappedRationalStationBoreLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat,
	o WrappedRationalStationLineOptions) (WrappedRationalStationLine, error) {
	var empty WrappedRationalStationLine
	o, bits, err := prepareRationalStationLine(ctx, origin, direction, minT, maxT, o)
	if err != nil {
		return empty, err
	}
	proof, err := s.ValidateWrappedRationalStationBore(ctx, o.Geometry.Surface)
	if err != nil {
		return empty, err
	}
	profiles, bands := s.wrappedRationalStationBoreProfiles(proof)
	heights := profiles[0].Heights
	endpoints := [2]*big.Rat{heights[0], heights[len(heights)-1]}
	if direction[2].Sign() == 0 && (origin[2].Cmp(endpoints[0]) == 0 || origin[2].Cmp(endpoints[1]) == 0) {
		return empty, geom.ErrUnrepresentable
	}
	var hits []WrappedLoftLineHit
	for i, p := range profiles {
		sides, err := rationalStationLineEvents(ctx, p, bands[i], origin, direction, minT, maxT, o, bits, false)
		if err != nil {
			return empty, err
		}
		if len(hits)+len(sides) > o.MaxEvents {
			return empty, geom.ErrConvergence
		}
		hits = append(hits, sides...)
	}
	locate := func(point [3]*big.Rat, po WrappedRationalStationPointOptions) (WrappedRationalStationPoint, error) {
		po.MaxPointOperations /= 2
		if po.MaxPointOperations < 1 {
			return WrappedRationalStationPoint{}, geom.ErrConvergence
		}
		return locateWrappedRationalStationBorePoint(ctx, profiles, bands, point, po)
	}
	// Reserve the exact point quota for the two possible terminal cap
	// checks and the greatest number of intervals after their insertion.
	capAllowance := o.MaxPointOperations / (len(hits) + 5)
	for i, z := range endpoints {
		if direction[2].Sign() == 0 {
			break
		}
		delta := new(big.Rat).Sub(z, origin[2])
		if !validRationalStationLineScalar(delta, bits) {
			return empty, geom.ErrConvergence
		}
		t := delta.Quo(delta, direction[2])
		if !validRationalStationLineScalar(t, bits) {
			return empty, geom.ErrConvergence
		}
		if t.Cmp(minT) < 0 || t.Cmp(maxT) > 0 {
			continue
		}
		point, err := pointOnRationalStationLine(origin, direction, t, bits)
		if err != nil {
			return empty, err
		}
		loc, err := locate(point, WrappedRationalStationPointOptions{
			Source: o.Geometry.Surface, MaxPointOperations: capAllowance, MaxCoefficientBits: bits,
		})
		if err != nil {
			return empty, err
		}
		o.MaxPointOperations -= capAllowance
		if loc.Location == parametric.Boundary {
			if len(hits) >= o.MaxEvents {
				return empty, geom.ErrConvergence
			}
			parts := [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap}[i]
			if len(loc.Faces) > 1 {
				parts |= geom.CylinderLateral
			}
			hits = append(hits, WrappedLoftLineHit{
				Parameter: geom.ToroidLineRoot{Min: t, Max: new(big.Rat).Set(t), Multiplicity: 1},
				Faces:     loc.Faces, Parts: parts,
			})
		}
	}
	return clipRationalStationLine(ctx, origin, direction, minT, maxT, o, bits, hits, locate)
}

// ClipWrappedRationalStationBoreCapLineRational reports coincident spans on
// one original terminal annulus. The uncovered aperture is excluded, and
// each inner or outer rim event owns its exact original wall and cap.
func (s Solid) ClipWrappedRationalStationBoreCapLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat,
	o WrappedRationalStationLineOptions) (WrappedRationalStationCapLine, error) {
	var empty WrappedRationalStationCapLine
	o, bits, err := prepareRationalStationLine(ctx, origin, direction, minT, maxT, o)
	if err != nil {
		return empty, err
	}
	if direction[2].Sign() != 0 {
		return empty, geom.ErrInvalid
	}
	proof, err := s.ValidateWrappedRationalStationBore(ctx, o.Geometry.Surface)
	if err != nil {
		return empty, err
	}
	profiles, bands := s.wrappedRationalStationBoreProfiles(proof)
	heights := profiles[0].Heights
	cap := 0
	if origin[2].Cmp(heights[0]) != 0 {
		if origin[2].Cmp(heights[len(heights)-1]) != 0 {
			return empty, geom.ErrDomain
		}
		cap = 1
	}
	capFace := proof.Caps[cap]
	capPart := [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap}[cap]
	var hits []WrappedLoftLineHit
	for i, p := range profiles {
		rims, err := rationalStationLineEvents(ctx, p, bands[i], origin, direction, minT, maxT, o, bits, false)
		if err != nil {
			return empty, err
		}
		if len(hits)+len(rims) > o.MaxEvents {
			return empty, geom.ErrConvergence
		}
		for _, rim := range rims {
			rim.Parts |= capPart
			rim.Faces = append(rim.Faces, capFace)
			hits = append(hits, rim)
		}
	}
	// The common event ledger can order the annular cap spans by using a
	// material predicate in the cap plane. The output records cap coincidence.
	clipped, err := clipRationalStationLine(ctx, origin, direction, minT, maxT, o, bits, hits,
		func(point [3]*big.Rat, po WrappedRationalStationPointOptions) (WrappedRationalStationPoint, error) {
			po.MaxPointOperations /= 2
			if po.MaxPointOperations < 1 {
				return WrappedRationalStationPoint{}, geom.ErrConvergence
			}
			loc, err := locateWrappedRationalStationBorePoint(ctx, profiles, bands, point, po)
			if loc.Location == parametric.Boundary && len(loc.Faces) == 1 && loc.Faces[0] == capFace {
				loc.Location = parametric.Inside
			}
			return loc, err
		})
	if err != nil {
		return empty, err
	}
	if len(clipped.Hits)+len(clipped.Interior) > o.MaxEvents {
		return empty, geom.ErrConvergence
	}
	endpointOwners := func(q geom.ToroidLineRoot) []int {
		for _, hit := range clipped.Hits {
			if hit.Parameter.Min.Cmp(q.Min) == 0 && hit.Parameter.Max.Cmp(q.Max) == 0 {
				return append([]int(nil), hit.Faces...)
			}
		}
		return []int{capFace}
	}
	out := WrappedRationalStationCapLine{Hits: clipped.Hits}
	for _, span := range clipped.Interior {
		out.Boundary = append(out.Boundary, WrappedRationalBoundarySpan{
			Start: span.Start, End: span.End, Faces: []int{capFace},
			StartFaces: endpointOwners(span.Start), EndFaces: endpointOwners(span.End),
		})
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	return out, nil
}
