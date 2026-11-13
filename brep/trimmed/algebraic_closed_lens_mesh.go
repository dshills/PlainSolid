package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type ClosedAlgebraicLensMeshOptions struct {
	Boundary                       AlgebraicLensBoundaryOptions
	Interior                       CurvedQuadraticTrimMeshOptions // Controls triangle residual separately from boundary approximation.
	MaxSeedVertices, MaxPredicates int
	Collision                      collision.Options
}

type CertifiedClosedLensHalf struct {
	Side                       int
	Mesh                       mesh.Mesh
	OriginalFace               []int // -1 marks cap triangles.
	NetworkVertices            map[int]int
	FaceResidualSquared        [2]*big.Rat
	SourceImageSquaredDistance *big.Rat
	SignedDisplayVolume        *big.Rat
	Collision                  collision.Report
}

// CertifiedClosedAlgebraicLensMesh proves each display half is a connected
// closed oriented manifold with no non-incidence contacts. SourceImageSquaredDistance
// bounds both directed distances of boundary images, not volume error/material
// Hausdorff distance. Supported sources have independently curved
// degree<=2 positive-rational upper and lower graphs. The entry point selects
// single-span or C0 multi-span residual certification.
type CertifiedClosedAlgebraicLensMesh struct {
	Boundary                        CertifiedAlgebraicLensBoundary
	Negative, Positive              CertifiedClosedLensHalf
	SpanCellChecks, SpanPieceChecks int
	Predicates, TriangleChecks      int
}

