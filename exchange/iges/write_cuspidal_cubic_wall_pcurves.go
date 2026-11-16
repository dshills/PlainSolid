package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/cuspidal"
)

func writeCuspidalCubicIGESWithWallElevation(ctx context.Context, w io.Writer, s trimmed.Solid, unit float64, l Limits, o CuspidalCubicIGESOptions, origin cuspidal.KnotOrigin) error {
	prepare := func(ctx context.Context, b trimmed.ExactBezierBoundary, proof edgecheck.Options) error {
		if !origin.HasSharedLongitudinalKnot(b) {
			return nil
		}
		return cuspidal.ElevateWallPCurves(ctx, b, proof, exactIGESSplineDecimal)
	}
	return writeExactBezierSingleIGESWithBoundary(ctx, w, s, unit, l, o, true, prepare)
}
