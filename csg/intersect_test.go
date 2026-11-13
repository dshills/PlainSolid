package csg

import (
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func box(t *testing.T, min, max geom.Vec3) brep.Solid {
	t.Helper()
	s, e := brep.Box(min, max)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func near(t *testing.T, a, b float64) {
	t.Helper()
	if math.Abs(a-b) > 1e-9*math.Max(1, math.Abs(b)) {
		t.Fatalf("got %.15g want %.15g", a, b)
	}
}
func intersect(t *testing.T, a, b brep.Solid) brep.Solid {
	t.Helper()
	s, e := Intersect(a, b, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Validate(); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestAxisAlignedIntersection(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	b := box(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 4, Y: 4, Z: 4})
	s := intersect(t, a, b)
	near(t, s.Tessellate().SignedVolume(), 6)
	if s.Topology().EulerCharacteristic != 2 {
		t.Fatal(s.Topology())
	}
	p, e := s.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Centroid.X, 1.5)
	near(t, p.Centroid.Y, 2)
	near(t, p.Centroid.Z, 2.5)
	r := intersect(t, b, a)
	near(t, r.Tessellate().SignedVolume(), 6)
}
func TestRotatedCube(t *testing.T) {
	a := box(t, geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	rotation, e := geom.Rotation(geom.Vec3{Z: 1}, math.Pi/4)
	if e != nil {
		t.Fatal(e)
	}
	b, e := a.Transformed(rotation)
	if e != nil {
		t.Fatal(e)
	}
	s := intersect(t, a, b)
	near(t, s.Tessellate().SignedVolume(), 16*(math.Sqrt2-1))
	top := s.Topology()
	if top.Vertices != 16 || top.Faces != 10 || top.EulerCharacteristic != 2 {
		t.Fatal(top)
	}
	r := intersect(t, b, a)
	near(t, r.Tessellate().SignedVolume(), s.Tessellate().SignedVolume())
}
func TestSelfContainmentAndEmpty(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	small := box(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 2, Y: 2, Z: 2})
	near(t, intersect(t, a, a).Tessellate().SignedVolume(), 64)
	near(t, intersect(t, a, small).Tessellate().SignedVolume(), 1)
	for _, min := range []geom.Vec3{{X: 5}, {X: 4}, {X: 4, Y: 4}, {X: 4, Y: 4, Z: 4}} {
		b := box(t, min, min.Add(geom.Vec3{X: 1, Y: 1, Z: 1}))
		s := intersect(t, a, b)
		if len(s.Vertices()) != 0 {
			t.Fatal("expected empty regularized intersection", s.Topology())
		}
	}
	if s := intersect(t, a, brep.Solid{}); len(s.Vertices()) != 0 {
		t.Fatal("empty input")
	}
}
func TestInvalidInputs(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	for _, tol := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, e := Intersect(a, a, tol); e == nil {
			t.Fatal("accepted bad tolerance")
		}
	}
	if _, e := Intersect(a, a.Reversed(), 1e-9); e == nil {
		t.Fatal("accepted inward solid")
	}
	concave, e := brep.Prism([]geom.Vec3{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Intersect(a, concave, 1e-9); e == nil {
		t.Fatal("accepted nonconvex solid")
	}
	points := make([]geom.Vec3, 64)
	for i := range points {
		angle := 2 * math.Pi * float64(i) / float64(len(points))
		points[i] = geom.Vec3{X: math.Cos(angle), Y: math.Sin(angle)}
	}
	many, e := brep.Prism(points, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Intersect(a, many, 1e-9); e == nil {
		t.Fatal("accepted work-limit overflow")
	}
}
func TestTranslationStability(t *testing.T) {
	offset := geom.Vec3{X: 1e9, Y: -1e9, Z: 1e9}
	a := box(t, offset, offset.Add(geom.Vec3{X: 2, Y: 2, Z: 2}))
	b := box(t, offset.Add(geom.Vec3{X: 1}), offset.Add(geom.Vec3{X: 3, Y: 2, Z: 2}))
	near(t, intersect(t, a, b).Tessellate().SignedVolume(), 4)
}
func TestThreeDimensionalRotation(t *testing.T) {
	a := box(t, geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	rot, e := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, 0.7)
	if e != nil {
		t.Fatal(e)
	}
	b, e := a.Transformed(rot)
	if e != nil {
		t.Fatal(e)
	}
	s := intersect(t, a, b)
	v := s.Tessellate().SignedVolume()
	if v <= 0 || v >= 8 {
		t.Fatal(v)
	}
	r := intersect(t, b, a)
	near(t, v, r.Tessellate().SignedVolume())
	for _, p := range s.Vertices() {
		for _, input := range []brep.Solid{a, b} {
			for i := range input.Faces() {
				plane, e := input.FacePlane(i)
				if e != nil {
					t.Fatal(e)
				}
				if plane.SignedDistance(p) > 1e-9 {
					t.Fatal("result vertex lies outside input")
				}
			}
		}
	}
}
func FuzzIntersectBoxes(f *testing.F) {
	f.Add(0.5, 0.25, 0.75)
	f.Fuzz(func(t *testing.T, x, y, z float64) {
		if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) || math.Abs(x) > 3 || math.Abs(y) > 3 || math.Abs(z) > 3 {
			return
		}
		a := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
		p := geom.Vec3{X: x, Y: y, Z: z}
		b := box(t, p, p.Add(geom.Vec3{X: 1, Y: 1, Z: 1}))
		s := intersect(t, a, b)
		expected := math.Max(0, 1-math.Abs(x)) * math.Max(0, 1-math.Abs(y)) * math.Max(0, 1-math.Abs(z))
		near(t, s.Tessellate().SignedVolume(), expected)
	})
}
