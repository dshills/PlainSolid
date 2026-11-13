package offset

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WrappedCircularToroid offsets the canonical, exactly circular rational
// toroid by a Euclidean normal distance, preserving its spline support and
// seam topology. The ideal geometry is exact; the solid constructor rejects
// a resulting seam vertex that cannot be represented exactly. General
// offsets, collapsed tubes and topology transitions are outside this API.
func WrappedCircularToroid(ctx context.Context, support *geom.WrappedSurfacePatch, distance, tolerance float64, o geom.SurfacePatchOptions) (trimmed.Solid, geom.WrappedToroidEmbedding, error) {
	w, proof, e := geom.OffsetWrappedCircularToroid(ctx, support, distance, o)
	if e != nil {
		return trimmed.Solid{}, proof, e
	}
	return trimmed.NewWrappedToroid(ctx, w, tolerance, o)
}

// WrappedCircularToroidWall forms material between two strictly ordered
// normal offsets of the same circular wrapped toroid. The inner boundary
// is a reversed cavity. Each shell preserves its rational support and
// both seams; offsets that collapse either tube or close its hole fail.
func WrappedCircularToroidWall(ctx context.Context, support *geom.WrappedSurfacePatch, innerOffset, outerOffset, tolerance float64,
	o geom.SurfacePatchOptions) (trimmed.Solid, error) {
	if innerOffset >= outerOffset {
		return trimmed.Solid{}, geom.ErrInvalid
	}
	outer, _, e := WrappedCircularToroid(ctx, support, outerOffset, tolerance, o)
	if e != nil {
		return trimmed.Solid{}, e
	}
	inner, _, e := WrappedCircularToroid(ctx, support, innerOffset, tolerance, o)
	if e != nil {
		return trimmed.Solid{}, e
	}
	out, _, e := trimmed.CombineNestedSplineComponents(ctx, outer, inner, parametric.Difference,
		trimmed.SplineShellBooleanOptions{Embedding: trimmed.SplineRectangleEmbeddingOptions{Patches: o}})
	return out, e
}
