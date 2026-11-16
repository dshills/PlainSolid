package conicalfillet

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// DisplayRecipe retains the authenticated stock and complete rolling-rim
// and through-bore recipe. The neutral decoders publish it only after
// checking the original directed source graph.
type DisplayRecipe struct {
	stock        trimmed.Solid
	lower, upper float64
	center       geom.Vec2
	radius       float64
}

func (r DisplayRecipe) TessellateCertified(ctx context.Context, o trimmed.CertifiedWrappedFrustumMeshOptions) (trimmed.CertifiedFilletedFrustumBoreMesh, error) {
	if ctx == nil || r.radius <= 0 {
		return trimmed.CertifiedFilletedFrustumBoreMesh{}, geom.ErrInvalid
	}
	return trimmed.TessellateFilletedFrustumThroughBoreCertified(ctx, r.stock, r.lower, r.upper, r.center, r.radius, o)
}
