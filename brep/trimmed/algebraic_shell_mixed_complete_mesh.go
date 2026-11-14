package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MixedCompleteMeshOptions bounds the final assembly checks, independently of
// the already completed part proofs. Zero selects 32,768 triangles and 8
// million pairs.
type MixedCompleteMeshOptions struct{ MaxTriangles, MaxPairs int }
type MixedCompleteMeshWork struct {
	BoundarySegments, BoundaryOperations, BoundaryEmbeddingPairs                 int
	SourceOperations, TriangleChecks, FragmentContactPairs, AssemblyContactPairs int
	StrictMaterialCells, BoundaryCollarCells, CollarCandidatePairs               int
}

// CertifiedMixedAlgebraicShellMesh binds a closed, indexed, embedded display to
// its ORIGINAL immutable mixed material source. No source recipe/version or
// original parameter is replaced by the mesh. Per-face source correspondence
// bounds give both directed distances for the complete boundary union.
type CertifiedMixedAlgebraicShellMesh struct {
	network       *CertifiedMixedAlgebraicShellBoundary
	geometry      mesh.Mesh
	faces         []int
	bound, volume *big.Rat
	work          MixedCompleteMeshWork
}

func (r *CertifiedMixedAlgebraicShellMesh) Mesh() mesh.Mesh {
	if r == nil {
		return mesh.Mesh{}
	}
	return mesh.Mesh{Vertices: append([]geom.Vec3(nil), r.geometry.Vertices...), Triangles: append([][3]int(nil), r.geometry.Triangles...)}
}
func (r *CertifiedMixedAlgebraicShellMesh) TriangleFaces() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.faces...)
}
func (r *CertifiedMixedAlgebraicShellMesh) SourceMaterial() *MixedAlgebraicShellBoolean {
	if r == nil {
		return nil
	}
	return r.network.source
}
func (r *CertifiedMixedAlgebraicShellMesh) SourceImageSquaredDistance() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.bound)
}

// DisplaySignedVolume measures only the certified display, not the exact body.
func (r *CertifiedMixedAlgebraicShellMesh) DisplaySignedVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}
func (r *CertifiedMixedAlgebraicShellMesh) Work() MixedCompleteMeshWork {
	if r == nil {
		return MixedCompleteMeshWork{}
	}
	return r.work
}
func (r *CertifiedMixedAlgebraicShellMesh) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

