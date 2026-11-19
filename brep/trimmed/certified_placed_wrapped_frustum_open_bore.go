//go:build research

package trimmed

import (
	"context"

	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedPlacedWrappedFrustumOpenBore keeps the original local proof and
// its exact world image. The same vertex, triangle and owner indices identify
// both meshes. Angular and two-sided distance bounds transfer by isometry.
type CertifiedPlacedWrappedFrustumOpenBore struct {
	Local     CertifiedWrappedFrustumOpenBoreNormals
	Mesh      mesh.Mesh
	Placement geom.Affine
}

// TessellatePlacedWrappedFrustumOpenBoreCertified preserves the exact local
// shell, applies a proper signed-coordinate rigid placement, and proves that
// every world display coordinate is representable without rounding.
func TessellatePlacedWrappedFrustumOpenBoreCertified(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	pose placement.Rigid, angle float64, o CertifiedWrappedFrustumMeshOptions) (CertifiedPlacedWrappedFrustumOpenBore, error) {
	var empty CertifiedPlacedWrappedFrustumOpenBore
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	matrix, e := pose.Matrix()
	if e != nil {
		return empty, e
	}
	local, e := TessellateWrappedFrustumOpenBoreNormalsCertified(ctx, outer, inner, angle, o)
	if e != nil {
		return empty, e
	}
	world, e := placeCertifiedMeshExactly(ctx, local.Mesh, matrix)
	if e != nil {
		return empty, e
	}
	return CertifiedPlacedWrappedFrustumOpenBore{Local: local, Mesh: world, Placement: matrix}, nil
}
