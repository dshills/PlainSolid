package query

import (
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"testing"
)

func cube(t *testing.T) *Index {
	t.Helper()
	m, e := mesh.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	q, e := New(m)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func TestRay(t *testing.T) {
	q := cube(t)
	h, ok, e := q.Ray(geom.Vec3{X: -2, Y: 1, Z: 1}, geom.Vec3{X: 4, Y: 0, Z: 0}, 10)
	if e != nil || !ok || h.Distance != 2 || h.Point != (geom.Vec3{X: 0, Y: 1, Z: 1}) {
		t.Fatal(h, ok, e)
	}
	if math.Abs(h.Barycentric.X+h.Barycentric.Y+h.Barycentric.Z-1) > 1e-14 {
		t.Fatal(h)
	}
	for _, tt := range []struct {
		o, d geom.Vec3
		max  float64
		hit  bool
	}{{geom.Vec3{X: -2, Y: 1, Z: 1}, geom.Vec3{X: 1, Y: 0, Z: 0}, 1, false}, {geom.Vec3{X: -2, Y: 1, Z: 1}, geom.Vec3{X: 1, Y: 0, Z: 0}, 2, true}, {geom.Vec3{X: -2, Y: 3, Z: 1}, geom.Vec3{X: 1, Y: 0, Z: 0}, 10, false}, {geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 1, Y: 0, Z: 0}, 10, true}} {
		_, ok, e = q.Ray(tt.o, tt.d, tt.max)
		if e != nil || ok != tt.hit {
			t.Fatal(tt, ok, e)
		}
	}
}
func TestClosest(t *testing.T) {
	q := cube(t)
	for _, tt := range []struct {
		p geom.Vec3
		d float64
	}{{geom.Vec3{X: -2, Y: 1, Z: 1}, 2}, {geom.Vec3{X: 1, Y: 1, Z: 1}, 1}, {geom.Vec3{X: 3, Y: 3, Z: 3}, math.Sqrt(3)}, {geom.Vec3{X: 3, Y: 3, Z: 1}, math.Sqrt(2)}, {geom.Vec3{X: 0, Y: 0, Z: 0}, 0}} {
		h, ok, e := q.Closest(tt.p)
		if e != nil || !ok || math.Abs(h.Distance-tt.d) > 1e-12 {
			t.Fatal(tt, h, e)
		}
	}
}
func TestClassify(t *testing.T) {
	q := cube(t)
	for _, tt := range []struct {
		p    geom.Vec3
		want Location
	}{{geom.Vec3{X: 1, Y: 1, Z: 1}, Inside}, {geom.Vec3{X: 0, Y: 1, Z: 1}, Boundary}, {geom.Vec3{X: -1, Y: 1, Z: 1}, Outside}, {geom.Vec3{X: 2 + 1e-10, Y: 1, Z: 1}, Boundary}} {
		v, e := q.Classify(tt.p, 1e-9)
		if e != nil || v != tt.want {
			t.Fatal(tt, v, e)
		}
	}
}
func TestTreeAndOwnership(t *testing.T) {
	m, _ := mesh.Sphere(geom.Vec3{}, 1, 32, 16)
	q, e := New(m)
	if e != nil {
		t.Fatal(e)
	}
	m.Vertices[0] = geom.Vec3{X: 1000, Y: 1000, Z: 1000}
	h, ok, e := q.Ray(geom.Vec3{X: 0, Y: 0, Z: 3}, geom.Vec3{X: 0, Y: 0, Z: -1}, math.Inf(1))
	if e != nil || !ok || math.Abs(h.Distance-2) > 1e-12 {
		t.Fatal(h, ok, e)
	}
	h, ok, e = q.Closest(geom.Vec3{X: 0, Y: 0, Z: 3})
	if e != nil || !ok || math.Abs(h.Distance-2) > 1e-12 {
		t.Fatal(h, ok, e)
	}
}
func TestEmptyInvalid(t *testing.T) {
	q, e := New(mesh.Mesh{})
	if e != nil {
		t.Fatal(e)
	}
	if _, ok, e := q.Closest(geom.Vec3{}); ok || e != nil {
		t.Fatal(ok, e)
	}
	if _, ok, e := q.Ray(geom.Vec3{}, geom.Vec3{X: 1, Y: 0, Z: 0}, 1); ok || e != nil {
		t.Fatal(ok, e)
	}
	if v, e := q.Classify(geom.Vec3{}, 0); v != Outside || e != nil {
		t.Fatal(v, e)
	}
	if _, _, e := q.Ray(geom.Vec3{}, geom.Vec3{}, 1); e == nil {
		t.Fatal("zero direction")
	}
	if _, e := q.Classify(geom.Vec3{}, -1); e == nil {
		t.Fatal("negative tolerance")
	}
	if _, _, e := q.Closest(geom.Vec3{X: math.NaN(), Y: 0, Z: 0}); e == nil {
		t.Fatal("NaN")
	}
	if _, e := New(mesh.Mesh{Triangles: [][3]int{{0, 1, 2}}}); e == nil {
		t.Fatal("indices")
	}
}
func TestCavity(t *testing.T) {
	a, _ := mesh.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	b, _ := mesh.Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	b = b.Reversed()
	offset := len(a.Vertices)
	a.Vertices = append(a.Vertices, b.Vertices...)
	for _, f := range b.Triangles {
		a.Triangles = append(a.Triangles, [3]int{f[0] + offset, f[1] + offset, f[2] + offset})
	}
	q, e := New(a)
	if e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct {
		p geom.Vec3
		v Location
	}{{geom.Vec3{X: 2, Y: 2, Z: 2}, Outside}, {geom.Vec3{X: .5, Y: 2, Z: 2}, Inside}} {
		v, e := q.Classify(tt.p, 1e-9)
		if e != nil || v != tt.v {
			t.Fatal(v, e)
		}
	}
}
func BenchmarkClosest(b *testing.B) {
	m, _ := mesh.Sphere(geom.Vec3{}, 1, 64, 32)
	q, _ := New(m)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = q.Closest(geom.Vec3{X: 2, Y: 3, Z: 4})
	}
}

func FuzzTriangleQueries(f *testing.F) {
	f.Add(int16(0), .25, .25)
	f.Add(int16(-150), .1, .4)
	f.Add(int16(150), .3, .3)
	f.Fuzz(func(t *testing.T, e int16, u, v float64) {
		if e < -150 || e > 150 || math.IsNaN(u) || math.IsNaN(v) || u < .001 || v < .001 || u+v > .99 {
			return
		}
		s := math.Pow10(int(e))
		m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: s}, {Y: s}}, Triangles: [][3]int{{0, 1, 2}}}
		q, err := New(m)
		if err != nil {
			t.Fatal(err)
		}
		p := geom.Vec3{X: s * u, Y: s * v, Z: s}
		expected := geom.Vec3{X: p.X, Y: p.Y}
		h, ok, err := q.Closest(p)
		if err == nil && (!ok || math.Abs(h.Distance/s-1) > 1e-10 || h.Point.Sub(expected).Length()/s > 1e-10) {
			t.Fatalf("closest at %g: %+v %v", s, h, ok)
		}
		h, ok, err = q.Ray(p, geom.Vec3{Z: -1}, math.Inf(1))
		if err == nil && (!ok || math.Abs(h.Distance/s-1) > 1e-10 || h.Point.Sub(expected).Length()/s > 1e-10) {
			t.Fatalf("ray at %g: %+v %v", s, h, ok)
		}
	})
}
