package geom

import (
	"errors"
	"math"
	"testing"
)

func TestPlaneSphereSections(t *testing.T) {
	sphere, _ := NewSphere(Vec3{1, 2, 3}, 5)
	plane, _ := NewPlane(Vec3{0, 0, 6}, Vec3{0, 0, 1})
	result, e := IntersectPlaneSphere(plane, sphere, 1e-10)
	if e != nil || result.Kind != IntersectionCircle {
		t.Fatal(result, e)
	}
	for i := 0; i < 20; i++ {
		p, _ := result.Ellipse.Eval(float64(i))
		if math.Abs(plane.SignedDistance(p)) > 1e-13 || math.Abs(sphere.SignedDistance(p)) > 1e-13 {
			t.Fatal(p)
		}
	}
	plane, _ = NewPlane(Vec3{0, 0, 8}, Vec3{0, 0, 1})
	result, e = IntersectPlaneSphere(plane, sphere, 1e-10)
	if e != nil || result.Kind != IntersectionPoint || !result.Tangent {
		t.Fatal(result, e)
	}
	plane, _ = NewPlane(Vec3{0, 0, 9}, Vec3{0, 0, 1})
	result, e = IntersectPlaneSphere(plane, sphere, 1e-10)
	if e != nil || result.Kind != IntersectionNone {
		t.Fatal(result, e)
	}
}
func TestPlaneCylinderSections(t *testing.T) {
	frame, _ := FrameFromNormal(Vec3{1, 2, 3}, Vec3{0, 0, 1})
	cylinder, _ := NewCylinderSurface(frame, 2)
	for _, normal := range []Vec3{{0, 0, 1}, {1, 0, 1}, {-1, 2, -3}} {
		plane, _ := NewPlane(Vec3{4, 5, 6}, normal)
		result, e := IntersectPlaneCylinder(plane, cylinder, IntersectionTolerance{1e-10, 1e-12})
		if e != nil || result.Kind != IntersectionCircle {
			t.Fatal(result, e)
		}
		for i := 0; i < 30; i++ {
			p, _ := result.Ellipse.Eval(float64(i))
			local := frame.ToLocal(p)
			if math.Abs(plane.SignedDistance(p)) > 1e-12 || math.Abs(math.Hypot(local.X, local.Y)-2) > 1e-12 {
				t.Fatal(p)
			}
		}
	}
	for _, tt := range []struct {
		x       float64
		lines   int
		tangent bool
	}{{1, 2, false}, {3, 1, true}, {4, 0, false}} {
		plane, _ := NewPlane(Vec3{tt.x, 0, 0}, Vec3{1, 0, 0})
		result, e := IntersectPlaneCylinder(plane, cylinder, IntersectionTolerance{1e-10, 1e-12})
		if e != nil || len(result.Lines) != tt.lines || result.Tangent != tt.tangent {
			t.Fatal(tt, result, e)
		}
		for _, line := range result.Lines {
			for _, u := range []float64{-10, 0, 10} {
				p, _ := line.Eval(u)
				local := frame.ToLocal(p)
				if math.Abs(plane.SignedDistance(p)) > 1e-12 || math.Abs(math.Hypot(local.X, local.Y)-2) > 1e-12 {
					t.Fatal(p)
				}
			}
		}
	}
	plane, _ := NewPlane(Vec3{}, Vec3{1, 0, 1e-15})
	if _, e := IntersectPlaneCylinder(plane, cylinder, IntersectionTolerance{1e-10, 1e-12}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
