package geom

import (
	"errors"
	"math"
	"testing"
)

func TestSurfaceSeparationOverlappingAABBs(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v + .25} })
	if _, ok := a.bounds.Intersect(b.bounds); !ok {
		t.Fatal("fixture boxes must overlap")
	}
	r, e := SeparateSurfacePatches(a, b, SurfaceSeparationOptions{})
	if e != nil || !r.Separated || !r.ExactStrictGap || r.TestedDirections <= 3 || r.GapLower <= 0 {
		t.Fatal(r, e)
	}
	r, e = SeparateSurfacePatches(b, a, SurfaceSeparationOptions{})
	if e != nil || !r.Separated {
		t.Fatal(r, e)
	}
	cover, e := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{SeparatingProjections: true, MaxPairs: 1})
	if e != nil || !cover.Complete || cover.ProjectionExclusions != 1 || len(cover.Candidates) != 0 {
		t.Fatal(cover, e)
	}
	cover, e = BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{SeparatingProjections: true, MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || cover.Complete || len(cover.Pending) != 1 {
		t.Fatal(cover, e)
	}
}
func TestSurfaceSeparationPreservesTangencyAndNearGap(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b, _ := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	r, e := SeparateSurfacePatches(a, b, SurfaceSeparationOptions{})
	if e != nil || r.Separated {
		t.Fatal(r, e)
	}
	b = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, math.SmallestNonzeroFloat64} })
	r, e = SeparateSurfacePatches(a, b, SurfaceSeparationOptions{})
	if e != nil || !r.Separated || r.GapLower != math.SmallestNonzeroFloat64 {
		t.Fatal(r, e)
	}
	r, e = SeparateSurfacePatches(a, a, SurfaceSeparationOptions{})
	if e != nil || r.Separated {
		t.Fatal(r, e)
	}
	if _, e = SeparateSurfacePatches(a, b, SurfaceSeparationOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
