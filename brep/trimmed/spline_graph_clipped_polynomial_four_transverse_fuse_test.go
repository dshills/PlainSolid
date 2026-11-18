package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func clippedFourTransverseRoofFixture(t *testing.T, axis int) *geom.BSplineSurface {
	t.Helper()
	_, roof, _ := nativeQuarticRoofPairFixture(t, axis, big.NewRat(1, 1024),
		[4][2]int64{{1, 4}, {3, 8}, {5, 8}, {3, 4}})
	return roof
}

func TestNativeClippedPolynomialFourTransverseFuseClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := range 2 {
		roof := clippedFourTransverseRoofFixture(t, axis)
		for layout := range 3 {
			stocks := clippedTransverseFuseStock(t, axis, layout, roof)
			for order := range 2 {
				t.Run(fmt.Sprintf("axis%d/layout%d/order%d", axis, layout, order), func(t *testing.T) {
					fuse, err := FuseClippedPolynomialEnvelope(ctx, stocks[0], stocks[1+order], stocks[2-order], axis,
						1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
					if err != nil {
						t.Fatal(err)
					}
					checkClippedFuseDisplay(t, fuse)
				})
			}
		}
	}
}
