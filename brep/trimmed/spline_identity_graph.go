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
	return requireIdentityPatchXY(ctx, p, limits)
}

func requireIdentityPatchXY(ctx context.Context, p *geom.SurfacePatch, limits geom.SurfacePatchOptions) error {
	d := p.Domain()
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{X: d.UMin, Y: d.VMin}, {X: d.UMin, Y: d.VMax}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}},
		[]float64{d.UMin, d.UMin, d.UMax, d.UMax}, []float64{d.VMin, d.VMin, d.VMax, d.VMax}, nil)
	if e != nil {
		return e
	}
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

func provedIdentitySplineQueryChart(ctx context.Context, base geom.Surface, patch *geom.SurfacePatch, o geom.SurfacePatchOptions) bool {
	if bs, ok := base.(*geom.BSplineSurface); ok {
		return requireIdentitySplineChart(ctx, bs, o) == nil
	}
	return requireIdentityPatchXY(ctx, patch, o) == nil
}
