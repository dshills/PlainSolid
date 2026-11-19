//go:build research

package trimmed

import "context"

// SplineSeparatedPlaneCuts contains the source and section ancestry of a
// two-halfspace intersection; its caps may have different outward directions.
type SplineSeparatedPlaneCuts = SplineParallelSlab

// CombineSplineSeparatedPlaneCuts intersects stock with two independently
// certified active halfspaces. Unlike CombineSplineParallelSlab, their planes
// need not be parallel. Both complete cap polygons must be exactly convex and
// strictly inside the other kept halfspace. Cap contact/intersection is rejected.
// Crossing stock supports must be affine; untouched curved supports are retained
// only after both original full-domain halfspace proofs. No original vertex
// events are supported. The single-plane proof and per-reproof limits are those
// of CombineSplinePlaneClip. Failure returns no partial solid or ancestry.
func CombineSplineSeparatedPlaneCuts(ctx context.Context, stock Solid, tools [2]Solid, active [2]int, o SplineRectangleEmbeddingOptions) (SplineSeparatedPlaneCuts, error) {
	return combineSplineSeparatedPlaneCuts(ctx, stock, tools, active, o, false)
}
