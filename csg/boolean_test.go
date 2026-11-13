package csg

import (
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestTransverseBoxUnionDifference(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	b := box(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	union, e := Union(a, b, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	difference, e := Difference(a, b, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, union.Tessellate().SignedVolume(), 15)
	near(t, difference.Tessellate().SignedVolume(), 7)
	if e = union.Validate(); e != nil {
		t.Fatal(e)
	}
	if e = difference.Validate(); e != nil {
		t.Fatal(e)
	}
	if union.Topology().EulerCharacteristic != 2 || difference.Topology().EulerCharacteristic != 2 {
		t.Fatal(union.Topology(), difference.Topology())
	}
	reverse, e := Union(b, a, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, reverse.Tessellate().SignedVolume(), 15)
}
func TestBooleanDisjointContainmentAndEmpty(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	inner := box(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 2, Y: 2, Z: 2})
	far := box(t, geom.Vec3{X: 10}, geom.Vec3{X: 11, Y: 1, Z: 1})
	cases := []struct {
		name                          string
		a, b                          brep.Solid
		union, difference             float64
		unionShells, differenceShells int
	}{
		{"contained", a, inner, 64, 63, 1, 2}, {"inside", inner, a, 64, 0, 1, 0}, {"disjoint", a, far, 65, 64, 2, 1}, {"equal", a, a, 64, 0, 1, 0}, {"empty rhs", a, brep.Solid{}, 64, 64, 1, 1}, {"empty lhs", brep.Solid{}, a, 64, 0, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, e := Union(tc.a, tc.b, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			d, e := Difference(tc.a, tc.b, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			near(t, u.Tessellate().SignedVolume(), tc.union)
			near(t, d.Tessellate().SignedVolume(), tc.difference)
			if u.Topology().Shells != tc.unionShells || d.Topology().Shells != tc.differenceShells {
				t.Fatal(u.Topology(), d.Topology())
			}
		})
	}
}
func TestRotatedConvexBooleanVolumeIdentities(t *testing.T) {
	a := box(t, geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	rotation, e := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, 0.7)
	if e != nil {
		t.Fatal(e)
	}
	b, e := a.Transformed(geom.Translation(geom.Vec3{X: 0.45, Y: 0.31, Z: 0.22}).Mul(rotation))
	if e != nil {
		t.Fatal(e)
	}
	intersection, e := Intersect(a, b, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	union, e := Union(a, b, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	difference, e := Difference(a, b, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	// Intersection reconstructs plane triples, independently of the boundary
	// splitting and sewing used by union and difference.
	iv := intersection.Tessellate().SignedVolume()
	near(t, union.Tessellate().SignedVolume(), 16-iv)
	near(t, difference.Tessellate().SignedVolume(), 8-iv)
}
func TestBooleanUnsupportedContacts(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	touching := box(t, geom.Vec3{X: 2, Y: 2}, geom.Vec3{X: 3, Y: 3, Z: 1})
	pointTouch := box(t, geom.Vec3{X: 2, Y: 2, Z: 2}, geom.Vec3{X: 3, Y: 3, Z: 3})
	for _, b := range []brep.Solid{touching, pointTouch} {
		if _, e := Union(a, b, 1e-9); !errors.Is(e, ErrUnsupported) {
			t.Fatal("expected explicit unsupported event", e)
		}
		if _, e := Difference(a, b, 1e-9); !errors.Is(e, ErrUnsupported) {
			t.Fatal(e)
		}
	}
	for _, tol := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, e := Union(a, a, tol); e == nil {
			t.Fatal("bad tolerance")
		}
	}
	if _, e := Difference(a.Reversed(), a, 1e-9); e == nil {
		t.Fatal("inward operand")
	}
}

func TestDifferenceSplitsAndThroughHole(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 3, Y: 3, Z: 3})
	slab := box(t, geom.Vec3{X: 1, Y: -1, Z: -2}, geom.Vec3{X: 2, Y: 4, Z: 5})
	split, e := Difference(a, slab, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, split.Tessellate().SignedVolume(), 18)
	if split.Topology().Shells != 2 || split.Topology().EulerCharacteristic != 4 {
		t.Fatal(split.Topology())
	}
	cutter := box(t, geom.Vec3{X: 1, Y: 1, Z: -1}, geom.Vec3{X: 2, Y: 2, Z: 4})
	tube, e := Difference(a, cutter, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	properties, e := tube.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	near(t, properties.Volume, 24)
	near(t, properties.SurfaceArea, 64)
	if tube.Topology().EulerCharacteristic != 0 || tube.Topology().Shells != 1 {
		t.Fatal(tube.Topology())
	}
	union, e := Union(a, cutter, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, union.Tessellate().SignedVolume(), 29)
}
func TestDisjointWithOverlappingBounds(t *testing.T) {
	a := solidFromTriangles(t, []geom.Vec3{{}, {X: 1}, {Y: 1}, {Z: 1}}, [][3]int{{1, 2, 3}, {0, 3, 2}, {0, 1, 3}, {0, 2, 1}})
	b, e := a.Transformed(geom.Translation(geom.Vec3{X: 1, Y: 1, Z: 1}).Mul(geom.Scaling(geom.Vec3{X: -1, Y: -1, Z: -1})))
	if e != nil {
		t.Fatal(e)
	}
	union, e := Union(a, b, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, union.Tessellate().SignedVolume(), 1.0/3)
	if union.Topology().Shells != 2 {
		t.Fatal(union.Topology())
	}
	difference, e := Difference(a, b, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, difference.Tessellate().SignedVolume(), 1.0/6)
}
func FuzzTransverseBoxBooleans(f *testing.F) {
	f.Add(0.5, 0.75, 1.25)
	f.Fuzz(func(t *testing.T, x, y, z float64) {
		if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) || x < 0.1 || y < 0.1 || z < 0.1 || x > 1.9 || y > 1.9 || z > 1.9 {
			return
		}
		a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
		p := geom.Vec3{X: x, Y: y, Z: z}
		b := box(t, p, p.Add(geom.Vec3{X: 2, Y: 2, Z: 2}))
		intersection := (2 - x) * (2 - y) * (2 - z)
		union, e := Union(a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		difference, e := Difference(a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		near(t, union.Tessellate().SignedVolume(), 16-intersection)
		near(t, difference.Tessellate().SignedVolume(), 8-intersection)
	})
}

func TestBooleanFaceOrderAndNearPlaneEvent(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	b := box(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	faces := a.Faces()
	for i, j := 0, len(faces)-1; i < j; i, j = i+1, j-1 {
		faces[i], faces[j] = faces[j], faces[i]
	}
	reordered, e := brep.NewSolid(a.Vertices(), a.Edges(), faces)
	if e != nil {
		t.Fatal(e)
	}
	union, e := Union(reordered, b, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	difference, e := Difference(reordered, b, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, union.Tessellate().SignedVolume(), 15)
	near(t, difference.Tessellate().SignedVolume(), 7)
	nearPlane := box(t, geom.Vec3{X: 1e-10, Y: 0.3, Z: 0.4}, geom.Vec3{X: 2 + 1e-10, Y: 2.3, Z: 2.4})
	if _, e = Union(a, nearPlane, 1e-8); !errors.Is(e, ErrUnsupported) {
		t.Fatal("unresolved near-plane event must fail explicitly", e)
	}
}
