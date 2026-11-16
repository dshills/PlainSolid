package trimmed

import (
	"context"
	"fmt"
	"testing"
)

func TestNativeClippedPolynomialThreeTransverseFuseClosedDisplay(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		_, roof, _ := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
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
