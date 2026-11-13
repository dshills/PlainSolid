package geom

import (
	"math"
	"testing"
)

func TestFrame(t *testing.T) {
	f, e := NewFrame(Vec3{3, 4, 5}, Vec3{1, 0, 1}, Vec3{0, 0, 2})
	if e != nil {
		t.Fatal(e)
	}
	near(t, f.XAxis(), Vec3{1, 0, 0}, 0)
	near(t, f.YAxis(), Vec3{0, 1, 0}, 0)
	near(t, f.ZAxis(), Vec3{0, 0, 1}, 0)
	p := Vec3{5, 6, 7}
	near(t, f.ToLocal(f.ToWorld(p)), p, 0)
	if _, e = NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{2, 0, 0}); e == nil {
		t.Fatal("parallel axes")
	}
	f, e = FrameFromNormal(Vec3{}, Vec3{1, 0, 0})
	if e != nil || math.Abs(f.XAxis().Dot(f.ZAxis())) > 1e-15 {
		t.Fatal(f, e)
	}
}
func TestAnalyticCurves(t *testing.T) {
	l, e := NewLine(Vec3{1, 0, 0}, Vec3{0, 2, 0})
	if e != nil {
		t.Fatal(e)
	}
	p, e := l.Eval(3)
	if e != nil {
		t.Fatal(e)
	}
	near(t, p, Vec3{1, 3, 0}, 0)
	d, _ := l.Derivative(0)
	near(t, d, Vec3{0, 1, 0}, 0)
	c, e := NewCircle(Vec3{1, 2, 3}, Vec3{0, 0, 1}, 2)
	if e != nil {
		t.Fatal(e)
	}
	p, _ = c.Eval(math.Pi / 2)
	near(t, p, Vec3{1, 4, 3}, 1e-15)
	d, _ = c.Derivative(0)
	near(t, d, Vec3{0, 2, 0}, 0)
	s, e := c.RationalBSpline()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i <= 100; i++ {
		p, e := s.Eval(float64(i) / 25)
		if e != nil || math.Abs(p.Distance(Vec3{1, 2, 3})-2) > 1e-15 {
			t.Fatal(p, e)
		}
	}
	f, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	ellipse, e := NewEllipse(f, 3, 2)
	if e != nil {
		t.Fatal(e)
	}
	p, _ = ellipse.Eval(.7)
	if math.Abs(p.X*p.X/9+p.Y*p.Y/4-1) > 1e-15 {
		t.Fatal(p)
	}
	if _, e = NewCircle(Vec3{}, Vec3{}, 1); e == nil {
		t.Fatal("bad axis")
	}
	if _, e = NewEllipse(Frame{}, 1, 2); e == nil {
		t.Fatal("bad frame")
	}
	if _, e = c.Eval(math.NaN()); e == nil {
		t.Fatal("nan")
	}
}
func TestAnalyticSurfaces(t *testing.T) {
	f, _ := FrameFromNormal(Vec3{3, 4, 5}, Vec3{1, 2, 3})
	plane, _ := NewPlaneSurface(f)
	cylinder, _ := NewCylinderSurface(f, 2)
	cone, _ := NewConeSurface(f, .4)
	sphere, _ := NewSphereSurface(f, 3)
	torus, _ := NewTorusSurface(f, 4, 1)
	surfaces := []Surface{plane, cylinder, cone, sphere, torus}
	for _, s := range surfaces {
		u, v := .6, .7
		du, dv, e := s.Partials(u, v)
		if e != nil {
			t.Fatal(e)
		}
		p, _ := s.Eval(u+1e-6, v)
		q, _ := s.Eval(u-1e-6, v)
		near(t, du, p.Sub(q).Scale(5e5), 2e-9)
		p, _ = s.Eval(u, v+1e-6)
		q, _ = s.Eval(u, v-1e-6)
		near(t, dv, p.Sub(q).Scale(5e5), 2e-9)
	}
	p, _ := sphere.Eval(1, .5)
	if math.Abs(p.Distance(f.Origin())-3) > 1e-14 {
		t.Fatal(p)
	}
	p, _ = cylinder.Eval(.2, 7)
	local := f.ToLocal(p)
	near(t, Vec3{math.Hypot(local.X, local.Y), local.Z, 0}, Vec3{2, 7, 0}, 1e-14)
	p, _ = torus.Eval(.3, .8)
	local = f.ToLocal(p)
	if math.Abs(math.Hypot(math.Hypot(local.X, local.Y)-4, local.Z)-1) > 1e-14 {
		t.Fatal(p)
	}
	p, _ = cone.Eval(.8, 2)
	local = f.ToLocal(p)
	if math.Abs(math.Hypot(local.X, local.Y)-2*math.Tan(.4)) > 1e-14 {
		t.Fatal(p)
	}
}
func TestAnalyticRejectsInvalid(t *testing.T) {
	f, _ := FrameFromNormal(Vec3{}, Vec3{0, 0, 1})
	if _, e := NewPlaneSurface(Frame{}); e == nil {
		t.Fatal("frame")
	}
	if _, e := NewCylinderSurface(f, -1); e == nil {
		t.Fatal("radius")
	}
	if _, e := NewConeSurface(f, math.Pi/2); e == nil {
		t.Fatal("angle")
	}
	if _, e := NewSphereSurface(f, math.Inf(1)); e == nil {
		t.Fatal("radius")
	}
	if _, e := NewTorusSurface(f, 1, 2); e == nil {
		t.Fatal("self intersecting")
	}
	for _, s := range []Surface{PlaneSurface{}, CylinderSurface{}, ConeSurface{}, SphereSurface{}, TorusSurface{}} {
		if _, e := s.Eval(0, 0); e == nil {
			t.Fatal("zero surface")
		}
		if _, _, e := s.Partials(0, 0); e == nil {
			t.Fatal("zero surface derivative")
		}
	}
	sphere, _ := NewSphereSurface(f, 1)
	if _, e := sphere.Eval(0, math.Pi); e == nil {
		t.Fatal("latitude")
	}
	cone, _ := NewConeSurface(f, .4)
	if _, e := cone.Eval(0, -1); e == nil {
		t.Fatal("cone parameter")
	}
}
