package trimmed

import (
	"testing"

	"plainsolid/geom"
)

func clippedTransverseFuseStock(t *testing.T, axis, layout int, original *geom.BSplineSurface) [3]*geom.BSplineSurface {
	t.Helper()
	if layout < 2 {
		return clippedTransverseStock(t, axis, layout, original)
	}
	s := clippedTransverseStock(t, axis, 0, original)
	s[1] = restrictOriginalClipStock(t, s[1], geom.ParameterRectangle{UMin: -2, UMax: 1.5, VMin: 8.25, VMax: 10})
	s[2] = restrictOriginalClipStock(t, s[2], geom.ParameterRectangle{UMin: 2, UMax: 4.625, VMin: -.75, VMax: 1})
	return s
}
