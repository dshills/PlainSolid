package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/cuspidal"
)

func writeCuspidalCubicSTEPWithWallElevation(ctx context.Context, w io.Writer, s trimmed.Solid, name string, lengthUnit float64, l Limits, o CuspidalCubicSTEPOptions, origin cuspidal.KnotOrigin) error {
	prepare := func(ctx context.Context, b trimmed.ExactBezierBoundary, proof edgecheck.Options) error {
		if !origin.HasSharedLongitudinalKnot(b) {
			return nil
		}
		return cuspidal.ElevateWallPCurves(ctx, b, proof, mappedChainStepDecimal)
	}
	return writeExactBezierSingleSTEPWithBoundary(ctx, w, s, name, lengthUnit, l, o, true, prepare)
}
