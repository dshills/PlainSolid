package iges

import (
	"context"
	"math/big"
	"strconv"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/parabolacircle"
	"plainsolid/geom"
)

func emitTwoCrossingCirclesIGESBody(ctx context.Context, rings [2][]int) exactBezierIGESBodyEmitter {
	return emitParabolaCircleIGESBody(ctx, rings, false)
}

func emitParabolaCircleIGESBody(ctx context.Context, rings [2][]int, anchored bool) exactBezierIGESBodyEmitter {
	return func(a *topologyAuthor, b trimmed.ExactBezierBoundary, bound *big.Rat, consume func(int) error) (int, error) {
		var windows map[int]parabolacircle.CardinalCircleWindow
		var err error
		if anchored {
			windows, err = parabolacircle.PrepareAnchoredCircleWindows(ctx, b, rings[0], exactIGESSplineDecimal)
		} else {
			windows, err = parabolacircle.PrepareCardinalCircleWindows(ctx, b, rings, exactIGESSplineDecimal)
		}
		if err != nil {
			return 0, err
		}
		return emitExactBezierIGESBodyWithCurves(a, b, bound, consume,
			func(edge int, span geom.ExactBezierSpan, planar bool, tolerance *big.Rat) (string, error) {
				window, found := windows[edge]
				if !found {
					return exactIGESBezier(span, planar, tolerance)
				}
				parent := window.Model
				if planar {
					parent = window.UV
				}
				return exactIGESBezierInWindow(parent, planar, tolerance, strconv.FormatInt(window.DomainEnd, 10)+".",
					strconv.FormatInt(window.First, 10)+".", strconv.FormatInt(window.Last, 10)+".")
			})
	}
}
