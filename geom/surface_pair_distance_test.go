package geom

import (
	"errors"
	"testing"
)

func TestSurfacePairDistanceParallelAndContact(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 2} })
	r, e := BoundSurfacePairDistance(a, b, SurfacePairDistanceOptions{})
	if e != nil || !r.Complete || !r.HasWitness || r.SquaredDistanceMin != 4 || r.SquaredDistanceMax != 4 {
		t.Fatal(r, e)
	}
	r, e = BoundSurfacePairDistance(a, a, SurfacePairDistanceOptions{})
	if e != nil || !r.Complete || r.SquaredDistanceMax != 0 {
		t.Fatal(r, e)
	}
}
func TestSurfacePairDistanceRationalCurved(t *testing.T) {
	a, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{3, 3 + u, 2 * v} })
	r, e := BoundSurfacePairDistance(a, b, SurfacePairDistanceOptions{SquaredTolerance: .05, MaxOperations: 8000000})
	if e != nil || !r.Complete || r.SquaredDistanceMin > 5.02943725152286 || r.SquaredDistanceMax < 5.02943725152285 {
		t.Fatal(r, e)
	}
}
func TestSurfacePairDistanceLimitsRetainBounds(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v + .3} })
	for _, o := range []SurfacePairDistanceOptions{{MaxPairs: 1}, {MaxOperations: 1}, {MaxDepth: 1}} {
		r, e := BoundSurfacePairDistance(a, b, o)
		if !errors.Is(e, ErrConvergence) || r.Complete || r.SquaredDistanceMin > .03 || r.SquaredDistanceMax < .03 {
			t.Fatal(r, e)
		}
	}
}
