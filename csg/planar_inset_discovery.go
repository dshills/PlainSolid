package csg

import (
	"context"
	"fmt"

	"plainsolid/brep"
	"plainsolid/collision"
)

// JoinCoincidentPlanarInsets discovers one original host face and all disjoint
// complete tool footprints before proving and constructing the inset join.
func JoinCoincidentPlanarInsets(ctx context.Context, host, tool brep.Solid, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarInsetSetJoinHistory, error) {
	var zero PlanarInsetSetJoinHistory
	regions, err := collision.FindPlanarRegionCandidates(ctx, host, tool, o)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	if len(regions.Faces[0]) != 1 {
		return brep.Solid{}, zero, fmt.Errorf("%w: inset tooling must meet one original host face", collision.ErrInvalid)
	}
	return JoinPlanarInsets(ctx, host, tool, regions.Faces[0][0], regions.Faces[1], o)
}
