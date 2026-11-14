package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// compareWrappedQuadraticUse verifies whole exact model pieces in original
// normalized parameters, not merely common endpoints or control identity.
// It supports only unchanged source chart coordinates and equal placements.
func compareWrappedQuadraticUse(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, int, error) {
	m, ok := model.Curve.(surfaceTrimCurve)
	if !ok || model.Range != (parametric.Interval{Max: 1}) {
		return false, 0, nil
	}
	qa, ok := m.curve.(ExactQuadraticUV)
	if !ok {
		return false, 0, nil
	}
	qb, ok := use.Curve.(ExactQuadraticUV)
	if !ok {
		return false, 0, nil
	}
	abase, am := parametric.SurfaceMapping(m.surface)
	bbase, bm := parametric.SurfaceMapping(surface)
	a, ok := abase.(*geom.WrappedSurfacePatch)
	if !ok {
		return false, 0, nil
	}
	b, ok := bbase.(*geom.WrappedSurfacePatch)
	if !ok || am != bm {
		return false, 0, nil
	}
	if o.MaxWork == 0 {
		o.MaxWork = 8000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxWork < 1 || o.MaxBits < 64 {
		return false, 0, geom.ErrInvalid
	}
	relation, proof, e := geom.CompareWrappedSurfaceCharts(ctx, a, b, geom.ExactSurfaceComparisonOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits, MaxCells: o.MaxIntervals})
	if e != nil {
		return false, 0, e
	}
	if relation == nil || !proof.Complete || !proof.Equal {
		return false, proof.ExactOperations, nil
	}
	used := proof.ExactOperations
	var groups [2]geom.SurfaceAffineCurvePieces
	for i, q := range []ExactQuadraticUV{qa, qb} {
		if used >= o.MaxWork {
			return false, used, geom.ErrConvergence
		}
		uv, e := q.modelParameterCurve(ctx, geom.SurfacePatchOptions{MaxOperations: o.MaxWork - used, MaxCoefficientBits: o.MaxBits})
		if e != nil {
			return false, used, e
		}
		used += uv.ExactOperations()
		if used >= o.MaxWork {
			return false, used, geom.ErrConvergence
		}
		w := a
		if i == 1 {
			w = b
		}
		parts, e := w.ComposeParameterCurvePieces(ctx, uv, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: o.MaxWork - used, MaxCoefficientBits: o.MaxBits}, MaxPieces: o.MaxIntervals})
		if e != nil {
			return false, used, e
		}
		used += parts.ExactOperations
		if !parts.Complete || !parts.Continuous {
			return false, used, geom.ErrConvergence
		}
		for _, p := range parts.Pieces {
			groups[i].Pieces = append(groups[i].Pieces, geom.SurfaceAffineCurvePiece{Start: p.Start, End: p.End, Curve: p.Curve, Cell: p.Cell})
		}
	}
	if used >= o.MaxWork {
		return false, used, geom.ErrConvergence
	}
	comparison, e := geom.CompareAffineParameterCurvePieces(ctx, groups[0], groups[1], geom.ExactCurveComparisonOptions{ReverseB: use.Reversed, MaxSpans: o.MaxIntervals, MaxOperations: o.MaxWork - used, MaxCoefficientBits: o.MaxBits})
	used += comparison.ExactOperations
	return e == nil && comparison.Complete && comparison.Equal, used, e
}
