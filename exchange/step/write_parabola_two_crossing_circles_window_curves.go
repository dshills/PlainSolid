package step

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/parabolacircle"
	"plainsolid/geom"
)

func emitTwoCrossingCirclesSTEPBody(rings [2][]int) exactBezierSTEPBodyEmitter {
	return func(ctx context.Context, emit func(string) int64, b trimmed.ExactBezierBoundary, name string, tolerance *big.Rat) (int64, error) {
		windows, err := parabolacircle.PrepareCardinalCircleWindows(ctx, b, rings, mappedChainStepDecimal)
		if err != nil {
			return 0, err
		}
		return emitExactBezierSTEPBodyWithCurves(ctx, emit, b, name, tolerance,
			func(emit func(string) int64, edge int, span geom.ExactBezierSpan, dim int, tolerance *big.Rat) (int64, error) {
				window, found := windows[edge]
				if !found {
					return emitClippedFuseSTEPBezier(emit, span, dim, tolerance)
				}
				parent := window.Model
				if dim == 2 {
					parent = window.UV
				}
				spline := mappedChainStepSplineFromBezier(parent.Control, dim, new(big.Rat), big.NewRat(window.DomainEnd, 1))
				support, err := spline.emit(emit)
				if err != nil {
					return 0, err
				}
				return emit(fmt.Sprintf("TRIMMED_CURVE('',#%d,(PARAMETER_VALUE(%d.)),(PARAMETER_VALUE(%d.)),.T.,.PARAMETER.)",
					support, window.First, window.Last)), nil
			})
	}
}
