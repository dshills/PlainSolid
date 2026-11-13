package geom

import (
	"errors"
	"testing"
)

func TestCurveSurfaceDistancePlaneContactAndOffset(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, tc := range []struct {
		poles    []Vec3
		distance float64
	}{{[]Vec3{{0, .5, 2}, {1, .5, 2}}, 4}, {[]Vec3{{.5, .5, -1}, {.5, .5, 1}}, 0}, {[]Vec3{{2, 0, 0}, {2, 1, 0}}, 1}} {
		c, _ := NewBezier(tc.poles, nil)
		r, e := BoundCurveSurfaceDistance(c, p, CurveSurfaceDistanceOptions{SquaredTolerance: 1e-5})
		if e != nil || !r.Complete || !r.HasWitness || r.SquaredDistanceMin > tc.distance || r.SquaredDistanceMax < tc.distance {
			t.Fatal(r, e)
		}
	}
}
func TestCurveSurfaceDistanceRational(t *testing.T) {
	circle, _ := NewCircle(Vec3{Z: 3}, Vec3{Z: 1}, 1)
	curve, _ := circle.RationalBSpline()
	curve, _ = curve.InsertKnot(.13, 1)
	plane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{4*u - 2, 4*v - 2, 0} })
	r, e := BoundCurveSurfaceDistance(curve, plane, CurveSurfaceDistanceOptions{SquaredTolerance: 1e-3})
	if e != nil || !r.Complete || r.SquaredDistanceMin > 9 || r.SquaredDistanceMax < 9 {
		t.Fatal(r, e)
	}
	cylinder, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	line, _ := NewBezier([]Vec3{{3, 0, 0}, {3, 0, 2}}, nil)
	r, e = BoundCurveSurfaceDistance(line, cylinder, CurveSurfaceDistanceOptions{})
	if e != nil || !r.Complete || r.SquaredDistanceMin > 1 || r.SquaredDistanceMax < 1 {
		t.Fatal(r, e)
	}
}
func TestCurveSurfaceDistanceLimits(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	curve, _ := NewBezier([]Vec3{{.3, .2, 1}, {.4, .3, 2}}, nil)
	for _, o := range []CurveSurfaceDistanceOptions{{MaxPairs: 1}, {MaxOperations: 1}, {MaxQueuedPoles: 8}} {
		r, e := BoundCurveSurfaceDistance(curve, p, o)
		if !errors.Is(e, ErrConvergence) || r.Complete || r.SquaredDistanceMin > r.SquaredDistanceMax {
			t.Fatal(r, e)
		}
	}
}
