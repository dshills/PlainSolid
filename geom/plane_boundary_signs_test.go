package geom

import (
	"errors"
	"testing"
)

func TestPlaneBoundarySignsClosedContour(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p, _ := NewSurfacePatch(quadraticContactSurface(t, -.12), SurfacePatchOptions{})
	r, e := ClassifyPlaneBoundarySigns(p, plane, PlaneBoundarySignOptions{})
	if e != nil || !r.Complete || r.StrictSign != 1 || r.SignChanges || r.Coincident {
		t.Fatal(r, e)
	}
}
func TestPlaneBoundarySignsCrossingTangentCoincident(t *testing.T) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .3} })
	r, e := ClassifyPlaneBoundarySigns(p, plane, PlaneBoundarySignOptions{})
	if e != nil || !r.SignChanges || r.StrictSign != 0 {
		t.Fatal(r, e)
	}
	p, _ = NewSurfacePatch(quadraticContactSurface(t, -.25), SurfacePatchOptions{})
	r, e = ClassifyPlaneBoundarySigns(p, plane, PlaneBoundarySignOptions{})
	if e != nil || !r.Tangencies || r.WeakSign != 1 || r.StrictSign != 0 {
		t.Fatal(r, e)
	}
	p = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u} })
	r, e = ClassifyPlaneBoundarySigns(p, plane, PlaneBoundarySignOptions{})
	if e != nil || !r.Coincident || r.WeakSign != 1 || r.StrictSign != 0 {
		t.Fatal(r, e)
	}
}
func TestPlaneBoundarySignsRationalAndKnots(t *testing.T) {
	s := quarterCylinder(t)
	s, _ = s.InsertUKnot(.3, 1)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	plane, _ := NewPlane(Vec3{Z: -1}, Vec3{Z: 1})
	r, e := ClassifyPlaneBoundarySigns(p, plane, PlaneBoundarySignOptions{})
	if e != nil || r.StrictSign != 1 || len(r.Spans) != 6 {
		t.Fatal(r, e)
	}
	r, e = ClassifyPlaneBoundarySigns(p, plane, PlaneBoundarySignOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.StrictSign != 0 {
		t.Fatal(r, e)
	}
}
