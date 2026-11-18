//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedPlacedWrappedBoreNormals separates the original canonical source
// certificate from its world-space display. Local vertex/triangle/source-owner
// indices also index Mesh. Placement is the exact interpretation of the source;
// no rounded replacement control net is constructed.
type CertifiedPlacedWrappedBoreNormals struct {
	Local     CertifiedWrappedBoreNormals
	Mesh      mesh.Mesh
	Placement geom.Affine
}

// TessellatePlacedWrappedCircularBoreNormalsCertified applies an immutable proper
// signed-coordinate rigid placement after independently certifying the canonical
// bore. Every resulting display coordinate must be exactly representable. A
// translation that would round even one vertex is rejected atomically, including
// tiny translations lost at large coordinates. Rotation, translation and source
// evaluation never use a tolerance-based equality.
//
// A determinant-one isometry preserves the Local angular and two-sided distance
// certificates, exact signed display volume, manifold incidence and absence of
// non-incidence contacts. Local remains in canonical coordinates; Mesh is world
// space. Additional work is linear in the existing bounded vertex count, with
// three exact represented-coordinate checks per vertex and cancellation checks.
func TessellatePlacedWrappedCircularBoreNormalsCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, pose placement.Rigid, angle float64, o CertifiedWrappedFrustumMeshOptions) (out CertifiedPlacedWrappedBoreNormals, err error) {
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
	local, e := TessellateWrappedCircularBoreNormalsCertified(ctx, outer, inner, angle, o)
	if e != nil {
		return out, e
	}
	world, e := placeCertifiedMeshExactly(ctx, local.Mesh, matrix)
	if e != nil {
		return out, e
	}
	return CertifiedPlacedWrappedBoreNormals{Local: local, Mesh: world, Placement: matrix}, nil
}
