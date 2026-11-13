package geom

import (
	"context"
	"errors"
	"testing"
)

func TestSurfaceAffineProjectionClipping(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	plane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{v, .25 + .5*u, 0} })
	chart, e := NewAffineSurfaceChart(ctx, plane, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := ClipSurfaceAffineProjection(ctx, a, chart, SurfacePatchOptions{})
	if e != nil || r.Empty || r.Domain != (ParameterRectangle{UMin: 0, UMax: 1, VMin: .25, VMax: .75}) || r.Axis != ([2]int{1, 0}) {
		t.Fatal(r, e)
	}
	r.Scale[0].SetInt64(100)
	again, e := ClipSurfaceAffineProjection(ctx, a, chart, SurfacePatchOptions{})
	if e != nil || again.Scale[0].RatString() != "2" {
		t.Fatal(again, e)
	}
	skew := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u + v, v, 0} })
	skewChart, _ := NewAffineSurfaceChart(ctx, skew, AffineSurfaceChartOptions{})
	_, e = ClipSurfaceAffineProjection(ctx, a, skewChart, SurfacePatchOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	scaled := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{3 * u, v, u * v} })
	flat := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	flatChart, _ := NewAffineSurfaceChart(ctx, flat, AffineSurfaceChartOptions{})
	_, e = ClipSurfaceAffineProjection(ctx, scaled, flatChart, SurfacePatchOptions{})
	if !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
	_, e = ClipSurfaceAffineProjection(ctx, a, chart, SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = ClipSurfaceAffineProjection(canceled, a, chart, SurfacePatchOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
