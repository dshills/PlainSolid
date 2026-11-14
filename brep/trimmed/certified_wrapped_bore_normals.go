package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifiedWrappedBoreNormals adds continuous material-oriented facet-normal
// bounds to both walls and caps. Inner normals point into the bore.
type CertifiedWrappedBoreNormals struct {
	CertifiedWrappedBoreMesh
	SquaredNormalTangent *big.Rat
}

// TessellateWrappedCircularBoreNormalsCertified requires the existing canonical
// coaxial bore proof and exactly circular profiles. Angle is in radians (0,1].
// It preserves the existing distance-driven grid and complete contact check;
// insufficient angular resolution fails atomically without resetting budgets.
// At most two exact source evaluations per wall triangle use the unchanged
// Patch.Evaluation limits. MaxTriangles bounds this additional stage. The
// circular normal-cone proof is identical to the single-frustum certificate,
// with reversed incidence for the inner wall; caps use constant axial normals.
func TessellateWrappedCircularBoreNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, angle float64, o CertifiedWrappedFrustumMeshOptions) (out CertifiedWrappedBoreNormals, err error) {
	if ctx == nil || !finite(angle) || angle <= 0 || angle > 1 {
		return out, geom.ErrInvalid
	}
	walls := [2]*geom.WrappedSurfacePatch{outer, inner}
	var proofs [2]geom.WrappedFrustumEmbedding
	for i, w := range walls {
		p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o.Embedding)
		if e != nil {
			return out, e
		}
		if !p.Complete || !p.Circular {
			return out, geom.ErrConvergence
		}
		proofs[i] = p
	}
	m, e := TessellateWrappedBoreClosedCertified(ctx, outer, inner, o)
	if e != nil {
		return out, e
	}
	n := m.Subdivisions
	if n < 1 || n > 128 {
		return out, geom.ErrConvergence
	}
	// Existing builder interleaves outer/inner triangles, two per UV cell,
	// with n axial rows and four circumferential spans of n intervals each.
	wallCount := 16 * n * n
	if len(m.Mesh.Triangles) != wallCount+16*n {
		return out, geom.ErrConvergence
	}
	limit := ratInset(angle)
	limit.Mul(limit, limit)
	maximum := new(big.Rat)
	for i, tri := range m.Mesh.Triangles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		a, b, c := exactBoundPoint(m.Mesh.Vertices[tri[0]]), exactBoundPoint(m.Mesh.Vertices[tri[1]]), exactBoundPoint(m.Mesh.Vertices[tri[2]])
		normal := ddCross(ddSub(b, a), ddSub(c, a))
		var bound *big.Rat
		if i < wallCount {
			owner := i % 2
			if m.OriginalFace[i] != owner {
				return out, geom.ErrConvergence
			}
			column := (i / 2) / (2 * n)
			span, interval := column/n, column%n
			p := proofs[owner]
			lo := ratInset(p.QuarterParameters[span])
			width := new(big.Rat).Sub(ratInset(p.QuarterParameters[span+1]), lo)
			hi := new(big.Rat).Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(interval+1), int64(n))))
			lo.Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(interval), int64(n))))
			bound, e = circularFrustumFacetNormalBound(ctx, walls[owner].FundamentalPatch(), p, lo, hi, normal, owner == 1, o.Patch.Evaluation)
			if e != nil {
				return out, e
			}
		} else {
			owner := m.OriginalFace[i]
			if owner != 2 && owner != 3 {
				return out, geom.ErrConvergence
			}
			sign := int64(-1)
			if owner == 3 {
				sign = 1
			}
			var ok bool
			bound, ok = ddTanSquared(normal, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(sign, 1)})
			if !ok {
				return out, geom.ErrConvergence
			}
		}
		if bound.Cmp(limit) > 0 {
			return out, geom.ErrConvergence
		}
		if bound.Cmp(maximum) > 0 {
			maximum.Set(bound)
		}
	}
	return CertifiedWrappedBoreNormals{CertifiedWrappedBoreMesh: m, SquaredNormalTangent: maximum}, nil
}
