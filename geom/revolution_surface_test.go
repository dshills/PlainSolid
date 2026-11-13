package geom

import (
	"math"
	"testing"
)

func TestRevolutionSurfaceParameters(t *testing.T) {
	c, e := NewBSpline(2, []Vec3{{X: 2}, {X: 4, Z: 1}, {X: 3, Z: 3}}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, axis := range []Vec3{{Z: 1}, {Z: -1}} {
		s, e := NewRevolutionSurface(c, Vec3{}, axis, -.5, 1.25)
		if e != nil {
			t.Fatal(e)
		}
		u, v := .5, .5
		p, e := s.Eval(u, v)
		sn, cs := math.Sincos(u)
		want := Vec3{X: 3.25 * cs, Y: axis.Z * 3.25 * sn, Z: 1.25}
		if e != nil || p.Distance(want) > 1e-14 {
			t.Fatal(p, want, e)
		}
		du, dv, e := s.Partials(u, v)
		if e != nil || du.Distance(Vec3{X: -3.25 * sn, Y: axis.Z * 3.25 * cs}) > 1e-14 || dv.Distance(Vec3{X: cs, Y: axis.Z * sn, Z: 3}) > 1e-14 {
			t.Fatal(du, dv, e)
		}
		if _, e = s.Eval(-1, v); e == nil {
			t.Fatal("range")
		}
	}
}
func TestRevolutionSurfaceRejects(t *testing.T) {
	c, _ := NewBSpline(1, []Vec3{{X: 2}, {X: 3, Z: 3}}, []float64{0, 0, 1, 1}, nil)
	for _, angle := range [][2]float64{{0, 2 * math.Pi}, {1, 0}, {-4, -3}, {-2, 2}, {0, math.NaN()}} {
		if _, e := NewRevolutionSurface(c, Vec3{}, Vec3{Z: 1}, angle[0], angle[1]); e == nil {
			t.Fatal(angle)
		}
	}
	for _, axis := range []Vec3{{}, {Z: 2}, {X: 1, Z: 1}} {
		if _, e := NewRevolutionSurface(c, Vec3{}, axis, 0, 1); e == nil {
			t.Fatal(axis)
		}
	}
	for _, poles := range [][]Vec3{{{X: 2}, {X: 3}}, {{}, {X: 3, Z: 1}}, {{X: 2}, {X: 3, Y: 1, Z: 1}}, {{X: 2}, {X: -3, Z: 1}}, {{X: 2, Y: 1}, {X: 3, Y: 1, Z: 1}}} {
		p, _ := NewBSpline(1, poles, []float64{0, 0, 1, 1}, nil)
		if _, e := NewRevolutionSurface(p, Vec3{}, Vec3{Z: 1}, 0, 1); e == nil {
			t.Fatal(poles)
		}
	}
	r, _ := NewBSpline(1, []Vec3{{X: 2}, {X: 3, Z: 1}}, []float64{0, 0, 1, 1}, []float64{1, .75})
	if _, e := NewRevolutionSurface(r, Vec3{}, Vec3{Z: 1}, 0, 1); e != nil {
		t.Fatal("certifiable rational meridian", e)
	}
}

func TestRevolutionSurfaceOwnedProfileAndKnotGate(t *testing.T) {
	c, _ := NewBSpline(1, []Vec3{{X: 2, Z: 3}, {X: 3}}, []float64{0, 0, 1, 1}, nil)
	s, e := NewRevolutionSurface(c, Vec3{}, Vec3{Z: 1}, -.5, 1)
	if e != nil {
		t.Fatal(e)
	}
	*c = BSpline{}
	copy := s.Profile()
	*copy = BSpline{}
	p, e := s.Eval(0, .5)
	if e != nil || p.Distance(Vec3{X: 2.5, Z: 1.5}) > 1e-14 {
		t.Fatal("profile alias", p, e)
	}
	_, dv, e := s.Partials(0, .5)
	if e != nil || dv.Z != -3 {
		t.Fatal("decreasing profile", dv, e)
	}
	discontinuous, _ := NewBSpline(1, []Vec3{{X: 2}, {X: 3, Z: 1}, {X: 4, Z: 2}, {X: 5, Z: 3}}, []float64{0, 0, .5, .5, 1, 1}, nil)
	if _, e = NewRevolutionSurface(discontinuous, Vec3{}, Vec3{Z: 1}, 0, 1); e == nil {
		t.Fatal("interior discontinuity")
	}
	folded, _ := NewBSpline(2, []Vec3{{X: 2}, {X: 3, Z: 2}, {X: 4, Z: 1}}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if _, e = NewRevolutionSurface(folded, Vec3{}, Vec3{Z: 1}, 0, 1); e == nil {
		t.Fatal("axial fold")
	}
}

func TestRevolutionSurfacePlacedAxis(t *testing.T) {
	c, _ := NewBSpline(1, []Vec3{{X: 1, Y: 4, Z: 3}, {X: 4, Y: 5, Z: 3}}, []float64{0, 0, 1, 1}, nil)
	s, e := NewRevolutionSurface(c, Vec3{X: 1, Y: 2, Z: 3}, Vec3{X: 1}, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	sn, cs := math.Sincos(.5)
	p, e := s.Eval(.5, .5)
	if e != nil || p.Distance(Vec3{X: 2.5, Y: 2 + 2.5*cs, Z: 3 + 2.5*sn}) > 1e-14 {
		t.Fatal(p, e)
	}
	du, dv, e := s.Partials(.5, .5)
	if e != nil || du.Distance(Vec3{Y: -2.5 * sn, Z: 2.5 * cs}) > 1e-14 || dv.Distance(Vec3{X: 3, Y: cs, Z: sn}) > 1e-14 {
		t.Fatal(du, dv, e)
	}
}
