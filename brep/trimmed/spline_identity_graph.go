package trimmed

import (
	"context"
	"fmt"

	"plainsolid/geom"
)

// requireIdentitySplineChart keeps the cheap polynomial path and falls back
// to exact rational cross products for a different homogeneous XY encoding.
func requireIdentitySplineChart(ctx context.Context, s *geom.BSplineSurface, limits geom.SurfacePatchOptions) error {
	if _, e := checkUnitGraph(s); e == nil {
		return nil
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	p, e := geom.NewSurfacePatch(s, limits)
	if e != nil {
		return e
	}
	if p.Domain() != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return geom.ErrDomain
	}
	k := []float64{0, 0, 1, 1}
	plane, _ := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	q, e := geom.NewSurfacePatch(plane, limits)
	if e != nil {
		return e
	}
	_, e = geom.NewGeneralSurfaceHeightDifferencePatch(ctx, p, q, geom.GeneralSurfaceHeightOptions{
		MaxPoles: limits.MaxPoles, MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits,
	})
	if e != nil {
		return fmt.Errorf("trimmed: could not prove exact identity XY chart: %w", e)
	}
	return nil
}
