package geom

import (
	"errors"
	"testing"
)

func TestSurfaceRegularityOrientation(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	r, e := CertifySurfaceRegularity(p, SurfaceRegularityOptions{ReferenceDirection: Vec3{Z: 1}})
	if e != nil || !r.Complete || !r.ConsistentOrientation || r.Orientation != 1 {
		t.Fatal(r, e)
	}
	p = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{v, u, u * v} })
	r, e = CertifySurfaceRegularity(p, SurfaceRegularityOptions{ReferenceDirection: Vec3{Z: 1}})
	if e != nil || r.Orientation != -1 || !r.ConsistentOrientation {
		t.Fatal(r, e)
	}
}
func TestSurfaceRegularityRationalAndSingular(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	r, e := CertifySurfaceRegularity(p, SurfaceRegularityOptions{})
	if e != nil || !r.Complete || len(r.Certified) == 0 {
		t.Fatal(r, e)
	}
	r, e = CertifySurfaceRegularity(p, SurfaceRegularityOptions{ReferenceDirection: Vec3{X: 1, Y: 1}})
	if e != nil || !r.ConsistentOrientation || r.Orientation != 1 {
		t.Fatal(r, e)
	}
	p = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 0, 0} })
	r, e = CertifySurfaceRegularity(p, SurfaceRegularityOptions{MaxDepth: 4})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
}
func TestSurfaceRegularityBoundsAndBudget(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	r, e := CertifySurfaceRegularity(p, SurfaceRegularityOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) != 1 {
		t.Fatal(r, e)
	}
	r, e = CertifySurfaceRegularity(p, SurfaceRegularityOptions{ReferenceDirection: Vec3{X: 1}, MaxDepth: 3})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
}