// TessellateClosedHalvesCertified proves represented UV polygon coverage and
// fixes all prescribed model boundary samples during interior refinement.
// The constructor's exact identity-XY charts make model boundary error also a
// valid UV homotopy error. Winding transports the closed source fragment fill
// to its represented polygon; the chart Lipschitz bound and exact triangle
// residual then bound both directed image distances. The planar cap uses the
// same winding argument directly. The returned total bound is reported, not
// assumed equal to Interior.Deflection, and is not a mass/volume error bound.
func (s AlgebraicLensPlaneSplit) TessellateClosedHalvesCertified(ctx context.Context, o ClosedAlgebraicLensMeshOptions) (CertifiedClosedAlgebraicLensMesh, error) {
	return s.tessellateClosedHalves(ctx, o, nil)
}
func (s AlgebraicLensPlaneSplit) tessellateClosedHalves(ctx context.Context, o ClosedAlgebraicLensMeshOptions, spans *multiSpanMeshWork) (CertifiedClosedAlgebraicLensMesh, error) {
	var empty CertifiedClosedAlgebraicLensMesh
	if ctx == nil || !finite(o.Interior.Deflection) || o.Interior.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if o.Interior.MaxSubdivisions == 0 {
		o.Interior.MaxSubdivisions = 32
	}
	if o.Interior.MaxTriangles == 0 {
		o.Interior.MaxTriangles = 100000
	}
	if o.Interior.MaxTriangleChecks == 0 {
		o.Interior.MaxTriangleChecks = 100000
	}
	if o.MaxSeedVertices == 0 {
		o.MaxSeedVertices = 1024
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 10000000
	}
	if o.Interior.MaxSubdivisions < 2 || o.Interior.MaxSubdivisions > 128 || o.Interior.MaxSubdivisions&(o.Interior.MaxSubdivisions-1) != 0 || o.Interior.MaxTriangles < 1 || o.Interior.MaxTriangles > 1000000 || o.Interior.MaxTriangleChecks < 1 || o.Interior.MaxTriangleChecks > 1000000 || o.MaxSeedVertices < 3 || o.MaxSeedVertices > 4096 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return empty, geom.ErrInvalid
	}
	if len(s.source.subdivision.source.faces) != 2 {
		return empty, geom.ErrConvergence
	}
	lower, e := exactFaceSupportPatch(s.source.subdivision.source.faces[1], o.Boundary.Proof.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	// Constant-height supports admit exact height conditioning. Other supports
	// retain the independently bounded common rim representatives; every owner
	// and every incident triangle still re-certifies those prescribed points.
	local := s
	if affine, err := geom.NewAffineSurfaceChart(ctx, lower, geom.AffineSurfaceChartOptions{}); err == nil {
		a, ea := affine.EvalExact(new(big.Rat), new(big.Rat))
		b, eb := affine.EvalExact(big.NewRat(1, 1), new(big.Rat))
		c, ec := affine.EvalExact(new(big.Rat), big.NewRat(1, 1))
		if ea == nil && eb == nil && ec == nil && a[2].Cmp(b[2]) == 0 && a[2].Cmp(c[2]) == 0 {
			if z, exact := a[2].Float64(); exact && finite(z) {
				local.source.subdivision.vertices = s.source.subdivision.Vertices()
				for i := range local.source.subdivision.vertices {
					local.source.subdivision.vertices[i].Display.Z = z
				}
			}
		}
	}
	network, e := local.TessellateSharedBoundaryCertified(ctx, o.Boundary)
	if e != nil {
		return empty, e
	}
	result := CertifiedClosedAlgebraicLensMesh{Boundary: network}
	for _, side := range []int{-1, 1} {
		half, e := s.Half(side)
		if e != nil {
			return empty, e
		}
		out := CertifiedClosedLensHalf{Side: side, NetworkVertices: map[int]int{}, SourceImageSquaredDistance: new(big.Rat)}
		addShared := func(id int) int {
			if old, ok := out.NetworkVertices[id]; ok {
				return old
			}
			next := len(out.Mesh.Vertices)
			out.NetworkVertices[id] = next
			out.Mesh.Vertices = append(out.Mesh.Vertices, network.Vertices[id])
			return next
		}
		for original, fi := range half.Fragments {
			face := s.source.faces[fi]
			if face.Original != original {
				return empty, geom.ErrConvergence
			}
			var ring []int
			for _, owner := range network.Owners {
				if owner.Side != side || owner.Face != fi || owner.Cap {
					continue
				}
				path := append([]int(nil), owner.Vertices...)
				if face.Reversed {
					for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
						path[a], path[b] = path[b], path[a]
					}
				}
				if len(ring) > 0 {
					if ring[len(ring)-1] != path[0] {
						return empty, geom.ErrConvergence
					}
					path = path[1:]
				}
				ring = append(ring, path...)
			}
			if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
				return empty, geom.ErrConvergence
			}
			ring = ring[:len(ring)-1]
			if len(ring) > o.MaxSeedVertices {
				return empty, geom.ErrConvergence
			}
			seed := CertifiedAffineQuadraticTrimMesh{Region: AffineQuadraticTrimRegionArea{FillSquaredDistance: new(big.Rat)}}
			display := map[int]geom.Vec3{}
			localRing := make([]int, len(ring))
			for i, id := range ring {
				p := network.Vertices[id]
				seed.ExactUV = append(seed.ExactUV, [2]*big.Rat{ratInset(p.X), ratInset(p.Y)})
				display[i] = p
				localRing[i] = i
			}
			seed.BoundaryLoops = [][]int{localRing}
			proof := partitionFillProof{ctx: ctx, limit: o.MaxPredicates - result.Predicates, points: seed.ExactUV}
			if proof.limit < 1 {
				return empty, geom.ErrConvergence
			}
			if e := proof.simple(localRing); e != nil {
				return empty, e
			}
			area := proof.area(localRing)
			if area.Sign() <= 0 {
				return empty, geom.ErrConvergence
			}
			seed.Mesh.Triangles, e = proof.triangulate(localRing)
			if e != nil {
				return empty, e
			}
			sum := new(big.Rat)
			chain := map[[2]int]int{}
			for j, id := range localRing {
				partitionChainAdd(chain, id, localRing[(j+1)%len(localRing)], -1)
			}
			for _, tri := range seed.Mesh.Triangles {
				sum.Add(sum, proof.area(tri[:]))
				for j, id := range tri {
					partitionChainAdd(chain, id, tri[(j+1)%3], 1)
				}
			}
			if len(chain) != 0 || sum.Cmp(area) != 0 {
				return empty, geom.ErrConvergence
			}
			result.Predicates += proof.work
			patch, e := exactFaceSupportPatch(s.source.subdivision.source.faces[original], o.Boundary.Proof.Composition.SurfacePatchOptions)
			if e != nil {
				return empty, e
			}
			l2, _, ok := quadraticPatchLipschitz(patch)
			if spans != nil {
				l2, _, e = spans.lipschitz(ctx, patch)
				if e != nil {
					return empty, e
				}
				ok = true
			}
			if !ok {
				return empty, geom.ErrConvergence
			}
			options := o.Interior
			options.MaxTriangleChecks -= result.TriangleChecks
			if options.MaxTriangleChecks < 1 {
				return empty, geom.ErrConvergence
			}
			if len(seed.Mesh.Triangles) > options.MaxTriangles {
				return empty, geom.ErrConvergence
			}
			refined, e := tessellateCertifiedTrimBoundaryDisplayMode(ctx, patch, seed, options, 2, spans, l2, true, display)
			if e != nil {
				return empty, e
			}
			result.TriangleChecks += refined.TriangleChecks
			out.FaceResidualSquared[original] = refined.SurfaceSquaredDistance
			delta := new(big.Rat)
			for _, use := range face.Outer {
				edge := s.source.edges[use.Edge]
				bound := 0.
				if edge.Boundary >= 0 {
					bound = network.Rim.Certificates[edge.Boundary].MaximumError
				} else {
					for ci, id := range network.Cap.SourceEdges {
						if id == use.Edge {
							bound = network.Cap.Curves[ci].MaximumError
						}
					}
				}
				q := ratInset(bound)
				q.Mul(q, q)
				if q.Cmp(delta) > 0 {
					delta.Set(q)
				}
			}
			distance := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(l2, delta), refined.SurfaceSquaredDistance))
			if distance.Cmp(out.SourceImageSquaredDistance) > 0 {
				out.SourceImageSquaredDistance.Set(distance)
			}
			remap := make([]int, len(refined.Mesh.Vertices))
			for i, p := range refined.Mesh.Vertices {
				if i < len(ring) {
					if p != network.Vertices[ring[i]] {
						return empty, geom.ErrConvergence
					}
					remap[i] = addShared(ring[i])
				} else {
					remap[i] = len(out.Mesh.Vertices)
					out.Mesh.Vertices = append(out.Mesh.Vertices, p)
				}
			}
			for _, tri := range refined.Mesh.Triangles {
				q := [3]int{remap[tri[0]], remap[tri[1]], remap[tri[2]]}
				if face.Reversed {
					q[1], q[2] = q[2], q[1]
				}
				out.Mesh.Triangles = append(out.Mesh.Triangles, q)
				out.OriginalFace = append(out.OriginalFace, original)
			}
		}
		capMap := make([]int, len(network.Cap.Mesh.Vertices))
		for i := range capMap {
			capMap[i] = -1
		}
		for ci, path := range network.Cap.CurveVertices {
			global := network.Edges[network.Cap.SourceEdges[ci]]
			for j, id := range path {
				mapped := addShared(global[j])
				if capMap[id] >= 0 && capMap[id] != mapped {
					return empty, geom.ErrConvergence
				}
				capMap[id] = mapped
			}
		}
		for _, tri := range network.Cap.Mesh.Triangles {
			q := [3]int{capMap[tri[0]], capMap[tri[1]], capMap[tri[2]]}
			if q[0] < 0 || q[1] < 0 || q[2] < 0 {
				return empty, geom.ErrConvergence
			}
			if side != network.Cap.Side {
				q[1], q[2] = q[2], q[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, q)
			out.OriginalFace = append(out.OriginalFace, -1)
		}
		if len(out.Mesh.Triangles) > o.Interior.MaxTriangles {
			return empty, geom.ErrConvergence
		}
		if network.Cap.BoundarySquaredDistance.Cmp(out.SourceImageSquaredDistance) > 0 {
			out.SourceImageSquaredDistance.Set(network.Cap.BoundarySquaredDistance)
		}
		if e := out.Mesh.ValidateSolid(); e != nil {
			return empty, e
		}
		ids := make([]int, len(out.Mesh.Vertices))
		for i := range ids {
			ids[i] = i
		}
		if e := validateQuadraticJunctionFans(out.Mesh, ids); e != nil {
			return empty, e
		}
		adjacency := make([][]int, len(ids))
		for _, tri := range out.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				adjacency[a] = append(adjacency[a], b)
				adjacency[b] = append(adjacency[b], a)
			}
		}
		seen := make([]bool, len(ids))
		seen[0] = true
		stack := []int{0}
		count := 0
		for len(stack) > 0 {
			if err := ctx.Err(); err != nil {
				return empty, err
			}
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			count++
			for _, next := range adjacency[id] {
				if !seen[next] {
					seen[next] = true
					stack = append(stack, next)
				}
			}
		}
		if count != len(ids) {
			return empty, geom.ErrConvergence
		}
		volume := new(big.Rat)
		for _, tri := range out.Mesh.Triangles {
			if err := ctx.Err(); err != nil {
				return empty, err
			}
			a, b, c := exactBoundPoint(out.Mesh.Vertices[tri[0]]), exactBoundPoint(out.Mesh.Vertices[tri[1]]), exactBoundPoint(out.Mesh.Vertices[tri[2]])
			for i := 0; i < 3; i++ {
				j, k := (i+1)%3, (i+2)%3
				volume.Add(volume, new(big.Rat).Mul(a[i], new(big.Rat).Sub(new(big.Rat).Mul(b[j], c[k]), new(big.Rat).Mul(b[k], c[j]))))
			}
		}
		volume.Quo(volume, big.NewRat(6, 1))
		if volume.Sign() <= 0 {
			return empty, geom.ErrDegenerate
		}
		out.SignedDisplayVolume = volume
		report, e := collision.SelfIntersectionsBVHContext(ctx, out.Mesh, o.Collision)
		if e != nil {
			return empty, e
		}
		if !report.Complete || len(report.Contacts) != 0 {
			return empty, geom.ErrConvergence
		}
		out.Collision = report
		if side < 0 {
			result.Negative = out
		} else {
			result.Positive = out
		}
	}
	if spans != nil {
		result.SpanCellChecks, result.SpanPieceChecks = spans.cells, spans.pieces
	}
	return result, nil
}
