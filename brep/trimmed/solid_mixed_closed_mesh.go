package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// StoredMixedClosedMeshOptions bounds all face proofs as in the network API,
// then caps combined triangles (default100000, maximum collision.MaxTriangles)
// and exact BVH non-incidence checks. MaxRounds defaults2, with maximum8.
type StoredMixedClosedMeshOptions struct {
	Faces                   []SolidMixedFaceMeshOptions
	MaxRounds, MaxTriangles int
	Collision               collision.Options
}

// CertifiedStoredMixedClosedMesh is one connected, oriented manifold DISPLAY
// shell with positive exact represented volume and no non-incidence contacts.
// FaceVertices maps every local face vertex to its global display ID; boundary
// IDs equal Network's exact shared IDs, while interior IDs remain private.
// SquaredDistance bounds both directed source-image/display distances. This is
// not proof of source-shell embedding, ideal volume, or generic material CSG.
type CertifiedStoredMixedClosedMesh struct {
	Network          StoredMixedSeamNetwork
	Mesh             mesh.Mesh
	FaceVertices     [][]int
	OriginalFace     []int
	DisplayVolume    *big.Rat
	SquaredDistance  *big.Rat
	DisplayCollision collision.Report
}

func (s Solid) TessellateMixedClosedCertified(ctx context.Context, o StoredMixedClosedMeshOptions) (CertifiedStoredMixedClosedMesh, error) {
	var empty CertifiedStoredMixedClosedMesh
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if o.MaxRounds == 0 {
		o.MaxRounds = 2
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > collision.MaxTriangles || o.Collision.MaxPairs < 0 || o.Collision.MaxContacts < 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxTriangles < len(s.faces) {
		return empty, geom.ErrConvergence
	}
	network, e := s.TessellateMixedBoundaryNetworkCertified(ctx, o.Faces, o.MaxRounds)
	if e != nil {
		return empty, e
	}
	count := 0
	for _, f := range network.Faces {
		if len(f.Mesh.Triangles) > o.MaxTriangles-count {
			return empty, geom.ErrConvergence
		}
		count += len(f.Mesh.Triangles)
	}
	r := CertifiedStoredMixedClosedMesh{Network: network, Mesh: mesh.Mesh{Vertices: append([]geom.Vec3(nil), network.Points...)}, FaceVertices: make([][]int, len(network.Faces)), SquaredDistance: new(big.Rat).Set(network.SquaredDistance)}
	for fi, f := range network.Faces {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		ids := make([]int, len(f.Mesh.Vertices))
		for local, p := range f.Mesh.Vertices {
			if id, ok := network.FaceVertices[fi][local]; ok {
				ids[local] = id
			} else {
				ids[local] = len(r.Mesh.Vertices)
				r.Mesh.Vertices = append(r.Mesh.Vertices, p)
			}
		}
		r.FaceVertices[fi] = ids
		for _, tri := range f.Mesh.Triangles {
			r.Mesh.Triangles = append(r.Mesh.Triangles, [3]int{ids[tri[0]], ids[tri[1]], ids[tri[2]]})
			r.OriginalFace = append(r.OriginalFace, fi)
		}
	}
	// This API deliberately restricts to one connected display component. Fan
	// validation below rules out components joined only at a nonmanifold vertex.
	parent := make([]int, len(r.Mesh.Vertices))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for _, t := range r.Mesh.Triangles {
		for _, i := range t[1:] {
			parent[root(i)] = root(t[0])
		}
	}
	for i := range parent {
		if root(i) != root(0) {
			return empty, geom.ErrConvergence
		}
	}
	volume, report, e := checkClosedPeriodicMesh(ctx, r.Mesh, o.Collision)
	if e != nil {
		return empty, e
	}
	r.DisplayVolume = volume
	r.DisplayCollision = report
	return r, nil
}
