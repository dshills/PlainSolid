package trimmed

import (
	"context"
	"fmt"
	"testing"

	"plainsolid/geom"
)

func TestNativeClippedPolynomialFuseTwoAxesClosedDisplay(t *testing.T) {
	checkClippedFuseDisplayLayouts(t, twoAxisClippedFuseStock)
}

func TestNativeClippedPolynomialFuseTwoAxisContainmentDisplay(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		for order := range 2 {
			t.Run(fmt.Sprintf("axis%d/order%d", axis, order), func(t *testing.T) {
				t.Parallel()
				src := clippedPolynomialStock(t, axis, 0, 1)
				src[2] = restrictOriginalClipStock(t, src[2], geom.ParameterRectangle{
					UMin: 2.375, UMax: 4.625, VMin: -.75, VMax: .75,
				})
				fuse, err := FuseClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9,
					PolynomialCubicCrossingTangencyEnvelopeOptions{})
				if err != nil {
					t.Fatal(err)
				}
				checkClippedFuseDisplay(t, fuse)
			})
		}
	}
}
