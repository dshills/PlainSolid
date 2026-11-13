package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifiedPairedBezierGraphMesh retains two independently certified face
// displays. FaceB triangle/loop winding is reversed. SharedVertices lists one
// A/B index pair per upper-chain sample, ordered in increasing source parameter;
// A's boundary traverses it backwards, B's forwards. No other vertices are
// merged, and no shell, fan, nonintersection or injectivity claim is made.
// SquaredDistance bounds each source face image against its own display mesh.
type CertifiedPairedBezierGraphMesh struct {
	FaceA, FaceB    CertifiedCurvedBezierGraphChain
	Agreement       []geom.SurfaceCurveAlgebraicAgreement
	SharedVertices  [][2]int
	SquaredDistance *big.Rat
}

// TessellatePairedBezierGraphChainsCertified proves exact same-parameter model
// identity for corresponding increasing graph trims on independent charts.
// It requires identical exact adaptive parameter partitions, then recertifies
// B with A's bit-equal displayed seam points. Unsupported partitions reject
// atomically, never weld by proximity. Options apply independently to two face
// proofs, one B recertification, and each (at most64) seam agreement proof.
func TessellatePairedBezierGraphChainsCertified(ctx context.Context, a *geom.SurfacePatch, qa []ExactBezierUV, baseA *big.Rat, b *geom.SurfacePatch, qb []ExactBezierUV, baseB *big.Rat, o CurvedBezierGraphChainOptions) (CertifiedPairedBezierGraphMesh, error) {
	var empty CertifiedPairedBezierGraphMesh
	if ctx == nil || a == nil || b == nil || baseA == nil || baseB == nil || len(qa) == 0 || len(qa) > 64 || len(qa) != len(qb) {
		return empty, geom.ErrInvalid
	}
	proof := make([]geom.SurfaceCurveAlgebraicAgreement, len(qa))
	for i := range qa {
		ca, e := qa[i].descriptor(ctx, o.Boundary.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		cb, e := qb[i].descriptor(ctx, o.Boundary.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		proof[i], e = geom.CompareSurfaceCurveCompositions(ctx, a, ca, b, cb, o.Boundary.Composition)
		if e != nil {
			return empty, e
		}
		if !proof[i].Complete || !proof[i].Equal {
			return empty, geom.ErrConvergence
		}
	}
	left, e := TessellateCurvedBezierGraphChainCertified(ctx, a, qa, baseA, o)
	if e != nil {
		return empty, e
	}
	right, e := TessellateCurvedBezierGraphChainCertified(ctx, b, qb, baseB, o)
	if e != nil {
		return empty, e
	}
	r := CertifiedPairedBezierGraphMesh{FaceA: left, Agreement: proof}
	overrides := map[int]geom.Vec3{}
	seen := map[int]int{}
	for i := range qa {
		va, vb := left.GraphChain.Pieces[i].Curve.Vertices, right.GraphChain.Pieces[i].Curve.Vertices
		if len(va) != len(vb) {
			return empty, geom.ErrConvergence
		}
		for j := range va {
			if va[j].Parameter.Lower.Cmp(vb[j].Parameter.Lower) != 0 || va[j].Parameter.Upper.Cmp(vb[j].Parameter.Upper) != 0 {
				return empty, geom.ErrConvergence
			}
			ia, ib := left.GraphChain.PieceVertices[i][j], right.GraphChain.PieceVertices[i][j]
			for axis := 0; axis < 3; axis++ {
				if left.ExactPoints[ia][axis].Cmp(right.ExactPoints[ib][axis]) != 0 {
					return empty, geom.ErrConvergence
				}
			}
			if old, ok := seen[ia]; ok {
				if old != ib {
					return empty, geom.ErrConvergence
				}
				continue
			}
			seen[ia] = ib
			overrides[ib] = left.Mesh.Vertices[ia]
			r.SharedVertices = append(r.SharedVertices, [2]int{ia, ib})
		}
	}
	m := o.Interior
	if m.MaxSubdivisions == 0 {
		m.MaxSubdivisions = 32
	}
	if m.MaxTriangleChecks == 0 {
		m.MaxTriangleChecks = 100000
	}
	if m.MaxTriangles == 0 {
		m.MaxTriangles = 100000
	}
	fresh, e := tessellateCertifiedTrimBoundaryDisplayMode(ctx, b, right.Boundary, m, 1, nil, right.LipschitzSquared, true, overrides)
	if e != nil {
		return empty, e
	}
	right.CertifiedCurvedQuadraticTrimMesh = fresh
	for i := range right.Mesh.Triangles {
		right.Mesh.Triangles[i][1], right.Mesh.Triangles[i][2] = right.Mesh.Triangles[i][2], right.Mesh.Triangles[i][1]
	}
	right.BoundaryLoops = append([][]int(nil), right.BoundaryLoops...)
	for i, loop := range right.BoundaryLoops {
		copyLoop := append([]int(nil), loop...)
		for j, k := 0, len(copyLoop)-1; j < k; j, k = j+1, k-1 {
			copyLoop[j], copyLoop[k] = copyLoop[k], copyLoop[j]
		}
		right.BoundaryLoops[i] = copyLoop
	}
	for _, pair := range r.SharedVertices {
		if left.Mesh.Vertices[pair[0]] != right.Mesh.Vertices[pair[1]] {
			return empty, geom.ErrConvergence
		}
	}
	// Independently audit directed boundary incidence after the fixed-boundary
	// refinements. Reversal is topological, not inferred from point proximity.
	counts := func(triangles [][3]int) map[[2]int]int {
		c := map[[2]int]int{}
		for _, tri := range triangles {
			for j, a := range tri {
				c[[2]int{a, tri[(j+1)%3]}]++
			}
		}
		return c
	}
	ea, eb := counts(left.Mesh.Triangles), counts(right.Mesh.Triangles)
	for i := 1; i < len(r.SharedVertices); i++ {
		p, q := r.SharedVertices[i-1], r.SharedVertices[i]
		if ea[[2]int{q[0], p[0]}] != 1 || ea[[2]int{p[0], q[0]}] != 0 || eb[[2]int{p[1], q[1]}] != 1 || eb[[2]int{q[1], p[1]}] != 0 {
			return empty, geom.ErrConvergence
		}
	}
	r.FaceB = right
	r.SquaredDistance = new(big.Rat).Set(left.FillSquaredDistance)
	if right.FillSquaredDistance.Cmp(r.SquaredDistance) > 0 {
		r.SquaredDistance.Set(right.FillSquaredDistance)
	}
	return r, nil
}
