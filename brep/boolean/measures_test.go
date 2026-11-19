package boolean

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// slanted is a cylinder about any axis, as a patch solid.
func slanted(t *testing.T, origin, axis geom.Vec3, radius, height float64) trimmed.Solid {
	t.Helper()
	frame, err := geom.FrameFromNormal(origin, axis)
	if err != nil {
		t.Fatal(err)
	}
	c, err := parametric.Cylinder(frame, radius, height)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromParametric(c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestMeasures compares the closed forms of volume, area and centroid
// with values worked out by hand: faces with straight edges, with arcs,
// with a seam, walls that go part of the way round, and the elliptic
// rims of an oblique bore. TestMatrix compares the same closed forms
// with numerical integration on every leaf.
func TestMeasures(t *testing.T) {
	root := math.Sqrt(17)
	bored := 8000 - 5*root*math.Pi // a box of side 20 less a bore of radius 1 along (1,0,4)
	for _, c := range []struct {
		name         string
		a, b         trimmed.Solid
		op           Operation
		volume, area float64
		centroid     geom.Vec3
	}{
		{"a box", lifted(t, box(t, 1, 2, -1, 3, 5, 4)), trimmed.Solid{}, Union, 30, 62, geom.Vec3{X: 2, Y: 3.5, Z: 1.5}},
		{"a cylinder that leans", slanted(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 1, Y: 2, Z: 2}, 1, 6), trimmed.Solid{}, Union,
			6 * math.Pi, 14 * math.Pi, geom.Vec3{X: 2, Y: 3, Z: 3}},
		{"a box with a bore", lifted(t, box(t, 0, 0, 0, 4, 4, 4)), tube(t, 2, 2, -1, 1, 6), Cut,
			64 - 4*math.Pi, 96 + 6*math.Pi, geom.Vec3{X: 2, Y: 2, Z: 2}},
		{"half a cylinder", lifted(t, box(t, 0, 0, 0, 20, 20, 8)), tube(t, 10, 0, 0, 3, 5), Common,
			22.5 * math.Pi, 24*math.Pi + 30, geom.Vec3{X: 10, Y: 4 / math.Pi, Z: 2.5}},
		{"a box with an oblique bore", lifted(t, box(t, 0, 0, 0, 20, 20, 20)), slanted(t, geom.Vec3{X: 1, Y: 4, Z: -4}, geom.Vec3{X: 1, Z: 4}, 1, 30), Cut,
			bored, 2400 - root*math.Pi/2 + 10*root*math.Pi,
			geom.Vec3{X: (80000 - 4.5*5*root*math.Pi) / bored, Y: (80000 - 4*5*root*math.Pi) / bored, Z: 10}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := Combine(context.Background(), c.a, c.b, c.op, matrixOptions)
			if err != nil {
				t.Fatal(err)
			}
			p := r.Properties
			if math.Abs(p.Volume-c.volume) > 1e-10*c.volume || math.Abs(p.Area-c.area) > 1e-10*c.area || p.Centroid.Distance(c.centroid) > 1e-10 {
				t.Fatalf("volume %.14g, area %.14g, centroid %v; want %.14g, %.14g, %v", p.Volume, p.Area, p.Centroid, c.volume, c.area, c.centroid)
			}
			// The operand is measured by the same closed form.
			if c.op == Union && math.Abs(r.VolumeA-c.volume) > 1e-10*c.volume {
				t.Fatalf("operand volume %.14g, want %.14g", r.VolumeA, c.volume)
			}
		})
	}
}

// TestSeries covers the antiderivative of tᵐ·e^{ikt} against Gauss
// quadrature of the same function, and a product that leaves the table.
func TestSeries(t *testing.T) {
	x, y, length := path{base: geom.Vec2{X: 0.3, Y: -1}, lin: geom.Vec2{X: 0.7, Y: 0.2}, cos: geom.Vec2{X: 1.5, Y: 0.4}, sin: geom.Vec2{X: -0.6, Y: 2}, t0: 0.4, t1: 2.9}.along()
	product := x.times(x).times(y).times(y.slope())
	p := path{base: geom.Vec2{X: 0.3, Y: -1}, lin: geom.Vec2{X: 0.7, Y: 0.2}, cos: geom.Vec2{X: 1.5, Y: 0.4}, sin: geom.Vec2{X: -0.6, Y: 2}, t0: 0.4, t1: 2.9}
	want := p.integral(64, func(q, d geom.Vec2) float64 { return q.X * q.X * q.Y * d.Y })
	if got := product.over(length); product.lost || math.Abs(got-want) > 1e-11 {
		t.Fatalf("antiderivative gives %.15g, quadrature %.15g", got, want)
	}
	high := x
	for range 6 {
		high = high.times(x)
	}
	if !high.lost {
		t.Fatal("a product beyond the table is not reported")
	}
}
