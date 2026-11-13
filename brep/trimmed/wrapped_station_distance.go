package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// BoundWrappedCircularStationLoftDistance encloses the squared distance to
// the complete certified loft boundary, retaining the original witness face.
func (s Solid) BoundWrappedCircularStationLoftDistance(ctx context.Context, point geom.Vec3,
	o geom.FrustumDistanceOptions) (geom.CircularMeridianDistanceBounds, error) {
	p, e := s.ValidateWrappedCircularStationLoft(ctx, o.Surface)
	if e != nil {
		return geom.CircularMeridianDistanceBounds{}, e
	}
	seg := wrappedStationWallSegments(nil, p, p.History.WallFaces)
	seg = appendWrappedStationCapSegments(seg, p, [2]*big.Rat{new(big.Rat), new(big.Rat)}, p.History.CapFaces)
	return geom.BoundCircularMeridianSegmentDistance(ctx, p.Center, seg, point, o)
}

// BoundWrappedCircularStationBoreDistance encloses squared distance to
// the outer and inner bands and the two annular caps. It never projects
// onto the fictitious disk across the central opening.
func (s Solid) BoundWrappedCircularStationBoreDistance(ctx context.Context, point geom.Vec3,
	o geom.FrustumDistanceOptions) (geom.CircularMeridianDistanceBounds, error) {
	p, e := s.ValidateWrappedCircularStationBore(ctx, o.Surface)
	if e != nil {
		return geom.CircularMeridianDistanceBounds{}, e
	}
	seg := wrappedStationWallSegments(nil, p.Outer, p.OuterWalls)
	seg = wrappedStationWallSegments(seg, p.Inner, p.InnerWalls)
	seg = appendWrappedStationCapSegments(seg, p.Outer, [2]*big.Rat{p.Inner.Radii[0], p.Inner.Radii[len(p.Inner.Radii)-1]}, p.Caps)
	return geom.BoundCircularMeridianSegmentDistance(ctx, p.Outer.Center, seg, point, o)
}

func wrappedStationWallSegments(seg []geom.CircularMeridianSegment, p WrappedStationLoftProof, ids []int) []geom.CircularMeridianSegment {
	for i, id := range ids {
		seg = append(seg, geom.CircularMeridianSegment{
			Start: [2]*big.Rat{p.Radii[i], p.Heights[i]}, End: [2]*big.Rat{p.Radii[i+1], p.Heights[i+1]},
			ID: id, Part: geom.CylinderLateral,
		})
	}
	return seg
}

func appendWrappedStationCapSegments(seg []geom.CircularMeridianSegment, p WrappedStationLoftProof,
	inner [2]*big.Rat, ids [2]int) []geom.CircularMeridianSegment {
	for i, j := range [2]int{0, len(p.Heights) - 1} {
		seg = append(seg, geom.CircularMeridianSegment{
			Start: [2]*big.Rat{inner[i], p.Heights[j]}, End: [2]*big.Rat{p.Radii[j], p.Heights[j]},
			ID: ids[i], Part: [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap}[i],
		})
	}
	return seg
}
