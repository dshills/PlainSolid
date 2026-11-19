package trimmed

import (
	"context"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func twoAxisClippedFuseStock(t *testing.T, axis, profile, layout int) [3]*geom.BSplineSurface {
	t.Helper()
	s := clippedPolynomialStock(t, axis, profile, 0)
	a := [2]geom.ParameterRectangle{
		{UMin: -2, UMax: 1.5, VMin: 8, VMax: 10},
		{UMin: -2, UMax: 2, VMin: 8.25, VMax: 10},
	}[axis]
	b := geom.ParameterRectangle{UMin: 2, UMax: 4.625, VMin: -1, VMax: 1}
	if layout == 0 {
		if axis == 0 {
			a.VMin = 8.25
		} else {
			a.UMax = 1.5
		}
		b.VMin = -.75
	} else {
		if axis == 0 {
			a.VMax = 9.75
		} else {
			a.UMin = -1.5
		}
		b.VMax = .75
	}
	s[1], s[2] = restrictOriginalClipStock(t, s[1], a), restrictOriginalClipStock(t, s[2], b)
	return s
}

func TestNativeClippedPolynomialFuseTwoAxisContainment(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := range 2 {
		src := clippedPolynomialStock(t, axis, 0, 1)
		src[2] = restrictOriginalClipStock(t, src[2], geom.ParameterRectangle{UMin: 2.375, UMax: 4.625, VMin: -.75, VMax: .75})
		for order := range 2 {
			out, e := FuseClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9,
				PolynomialCubicCrossingTangencyEnvelopeOptions{})
			if e != nil {
				t.Fatal("stock contained in both physical axes", axis, order, e)
			}
			// The contained roof wins only inside the retained central band.
			// Removing its covered dependent margins preserves the exact
			// source integral for one-axis containment.
			checkClippedFuseMass(t, out, 1, axis, 0)
		}
	}
}
