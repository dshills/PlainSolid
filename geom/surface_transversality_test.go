package geom

import (
	"errors"
	"testing"
)

func TestSurfaceTransversalityPlanes(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 0, v} })
	r, e := CertifySurfaceTransversality(a, b, SurfaceTransversalityOptions{})
	if e != nil || !r.ProvenTransverse || r.SeparatingComponent != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	r, e = CertifySurfaceTransversality(a, a, SurfaceTransversalityOptions{})
	if e != nil || r.ProvenTransverse {
		t.Fatalf("%+v %v", r, e)
	}
	collapsed := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 0, 0} })
	r, e = CertifySurfaceTransversality(a, collapsed, SurfaceTransversalityOptions{})
	if e != nil || r.ProvenTransverse {
		t.Fatalf("%+v %v", r, e)
	}
	_, e = CertifySurfaceTransversality(a, b, SurfaceTransversalityOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
func TestSurfaceTransversalityRationalCylinder(t *testing.T) {
	cylinder, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	cylinder, e := cylinder.Trim(ParameterRectangle{.2, .4, .1, .8})
	if e != nil {
		t.Fatal(e)
	}
	plane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, .75} })
	r, e := CertifySurfaceTransversality(cylinder, plane, SurfaceTransversalityOptions{})
	if e != nil || !r.ProvenTransverse {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestSurfaceTransversalityRejectsPotentialTangency(t *testing.T) {
	plane := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	bowl, _ := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	r, e := CertifySurfaceTransversality(plane, bowl, SurfaceTransversalityOptions{})
	if e != nil || r.ProvenTransverse {
		t.Fatalf("%+v %v", r, e)
	}
	away, e := bowl.Trim(ParameterRectangle{.7, .9, .7, .9})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CertifySurfaceTransversality(plane, away, SurfaceTransversalityOptions{})
	if e != nil || !r.ProvenTransverse {
		t.Fatalf("%+v %v", r, e)
	}
	// The latter certificate is valid despite the absence of intersection.
}

func TestSurfaceTransversalityExactSignBelowFloatRange(t *testing.T) {
	scale := 1e-100
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{scale * u, scale * v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{scale * u, 0, scale * v} })
	r, e := CertifySurfaceTransversality(a, b, SurfaceTransversalityOptions{})
	if e != nil || !r.ProvenTransverse || r.CrossNormals[0].Sign != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	if r.CrossNormals[0].Min != 0 {
		t.Fatal("fixture must underflow outward lower bound")
	}
}
