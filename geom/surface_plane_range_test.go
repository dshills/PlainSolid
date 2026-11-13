package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCertifiedSurfacePlaneExtrema(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 2*u + 3*v + 1} })
	plane, _ := NewPlane(Vec3{}, Vec3{0, 0, 1})
	r, e := BoundSurfacePlaneRange(p, plane, SurfacePlaneRangeOptions{})
	if e != nil || !r.Complete || r.StrictSign != 1 || r.Minimum.Min != 1 || r.Minimum.Max != 1 || r.Maximum.Min != 6 || r.Maximum.Max != 6 {
		t.Fatal(r, e)
	}
	p, _ = NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	plane, _ = NewPlane(Vec3{}, Vec3{1, 1, 0})
	r, e = BoundSurfacePlaneRange(p, plane, SurfacePlaneRangeOptions{Tolerance: .01})
	if e != nil || !r.Complete || r.Minimum.Min > math.Sqrt(2)+1e-12 || r.Minimum.Max < math.Sqrt(2)-1e-12 || r.Maximum.Min > 2+1e-12 || r.Maximum.Max < 2-1e-12 {
		t.Fatal(r, e)
	}
}
func TestCertifiedSurfacePlaneExtremaLimits(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{}, Vec3{1, 1, 0})
	r, e := BoundSurfacePlaneRange(p, plane, SurfacePlaneRangeOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || !finite(r.Maximum.Max) || r.Maximum.Min > 2+1e-12 || r.Maximum.Max < 2-1e-12 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = BoundSurfacePlaneRangeContext(ctx, p, plane, SurfacePlaneRangeOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
	r, e = BoundSurfacePlaneRangeContext(distanceCancelContext(3), p, plane, SurfacePlaneRangeOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete || r.Cells < 2 || !finite(r.Maximum.Max) {
		t.Fatal(r, e)
	}
}
