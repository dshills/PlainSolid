package geom

import (
	"math"
	"testing"
)

func near(t *testing.T, got, want Vec3, tol float64) {
	t.Helper()
	if got.Distance(want) > tol {
		t.Fatalf("got %v, want %v (tolerance %g)", got, want, tol)
	}
}
func TestVector(t *testing.T) {
	a := Vec3{1, 2, 3}
	b := Vec3{4, 5, 6}
	near(t, a.Cross(b), Vec3{-3, 6, -3}, 0)
	if a.Dot(b) != 32 {
		t.Fatal("dot")
	}
	for _, v := range []Vec3{{1e308, 1e308, 1e308}, {1e-320, 0, 0}} {
		u, e := v.Unit()
		if e != nil || math.Abs(u.Length()-1) > 1e-15 {
			t.Fatal(u, e)
		}
	}
	if _, e := (Vec3{}).Unit(); e == nil {
		t.Fatal("zero vector accepted")
	}
	if _, e := (Vec3{math.NaN(), 0, 0}).Unit(); e == nil {
		t.Fatal("nan accepted")
	}
}
func TestBounds(t *testing.T) {
	b, e := Bounds([]Vec3{{-2, 3, 0}, {4, -1, 5}})
	if e != nil || b.Volume() != 120 || !b.Contains(Vec3{}) {
		t.Fatal(b, e)
	}
	c, ok := b.Intersect(Box{Vec3{4, 0, 0}, Vec3{5, 1, 1}})
	if !ok || c.Volume() != 0 {
		t.Fatal(c, ok)
	}
	if _, e = Bounds(nil); e == nil {
		t.Fatal("empty accepted")
	}
}
func TestOrient2D(t *testing.T) {
	cases := []struct {
		a, b, c Vec2
		want    int
	}{{Vec2{0, 0}, Vec2{1, 0}, Vec2{0, 1}, 1}, {Vec2{0, 0}, Vec2{0, 1}, Vec2{1, 0}, -1}, {Vec2{1e308, 1e308}, Vec2{-1e308, -1e308}, Vec2{0, 1}, -1}, {Vec2{0, 0}, Vec2{1, 1}, Vec2{2, 2}, 0}, {Vec2{0, 0}, Vec2{1e-300, 0}, Vec2{0, 1e-300}, 1}}
	for _, c := range cases {
		got, e := Orient2D(c.a, c.b, c.c)
		if e != nil || got != c.want {
			t.Fatal(c, got, e)
		}
	}
}
func TestClosestPlane(t *testing.T) {
	p, u, e := ClosestOnSegment(Vec3{3, 2, 0}, Vec3{}, Vec3{2, 0, 0})
	if e != nil || u != 1 {
		t.Fatal(p, u, e)
	}
	near(t, p, Vec3{2, 0, 0}, 0)
	plane, e := NewPlane(Vec3{0, 0, 2}, Vec3{0, 0, 4})
	if e != nil {
		t.Fatal(e)
	}
	near(t, plane.Project(Vec3{1, 2, 8}), Vec3{1, 2, 2}, 0)
	p, u, ok, e := plane.IntersectRay(Vec3{}, Vec3{0, 0, 2}, 1e-12)
	if e != nil || !ok || u != 1 {
		t.Fatal(p, u, ok, e)
	}
	near(t, p, Vec3{0, 0, 2}, 0)
}
func TestAffine(t *testing.T) {
	r, e := Rotation(Vec3{0, 0, 3}, math.Pi/2)
	if e != nil {
		t.Fatal(e)
	}
	near(t, r.Apply(Vec3{1, 0, 0}), Vec3{0, 1, 0}, 1e-15)
	a := Translation(Vec3{3, -4, 2}).Mul(r).Mul(Scaling(Vec3{2, 3, 4}))
	inv, e := a.Inverse()
	if e != nil {
		t.Fatal(e)
	}
	v := Vec3{7, -8, 1}
	near(t, inv.Apply(a.Apply(v)), v, 1e-13)
	n, e := a.ApplyNormal(Vec3{1, 0, 0})
	if e != nil {
		t.Fatal(e)
	}
	near(t, n, Vec3{0, 1, 0}, 1e-15)
	if _, e = (Affine{}).Inverse(); e == nil {
		t.Fatal("singular accepted")
	}
	box, e := a.ApplyBox(Box{Vec3{}, Vec3{1, 1, 1}})
	if e != nil || math.Abs(box.Volume()-24) > 1e-12 {
		t.Fatal(box, e)
	}
}
func TestRationalQuarterCircle(t *testing.T) {
	poles := []Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}
	c, e := NewBSpline(2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{1, math.Sqrt(.5), 1})
	if e != nil {
		t.Fatal(e)
	}
	poles[0] = Vec3{99, 99, 99}
	for i := 0; i <= 100; i++ {
		u := float64(i) / 100
		p, e := c.Eval(u)
		if e != nil || math.Abs(p.Length()-1) > 3e-16 {
			t.Fatal(u, p, e)
		}
		d, e := c.Derivative(u)
		if e != nil || math.Abs(p.Dot(d)) > 1e-15 {
			t.Fatal(u, p, d, e)
		}
	}
	p, _ := c.Eval(.5)
	near(t, p, Vec3{math.Sqrt(.5), math.Sqrt(.5), 0}, 1e-15)
	d, _ := c.Derivative(0)
	near(t, d, Vec3{0, math.Sqrt(2), 0}, 1e-15)
	if _, e = c.Eval(-.1); e == nil {
		t.Fatal("extrapolation accepted")
	}
}
func TestSplineRepeatedKnotsAndDerivative(t *testing.T) {
	c, e := NewBSpline(2, []Vec3{{0, 0, 0}, {1, 2, 0}, {2, 1, 0}, {3, 2, 0}, {4, 0, 0}}, []float64{0, 0, 0, .5, .5, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []float64{.1, .3, .7, .9} {
		p, _ := c.Eval(u + 1e-6)
		q, _ := c.Eval(u - 1e-6)
		d, e := c.Derivative(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, d, p.Sub(q).Scale(5e5), 1e-8)
	}
	p, e := c.Eval(.5)
	if e != nil {
		t.Fatal(e)
	}
	near(t, p, Vec3{2, 1, 0}, 0)
	points, e := c.Sample(5)
	if e != nil || len(points) != 5 {
		t.Fatal(e)
	}
	near(t, points[4], Vec3{4, 0, 0}, 0)
}
func TestSplineUnclamped(t *testing.T) {
	c, e := NewBSpline(2, []Vec3{{0, 0, 0}, {2, 0, 0}, {4, 0, 0}}, []float64{-2, -1, 0, 1, 2, 3}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := c.Eval(0)
	near(t, p, Vec3{1, 0, 0}, 0)
	p, _ = c.Eval(1)
	near(t, p, Vec3{3, 0, 0}, 0)
	d, _ := c.Derivative(1)
	near(t, d, Vec3{2, 0, 0}, 0)
}
func TestSplineRejectsInvalid(t *testing.T) {
	cases := []struct {
		d    int
		p    []Vec3
		k, w []float64
	}{{0, []Vec3{{}, {}}, []float64{0, 0, 1, 1}, nil}, {1, []Vec3{{}, {}}, []float64{0, 1, 0, 1}, nil}, {1, []Vec3{{}, {}}, []float64{0, 0, 0, 0}, nil}, {1, []Vec3{{}, {}}, []float64{0, 0, 1, 1}, []float64{0, 1}}, {1, []Vec3{{math.Inf(1), 0, 0}, {}}, []float64{0, 0, 1, 1}, nil}}
	for _, c := range cases {
		if _, e := NewBSpline(c.d, c.p, c.k, c.w); e == nil {
			t.Fatal(c)
		}
	}
}
func TestSurface(t *testing.T) {
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 2, 0}, {3, 0, 0}, {3, 2, 0}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.Eval(.25, .75)
	if e != nil {
		t.Fatal(e)
	}
	near(t, p, Vec3{.75, 1.5, 0}, 1e-15)
	du, dv, e := s.Partials(.2, .6)
	if e != nil {
		t.Fatal(e)
	}
	near(t, du, Vec3{3, 0, 0}, 1e-15)
	near(t, dv, Vec3{0, 2, 0}, 1e-15)
	n, e := s.Normal(.5, .5)
	if e != nil {
		t.Fatal(e)
	}
	near(t, n, Vec3{0, 0, 1}, 1e-15)
}
func TestRationalSurface(t *testing.T) {
	w := math.Sqrt(.5)
	s, e := NewBSplineSurface(2, 1, 3, 2, []Vec3{{1, 0, 0}, {1, 0, 2}, {1, 1, 0}, {1, 1, 2}, {0, 1, 0}, {0, 1, 2}}, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, w, w, 1, 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []float64{0, .2, .5, .8, 1} {
		p, e := s.Eval(u, .3)
		if e != nil || math.Abs(math.Hypot(p.X, p.Y)-1) > 4e-16 || math.Abs(p.Z-.6) > 1e-15 {
			t.Fatal(p, e)
		}
		du, dv, e := s.Partials(u, .3)
		if e != nil || math.Abs(du.Dot(Vec3{p.X, p.Y, 0})) > 1e-15 {
			t.Fatal(du, e)
		}
		near(t, dv, Vec3{0, 0, 2}, 1e-15)
	}
}
func FuzzOrientAntisymmetry(f *testing.F) {
	f.Add(0.0, 0.0, 1.0, 0.0, 0.0, 1.0)
	f.Fuzz(func(t *testing.T, ax, ay, bx, by, cx, cy float64) {
		a, b, c := Vec2{ax, ay}, Vec2{bx, by}, Vec2{cx, cy}
		x, e := Orient2D(a, b, c)
		if e != nil {
			return
		}
		y, e := Orient2D(b, a, c)
		if e != nil || x != -y {
			t.Fatal(x, y, e)
		}
	})
}
func BenchmarkSplineEval(b *testing.B) {
	c, _ := NewBSpline(2, []Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, math.Sqrt(.5), 1})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = c.Eval(.3)
	}
}

func BenchmarkSplineSurfaceEval(b *testing.B) {
	benchmarkSplineSurface(b, 0)
}

func BenchmarkSplineSurfacePartials(b *testing.B) {
	benchmarkSplineSurface(b, 1)
}

func BenchmarkSplineSurfaceSecondPartials(b *testing.B) {
	benchmarkSplineSurface(b, 2)
}

func benchmarkSplineSurface(b *testing.B, order int) {
	knots := []float64{0, 0, 0, .25, .5, .75, 1, 1, 1}
	poles := make([]Vec3, 36)
	weights := make([]float64, len(poles))
	for i := range poles {
		x, y := float64(i/6), float64(i%6)
		poles[i], weights[i] = Vec3{x, y, x * y / 8}, 1+float64(i%5)/8
	}
	s, e := NewBSplineSurface(2, 2, 6, 6, poles, knots, knots, weights)
	if e != nil {
		b.Fatal(e)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u, v := float64(i%101)/100, float64(i%97)/96
		switch order {
		case 2:
			_, _, _, _ = s.SecondPartials(u, v)
		case 1:
			_, _, _ = s.Partials(u, v)
		default:
			_, _ = s.Eval(u, v)
		}
	}
}

func TestSplineSurfacePointUsesOriginalKnotSides(t *testing.T) {
	// A full-multiplicity knot has different limits. At the knot use the
	// right chart, and at the domain maximum use the final left chart.
	knots := []float64{0, 0, .5, .5, 1, 1}
	poles := make([]Vec3, 16)
	for i, x := range []float64{0, 1, 10, 11} {
		for j, y := range []float64{0, 2, 20, 22} {
			poles[4*i+j] = Vec3{x, y, x + y}
		}
	}
	s, e := NewBSplineSurface(1, 1, 4, 4, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		u, v float64
		want Vec3
	}{
		{0, 0, Vec3{}}, {.25, .25, Vec3{.5, 1, 1.5}},
		{.5, .25, Vec3{10, 1, 11}}, {.25, .5, Vec3{.5, 20, 20.5}},
		{.5, .5, Vec3{10, 20, 30}}, {1, 1, Vec3{11, 22, 33}},
	} {
		got, err := s.Eval(c.u, c.v)
		if err != nil || got != c.want {
			t.Fatalf("(%v, %v): got %v, %v; want %v", c.u, c.v, got, err, c.want)
		}
	}
}

func TestSplineSurfaceRationalPartials(t *testing.T) {
	// Separable weights give S=(x,y,xy), x=2u/(1+u), y=3v/(1+2v).
	knots := []float64{0, 0, 1, 1}
	s, err := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {0, 1, 0}, {1, 0, 0}, {1, 1, 1}}, knots, knots, []float64{1, 3, 2, 6})
	if err != nil {
		t.Fatal(err)
	}
	for _, uv := range []Vec2{{0, 0}, {.25, .6}, {1, 1}} {
		u, v := uv.X, uv.Y
		x, y := 2*u/(1+u), 3*v/(1+2*v)
		dx, dy := 2/((1+u)*(1+u)), 3/((1+2*v)*(1+2*v))
		du, dv, err := s.Partials(u, v)
		if err != nil || du.Sub(Vec3{dx, 0, dx * y}).Length() > 2e-14 || dv.Sub(Vec3{0, dy, x * dy}).Length() > 2e-14 {
			t.Fatalf("%v: du=%v dv=%v err=%v", uv, du, dv, err)
		}
	}
}
