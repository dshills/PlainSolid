package geom

import (
	"math"
	"testing"
)

func TestAnalyticIntersections(t *testing.T) {
	tol := IntersectionTolerance{1e-10, 1e-12}
	plane, _ := NewPlane(Vec3{0, 0, 2}, Vec3{0, 0, 1})
	line, _ := NewLine(Vec3{}, Vec3{0, 0, 2})
	r, e := IntersectLinePlane(line, plane, tol)
	if e != nil || r.Kind != IntersectionPoint || r.Parameter != 2 {
		t.Fatal(r, e)
	}
	near(t, r.Point, Vec3{0, 0, 2}, 0)
	parallel, _ := NewLine(Vec3{}, Vec3{1, 0, 0})
	r, e = IntersectLinePlane(parallel, plane, tol)
	if e != nil || r.Kind != IntersectionNone {
		t.Fatal(r, e)
	}
	parallel, _ = NewLine(Vec3{0, 0, 2}, Vec3{1, 0, 0})
	r, e = IntersectLinePlane(parallel, plane, tol)
	if e != nil || r.Kind != IntersectionCoincident {
		t.Fatal(r, e)
	}
	other, _ := NewPlane(Vec3{3, 0, 0}, Vec3{1, 0, 0})
	pp, e := IntersectPlanes(plane, other, tol)
	if e != nil || pp.Kind != IntersectionLine {
		t.Fatal(pp, e)
	}
	for _, u := range []float64{-10, 0, 10} {
		p, _ := pp.Line.Eval(u)
		if math.Abs(plane.SignedDistance(p)) > 1e-12 || math.Abs(other.SignedDistance(p)) > 1e-12 {
			t.Fatal(p)
		}
	}
	pp, e = IntersectPlanes(plane, plane, tol)
	if e != nil || pp.Kind != IntersectionCoincident {
		t.Fatal(pp, e)
	}
}
func TestLineSphereIntersection(t *testing.T) {
	sphere, _ := NewSphere(Vec3{}, 2)
	line, _ := NewLine(Vec3{-10, 0, 0}, Vec3{1, 0, 0})
	p, e := IntersectLineSphere(line, sphere, 1e-12)
	if e != nil || len(p) != 2 {
		t.Fatal(p, e)
	}
	near(t, p[0], Vec3{-2, 0, 0}, 0)
	near(t, p[1], Vec3{2, 0, 0}, 0)
	line, _ = NewLine(Vec3{-10, 2, 0}, Vec3{1, 0, 0})
	p, e = IntersectLineSphere(line, sphere, 1e-12)
	if e != nil || len(p) != 1 {
		t.Fatal(p, e)
	}
	near(t, p[0], Vec3{0, 2, 0}, 0)
	line, _ = NewLine(Vec3{0, 3, 0}, Vec3{1, 0, 0})
	p, e = IntersectLineSphere(line, sphere, 1e-12)
	if e != nil || len(p) != 0 {
		t.Fatal(p, e)
	}
	q, e := sphere.Closest(Vec3{0, 6, 0})
	if e != nil {
		t.Fatal(e)
	}
	near(t, q, Vec3{0, 2, 0}, 0)
	if sphere.SignedDistance(q) != 0 {
		t.Fatal(q)
	}
	if _, e = sphere.Closest(Vec3{}); e == nil {
		t.Fatal("nonunique projection")
	}
}
func TestSphereSphereIntersection(t *testing.T) {
	a, _ := NewSphere(Vec3{}, 2)
	b, _ := NewSphere(Vec3{2, 0, 0}, 2)
	r, e := IntersectSpheres(a, b, 1e-12)
	if e != nil || r.Kind != IntersectionCircle {
		t.Fatal(r, e)
	}
	for i := 0; i < 20; i++ {
		p, e := r.Circle.Eval(float64(i))
		if e != nil || math.Abs(a.SignedDistance(p)) > 1e-14 || math.Abs(b.SignedDistance(p)) > 1e-14 {
			t.Fatal(p, e)
		}
	}
	b, _ = NewSphere(Vec3{4, 0, 0}, 2)
	r, e = IntersectSpheres(a, b, 1e-12)
	if e != nil || r.Kind != IntersectionPoint {
		t.Fatal(r, e)
	}
	near(t, r.Point, Vec3{2, 0, 0}, 0)
	b, _ = NewSphere(Vec3{1, 0, 0}, 1)
	r, e = IntersectSpheres(a, b, 1e-12)
	if e != nil || r.Kind != IntersectionPoint {
		t.Fatal(r, e)
	}
	near(t, r.Point, Vec3{2, 0, 0}, 0)
	r, e = IntersectSpheres(a, a, 1e-12)
	if e != nil || r.Kind != IntersectionCoincident {
		t.Fatal(r, e)
	}
	b, _ = NewSphere(Vec3{}, 1)
	r, e = IntersectSpheres(a, b, 1e-12)
	if e != nil || r.Kind != IntersectionNone {
		t.Fatal(r, e)
	}
}
func TestIntersectionInvalid(t *testing.T) {
	if _, e := IntersectLinePlane(Line{}, Plane{}, IntersectionTolerance{}); e == nil {
		t.Fatal("zero")
	}
	if _, e := IntersectPlanes(Plane{}, Plane{}, IntersectionTolerance{}); e == nil {
		t.Fatal("zero")
	}
	if _, e := IntersectLineSphere(Line{}, Sphere{}, 0); e == nil {
		t.Fatal("zero")
	}
	if _, e := IntersectSpheres(Sphere{}, Sphere{}, 0); e == nil {
		t.Fatal("zero")
	}
	if _, e := NewSphere(Vec3{}, -1); e == nil {
		t.Fatal("radius")
	}
}
