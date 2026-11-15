package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
)

// This search supplies only an attained upper witness. In particular its
// curve lower bound is never a lower bound for the trimmed face interior.
// Unsupported composition leaves the exhaustive face search unchanged.
func composedSplineTrimWitness(ctx context.Context, p *geom.SurfacePatch, use Use, point geom.Vec3, cells int, o SplineBoundaryDistanceOptions) (geom.UVBox, geom.Box, int, bool, error) {
	var curve *geom.ExactSplineCurve
	var err error
	switch q := use.Curve.(type) {
	case ExactBezierUV:
		curve, err = q.descriptor(ctx, o.Patches)
	case SplineUV:
		curve, err = geom.NewExactSplineCurve(q.Curve, o.Patches)
		if err == nil {
			curve, err = curve.Restrict(q.Range.Min, q.Range.Max, o.Patches)
		}
	default:
		return geom.UVBox{}, geom.Box{}, 0, false, nil
	}
	if err != nil {
		return geom.UVBox{}, geom.Box{}, 0, false, err
	}
	model, err := p.ComposeParameterCurve(ctx, curve, o.Patches)
	if errors.Is(err, geom.ErrConvergence) {
		sample, used, hit, e := composedKnotTrimWitness(ctx, p, use, point, cells, o)
		if errors.Is(e, geom.ErrConvergence) {
			return geom.UVBox{}, geom.Box{}, used, false, nil
		}
		return sample.UV, sample.Point, used, hit, e
	}
	if err != nil {
		return geom.UVBox{}, geom.Box{}, 0, false, err
	}
	result, err := geom.BoundPointExactCurveDistance(ctx, point, model, geom.CurveDistanceOptions{
		SquaredTolerance: o.SquaredTolerance, MaxCells: cells, MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits,
	})
	if err != nil && !errors.Is(err, geom.ErrConvergence) {
		return geom.UVBox{}, geom.Box{}, result.VisitedCells, false, err
	}
	if !result.HasWitness {
		return geom.UVBox{}, geom.Box{}, result.VisitedCells, false, nil
	}
	uv, err := curve.EvaluateJet(result.Parameter)
	if err != nil {
		return geom.UVBox{}, geom.Box{}, result.VisitedCells, false, err
	}
	return geom.UVBox{Min: geom.Vec2{X: uv.PointBounds.Min.X, Y: uv.PointBounds.Min.Y}, Max: geom.Vec2{X: uv.PointBounds.Max.X, Y: uv.PointBounds.Max.Y}}, result.PointBounds, result.VisitedCells, true, nil
}
