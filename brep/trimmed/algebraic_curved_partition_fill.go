package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type CurvedAlgebraicPartitionMeshOptions struct {
	Boundary                   AlgebraicBoundaryMeshOptions // UV deflection is derived from Mesh.Deflection.
	Mesh                       CurvedQuadraticTrimMeshOptions
	MaxVertices, MaxPredicates int
}

// CertifiedCurvedAlgebraicPartitionMesh bounds both directed distances from
// each exact source fragment image (and their union) to corresponding triangles.
// Exact UV winding/coverage transports through a proven chart Lipschitz bound;
// per-triangle rational residuals include every rounded model vertex. The chart
// is positive rational degree<=2 per axis; the entry point selects single-span
// or C0 multi-span certification. No embedding or source
// injectivity is claimed. All triangles share one refinement of the common cut.
type CertifiedCurvedAlgebraicPartitionMesh struct {
	Mesh                                                                                          mesh.Mesh
	UVBoundary                                                                                    CertifiedAlgebraicUVBoundary
	ExactUV                                                                                       [][2]*big.Rat
	ExactPoints                                                                                   [][3]*big.Rat
	OriginalFragment                                                                              []int
	SquaredErrors                                                                                 []*big.Rat
	LipschitzSquared, TransportedTrimSquaredDistance, SurfaceSquaredDistance, FillSquaredDistance *big.Rat
	SpanCellChecks, SpanPieceChecks                                                               int
	Subdivisions, TriangleChecks, Predicates                                                      int
}

