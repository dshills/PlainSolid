package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// CertifiedWrappedFrustumOpenBoreNormals bounds the tangent squared of the
// greatest angle between each display triangle and its original material
// normals. The inward wall and floor are oriented into the bore.
type CertifiedWrappedFrustumOpenBoreNormals struct {
	CertifiedWrappedFrustumOpenBoreMesh
	SquaredNormalTangent *big.Rat
}

// TessellateWrappedFrustumOpenBoreNormalsCertified checks continuous material
// normals on both original circular walls and all three plane pieces. The
// requested angle is a sufficient upper bound in radians (0 < angle <= 1).
// If the distance-driven grid cannot satisfy it, the call fails atomically.
func TessellateWrappedFrustumOpenBoreNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	angle float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	return tessellateParallelFrustumOpenBoreNormals(ctx, outer, inner, angle, o, true)
}

// TessellateWrappedEccentricFrustumOpenBoreNormalsCertified uses the original
// axis and taper of each separately certified circular wall.
func TessellateWrappedEccentricFrustumOpenBoreNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	angle float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	return tessellateParallelFrustumOpenBoreNormals(ctx, outer, inner, angle, o, false)
}

func tessellateParallelFrustumOpenBoreNormals(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	angle float64, o CertifiedWrappedFrustumMeshOptions, coaxial bool) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	var empty CertifiedWrappedFrustumOpenBoreNormals
	if ctx == nil || !finite(angle) || angle <= 0 || angle > 1 {
		return empty, geom.ErrInvalid
	}
	m, e := tessellateParallelFrustumOpenBore(ctx, outer, inner, o, coaxial)
	if e != nil {
		return empty, e
	}
	walls := [2]*geom.WrappedSurfacePatch{outer, inner}
	var proof [2]geom.WrappedFrustumEmbedding
	for i, w := range walls {
		proof[i], e = geom.CertifyWrappedFrustumEmbedding(ctx, w, o.Embedding)
		if e != nil {
			return empty, e
		}
	}
	openUpper := proof[0].UpperZ.Cmp(proof[1].UpperZ) == 0
	limit := ratInset(angle)
	limit.Mul(limit, limit)
	maximum := new(big.Rat)
	var next [2]int
	for i, tri := range m.Mesh.Triangles {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		a, b, c := exactBoundPoint(m.Mesh.Vertices[tri[0]]), exactBoundPoint(m.Mesh.Vertices[tri[1]]), exactBoundPoint(m.Mesh.Vertices[tri[2]])
		normal := ddCross(ddSub(b, a), ddSub(c, a))
		var bound *big.Rat
		owner := m.OriginalFace[i]
		switch owner {
		case 0, 3:
			wall := owner / 3
			if next[wall] >= len(m.Walls[wall].UVTriangles) {
				return empty, geom.ErrConvergence
			}
			uv := m.Walls[wall].UVTriangles[next[wall]]
			next[wall]++
			lo := min(uv[0].X, uv[1].X, uv[2].X)
			hi := max(uv[0].X, uv[1].X, uv[2].X)
			bound, e = circularFrustumFacetNormalBound(ctx, walls[wall].FundamentalPatch(), proof[wall],
				ratInset(lo), ratInset(hi), normal, wall == 1, o.Patch.Evaluation)
			if e != nil {
				return empty, e
			}
		case 1, 2, 4:
			sign := int64(-1)
			if owner == 2 || owner == 4 && openUpper {
				sign = 1
			}
			var ok bool
			bound, ok = ddTanSquared(normal, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(sign, 1)})
			if !ok {
				return empty, geom.ErrConvergence
			}
		default:
			return empty, geom.ErrConvergence
		}
		if bound.Cmp(limit) > 0 {
			return empty, geom.ErrConvergence
		}
		if bound.Cmp(maximum) > 0 {
			maximum.Set(bound)
		}
	}
	for i, n := range next {
		if n != len(m.Walls[i].UVTriangles) {
			return empty, geom.ErrConvergence
		}
	}
	return CertifiedWrappedFrustumOpenBoreNormals{m, maximum}, nil
}
