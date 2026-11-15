package csg

import (
	"context"

	"plainsolid/brep"
	"plainsolid/collision"
)

// PlanarRegionJoinHistory preserves both original face regions and maps the
// original entities that remain after deleting the internal contact.
type PlanarRegionJoinHistory struct {
	Owners  [2]PlanarFaceJoinOwner
	Contact collision.PlanarRegionContact
}

// JoinPlanarRegions makes a regularized union across coincident complete
// original face regions. Internal face subdivisions may differ; the complete
// original seam between the removed regions and the retained exterior must
// have the same edges on both operands.
func JoinPlanarRegions(ctx context.Context, a, b brep.Solid, faces [2][]int, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarRegionJoinHistory, error) {
	proof, err := collision.ValidatePlanarRegionContact(ctx, a, b, faces, o)
	if err != nil {
		return brep.Solid{}, PlanarRegionJoinHistory{}, err
	}
	input := planarInterface{faces: proof.Faces, vertices: proof.Vertices, edges: proof.Edges, volume: proof.SignedVolume}
	out, owners, err := joinPlanarInterface(ctx, a, b, input, o.Validation)
	if err != nil {
		return brep.Solid{}, PlanarRegionJoinHistory{}, err
	}
	return out, PlanarRegionJoinHistory{Owners: owners, Contact: proof}, nil
}
