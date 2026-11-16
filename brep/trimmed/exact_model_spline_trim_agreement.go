package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Certify the same entire clock for a standalone exact model spline and
// an original quadratic pcurve. Endpoint or sampled equality is insufficient.
func compareExactModelSplineQuadraticUse(ctx context.Context, m parametric.Edge, surface geom.Surface,
	reversed bool, q ExactQuadraticUV, o edgecheck.Options) (bool, error) {
	base, pose := parametric.SurfaceMapping(surface)
	if !supportsExactSplinePatch(base) || q.control[0][0] == nil {
		return false, nil
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	model, reverseModel, err := exactEdgeSplineWithOptions(m, limits)
	if err != nil || model == nil {
		return false, err
	}
	uv, err := exactQuadraticTrimParameterCurve(ctx, q.HomogeneousControls(), limits)
	if err != nil {
		return false, err
	}
	patch, err := exactSplineSupportPatch(base, limits)
	if err != nil {
		return false, err
	}
	if pose != geom.Identity() {
		patch, err = patch.Transformed(pose)
		if err != nil {
			return false, err
		}
	}
	result, err := patch.ComposeParameterCurvePieces(ctx, uv, geom.SurfaceCurveCompositionOptions{
		SurfacePatchOptions: limits, MaxPieces: o.MaxIntervals})
	if err != nil {
		return false, err
	}
	if !result.Complete || len(result.Pieces) != 1 || result.Pieces[0].Start.Sign() != 0 ||
		result.Pieces[0].End.Cmp(big.NewRat(1, 1)) != 0 {
		return false, geom.ErrConvergence
	}
	equality, err := geom.CompareExactCurvesContext(ctx, model, result.Pieces[0].Curve, geom.ExactCurveComparisonOptions{
		ReverseB: reverseModel != reversed, NormalizeDomains: true, MaxOperations: o.MaxWork,
		MaxCoefficientBits: o.MaxBits, MaxSpans: o.MaxIntervals})
	return err == nil && equality.Complete && equality.Equal, err
}
