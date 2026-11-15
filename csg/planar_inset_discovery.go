package csg

import (
	"context"

	"plainsolid/brep"
	"plainsolid/collision"
)

// JoinCoincidentPlanarInsets discovers one original host face and all disjoint
// complete tool footprints before proving and constructing the inset join.
func JoinCoincidentPlanarInsets(ctx context.Context, host, tool brep.Solid, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarInsetSetJoinHistory, error) {
	var zero PlanarInsetSetJoinHistory
	p, err := collision.DiscoverPlanarInsetContactSet(ctx, host, tool, o)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	return joinProvedPlanarInsets(ctx, host, tool, p, o.Validation)
}
