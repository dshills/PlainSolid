package geom

import (
	"errors"
	"math"
	"testing"
)

func TestSurfaceDistanceBoundsPlaneAndBoundary(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, tc := range []struct {
		point    Vec3
		distance float64
	}{{Vec3{.5, .5, 2}, 4}, {Vec3{2, .3, 0}, 1}, {Vec3{.3, .4, 0}, 0}} {
		r, e := BoundPointSurfaceDistance(tc.point, p, SurfaceDistanceOptions{SquaredTolerance: 1e-5})
		if e != nil || !r.Complete || r.SquaredDistanceMin > tc.distance || r.SquaredDistanceMax < tc.distance || r.SquaredDistanceMax-r.SquaredDistanceMin > 1.00001e-5 {
			t.Fatal(tc, r, e)
		}
	}
}
func TestSurfaceDistanceBoundsRationalCylinder(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	q := Vec3{3, 3, 1}
	distance := (math.Sqrt(18) - 2) * (math.Sqrt(18) - 2)
	r, e := BoundPointSurfaceDistance(q, p, SurfaceDistanceOptions{SquaredTolerance: 5e-3, MaxOperations: 8000000})
	if e != nil || !r.Complete || r.SquaredDistanceMin > distance+1e-12 || r.SquaredDistanceMax < distance-1e-12 {
		t.Fatal(r, e)
	}
}
func TestSurfaceDistanceBoundsPreservedAtLimit(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	r, e := BoundPointSurfaceDistance(Vec3{2, .3, 0}, p, SurfaceDistanceOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.SquaredDistanceMin > 1 || r.SquaredDistanceMax < 1 {
		t.Fatal(r, e)
	}
}
