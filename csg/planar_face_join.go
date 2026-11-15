package csg

import (
	"context"

	"plainsolid/brep"
	"plainsolid/collision"
)

// PlanarFaceJoinOwner maps the original vertex, edge, and face indices of one
// input to the result. EdgeUse.Reversed compares the stored directions. The
// removed faces and other entities wholly inside the interface map to -1.
type PlanarFaceJoinOwner struct {
	Vertices []int
	Edges    []brep.EdgeUse
	Faces    []int
}

// PlanarFaceJoinHistory records the two original owners and their certified
// interface. Only the perimeter vertices and edges have two original owners.
type PlanarFaceJoinHistory struct {
	Owners  [2]PlanarFaceJoinOwner
	Contact collision.PlanarFaceContact
}

// JoinPlanarFacePair returns the regularized union of two outward planar
// single-shell parts that meet only along a complete, oppositely oriented
// original face. The shared face may be concave and have matching inner wires.
// Neighboring faces and all other original incidences are preserved. Exact
// embedding and material volume are checked before returning the result.
func JoinPlanarFacePair(ctx context.Context, a, b brep.Solid, faceA, faceB int, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarFaceJoinHistory, error) {
	result, h, err := JoinPlanarFaceSet(ctx, a, b, [][2]int{{faceA, faceB}}, o)
	if err != nil {
		return brep.Solid{}, PlanarFaceJoinHistory{}, err
	}
	return result, PlanarFaceJoinHistory{Owners: h.Owners, Contact: h.Contact.Faces[0]}, nil
}
