package geom

import (
	"errors"
	"math"
	"testing"
)

func TestSurfaceSecondPartialsPolynomial(t *testing.T) {
	s := quadraticContactSurface(t, 0)
	for _, u := range []float64{0, .2, .5, 1} {
		for _, v := range []float64{0, .37, 1} {
			uu, uv, vv, e := s.SecondPartials(u, v)
			if e != nil || uu.Distance(Vec3{Z: 2}) > 1e-12 || uv.Length() > 1e-12 || vv.Distance(Vec3{Z: 2}) > 1e-12 {
				t.Fatal(uu, uv, vv, e)
			}
		}
	}
	c, e := s.Curvature(.5, .5)
	if e != nil || math.Abs(c.Gaussian-4) > 1e-12 || math.Abs(c.Mean-2) > 1e-12 || math.Abs(c.PrincipalMin-2) > 1e-12 {
		t.Fatal(c, e)
	}
}
func TestSurfaceSecondPartialsRational(t *testing.T) {
	s := quarterCylinder(t)
	for _, u := range []float64{.1, .35, .75, .9} {
		uu, uv, vv, e := s.SecondPartials(u, .4)
		if e != nil {
			t.Fatal(e)
		}
		h := 1e-5
		a, _, _ := s.Partials(u+h, .4)
		b, _, _ := s.Partials(u-h, .4)
		if uu.Distance(a.Sub(b).Scale(1/(2*h))) > 1e-7 || uv.Length() > 1e-12 || vv.Length() > 1e-12 {
			t.Fatal(uu, uv, vv)
		}
		c, e := s.Curvature(u, .4)
		if e != nil || math.Abs(c.Gaussian) > 1e-12 || math.Abs(c.Mean+.25) > 1e-12 || math.Abs(c.PrincipalMin+.5) > 1e-12 || math.Abs(c.PrincipalMax) > 1e-12 {
			t.Fatal(c, e)
		}
	}
}
func TestSurfaceSecondPartialsBilinearRational(t *testing.T) {
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}, {1, 1, 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 2, 3, 4})
	if e != nil {
		t.Fatal(e)
	}
	u, v, h := .37, .62, 1e-5
	uu, uv, vv, e := s.SecondPartials(u, v)
	if e != nil {
		t.Fatal(e)
	}
	pu, pv, _ := s.Partials(u+h, v)
	mu, mv, _ := s.Partials(u-h, v)
	_, vp, _ := s.Partials(u, v+h)
	_, vm, _ := s.Partials(u, v-h)
	if uu.Distance(pu.Sub(mu).Scale(1/(2*h))) > 1e-8 || uv.Distance(pv.Sub(mv).Scale(1/(2*h))) > 1e-8 || vv.Distance(vp.Sub(vm).Scale(1/(2*h))) > 1e-8 {
		t.Fatal(uu, uv, vv)
	}
}
func TestSurfaceCurvatureSingularAndDomain(t *testing.T) {
	s, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {}, {X: 1}, {X: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if _, e := s.Curvature(.5, .5); !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	if _, _, _, e := s.SecondPartials(-1, 0); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}

func TestSurfaceSecondPartialsRepeatedKnotSides(t *testing.T) {
	poles := []Vec3{}
	for i, z := range []float64{0, 0, 0, 0, .25} {
		for j := 0; j < 2; j++ {
			poles = append(poles, Vec3{float64(i) / 4, float64(j), z})
		}
	}
	s, e := NewBSplineSurface(2, 1, 5, 2, poles, []float64{0, 0, 0, .5, .5, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ u, z float64 }{{.49, 0}, {.5, 2}, {.51, 2}, {1, 2}} {
		uu, uv, vv, e := s.SecondPartials(tc.u, .3)
		if e != nil || math.Abs(uu.Z-tc.z) > 1e-12 || uv.Length() > 1e-12 || vv.Length() > 1e-12 {
			t.Fatal(tc, uu, uv, vv, e)
		}
	}
}
