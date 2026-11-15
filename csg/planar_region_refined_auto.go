package csg

import (
	"context"

	"plainsolid/brep"
	"plainsolid/collision"
)

// JoinRefinedCoincidentPlanarRegions finds original face candidates even
// when their faces or seam edges are divided differently. It conforms both
// source copies to the exact existing stations and proves their full regions
// before joining. Each cross sweep is bounded by Contact.Cross.
func JoinRefinedCoincidentPlanarRegions(ctx context.Context, a, b brep.Solid, o PlanarRegionRefinementOptions) (brep.Solid, PlanarRegionRefinedJoinHistory, error) {
	var zero PlanarRegionRefinedJoinHistory
	if o.MaxStationTests < 0 || o.MaxStations < 0 {
		return brep.Solid{}, zero, collision.ErrInvalid
	}
	sections, err := collision.FindPlanarRegionCandidates(ctx, a, b, o.Contact)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	return JoinRefinedPlanarRegions(ctx, a, b, sections.Faces, o)
}
