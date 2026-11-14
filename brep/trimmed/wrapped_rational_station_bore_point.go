package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateWrappedRationalStationBoreRational classifies exact points on a
// certified noncircular through-bore. An exposed annulus owns the cap; its
// opening has no cap material. Both original wall owners are returned at
// an internal crease. MaxPointOperations is applied to each profile.
func (s Solid) LocateWrappedRationalStationBoreRational(ctx context.Context, point [3]*big.Rat,
	o WrappedRationalStationPointOptions) (WrappedRationalStationPoint, error) {
	var empty WrappedRationalStationPoint
	query, o, err := prepareWrappedRationalStationPointQuery(ctx, point, o)
	if err != nil {
		return empty, err
	}
	p, err := s.ValidateWrappedRationalStationBore(ctx, o.Source)
	if err != nil {
		return empty, err
	}
	profiles := [2]WrappedStationLoftProof{p.Outer, p.Inner}
	owners := [2][]int{p.OuterWalls, p.InnerWalls}
	var classifications [2]WrappedRationalStationPoint
	for i, proof := range profiles {
		proof.History.WallFaces = owners[i]
		proof.History.CapFaces = p.Caps
		bands := make([]*geom.WrappedSurfacePatch, len(owners[i]))
		for j, face := range owners[i] {
			bands[j] = s.faces[face].Surface.(*geom.WrappedSurfacePatch)
		}
		classifications[i], err = locateWrappedRationalStationPoint(ctx, proof, bands, query, o)
		if err != nil {
			return empty, err
		}
		if i == 0 && classifications[i].Location == parametric.Outside {
			return classifications[i], nil
		}
	}
	outer, inner := classifications[0], classifications[1]
	if inner.Location == parametric.Inside {
		return WrappedRationalStationPoint{Location: parametric.Outside}, nil
	}
	if inner.Location == parametric.Boundary {
		walls := slices.DeleteFunc(inner.Faces, func(face int) bool {
			return face == p.Caps[0] || face == p.Caps[1]
		})
		if len(walls) == 0 { // strictly inside the excluded cap opening
			return WrappedRationalStationPoint{Location: parametric.Outside}, nil
		}
		outer.Location = parametric.Boundary
		outer.Faces = append(outer.Faces, walls...)
		slices.Sort(outer.Faces)
	}
	return outer, nil
}
