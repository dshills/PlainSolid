package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// WrappedAnisotropicStationAreaOptions bounds original support integration.
// Source has separate native authentication quotas. All original wall and cap arithmetic
// shares MaxOperations(default8M,max64M); MaxCells(default8192,max65536) is shared
// across final wall cells. MaxDepth(default24,max32) bounds joint refinement; profile
// segments(default1024,max16384) bound cap refinement. Zero tolerance is invalid;
// nil selects1/100. No partial result is returned on limits or cancellation.
type WrappedAnisotropicStationAreaOptions struct {
	Source                                                                    geom.SurfacePatchOptions
	AreaTolerance                                                             *big.Rat
	MaxOperations, MaxCells, MaxDepth, MaxProfileSegments, MaxCoefficientBits int
}
type WrappedAnisotropicFaceArea struct {
	Face int
	Area WrappedStationMassInterval
}
type WrappedAnisotropicStationAreaBounds struct {
	Source                                  WrappedAnisotropicStationProof
	Total, ProfileArea                      WrappedStationMassInterval
	Faces                                   []WrappedAnisotropicFaceArea
	Cells, ProfileSegments, ExactOperations int
}

// BoundWrappedAnisotropicStationArea integrates each original injective wall
// using exact rational Bernstein cross-product enclosures. Both terminal
// caps use the exact normalized profile enclosure times their independent sx*sy.
// Internal rims/caps contribute no boundary area. Square roots and every cell
// contribution are rounded outward to a 96-bit dyadic grid before summation.
func (s Solid) BoundWrappedAnisotropicStationArea(ctx context.Context, o WrappedAnisotropicStationAreaOptions) (WrappedAnisotropicStationAreaBounds, error) {
	empty := WrappedAnisotropicStationAreaBounds{}
	if o.AreaTolerance == nil {
		o.AreaTolerance = big.NewRat(1, 100)
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCells == 0 {
		o.MaxCells = 8192
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxProfileSegments == 0 {
		o.MaxProfileSegments = 1024
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if ctx == nil || o.AreaTolerance.Sign() <= 0 || o.MaxOperations < 1 || o.MaxOperations > 64000000 || o.MaxCells < 1 || o.MaxCells > 65536 || o.MaxDepth < 1 || o.MaxDepth > 32 || o.MaxProfileSegments < 4 || o.MaxProfileSegments > 16384 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return empty, geom.ErrInvalid
	}
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxCoefficientBits}
	tolerance := b.check(o.AreaTolerance)
	if b.err != nil {
		return empty, b.err
	}
	tolerance = new(big.Rat).Set(tolerance)
	proof, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, o.Source)
	if e != nil {
		return empty, e
	}
	segments, _, e := stationProfileSegments(bands[0], b)
	if e != nil {
		return empty, e
	}
	axial := 1
	for depth := 0; ; depth++ {
		cells := len(segments) * axial * len(bands)
		if cells > o.MaxCells {
			return empty, geom.ErrConvergence
		}
		out := WrappedAnisotropicStationAreaBounds{Source: proof, Total: WrappedStationMassInterval{new(big.Rat), new(big.Rat)}, Cells: cells, ProfileSegments: len(segments)}
		for _, face := range proof.History.WallFaces {
			out.Faces = append(out.Faces, WrappedAnisotropicFaceArea{face, WrappedStationMassInterval{new(big.Rat), new(big.Rat)}})
		}
		for _, segment := range segments {
			profile := anisotropicAreaProfileFrom(segment, b)
			if b.err != nil {
				return empty, b.err
			}
			for band := range bands {
				for j := 0; j < axial; j++ {
					var xy [2][2]*big.Rat
					for axis := 0; axis < 2; axis++ {
						delta := b.sub(proof.Scales[band+1][axis], proof.Scales[band][axis])
						for end := 0; end < 2; end++ {
							xy[axis][end] = b.add(proof.Scales[band][axis], b.mul(delta, big.NewRat(int64(j+end), int64(axial))))
						}
					}
					h := b.div(b.sub(proof.Heights[band+1], proof.Heights[band]), big.NewRat(int64(axial), 1))
					out.Faces[band].Area = b.plus(out.Faces[band].Area, profile.enclose(xy[0], xy[1], h, b))
					if b.err != nil {
						return empty, b.err
					}
				}
			}
		}
		area, _, e := stationProfileEnclosure(segments, false, b)
		if e != nil {
			return empty, e
		}
		out.ProfileArea = area
		for cap, station := range []int{0, len(proof.Scales) - 1} {
			factor := b.mul(proof.Scales[station][0], proof.Scales[station][1])
			out.Faces = append(out.Faces, WrappedAnisotropicFaceArea{proof.History.CapFaces[cap], b.scaled(area, factor)})
		}
		for _, face := range out.Faces {
			out.Total = b.plus(out.Total, face.Area)
		}
		width := b.sub(out.Total.Upper, out.Total.Lower)
		if b.err != nil {
			return empty, b.err
		}
		if width.Cmp(tolerance) <= 0 {
			out.ExactOperations = b.used
			if e = ctx.Err(); e != nil {
				return empty, e
			}
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