// AssembleCompleteMeshCertified admits parts from this SAME immutable network
// only. Every original material face must occur exactly once. Only canonical
// network indices are reused; independently evaluated interior points are never
// welded by rounded coordinate equality. All triangle intersections and every
// vertex link are checked independently after assembly.
func (n *CertifiedMixedAlgebraicShellBoundary) AssembleCompleteMeshCertified(ctx context.Context, planar *CertifiedMixedPlanarFragmentFill, roofs []*CertifiedMixedRoofFragment, o MixedCompleteMeshOptions) (*CertifiedMixedAlgebraicShellMesh, error) {
	if ctx == nil || n == nil || n.source == nil || !n.embedded || planar == nil || planar.base == nil || planar.base.source != n || len(roofs) == 0 {
		return nil, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 32768
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 8000000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 100000 || o.MaxPairs < 1 || o.MaxPairs > 16000000 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(roofs) > len(n.source.boundary.faces) {
		return nil, geom.ErrInvalid
	}
	if len(n.network.Vertices) > 3*o.MaxTriangles {
		return nil, geom.ErrConvergence
	}
	ordered := append([]*CertifiedMixedRoofFragment(nil), roofs...)
	for _, roof := range ordered {
		if roof == nil || roof.base == nil || roof.base.network != n {
			return nil, geom.ErrInvalid
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].base.face < ordered[j].base.face })
	r := &CertifiedMixedAlgebraicShellMesh{network: n, geometry: mesh.Mesh{Vertices: n.Vertices()}, bound: new(big.Rat), volume: new(big.Rat)}
	seen := make([]bool, len(n.source.boundary.faces))
	originalCount := len(r.geometry.Vertices)
	own := func(id int) error {
		if id < 0 || id >= len(seen) || seen[id] {
			return geom.ErrInvalid
		}
		seen[id] = true
		return nil
	}
	r.work.BoundarySegments, r.work.BoundaryOperations = n.Work()
	r.work.BoundaryEmbeddingPairs = n.pairs
	for _, roof := range ordered {
		if roof == nil || roof.base == nil || roof.base.network != n || roof.base.omitted != 0 {
			return nil, geom.ErrInvalid
		}
		if e := own(roof.base.face); e != nil {
			return nil, e
		}
		part := roof.base.geometry
		if len(part.Vertices) < originalCount || len(r.geometry.Triangles)+len(part.Triangles) > o.MaxTriangles {
			return nil, geom.ErrConvergence
		}
		mapping := make([]int, len(part.Vertices))
		for i, v := range part.Vertices {
			if i < originalCount {
				if v != r.geometry.Vertices[i] {
					return nil, geom.ErrInvalid
				}
				mapping[i] = i
			} else {
				mapping[i] = len(r.geometry.Vertices)
				r.geometry.Vertices = append(r.geometry.Vertices, v)
			}
		}
		for _, tri := range part.Triangles {
			r.geometry.Triangles = append(r.geometry.Triangles, [3]int{mapping[tri[0]], mapping[tri[1]], mapping[tri[2]]})
			r.faces = append(r.faces, roof.base.face)
		}
		if roof.base.imageBound.Cmp(r.bound) > 0 {
			r.bound.Set(roof.base.imageBound)
		}
		ops, checks, pairs, collarPairs := roof.Work()
		r.work.SourceOperations += ops
		r.work.TriangleChecks += checks
		r.work.FragmentContactPairs += pairs
		r.work.CollarCandidatePairs += collarPairs
		for _, c := range roof.cells {
			if c.VerticalClearance.Sign() > 0 {
				r.work.StrictMaterialCells++
			} else {
				r.work.BoundaryCollarCells++
			}
		}
	}
	if len(planar.base.geometry.Vertices) != originalCount {
		return nil, geom.ErrInvalid
	}
	for i, v := range planar.base.geometry.Vertices {
		if v != r.geometry.Vertices[i] {
			return nil, geom.ErrInvalid
		}
	}
	for _, f := range planar.base.fragments {
		if e := own(f.Face); e != nil {
			return nil, e
		}
		if len(r.geometry.Triangles)+len(f.Triangles) > o.MaxTriangles {
			return nil, geom.ErrConvergence
		}
		r.geometry.Triangles = append(r.geometry.Triangles, f.Triangles...)
		for range f.Triangles {
			r.faces = append(r.faces, f.Face)
		}
		distance := new(big.Rat).Mul(f.ErrorUpper, f.ErrorUpper)
		if distance.Cmp(r.bound) > 0 {
			r.bound.Set(distance)
		}
	}
	ops, _, pairs := planar.Work()
	r.work.SourceOperations += ops
	r.work.FragmentContactPairs += pairs
	for _, v := range seen {
		if !v {
			return nil, geom.ErrConvergence
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if e := r.geometry.ValidateSolid(); e != nil {
		return nil, e
	}
	ids := make([]int, len(r.geometry.Vertices))
	used := make([]bool, len(ids))
	edges := map[[2]int]bool{}
	for i := range ids {
		ids[i] = i
	}
	for _, tri := range r.geometry.Triangles {
		for j, v := range tri {
			used[v] = true
			a, b := v, tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			edges[[2]int{a, b}] = true
		}
	}
	for _, v := range used {
		if !v {
			return nil, geom.ErrInvalid
		}
	}
	topology, e := n.source.Topology()
	if e != nil {
		return nil, e
	}
	if topology.CollapsedEdges != 0 || len(ids)-len(edges)+len(r.geometry.Triangles) != topology.EulerCharacteristic {
		return nil, geom.ErrUnrepresentable
	}
	if e := validateQuadraticJunctionFans(r.geometry, ids); e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	report, e := collision.SelfIntersections(r.geometry, collision.Options{MaxPairs: o.MaxPairs, MaxContacts: 1})
	if e != nil {
		return nil, e
	}
	if !report.Complete || len(report.Contacts) > 0 {
		return nil, geom.ErrConvergence
	}
	r.work.AssemblyContactPairs = report.BroadPhasePairs
	for _, tri := range r.geometry.Triangles {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		a, b, c := exactBoundPoint(r.geometry.Vertices[tri[0]]), exactBoundPoint(r.geometry.Vertices[tri[1]]), exactBoundPoint(r.geometry.Vertices[tri[2]])
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			r.volume.Add(r.volume, new(big.Rat).Mul(a[i], new(big.Rat).Sub(new(big.Rat).Mul(b[j], c[k]), new(big.Rat).Mul(b[k], c[j]))))
		}
	}
	r.volume.Quo(r.volume, big.NewRat(6, 1))
	if r.volume.Sign() <= 0 {
		return nil, geom.ErrDegenerate
	}
	return r, nil
}