func (s AlgebraicSubdivision) TessellateCurvedPartitionCertified(ctx context.Context, face int, line [3]*big.Rat, o CurvedAlgebraicPartitionMeshOptions) (CertifiedCurvedAlgebraicPartitionMesh, error) {
	return s.tessellateCurvedPartition(ctx, face, line, o, nil)
}
func (s AlgebraicSubdivision) tessellateCurvedPartition(ctx context.Context, face int, line [3]*big.Rat, o CurvedAlgebraicPartitionMeshOptions, spans *multiSpanMeshWork) (CertifiedCurvedAlgebraicPartitionMesh, error) {
	var empty CertifiedCurvedAlgebraicPartitionMesh
	if ctx == nil || face < 0 || face >= len(s.faces) || !finite(o.Mesh.Deflection) || o.Mesh.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if o.Mesh.MaxSubdivisions == 0 {
		o.Mesh.MaxSubdivisions = 32
	}
	if o.Mesh.MaxTriangleChecks == 0 {
		o.Mesh.MaxTriangleChecks = 100000
	}
	if o.Mesh.MaxTriangles == 0 {
		o.Mesh.MaxTriangles = 100000
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 256
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.Mesh.MaxSubdivisions < 1 || o.Mesh.MaxSubdivisions > 128 || o.Mesh.MaxSubdivisions&(o.Mesh.MaxSubdivisions-1) != 0 || o.Mesh.MaxTriangleChecks < 1 || o.Mesh.MaxTriangleChecks > 1000000 || o.Mesh.MaxTriangles < 1 || o.Mesh.MaxTriangles > 1000000 || o.MaxVertices < 3 || o.MaxVertices > 1024 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return empty, geom.ErrInvalid
	}
	patch, e := exactFaceSupportPatch(s.source.faces[face], o.Boundary.Proof.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	l2, l1, ok := quadraticPatchLipschitz(patch)
	if spans != nil {
		l2, l1, e = spans.lipschitz(ctx, patch)
		if e != nil {
			return empty, e
		}
		ok = true
	}
	if !ok || l1.Sign() == 0 {
		return empty, geom.ErrConvergence
	}
	tolerance := new(big.Rat).Quo(ratInset(o.Mesh.Deflection), new(big.Rat).Mul(big.NewRat(4, 1), l1))
	d, _ := tolerance.Float64()
	if !finite(d) || d <= 0 {
		return empty, geom.ErrConvergence
	}
	if ratInset(d).Cmp(tolerance) > 0 {
		d = math.Nextafter(d, 0)
	}
	if d <= 0 || !finite(d) {
		return empty, geom.ErrConvergence
	}
	o.Boundary.Display.Deflection = d
	boundary, e := s.TessellatePartitionUVBoundaryCertified(ctx, face, line, o.Boundary)
	if e != nil {
		return empty, e
	}
	proof := partitionFillProof{ctx: ctx, limit: o.MaxPredicates, points: make([][2]*big.Rat, len(boundary.UV))}
	for i, p := range boundary.UV {
		proof.points[i] = [2]*big.Rat{ratInset(p.X), ratInset(p.Y)}
	}
	var original []int
	for _, use := range s.faces[face].Outer {
		path := append([]int(nil), boundary.Edges[use.Edge]...)
		if use.Reversed {
			for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
				path[a], path[b] = path[b], path[a]
			}
		}
		if len(original) > 0 {
			if original[len(original)-1] != path[0] {
				return empty, geom.ErrInvalid
			}
			path = path[1:]
		}
		original = append(original, path...)
	}
	if len(original) < 4 || original[0] != original[len(original)-1] {
		return empty, geom.ErrInvalid
	}
	original = original[:len(original)-1]
	rings := [][]int{original, boundary.Fragments[0], boundary.Fragments[1]}
	areas := make([]*big.Rat, 3)
	for i, ring := range rings {
		if len(ring) > o.MaxVertices {
			return empty, geom.ErrConvergence
		}
		if e := proof.simple(ring); e != nil {
			return empty, e
		}
		areas[i] = proof.area(ring)
		if areas[i].Sign() == 0 {
			return empty, geom.ErrDegenerate
		}
		rings[i] = append([]int(nil), ring...)
		if areas[i].Sign() < 0 {
			areas[i].Neg(areas[i])
			for a, b := 0, len(ring)-1; a < b; a, b = a+1, b-1 {
				rings[i][a], rings[i][b] = rings[i][b], rings[i][a]
			}
		}
	}
	chain := map[[2]int]int{}
	for i, ring := range rings {
		sign := 1
		if i == 0 {
			sign = -1
		}
		for j, a := range ring {
			partitionChainAdd(chain, a, ring[(j+1)%len(ring)], sign)
		}
	}
	if len(chain) != 0 || new(big.Rat).Add(areas[1], areas[2]).Cmp(areas[0]) != 0 {
		return empty, geom.ErrConvergence
	}
	seed := CertifiedAffineQuadraticTrimMesh{ExactUV: proof.points, BoundaryLoops: [][]int{rings[0]}, Region: AffineQuadraticTrimRegionArea{FillSquaredDistance: boundary.BoundarySquaredDistance}}
	var seedFragments []int
	for fi, ring := range rings[1:] {
		triangles, e := proof.triangulate(ring)
		if e != nil {
			return empty, e
		}
		sum := new(big.Rat)
		chain := map[[2]int]int{}
		for j, a := range ring {
			partitionChainAdd(chain, a, ring[(j+1)%len(ring)], -1)
		}
		for _, tri := range triangles {
			sum.Add(sum, proof.area(tri[:]))
			for j, a := range tri {
				partitionChainAdd(chain, a, tri[(j+1)%3], 1)
			}
			seed.Mesh.Triangles = append(seed.Mesh.Triangles, tri)
			seedFragments = append(seedFragments, fi)
		}
		if len(chain) != 0 || sum.Cmp(areas[fi+1]) != 0 {
			return empty, geom.ErrConvergence
		}
	}
	if len(seed.Mesh.Triangles) > o.Mesh.MaxTriangles {
		return empty, geom.ErrConvergence
	}
	refined, e := tessellateCertifiedTrimBoundaryMode(ctx, patch, seed, o.Mesh, 1, spans, l2, false)
	if e != nil {
		return empty, e
	}
	result := CertifiedCurvedAlgebraicPartitionMesh{Mesh: refined.Mesh, UVBoundary: boundary, ExactUV: refined.ExactUV, ExactPoints: refined.ExactPoints, SquaredErrors: refined.SquaredErrors, LipschitzSquared: l2, TransportedTrimSquaredDistance: refined.TransportedTrimSquaredDistance, SurfaceSquaredDistance: refined.SurfaceSquaredDistance, FillSquaredDistance: refined.FillSquaredDistance, Subdivisions: refined.Subdivisions, TriangleChecks: refined.TriangleChecks}
	// Independently recover refinement ancestry and prove each output triangle
	// lies within its seed, rather than inferring ancestry from centroid alone.
	proof.points = append([][2]*big.Rat(nil), refined.ExactUV...)
	for i, tri := range result.Mesh.Triangles {
		center := [2]*big.Rat{new(big.Rat), new(big.Rat)}
		for _, id := range tri {
			for axis := 0; axis < 2; axis++ {
				center[axis].Add(center[axis], refined.ExactUV[id][axis])
			}
		}
		for axis := range center {
			center[axis].Quo(center[axis], big.NewRat(3, 1))
		}
		cid := len(proof.points)
		proof.points = append(proof.points, center)
		owner := -1
		for si, seedTri := range seed.Mesh.Triangles {
			inside := true
			for j, a := range seedTri {
				sign, e := proof.orient(a, seedTri[(j+1)%3], cid)
				if e != nil {
					return empty, e
				}
				if sign <= 0 {
					inside = false
					break
				}
			}
			if !inside {
				continue
			}
			if owner >= 0 {
				return empty, geom.ErrConvergence
			}
			owner = si
		}
		if owner < 0 {
			return empty, geom.ErrConvergence
		}
		for _, id := range tri {
			for j, a := range seed.Mesh.Triangles[owner] {
				sign, e := proof.orient(a, seed.Mesh.Triangles[owner][(j+1)%3], id)
				if e != nil {
					return empty, e
				}
				if sign < 0 {
					return empty, geom.ErrConvergence
				}
			}
		}
		result.OriginalFragment = append(result.OriginalFragment, seedFragments[owner])
		if s.faces[face].Reversed {
			result.Mesh.Triangles[i][1], result.Mesh.Triangles[i][2] = result.Mesh.Triangles[i][2], result.Mesh.Triangles[i][1]
		}
	}
	result.SpanCellChecks, result.SpanPieceChecks = refined.SpanCellChecks, refined.SpanPieceChecks
	result.Predicates = proof.work
	return result, nil
}
