//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifiedWrappedFrustumNormals retains the closed display and an exact bound
// on tan² of the angle between each actual display facet and every source normal
// under that triangle's certified UV correspondence (or radial cap map).
type CertifiedWrappedFrustumNormals struct {
	CertifiedWrappedFrustumMesh
	SquaredNormalTangent *big.Rat
}

// TessellateWrappedCircularFrustumNormalsCertified adds a continuous angular
// certificate to the existing closed display. The angle is in radians, in (0,1].
// It accepts only the canonical Z-axis exactly circular wrapped frustum proof;
// noncircular profiles and transformed/noncanonical charts are rejected. The
// existing distance-driven grid is unchanged: an insufficient grid fails
// atomically, rather than restarting its proof budgets. A smaller deflection or
// an explicitly larger existing grid budget may permit a subsequent call.
// The additional stage performs at most two bounded exact surface evaluations
// per lateral triangle and one cap-normal check per cap triangle, all bounded
// by MaxTriangles; source evaluation keeps Patch.Evaluation limits unchanged.
//
// On one monotone quadrant, the outward normal is proportional to
// (height*cos(theta),height*sin(theta),-radiusChange). Its norm is constant.
// Positive radial facet dot products at both ends prove that its varying dot
// product is positive and concave on the whole arc, hence its minimum occurs
// at an endpoint. The fixed axial term does not change that minimum. Exact
// endpoint angle bounds therefore cover the entire source triangle, including
// the effect of rounding on the actual facet normal. Caps have constant normals.
func TessellateWrappedCircularFrustumNormalsCertified(ctx context.Context, w *geom.WrappedSurfacePatch, angle float64, o CertifiedWrappedFrustumMeshOptions) (out CertifiedWrappedFrustumNormals, err error) {
	if ctx == nil || !finite(angle) || angle <= 0 || angle > 1 {
		return out, geom.ErrInvalid
	}
	proof, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o.Embedding)
	if e != nil {
		return out, e
	}
	if !proof.Complete || !proof.Circular {
		return out, geom.ErrConvergence
	}
	m, e := TessellateWrappedFrustumClosedCertified(ctx, w, o)
	if e != nil {
		return out, e
	}
	limit := ratInset(angle)
	limit.Mul(limit, limit)
	maximum := new(big.Rat)
	p := w.FundamentalPatch()
	for i, tri := range m.Mesh.Triangles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		a, b, c := exactBoundPoint(m.Mesh.Vertices[tri[0]]), exactBoundPoint(m.Mesh.Vertices[tri[1]]), exactBoundPoint(m.Mesh.Vertices[tri[2]])
		normal := ddCross(ddSub(b, a), ddSub(c, a))
		check := func(source [3]*big.Rat) bool {
			q, ok := ddTanSquared(normal, source)
			if !ok || q.Cmp(limit) > 0 {
				return false
			}
			if q.Cmp(maximum) > 0 {
				maximum.Set(q)
			}
			return true
		}
		if m.OriginalFace[i] != 0 {
			z := int64(-1)
			if m.OriginalFace[i] == 2 {
				z = 1
			}
			if !check([3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(z, 1)}) {
				return out, geom.ErrConvergence
			}
			continue
		}
		// Closed mesh preserves the lateral triangle prefix and its original UVs.
		if i >= len(m.Lateral.UVTriangles) {
			return out, geom.ErrConvergence
		}
		uv := m.Lateral.UVTriangles[i]
		lo, hi := uv[0].X, uv[0].X
		for _, q := range uv[1:] {
			lo = min(lo, q.X)
			hi = max(hi, q.X)
		}
		q, e := circularFrustumFacetNormalBound(ctx, p, proof, ratInset(lo), ratInset(hi), normal, false, o.Patch.Evaluation)
		if e != nil {
			return out, e
		}
		if q.Cmp(limit) > 0 {
			return out, geom.ErrConvergence
		}
		if q.Cmp(maximum) > 0 {
			maximum.Set(q)
		}
	}
	return CertifiedWrappedFrustumNormals{CertifiedWrappedFrustumMesh: m, SquaredNormalTangent: maximum}, nil
}
