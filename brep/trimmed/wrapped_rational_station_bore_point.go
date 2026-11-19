//go:build research

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
	profiles, bands := s.wrappedRationalStationBoreProfiles(p)
	return locateWrappedRationalStationBorePoint(ctx, profiles, bands, query, o)
}

func (s Solid) wrappedRationalStationBoreProfiles(p WrappedStationBoreProof) ([2]WrappedStationLoftProof, [2][]*geom.WrappedSurfacePatch) {
	profiles := [2]WrappedStationLoftProof{p.Outer, p.Inner}
	var bands [2][]*geom.WrappedSurfacePatch
	for i, faces := range [2][]int{p.OuterWalls, p.InnerWalls} {
		profiles[i].History.WallFaces = faces
		profiles[i].History.CapFaces = p.Caps
		bands[i] = make([]*geom.WrappedSurfacePatch, len(faces))
		for j, face := range faces {
			bands[i][j] = s.faces[face].Surface.(*geom.WrappedSurfacePatch)
		}
	}
	return profiles, bands
}

func locateWrappedRationalStationBorePoint(ctx context.Context, profiles [2]WrappedStationLoftProof, bands [2][]*geom.WrappedSurfacePatch,
	query [3]*big.Rat, o WrappedRationalStationPointOptions) (WrappedRationalStationPoint, error) {
	var classifications [2]WrappedRationalStationPoint
	for i, proof := range profiles {
		var err error
		classifications[i], err = locateWrappedRationalStationPoint(ctx, proof, bands[i], query, o)
		if err != nil {
			return WrappedRationalStationPoint{}, err
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
		caps := profiles[0].History.CapFaces
		walls := slices.DeleteFunc(inner.Faces, func(face int) bool {
			return face == caps[0] || face == caps[1]
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
