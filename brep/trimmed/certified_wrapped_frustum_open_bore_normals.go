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
	return tessellateParallelFrustumOpenBoreNormals(ctx, outer, inner, angle, o, true, 1)
}

// TessellateWrappedEccentricFrustumOpenBoreNormalsCertified uses the original
// axis and taper of each separately certified circular wall.
func TessellateWrappedEccentricFrustumOpenBoreNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	angle float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	return tessellateParallelFrustumOpenBoreNormals(ctx, outer, inner, angle, o, false, 1)
}

// TessellateWrappedEccentricFrustumThroughBoreNormalsCertified certifies
// continuous material normals on both original walls and both annular caps.
func TessellateWrappedEccentricFrustumThroughBoreNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	angle float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	return tessellateParallelFrustumOpenBoreNormals(ctx, outer, inner, angle, o, false, 2)
}

// TessellateWrappedEccentricFrustumCavityNormalsCertified also certifies
// continuous material normals on the enclosed shell, including both inward
// planar caps.
func TessellateWrappedEccentricFrustumCavityNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	angle float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	return tessellateParallelFrustumOpenBoreNormals(ctx, outer, inner, angle, o, false, 0)
}

// TessellateCanonicalWrappedFrustumThroughBoreNormalsCertified also restores
// the receiver's native face IDs on the material-normal certificate.
func (s Solid) TessellateCanonicalWrappedFrustumThroughBoreNormalsCertified(ctx context.Context, angle float64,
	o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	return s.tessellateCanonicalWrappedFrustumBoreNormalsCertified(ctx, angle, o, "tube", 2)
}

// TessellateCanonicalWrappedFrustumCavityNormalsCertified preserves native
// face IDs when certifying the continuous material normals of a cavity.
func (s Solid) TessellateCanonicalWrappedFrustumCavityNormalsCertified(ctx context.Context, angle float64,
	o CertifiedWrappedFrustumMeshOptions) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	return s.tessellateCanonicalWrappedFrustumBoreNormalsCertified(ctx, angle, o, "cavity", 0)
}

func (s Solid) tessellateCanonicalWrappedFrustumBoreNormalsCertified(ctx context.Context, angle float64,
	o CertifiedWrappedFrustumMeshOptions, kind string, openings int) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	var empty CertifiedWrappedFrustumOpenBoreNormals
	p, e := s.ValidateCanonicalWrappedFrustumBore(ctx, o.Embedding)
	if e != nil {
		return empty, e
	}
	if p.Kind != kind {
		return empty, geom.ErrConvergence
	}
	m, e := tessellateParallelFrustumOpenBoreNormals(ctx, p.Outer, p.Inner, angle, o, false, openings)
	if e != nil {
		return empty, e
	}
	ids := canonicalFrustumBoreLineFaceIDs(p)
	for i, id := range m.OriginalFace {
		m.OriginalFace[i] = ids[id]
	}
	return m, nil
}

func tessellateParallelFrustumOpenBoreNormals(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	angle float64, o CertifiedWrappedFrustumMeshOptions, coaxial bool, openings int) (CertifiedWrappedFrustumOpenBoreNormals, error) {
	var empty CertifiedWrappedFrustumOpenBoreNormals
	if ctx == nil || !finite(angle) || angle <= 0 || angle > 1 {
		return empty, geom.ErrInvalid
	}
	m, e := tessellateParallelFrustumOpenBore(ctx, outer, inner, o, coaxial, openings)
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
		case 1, 2, 4, 5:
			sign := int64(-1)
			if owner == 2 || owner == 4 && (openUpper || openings == 0) {
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
