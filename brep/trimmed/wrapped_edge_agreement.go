package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// compareWrappedEdgeUse is deliberately separate from recipe identity. It
// proves an unchanged original parameter chart with an independent knot basis,
// then checks a constant integer seam offset over the entire affine use.
func compareWrappedEdgeUse(ctx context.Context, edge parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	a, ok := analyticModelLine(edge)
	if !ok {
		return false, nil
	}
	b, ok := analyticUseLine(surface, use)
	if !ok {
		return false, nil
	}
	x, mx := wrappedSurfaceModel(a.surface)
	y, my := wrappedSurfaceModel(b.surface)
	if x == nil || y == nil || mx != my {
		return false, nil
	}
	maxWork := o.MaxWork
	if maxWork == 0 {
		maxWork = 8000000
	}
	relation, proof, e := geom.CompareWrappedSurfaceCharts(ctx, x, y, geom.ExactSurfaceComparisonOptions{MaxOperations: maxWork, MaxCoefficientBits: o.MaxBits, MaxCells: o.MaxIntervals})
	if e != nil {
		return false, e
	}
	if relation == nil || !proof.Complete || !proof.Equal {
		return false, nil
	}
	left := maxWork - proof.ExactOperations
	if left < 1 {
		return false, geom.ErrConvergence
	}
	return relation.SameParameterLines(ctx, a.uv, b.uv, geom.SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxBits})
}
