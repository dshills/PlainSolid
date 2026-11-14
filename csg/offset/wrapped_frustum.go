package offset

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedCircularFrustumWall retains the material within thickness of the
// original capped boundary. The closed inward shell is its exact ball
// erosion. A collapsed or unrepresentable inner shell is rejected.
func WrappedCircularFrustumWall(ctx context.Context, support *geom.WrappedSurfacePatch, thickness, tolerance float64,
	o geom.SurfacePatchOptions) (trimmed.Solid, trimmed.SplineShellCombineHistory, error) {
	var empty trimmed.SplineShellCombineHistory
	if !(thickness > 0) {
		return trimmed.Solid{}, empty, geom.ErrInvalid
	}
	outer, proof, e := trimmed.NewWrappedFrustum(ctx, support, tolerance, o)
	if e != nil || !proof.Circular {
		if e == nil {
			e = geom.ErrConvergence
		}
		return trimmed.Solid{}, empty, e
	}
	inner, _, e := ErodeWrappedCircularFrustum(ctx, support, thickness, tolerance, o)
	if e != nil {
		return trimmed.Solid{}, empty, e
	}
	return trimmed.CombineNestedSplineComponents(ctx, outer, inner, parametric.Difference,
		trimmed.SplineShellBooleanOptions{Embedding: trimmed.SplineRectangleEmbeddingOptions{Patches: o}})
}

// WrappedCircularFrustumOpenWall removes one cap and offsets the remaining
// supports inward by thickness. The opening is the original cap plane and
// retains its annular shoulder; the opposite end has a closed inner floor.
func WrappedCircularFrustumOpenWall(ctx context.Context, support *geom.WrappedSurfacePatch, thickness, tolerance float64,
	openUpper bool, o geom.SurfacePatchOptions) (trimmed.Solid, trimmed.WrappedCylinderTubeHistory, error) {
	inner, _, e := geom.InsetWrappedCircularFrustumAtCap(ctx, support, thickness, openUpper, o)
	if e != nil {
		return trimmed.Solid{}, trimmed.WrappedCylinderTubeHistory{}, e
	}
	return trimmed.NewWrappedCircularFrustumBore(ctx, support, inner, tolerance, o)
}

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
