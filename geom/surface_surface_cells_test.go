package geom

import (
	"errors"
	"testing"
)

func pairCovers(c SurfaceSurfaceCell, u, v, s, t float64) bool {
	return u >= c.DomainA.UMin && u <= c.DomainA.UMax && v >= c.DomainA.VMin && v <= c.DomainA.VMax && s >= c.DomainB.UMin && s <= c.DomainB.UMax && t >= c.DomainB.VMin && t <= c.DomainB.VMax
}
func TestSurfaceSurfaceCoverCrossingAndSymmetry(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, .375, v - .5} })
	for pass := 0; pass < 2; pass++ {
		r, e := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{ParameterTolerance: .25})
		if e != nil || !r.Complete || len(r.Candidates) == 0 {
			t.Fatalf("%+v %v", r, e)
		}
		for _, c := range r.Candidates {
			if !c.AtRequestedResolution {
				t.Fatal(c)
			}
		}
		for i := 0; i <= 64; i++ {
			u := float64(i) / 64
			covered := false
			for _, c := range r.Candidates {
				if pass == 0 {
					covered = covered || pairCovers(c, u, .375, u, .5)
				} else {
					covered = covered || pairCovers(c, u, .5, u, .375)
				}
			}
			if !covered {
				t.Fatalf("missing contact %g", u)
			}
		}
		a, b = b, a
	}
}
func TestSurfaceSurfaceDisjointAndIdentical(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1} })
	r, e := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{})
	if e != nil || !r.Complete || len(r.Candidates) != 0 || len(r.Coincident) != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	b = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	r, e = BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{})
	if e != nil || len(r.Coincident) != 1 || len(r.Candidates) != 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestSurfaceSurfaceTangencyAndLimits(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b, _ := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	for _, o := range []SurfaceSurfaceIsolationOptions{{ParameterTolerance: .25}, {MaxPairs: 1}, {MaxOperations: 1}, {MaxDepth: 2}, {MaxQueuedPoles: 8}} {
		r, e := BoundSurfaceSurface(a, b, o)
		if e != nil && !errors.Is(e, ErrConvergence) {
			t.Fatal(e)
		}
		if e != nil && r.Complete {
			t.Fatal("false complete")
		}
		covered := false
		for _, c := range append(r.Candidates, r.Pending...) {
			covered = covered || pairCovers(c, .5, .5, .5, .5)
		}
		if !covered {
			t.Fatalf("lost tangent: %+v %v", r, e)
		}
	}
}

func TestSurfaceSurfaceRationalContactCover(t *testing.T) {
	s := quarterCylinder(t)
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{2 * u, 2 * v, .75} })
	r, e := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{ParameterTolerance: .25})
	if e != nil || !r.Complete {
		t.Fatalf("%+v %v", r, e)
	}
	for i := 0; i <= 32; i++ {
		u := float64(i) / 32
		point, e := s.Eval(u, .375)
		if e != nil {
			t.Fatal(e)
		}
		covered := false
		for _, c := range r.Candidates {
			covered = covered || pairCovers(c, u, .375, point.X/2, point.Y/2)
		}
		if !covered {
			t.Fatalf("lost rational contact %v", point)
		}
	}
}
func TestSurfaceSurfaceCoplanarIsNotEqualityProof(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{1 - u, v, 0} })
	r, e := BoundSurfaceSurface(a, b, SurfaceSurfaceIsolationOptions{ParameterTolerance: .5})
	if e != nil || !r.Complete || len(r.Coincident) != 0 || len(r.Candidates) == 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
