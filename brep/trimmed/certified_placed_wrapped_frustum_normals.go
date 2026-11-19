//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedPlacedWrappedFrustumNormals retains the canonical certificate and
// a separate world-space mesh with identical vertex/face ancestry indices.
// Placement describes the exact transformed source, not rounded control poles.
type CertifiedPlacedWrappedFrustumNormals struct {
	Local     CertifiedWrappedFrustumNormals
	Mesh      mesh.Mesh
	Placement geom.Affine
}

// TessellatePlacedWrappedCircularFrustumNormalsCertified certifies a circular
// solid cylinder/frustum and places its display by a proper signed-coordinate
// isometry. Every transformed display coordinate must remain exactly binary64;
// otherwise it returns no partial result. Arbitrary-angle rotations and geometry
// re-rounding are outside this API. The Local two-sided position, continuous
// angular, positive volume, manifold and non-incidence proofs transfer exactly
// by isometry. Local coordinates stay canonical; Mesh coordinates are world-space.
// Three exact checks per bounded display vertex are the only additional work;
// all existing geometry, collision and evaluation limits remain unchanged.
func TessellatePlacedWrappedCircularFrustumNormalsCertified(ctx context.Context, w *geom.WrappedSurfacePatch, pose placement.Rigid, angle float64, o CertifiedWrappedFrustumMeshOptions) (out CertifiedPlacedWrappedFrustumNormals, err error) {
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	matrix, e := pose.Matrix()
	if e != nil {
		return out, e
	}
	local, e := TessellateWrappedCircularFrustumNormalsCertified(ctx, w, angle, o)
	if e != nil {
		return out, e
	}
	world, e := placeCertifiedMeshExactly(ctx, local.Mesh, matrix)
	if e != nil {
		return out, e
	}
	return CertifiedPlacedWrappedFrustumNormals{Local: local, Mesh: world, Placement: matrix}, nil
}
