package offset

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ErodeWrappedCircularFrustum builds the capped exact ball erosion of the
// canonical circular frustum. Its native vertices must be representable.
func ErodeWrappedCircularFrustum(ctx context.Context, support *geom.WrappedSurfacePatch, distance, tolerance float64,
	o geom.SurfacePatchOptions) (trimmed.Solid, geom.WrappedFrustumEmbedding, error) {
	w, _, e := geom.ErodeWrappedCircularFrustum(ctx, support, distance, o)
	if e != nil {
		return trimmed.Solid{}, geom.WrappedFrustumEmbedding{}, e
	}
	return trimmed.NewWrappedFrustum(ctx, w, tolerance, o)
}

// WrappedCircularFrustum offsets the circular lateral face by a constant
// Euclidean normal distance, then caps the shifted endpoints with horizontal
// disks. The cap positions follow the shifted rim heights; this is not a
// constant-distance offset of the complete original capped boundary.
func WrappedCircularFrustum(ctx context.Context, support *geom.WrappedSurfacePatch, distance, tolerance float64,
	o geom.SurfacePatchOptions) (trimmed.Solid, geom.WrappedFrustumNormalOffset, error) {
	p, e := geom.OffsetWrappedCircularFrustum(ctx, support, distance, o)
	if e != nil {
		return trimmed.Solid{}, p, e
	}
	s, _, e := trimmed.NewWrappedFrustum(ctx, p.Support, tolerance, o)
	return s, p, e
}
