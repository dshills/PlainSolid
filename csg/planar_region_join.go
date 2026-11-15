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
	return joinProvedPlanarRegions(ctx, a, b, proof, o.Validation)
}

// JoinCoincidentPlanarRegions discovers the complete interface even when the
// contacting original faces are divided differently across the two bodies.
func JoinCoincidentPlanarRegions(ctx context.Context, a, b brep.Solid, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarRegionJoinHistory, error) {
	proof, err := collision.DiscoverPlanarRegionContact(ctx, a, b, o)
	if err != nil {
		return brep.Solid{}, PlanarRegionJoinHistory{}, err
	}
	return joinProvedPlanarRegions(ctx, a, b, proof, o.Validation)
}

func joinProvedPlanarRegions(ctx context.Context, a, b brep.Solid, proof collision.PlanarRegionContact, o collision.LocationOptions) (brep.Solid, PlanarRegionJoinHistory, error) {
	input := planarInterface{faces: proof.Faces, vertices: proof.Vertices, edges: proof.Edges, volume: proof.SignedVolume}
	out, owners, err := joinPlanarInterface(ctx, a, b, input, o)
	if err != nil {
		return brep.Solid{}, PlanarRegionJoinHistory{}, err
	}
	return out, PlanarRegionJoinHistory{Owners: owners, Contact: proof}, nil
}
