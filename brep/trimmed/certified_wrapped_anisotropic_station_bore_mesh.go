//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedWrappedAnisotropicStationBoreMesh retains both original station
// grids and actual source face/rim IDs. SquaredErrors bound both directions
// between each original wall/cap patch and its displayed triangle. They do not
// bound material volume error or normals. StationRimEdges use original U order;
// StationVertices use the synchronized subdivided original U network.
type CertifiedWrappedAnisotropicStationBoreMesh struct {
	Mesh                                            mesh.Mesh
	Source                                          WrappedAnisotropicStationBoreProof
	OriginalFace                                    []int
	StationVertices                                 [2][][]int
	StationRimEdges                                 [2][][4]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Subdivisions, TriangleChecks, StationSamples    int
}

// TessellateWrappedAnisotropicStationBoreCertified reauthenticates the full
// native material, including independent station grids and strict diagonal
// contraction of its common weighted profile. Terminal U knots must match.
// Both wall families share one bounded U refinement; annular cap charts join
// corresponding source points by exact affine interpolation. These are common
// profile parameters, not generally common model-space radial rays.
//
// On each quadrant the normalized profile has strictly monotone coordinates.
// The interpolated positive X/Y scales increase strictly from inner to outer,
// so the planar cap Jacobian has fixed nonzero sign. Intermediate profiles are
// strictly nested by the same downward-closed quadrant proof as the source
// bore. The cap chart therefore covers exactly the annulus, once. Exact patch
// residuals prove both directions to the indexed cap triangles. Inner wall
// orientation is reversed only after each original outward band is proved.
// Global closed links, positive signed volume and exact nonincident contact
// rejection remain mandatory. No tolerance welding or collision fallback.
func (s Solid) TessellateWrappedAnisotropicStationBoreCertified(ctx context.Context, o CertifiedWrappedStationMeshOptions) (CertifiedWrappedAnisotropicStationBoreMesh, error) {
	var empty CertifiedWrappedAnisotropicStationBoreMesh
	o, e := normalizeWrappedStationBoreMeshOptions(ctx, o)
	if e != nil {
		return empty, e
	}
	proof, sources, e := s.validateAnisotropicStationBore(ctx, o.Band.Embedding)
	if e != nil {
		return empty, e
	}
	var bands [2][]*geom.WrappedSurfacePatch
	histories := [2]WrappedLoftHistory{proof.Outer.History, proof.Inner.History}
	originals := [2][]int{proof.OuterWalls, proof.InnerWalls}
	var rims [2][][4]int
	for side, ids := range originals {
		for _, id := range ids {
			bands[side] = append(bands[side], s.faces[id].Surface.(*geom.WrappedSurfacePatch))
		}
		// Exact native reproof has already matched all coedges. Locate each
		// station occurrence on an incident original wall, never by model proximity.
		for station, ring := range histories[side].StationRims {
			band := station
			if band == len(ids) {
				band--
			}
			source := sources[side].faces[histories[side].WallFaces[band]]
			actual := s.faces[ids[band]]
			var mapped [4]int
			for k, edge := range ring {
				found := -1
				for _, u := range source.Outer {
					if u.Edge == edge {
						for _, v := range actual.Outer {
							if u.UVStart == v.UVStart && u.UVEnd == v.UVEnd {
								if found >= 0 && found != v.Edge {
									return empty, geom.ErrConvergence
								}
								found = v.Edge
							}
						}
					}
				}
				if found < 0 {
					return empty, geom.ErrConvergence
				}
				mapped[k] = found
			}
			rims[side] = append(rims[side], mapped)
		}
	}
	build := func(side int, opts CertifiedWrappedStationMeshOptions, minimum int, record *[]wrappedDisplayBand) (CertifiedWrappedStationMesh, error) {
		return tessellateProvedWrappedStationBands(ctx, opts, minimum, record, bands[side], histories[side], tessellateWrappedAnisotropicBandFrom)
	}
	q, e := tessellateProvedWrappedStationBore(ctx, o, nil, bands, histories, originals, proof.Caps, build)
	if e != nil {
		return empty, e
	}
	return CertifiedWrappedAnisotropicStationBoreMesh{q.Mesh, proof, q.OriginalFace, q.StationVertices, rims, q.SquaredErrors, q.SourceImageSquaredDistance, q.SignedDisplayVolume, q.Collision, q.Subdivisions, q.TriangleChecks, q.StationSamples}, nil
}
