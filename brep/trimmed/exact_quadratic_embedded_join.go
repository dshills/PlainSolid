package trimmed

import (
	"context"
	"plainsolid/collision"
	"plainsolid/geom"
)

// CertifiedEmbeddedExactQuadraticFaceJoin additionally proves the returned
// display mesh has manifold vertex links and no non-incidence exact contacts.
// This certificate applies to its current caller-owned Mesh, not a modified
// copy or the continuous source surfaces. It does not imply a closed solid.
type CertifiedEmbeddedExactQuadraticFaceJoin struct {
	CertifiedExactQuadraticFaceJoin
	DisplayCollision collision.Report
}

// StitchEmbeddedExactQuadraticFacesCertified keeps the synchronized exact
// boundary cuts fixed during all interior refinement. Non-boundary edges use
// shared exact midpoint IDs and conforming red/green refinement; an all-boundary
// triangle may acquire a centroid. No new chord point is moved onto the seam.
// Subdivisions remains1; InteriorRefinementPasses reports interior iterations.
// Both face-distance certificates and a complete exact BVH contact test must
// succeed. Fixed boundary error, non-incidence contacts or exhausted work
// limits reject with no partial mesh; no tolerance welding is used.
func StitchEmbeddedExactQuadraticFacesCertified(ctx context.Context, a, b ExactQuadraticFaceJoinInput, o ExactQuadraticFaceJoinOptions, contacts collision.Options) (CertifiedEmbeddedExactQuadraticFaceJoin, error) {
	empty := CertifiedEmbeddedExactQuadraticFaceJoin{}
	if ctx == nil || contacts.MaxPairs < 0 || contacts.MaxContacts < 0 {
		return empty, geom.ErrInvalid
	}
	result, e := stitchExactQuadraticFaces(ctx, a, b, o, true)
	if e != nil {
		return empty, e
	}
	vertices := make([]int, len(result.Mesh.Vertices))
	for i := range vertices {
		vertices[i] = i
	}
	if e = validateQuadraticJunctionFans(result.Mesh, vertices); e != nil {
		return empty, e
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, result.Mesh, contacts)
	if e != nil {
		return empty, e
	}
	if !report.Complete || len(report.Contacts) > 0 {
		return empty, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return CertifiedEmbeddedExactQuadraticFaceJoin{CertifiedExactQuadraticFaceJoin: result, DisplayCollision: report}, nil
}
