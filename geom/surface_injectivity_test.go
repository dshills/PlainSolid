package geom

import (
	"errors"
	"testing"
)

func TestSurfaceInjectivityGraphsAndReversal(t *testing.T) {
	for _, f := range []func(float64, float64) Vec3{func(u, v float64) Vec3 { return Vec3{u, v, u * v} }, func(u, v float64) Vec3 { return Vec3{v, u, u * v} }, func(u, v float64) Vec3 { return Vec3{u + 3*v, 2*u - v, u * v} }} {
		p := arcTestPatch(t, f)
		r, e := CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{})
		if e != nil || !r.ProvenInjective || !r.CenterInvertible {
			t.Fatal(r, e)
		}
	}
}
func TestSurfaceInjectivityRationalGraph(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	p, _ = p.Trim(ParameterRectangle{.1, .8, 0, 1})
	r, e := CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}})
	if e != nil || !r.ProvenInjective {
		t.Fatal(r, e)
	}
}
func TestSurfaceInjectivityFoldAndBudgets(t *testing.T) {
	poles := []Vec3{}
	for i, x := range []float64{.25, -.25, .25} {
		for j := 0; j < 2; j++ {
			poles = append(poles, Vec3{x, float64(j), float64(i) / 2})
		}
	}
	s, _ := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{})
	if !errors.Is(e, ErrConvergence) || r.ProvenInjective || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
	p = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	r, e = CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.ProvenInjective || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
}
func TestSurfaceInjectivityC0Knots(t *testing.T) {
	s, _ := NewBSplineSurface(1, 1, 3, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {.4, 0, 1}, {.4, 1, 1}, {1, 0, 0}, {1, 1, 0}}, []float64{0, 0, .4, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{})
	if e != nil || !r.ProvenInjective || len(r.Certified) != 2 {
		t.Fatal(r, e)
	}
}
