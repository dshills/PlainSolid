package draft

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedFrustum applies separate signed radial changes at the lower and
// upper cap of a canonical rational extrusion or frustum. The returned
// body preserves the profile's rational basis, weights and seam. End
// collapse and unrepresentable topology vertices are rejected.
func WrappedFrustum(ctx context.Context, support *geom.WrappedSurfacePatch, lowerChange, upperChange, tolerance float64, o geom.SurfacePatchOptions) (trimmed.Solid, geom.WrappedFrustumEmbedding, error) {
	w, proof, e := geom.DraftWrappedFrustum(ctx, support, lowerChange, upperChange, o)
	if e != nil {
		return trimmed.Solid{}, proof, e
	}
	return trimmed.NewWrappedFrustum(ctx, w, tolerance, o)
}
