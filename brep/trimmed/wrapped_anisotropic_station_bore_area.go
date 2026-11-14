package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// WrappedAnisotropicStationBoreAreaBounds encloses the complete exposed
// boundary area, with actual native owner IDs for both wall families and the
// two annular caps. Inward walls contribute positive area. No interior station
// disk or removed opening contributes. Caps are profile area times an exact
// positive difference of outer/inner diagonal scale products.
type WrappedAnisotropicStationBoreAreaBounds struct {
	Source                                  WrappedAnisotropicStationBoreProof
	Total, ProfileArea                      WrappedStationMassInterval
	Faces                                   []WrappedAnisotropicFaceArea
	Cells, ProfileSegments, ExactOperations int
}

// BoundWrappedAnisotropicStationBoreArea integrates each original injective
// wall with the existing rational Bernstein cross-product enclosure, preserving
// independent station cuts and aspect ratios. All wall/cap integration, outward
// square-root rounding and retries share one options ledger. MaxCells bounds
// final cells across both families; MaxOperations includes all retry work.
// Original source authentication retains its separately bounded passes.
func (s Solid) BoundWrappedAnisotropicStationBoreArea(ctx context.Context, o WrappedAnisotropicStationAreaOptions) (WrappedAnisotropicStationBoreAreaBounds, error) {
	var empty WrappedAnisotropicStationBoreAreaBounds
	o, e := normalizeAnisotropicStationAreaOptions(ctx, o)
	if e != nil {
		return empty, e
	}
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxCoefficientBits}
	tolerance := new(big.Rat).Set(b.check(o.AreaTolerance))
	if b.err != nil {
		return empty, b.err
	}
	source, _, e := s.validateAnisotropicStationBore(ctx, o.Source)
	if e != nil {
		return empty, e
	}
	band := s.faces[source.OuterWalls[0]].Surface.(*geom.WrappedSurfacePatch)
	segments, _, e := stationProfileSegments(band, b)
	if e != nil {
		return empty, e
	}
	proofs := [2]WrappedAnisotropicStationProof{source.Outer, source.Inner}
	ids := [2][]int{source.OuterWalls, source.InnerWalls}
	var factors [2]*big.Rat
	for cap := 0; cap < 2; cap++ {
		var products [2]*big.Rat
		for side, p := range proofs {
			station := 0
			if cap == 1 {
				station = len(p.Scales) - 1
			}
			products[side] = b.mul(p.Scales[station][0], p.Scales[station][1])
		}
		factors[cap] = b.sub(products[0], products[1])
		if b.err != nil {
			return empty, b.err
		}
		if factors[cap].Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
	}
	axial := 1
	for depth := 0; ; depth++ {
		cells := len(segments) * axial * (len(ids[0]) + len(ids[1]))
		if cells > o.MaxCells {
			return empty, geom.ErrConvergence
		}
		out := WrappedAnisotropicStationBoreAreaBounds{Source: source, Total: WrappedStationMassInterval{new(big.Rat), new(big.Rat)}, Cells: cells, ProfileSegments: len(segments)}
		for _, faces := range ids {
			for _, id := range faces {
				out.Faces = append(out.Faces, WrappedAnisotropicFaceArea{id, WrappedStationMassInterval{new(big.Rat), new(big.Rat)}})
			}
		}
		for _, segment := range segments {
			profile := anisotropicAreaProfileFrom(segment, b)
			if b.err != nil {
				return empty, b.err
			}
			offset := 0
			for side, p := range proofs {
				for band := range ids[side] {
					for j := 0; j < axial; j++ {
						var xy [2][2]*big.Rat
						for axis := 0; axis < 2; axis++ {
							delta := b.sub(p.Scales[band+1][axis], p.Scales[band][axis])
							for end := 0; end < 2; end++ {
								xy[axis][end] = b.add(p.Scales[band][axis], b.mul(delta, big.NewRat(int64(j+end), int64(axial))))
							}
						}
						h := b.div(b.sub(p.Heights[band+1], p.Heights[band]), big.NewRat(int64(axial), 1))
						area := profile.enclose(xy[0], xy[1], h, b)
						out.Faces[offset+band].Area = b.plus(out.Faces[offset+band].Area, area)
						if b.err != nil {
							return empty, b.err
						}
					}
				}
				offset += len(ids[side])
			}
		}
		area, _, e := stationProfileEnclosure(segments, false, b)
		if e != nil {
			return empty, e
		}
		out.ProfileArea = area
		for cap, id := range source.Caps {
			out.Faces = append(out.Faces, WrappedAnisotropicFaceArea{id, b.scaled(area, factors[cap])})
		}
		for _, face := range out.Faces {
			out.Total = b.plus(out.Total, face.Area)
		}
		width := b.sub(out.Total.Upper, out.Total.Lower)
		if b.err != nil {
			return empty, b.err
		}
		if width.Cmp(tolerance) <= 0 {
			if e = ctx.Err(); e != nil {
				return empty, e
			}
			out.ExactOperations = b.used
			return out, nil
		}
		if depth >= o.MaxDepth || len(segments) > o.MaxProfileSegments/2 || cells > o.MaxCells/4 {
			return empty, geom.ErrConvergence
		}
		segments = splitStationProfileSegments(segments, b)
		if b.err != nil {
			return empty, b.err
		}
		axial *= 2
	}
}
