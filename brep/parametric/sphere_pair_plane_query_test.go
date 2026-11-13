package parametric

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func TestBoundSpherePairPlaneQueries(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchEvaluationOptions{}
	n := geom.Vec3{Y: 1, Z: 1}
	s, e := NewSpherePairSet(ctx, Ball{Radius: 5}, Ball{Center: geom.Vec3{X: 4}, Radius: 3}, Difference, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.PlaneSupports(ctx, geom.Vec3{}, n, o)
	if e != nil || p.Supports[0].Sign != 1 {
		t.Fatal(p, e)
	}
	c, e := s.PlaneSection(ctx, geom.Vec3{}, n, o)
	if e != nil || len(c.Contacts) != 2 {
		t.Fatal(c, e)
	}
	b, e := s.PlaneBoundaryCycle(ctx, geom.Vec3{}, n, o)
	if e != nil || b.Arcs[1].CCW {
		t.Fatal(b, e)
	}
	s, e = NewSpherePairSet(ctx, Ball{Radius: 5}, Ball{Center: geom.Vec3{X: 1}, Radius: 2}, Difference, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.PlaneBoundaryRings(ctx, geom.Vec3{}, n, o)
	if e != nil || r.Region != geom.SphereSectionAnnulus {
		t.Fatal(r, e)
	}
	var invalid SpherePairSet
	if _, e := invalid.PlaneSupports(ctx, geom.Vec3{}, n, o); e != geom.ErrInvalid {
		t.Fatal(e)
	}
	if _, e := invalid.PlaneSection(ctx, geom.Vec3{}, n, o); e != geom.ErrInvalid {
		t.Fatal(e)
	}
	if _, e := invalid.PlaneBoundaryCycle(ctx, geom.Vec3{}, n, o); e != geom.ErrInvalid {
		t.Fatal(e)
	}
	if _, e := invalid.PlaneBoundaryRings(ctx, geom.Vec3{}, n, o); e != geom.ErrInvalid {
		t.Fatal(e)
	}
}
