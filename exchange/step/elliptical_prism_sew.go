package step

import (
	"context"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
)

func sewExactEllipticalPrism(ctx context.Context, frame geom.Frame, a, b, height, tolerance float64) (trimmed.Solid, [2][4]int, error) {
	return ellipticprism.Prism(ctx, frame, a, b, height, tolerance)
}
func sewEllipticalPrismControls(ctx context.Context, uv [4][]geom.Vec3, world [2][4][]geom.Vec3, caps [2]geom.Frame, tolerance float64) (trimmed.Solid, [2][4]int, error) {
	return ellipticprism.SewControls(ctx, uv, world, caps, tolerance)
}
