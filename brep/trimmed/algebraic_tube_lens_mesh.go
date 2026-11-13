package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type tubeRegionFillSeed struct {
	seed       CertifiedAffineQuadraticTrimMesh
	vertices   []geom.Vec3
	squared    *big.Rat
	predicates int
}

type AlgebraicTubeLensMeshOptions struct {
	Boundary                       AlgebraicTubeLensBoundaryOptions
	Interior                       CurvedQuadraticTrimMeshOptions // Residual tolerance; total source bound is reported separately.
	MaxSeedVertices, MaxPredicates int
	Collision                      collision.Options
}
type CertifiedAlgebraicTubeLensMesh struct {
	FaceRefinementPasses            [2]int // Equal levels; identical projected triangle connectivity.
	Boundary                        CertifiedAlgebraicTubeLensBoundary
	Mesh                            mesh.Mesh
	OriginalFace                    []int
	BoundaryVertices                map[int]int
	FaceResidualSquared             [2]*big.Rat
	SourceImageSquaredDistance      *big.Rat
	SignedDisplayVolume             *big.Rat
	Collision                       collision.Report
	Predicates, TriangleChecks      int
	SpanCellChecks, SpanPieceChecks int
	ResidualOperations              int
}

// TessellateClosedCertified supports two single-span positive-rational graphs
// of degree<=2 whose exact projections are X=U,Y=V. The source lens independently
// proves a regular Jordan section and strict height ordering. Its continuous
// pointwise boundary correspondence projects to a planar straight homotopy.
// Winding therefore bounds the distance of the two closed filled regions by
// the largest boundary error, including points on either boundary. Exact chart
// Lipschitz and triangle interpolation bounds transport this to both directed
// source-boundary/display distances. Original common model displays remain fixed
// and are recertified against both supports; no coordinate welding is used.
// Interior levels are synchronized, with matching represented XY and strictly
// positive interior graph separation required before the collision proof.
// Closed incidence, vertex links, connectivity, positive exact display volume,
// and absence of non-incidence display contacts are separately required.
// The reported distance is not a certified volume or material-distance error.
func (l AlgebraicTubeLens) TessellateClosedCertified(ctx context.Context, o AlgebraicTubeLensMeshOptions) (CertifiedAlgebraicTubeLensMesh, error) {
	return l.tessellateClosedTubeLens(ctx, o, nil, false, nil, nil)
}
func (l AlgebraicTubeLens) tessellateClosedTubeLens(ctx context.Context, o AlgebraicTubeLensMeshOptions, spans *multiSpanMeshWork, affine bool, provided *CertifiedAlgebraicTubeLensBoundary, fill *tubeRegionFillSeed) (CertifiedAlgebraicTubeLensMesh, error) {
	var empty CertifiedAlgebraicTubeLensMesh
	if ctx == nil || l.graphs[0] == nil || !finite(o.Interior.Deflection) || o.Interior.Deflection <= 0 {
		return empty, geom.ErrInvalid
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
	for _, g := range l.graphs {
		rows, e := g.InverseCoefficients()
		if e != nil {
			return empty, e
		}
		for i := 0; i < 2; i++ {
			for j := 0; j < 3; j++ {
				want := int64(0)
				if i == j {
					want = 1
				}
				if !affine && rows[i][j].Cmp(big.NewRat(want, 1)) != 0 {
					return empty, geom.ErrConvergence
				}
			}
		}
	}
	// Boundary display uncertainty must not routinely dominate the inward
	// chord sagitta. This is a conditioning choice, not an embedding premise:
	// exact interior height and final collision checks still decide acceptance.
	if o.Boundary.Run.Display.Point.Tolerance == 0 {
		tol := o.Boundary.Run.Display.Deflection * 1e-6
		if tol > 0 {
			o.Boundary.Run.Display.Point.Tolerance = tol
		}
	}
	var boundary CertifiedAlgebraicTubeLensBoundary
	var e error
	if fill != nil {
		// Source and holed polygon were proved together by the private caller.
	} else if provided != nil {
		boundary = *provided
	} else {
		boundary, e = l.TessellateBoundaryCertified(ctx, o.Boundary)
		if e != nil {
			return empty, fmt.Errorf("shared boundary: %w", e)
		}
	}
	out := CertifiedAlgebraicTubeLensMesh{Boundary: boundary, BoundaryVertices: map[int]int{}, SourceImageSquaredDistance: new(big.Rat)}

	var ring []int
	delta := new(big.Rat)
	seed := CertifiedAffineQuadraticTrimMesh{Region: AffineQuadraticTrimRegionArea{FillSquaredDistance: new(big.Rat)}}
	display := map[int]geom.Vec3{}
	modelBoundary := boundary.Run.Vertices
	if fill != nil {
		seed = fill.seed
		modelBoundary = fill.vertices
		delta.Set(fill.squared)
		out.Predicates = fill.predicates
		if len(modelBoundary) > o.MaxSeedVertices || out.Predicates > o.MaxPredicates {
			return empty, geom.ErrConvergence
		}
		for i, p := range modelBoundary {
			ring = append(ring, i)
			display[i] = p
		}
	} else {
		for i, path := range boundary.Run.PieceVertices {
			if len(path) < 2 {
				return empty, geom.ErrInvalid
			}
			if i > 0 && ring[len(ring)-1] != path[0] {
				return empty, geom.ErrInvalid
			}
			if i == 0 {
				ring = append(ring, path[0])
			}
			ring = append(ring, path[1:]...)
			for _, q := range boundary.Run.SquaredErrors[i] {
				if q.Cmp(delta) > 0 {
					delta.Set(q)
				}
			}
		}
		if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
			return empty, geom.ErrInvalid
		}
		ring = ring[:len(ring)-1]
		if len(ring) > o.MaxSeedVertices {
			return empty, geom.ErrConvergence
		}
		local := make([]int, len(ring))
		for i, id := range ring {
			p := modelBoundary[id]
			seed.ExactUV = append(seed.ExactUV, [2]*big.Rat{ratInset(p.X), ratInset(p.Y)})
			display[i] = p
			local[i] = i
		}
		proof := partitionFillProof{ctx: ctx, limit: o.MaxPredicates, points: seed.ExactUV}
		if e := proof.simple(local); e != nil {
			return empty, e
		}
		area := proof.area(local)
		if area.Sign() == 0 {
			return empty, geom.ErrDegenerate
		}
		if area.Sign() < 0 {
			for i, j := 0, len(local)-1; i < j; i, j = i+1, j-1 {
				local[i], local[j] = local[j], local[i]
			}
			area.Neg(area)
		}
		seed.BoundaryLoops = [][]int{local}
		seed.Mesh.Triangles, e = proof.triangulate(local)
		if e != nil {
			return empty, e
		}
		chain := map[[2]int]int{}
		sum := new(big.Rat)
		for i, id := range local {
			partitionChainAdd(chain, id, local[(i+1)%len(local)], -1)
		}
		for _, tri := range seed.Mesh.Triangles {
			sum.Add(sum, proof.area(tri[:]))
			for i, id := range tri {
				partitionChainAdd(chain, id, tri[(i+1)%3], 1)
			}
		}
		if len(chain) != 0 || sum.Cmp(area) != 0 {
			return empty, geom.ErrConvergence
		}
		out.Predicates = proof.work

	}
	type faceWork struct {
		patch       *geom.SurfacePatch
		seed        CertifiedAffineQuadraticTrimMesh
		l2, inverse *big.Rat
	}
	var work [2]faceWork
	var refinedFaces [2]CertifiedCurvedQuadraticTrimMesh
	for face := 0; face < 2; face++ {
		patch, e := geom.NewSurfacePatch(l.faces[face].Support, o.Boundary.Run.Partition.Patches)
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

		faceSeed := seed
		inverseBound := big.NewRat(1, 1)
		if affine {
			rows, e := l.graphs[face].InverseCoefficients()
			if e != nil {
				return empty, e
			}
			faceSeed.ExactUV = make([][2]*big.Rat, len(seed.ExactUV))
			for i, xy := range seed.ExactUV {
				for axis := 0; axis < 2; axis++ {
					faceSeed.ExactUV[i][axis] = new(big.Rat).Add(new(big.Rat).Set(rows[axis][2]), new(big.Rat).Add(new(big.Rat).Mul(rows[axis][0], xy[0]), new(big.Rat).Mul(rows[axis][1], xy[1])))
				}
			}
			// ||A||_2² <= ||A||_1 ||A||_infinity for the exact chart inverse.
			rowNorm, colNorm := new(big.Rat), new(big.Rat)
			for i := 0; i < 2; i++ {
				row, col := new(big.Rat), new(big.Rat)
				for j := 0; j < 2; j++ {
					row.Add(row, new(big.Rat).Abs(rows[i][j]))
					col.Add(col, new(big.Rat).Abs(rows[j][i]))
				}
				if row.Cmp(rowNorm) > 0 {
					rowNorm.Set(row)
				}
				if col.Cmp(colNorm) > 0 {
					colNorm.Set(col)
				}
			}
			inverseBound.Mul(rowNorm, colNorm)
		}

		work[face] = faceWork{patch, faceSeed, l2, inverseBound}
	}
	refine := func(face, minimum int) error {
		options := o.Interior
		options.MaxTriangleChecks -= out.TriangleChecks
		if options.MaxTriangleChecks < 1 || len(seed.Mesh.Triangles) > options.MaxTriangles {
			return geom.ErrConvergence
		}
		w := work[face]
		refined, e := tessellateCertifiedTrimBoundaryDisplayMode(ctx, w.patch, w.seed, options, minimum, spans, w.l2, true, display)
		if e != nil {
			return fmt.Errorf("face %d residual: %w", face, e)
		}
		out.TriangleChecks += refined.TriangleChecks
		refinedFaces[face] = refined
		return nil
	}
	for face := 0; face < 2; face++ {
		if e := refine(face, 2); e != nil {
			return empty, e
		}
	}
	// Identical UV combinatorics make the common graph bend interpolate on the
	// same triangles. Independent adaptive levels can create crossing displays
	// near a crease even when each separate Hausdorff certificate is valid.
	for refinedFaces[0].InteriorRefinementPasses != refinedFaces[1].InteriorRefinementPasses {
		low, high := 0, 1
		if refinedFaces[0].InteriorRefinementPasses > refinedFaces[1].InteriorRefinementPasses {
			low, high = 1, 0
		}
		passes := refinedFaces[high].InteriorRefinementPasses
		if passes > 7 {
			return empty, geom.ErrConvergence
		}
		if e := refine(low, 1<<passes); e != nil {
			return empty, e
		}
	}
	if len(refinedFaces[0].Mesh.Triangles) != len(refinedFaces[1].Mesh.Triangles) {
		return empty, geom.ErrConvergence
	}
	for i, tri := range refinedFaces[0].Mesh.Triangles {
		if tri != refinedFaces[1].Mesh.Triangles[i] {
			return empty, geom.ErrConvergence
		}
	}
	for i, p := range refinedFaces[0].Mesh.Vertices {
		q := refinedFaces[1].Mesh.Vertices[i]
		if p.X != q.X || p.Y != q.Y {
			return empty, fmt.Errorf("nonmatching projected node %d: %w", i, geom.ErrConvergence)
		}
		if i >= len(ring) {
			gap := p.Z - q.Z
			if l.upper == 1 {
				gap = -gap
			}
			if gap <= 0 {
				return empty, fmt.Errorf("nonpositive interior gap at %v: %w", p, geom.ErrConvergence)
			}
		}
	}
	for face := 0; face < 2; face++ {
		refined := refinedFaces[face]
		out.FaceRefinementPasses[face] = refined.InteriorRefinementPasses
		out.FaceResidualSquared[face] = refined.SurfaceSquaredDistance
		distance := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(work[face].l2, work[face].inverse), delta), refined.SurfaceSquaredDistance))
		if distance.Cmp(out.SourceImageSquaredDistance) > 0 {
			out.SourceImageSquaredDistance.Set(distance)
		}
		remap := make([]int, len(refined.Mesh.Vertices))
		for i, p := range refined.Mesh.Vertices {
			if i < len(ring) {
				id := ring[i]
				if p != modelBoundary[id] {
					return empty, geom.ErrConvergence
				}
				if old, ok := out.BoundaryVertices[id]; ok {
					remap[i] = old
					continue
				}
				out.BoundaryVertices[id] = len(out.Mesh.Vertices)
			}
			remap[i] = len(out.Mesh.Vertices)
			out.Mesh.Vertices = append(out.Mesh.Vertices, p)
		}
		for _, tri := range refined.Mesh.Triangles {
			q := [3]int{remap[tri[0]], remap[tri[1]], remap[tri[2]]}
			if face != l.upper {
				q[1], q[2] = q[2], q[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, q)
			out.OriginalFace = append(out.OriginalFace, face)
		}
		if len(out.Mesh.Triangles) > o.Interior.MaxTriangles {
			return empty, geom.ErrConvergence
		}
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
		for i, a := range tri {
			b := tri[(i+1)%3]
			adjacency[a] = append(adjacency[a], b)
			adjacency[b] = append(adjacency[b], a)
		}
	}
	seen := make([]bool, len(ids))
	seen[0] = true
	stack := []int{0}
	count := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return empty, e
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
		if e := ctx.Err(); e != nil {
			return empty, e
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
		return empty, fmt.Errorf("display collision (%d triangles, %d pairs, %d contacts): %w", len(out.Mesh.Triangles), report.BroadPhasePairs, len(report.Contacts), e)
	}
	if !report.Complete {
		return empty, geom.ErrConvergence
	}
	if len(report.Contacts) != 0 {
		return empty, fmt.Errorf("display contact (passes %v, first %+v): %w", out.FaceRefinementPasses, report.Contacts[0], geom.ErrConvergence)
	}
	out.Collision = report
	if spans != nil {
		out.SpanCellChecks, out.SpanPieceChecks = spans.cells, spans.pieces
		out.ResidualOperations = spans.operations
	}
	return out, nil
}

// TessellateAffineClosedCertified extends the single-span path to independently
// invertible affine XY chart projections. The common model-XY polygon is mapped
// to each exact UV chart; its certified fill distance is transported by the
// exact inverse-Jacobian norm bound before the chart Lipschitz bound. Reversed
// charts retain shared model vertex IDs and outward physical orientation.
func (l AlgebraicTubeLens) TessellateAffineClosedCertified(ctx context.Context, o AlgebraicTubeLensMeshOptions) (CertifiedAlgebraicTubeLensMesh, error) {
	return l.tessellateClosedTubeLens(ctx, o, nil, true, nil, nil)
}
