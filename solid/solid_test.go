package solid

import (
	"errors"
	"math"
	"plainsolid/geom"
	"testing"
)

func box(x, y, z, X, Y, Z float64) geom.Box {
	return geom.Box{Min: geom.Vec3{X: x, Y: y, Z: z}, Max: geom.Vec3{X: X, Y: Y, Z: Z}}
}
func must(t *testing.T, b ...geom.Box) Solid {
	t.Helper()
	s, e := New(b...)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestBoolean(t *testing.T) {
	a := must(t, box(0, 0, 0, 2, 2, 2))
	b := must(t, box(1, 1, 1, 3, 3, 3))
	for _, tt := range []struct {
		op Operation
		v  float64
	}{{Union, 15}, {Intersection, 1}, {Difference, 7}} {
		s, e := a.Boolean(b, tt.op, Options{})
		if e != nil {
			t.Fatal(e)
		}
		if s.Volume() != tt.v {
			t.Fatalf("op %d volume %g", tt.op, s.Volume())
		}
		m, e := s.Mesh(Options{})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if math.Abs(m.SignedVolume()-tt.v) > 1e-10 {
			t.Fatalf("mesh volume %g", m.SignedVolume())
		}
	}
	if a.Volume() != 8 || b.Volume() != 8 {
		t.Fatal("mutated")
	}
}
func TestNormalization(t *testing.T) {
	s := must(t, box(0, 0, 0, 2, 2, 2), box(1, 0, 0, 3, 2, 2), box(0, 0, 0, 2, 2, 2))
	if s.Volume() != 12 {
		t.Fatal(s.Volume())
	}
	c, e := s.Centroid()
	if e != nil || c != (geom.Vec3{X: 1.5, Y: 1, Z: 1}) {
		t.Fatal(c, e)
	}
	if !s.Contains(geom.Vec3{X: 3, Y: 2, Z: 2}) || s.Contains(geom.Vec3{X: 4, Y: 0, Z: 0}) || s.Contains(geom.Vec3{X: math.NaN(), Y: 0, Z: 0}) {
		t.Fatal("containment")
	}
	bs := s.Cells()
	bs[0].Min.X = 100
	if s.Cells()[0].Min.X == 100 {
		t.Fatal("alias")
	}
	bounds, e := s.Bounds()
	if e != nil || bounds != box(0, 0, 0, 3, 2, 2) {
		t.Fatal(bounds, e)
	}
}
func TestEmptyTouchingContained(t *testing.T) {
	a := must(t, box(0, 0, 0, 1, 1, 1))
	b := must(t, box(1, 0, 0, 2, 1, 1))
	s, e := a.Intersection(b)
	if e != nil || !s.Empty() {
		t.Fatal(s, e)
	}
	u, e := a.Union(b)
	if e != nil || u.Volume() != 2 {
		t.Fatal(u, e)
	}
	m, e := u.Mesh(Options{})
	if e != nil || m.ValidateSolid() != nil || m.Area() != 10 {
		t.Fatal(m.Area(), e)
	}
	d, e := a.Difference(a)
	if e != nil || !d.Empty() {
		t.Fatal(d, e)
	}
	empty := Solid{}
	u, e = a.Union(empty)
	if e != nil || u.Volume() != 1 {
		t.Fatal(e)
	}
	if _, e = empty.Centroid(); e == nil {
		t.Fatal("empty centroid")
	}
	if _, e = empty.Bounds(); e == nil {
		t.Fatal("empty bounds")
	}
	m, e = empty.Mesh(Options{})
	if e != nil || len(m.Triangles) != 0 {
		t.Fatal(e)
	}
	inner := must(t, box(.2, .2, .2, .8, .8, .8))
	d, e = a.Difference(inner)
	if e != nil || math.Abs(d.Volume()-.784) > 1e-12 {
		t.Fatal(d.Volume(), e)
	}
	m, e = d.Mesh(Options{})
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-.784) > 1e-12 {
		t.Fatal("cavity", e)
	}
}
func TestInvalidAndLimits(t *testing.T) {
	for _, b := range []geom.Box{box(0, 0, 0, 0, 1, 1), box(1, 0, 0, 0, 1, 1), box(0, 0, 0, math.Inf(1), 1, 1), box(0, 0, 0, 1e200, 1e200, 1e200)} {
		if _, e := New(b); e == nil {
			t.Fatal("accepted", b)
		}
	}
	if _, e := NewWithOptions(Options{MaxCells: 1}, box(0, 0, 0, 2, 2, 2), box(1, 1, 1, 3, 3, 3)); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e := (Solid{}).Boolean(Solid{}, 99, Options{}); e == nil {
		t.Fatal("operation")
	}
	if _, e := NewWithOptions(Options{MaxCells: -1}); e == nil {
		t.Fatal("negative limit")
	}
}
func TestThinAdjacentFloatCells(t *testing.T) {
	x := 1.
	y := math.Nextafter(x, 2)
	s := must(t, box(x, 0, 0, y, 1, 1))
	if s.Volume() != y-x {
		t.Fatal(s.Volume())
	}
}
func TestVolumeIdentity(t *testing.T) {
	for i := 0; i < 30; i++ {
		x := float64(i) / 10
		a := must(t, box(0, 0, 0, 2, 3, 4))
		b := must(t, box(x, -1, .5, x+1, 2, 5))
		u, e := a.Union(b)
		if e != nil {
			t.Fatal(e)
		}
		v, e := a.Intersection(b)
		if e != nil {
			t.Fatal(e)
		}
		d, e := a.Difference(b)
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(u.Volume()+v.Volume()-a.Volume()-b.Volume()) > 1e-10 || math.Abs(d.Volume()+v.Volume()-a.Volume()) > 1e-10 {
			t.Fatal("identity", i)
		}
	}
}
func BenchmarkBoolean(b *testing.B) {
	a, _ := New(box(0, 0, 0, 2, 2, 2))
	c, _ := New(box(1, 1, 1, 3, 3, 3))
	for i := 0; i < b.N; i++ {
		_, _ = a.Union(c)
	}
}

func TestWorkBudget(t *testing.T) {
	b := box(0, 0, 0, 1, 1, 1)
	if _, err := NewWithOptions(Options{MaxWork: 1}, b, b); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, err := NewWithOptions(Options{MaxWork: -1}); err == nil {
		t.Fatal("negative work limit")
	}
}
