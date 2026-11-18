package trimmed

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaTwoCrossingCirclesUnavailableCapSubdivision(t *testing.T) {
	// This second concave profile has four represented original nodes,
	// but its small r=9/256 circle has only cardinal dyadic endpoints.
	// Its true upper cap needs a noncardinal vertex for the native chord loop.
	f := parabolaTwoCrossingsSourceFixture{
		parabola: [3]*big.Rat{big.NewRat(357, 1024), big.NewRat(233, 128), big.NewRat(-2, 1)},
		discs:    [2]twoCircleStockProbe{twoCircleProbe(96, 105, 87, 256), twoCircleProbe(1408, 1269, 72, 2048)},
		scale:    big.NewRat(1225, 512),
	}
	ctx := context.Background()
	stocks := f.stocks(t, 0, false)
	patches := parabolaTwoCirclesPatches(t, stocks[1], stocks[2])
	if _, err := geom.DiscoverSplineGraphParabolaTwoCrossingCircles(ctx, patches[0], patches[1], 0, geom.GraphPointContactOptions{}); err != nil {
		t.Fatal("unavailable native vertex changed source solvability", err)
	}
	r, err := CombineParabolaTwoCrossingCirclesSplineGraphs(ctx, stocks[0], stocks[1], stocks[2], 0,
		parametric.Union, 1e-9, ParabolaCircleGraphOptions{})
	if r != nil || !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "original circle 1: trimmed: exact cap arc subdivision search exhausted") {
		t.Fatal("published a cap without a proved exact chord subdivision", r, err)
	}
}
