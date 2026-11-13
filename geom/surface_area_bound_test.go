package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCertifiedSurfaceAreaPlane(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{3 * u, 4 * v, 0} })
	r, e := BoundSurfaceArea(p, SurfaceAreaBoundOptions{})
	if e != nil || !r.Complete || r.Lower != 12 || r.Upper != 12 {
		t.Fatal(r, e)
	}
}
func TestCertifiedSurfaceAreaCylinder(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	r, e := BoundSurfaceArea(p, SurfaceAreaBoundOptions{Tolerance: .5, MaxOperations: 12000000})
	if e != nil || !r.Complete || r.Lower > 2*math.Pi+1e-12 || r.Upper < 2*math.Pi-1e-12 {
		t.Fatal(r, e)
	}
}
func TestCertifiedSurfaceAreaLimitsCancellation(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	r, e := BoundSurfaceArea(p, SurfaceAreaBoundOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || !finite(r.Upper) || r.Lower > 2*math.Pi || r.Upper < 2*math.Pi {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = BoundSurfaceAreaContext(ctx, p, SurfaceAreaBoundOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
	r, e = BoundSurfaceAreaContext(distanceCancelContext(4), p, SurfaceAreaBoundOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete || r.Cells < 2 || !finite(r.Upper) || r.Lower > 2*math.Pi || r.Upper < 2*math.Pi {
		t.Fatal(r, e)
	}
}

func TestCertifiedSurfaceAreaFoldAndParameterScale(t *testing.T) {
	// A collapsed patch has zero area even when its individual derivatives do not vanish.
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u + v, 0, 0} })
	r, e := BoundSurfaceArea(p, SurfaceAreaBoundOptions{})
	if e != nil || !r.Complete || r.Lower != 0 || r.Upper != 0 {
		t.Fatal(r, e)
	}
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 4, 0}, {3, 0, 0}, {3, 4, 0}}, []float64{2, 2, 6, 6}, []float64{-2, -2, 6, 6}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e = NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = BoundSurfaceArea(p, SurfaceAreaBoundOptions{})
	if e != nil || !r.Complete || r.Lower != 12 || r.Upper != 12 {
		t.Fatal(r, e)
	}
}
