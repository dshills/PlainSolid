package csg

import (
	"errors"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/heal"
	"testing"
)

func TestCoplanarBoxOwnership(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	cases := []struct {
		name                                         string
		min, max                                     geom.Vec3
		union, difference, unionArea, differenceArea float64
		differenceEuler                              int
	}{
		{"overlapping footprints", geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 3, Y: 3, Z: 2}, 14, 6, 38, 22, 2},
		{"aligned overlap", geom.Vec3{X: 1}, geom.Vec3{X: 3, Y: 2, Z: 2}, 12, 4, 32, 16, 2},
		{"full opposing face", geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 2, Z: 2}, 12, 8, 32, 24, 2},
		{"partial opposing face", geom.Vec3{X: 2, Y: 0.5, Z: 0.5}, geom.Vec3{X: 3, Y: 1.5, Z: 1.5}, 9, 8, 28, 24, 2},
		{"flush contained pocket", geom.Vec3{Y: 0.5, Z: 0.5}, geom.Vec3{X: 1, Y: 1.5, Z: 1.5}, 8, 7, 24, 28, 2},
		{"flush through hole", geom.Vec3{Y: 0.5, Z: 0.5}, geom.Vec3{X: 2, Y: 1.5, Z: 1.5}, 8, 6, 24, 30, 0},
		{"flush corner", geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1}, 8, 7, 24, 24, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := box(t, tc.min, tc.max)
			union, e := Union(a, b, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			difference, e := Difference(a, b, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			up, e := union.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			dp, e := difference.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			near(t, up.Volume, tc.union)
			near(t, dp.Volume, tc.difference)
			near(t, up.SurfaceArea, tc.unionArea)
			near(t, dp.SurfaceArea, tc.differenceArea)
			if difference.Topology().EulerCharacteristic != tc.differenceEuler {
				t.Fatal(difference.Topology())
			}
			reverse, e := Union(b, a, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			near(t, reverse.Tessellate().SignedVolume(), tc.union)
		})
	}
}
func TestExactlyCoplanarRotatedBoxes(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	b := box(t, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 3, Y: 3, Z: 2})
	// Dyadic affine coordinates preserve exact coplanarity. The first transform
	// rotates the rectangular boxes 45 degrees and scales their XY edge lengths.
	for _, transform := range []geom.Affine{{{1, -1, 0, 3}, {1, 1, 0, -2}, {0, 0, 1, 0.5}}, {{0, -1, 0, 3}, {1, 0, 0, -2}, {0, 0, 1, 0.5}}} {
		aa, e := a.Transformed(transform)
		if e != nil {
			t.Fatal(e)
		}
		bb, e := b.Transformed(transform)
		if e != nil {
			t.Fatal(e)
		}
		union, e := Union(aa, bb, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		difference, e := Difference(aa, bb, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		scale := transform.Determinant()
		near(t, union.Tessellate().SignedVolume(), 14*scale)
		near(t, difference.Tessellate().SignedVolume(), 6*scale)
	}
}
func TestCoplanarOperandOrderAndEquivalentTopology(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	faces := a.Faces()
	for i, j := 0, len(faces)-1; i < j; i, j = i+1, j-1 {
		faces[i], faces[j] = faces[j], faces[i]
	}
	same, e := brep.NewSolid(a.Vertices(), a.Edges(), faces)
	if e != nil {
		t.Fatal(e)
	}
	union, e := Union(a, same, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, union.Tessellate().SignedVolume(), 8)
	difference, e := Difference(a, same, 1e-9)
	if e != nil || len(difference.Vertices()) != 0 {
		t.Fatal(difference.Topology(), e)
	}
	b := box(t, geom.Vec3{X: 1}, geom.Vec3{X: 3, Y: 2, Z: 2})
	difference, e = Difference(b, a, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, difference.Tessellate().SignedVolume(), 4)
}
func TestNearCoplanarStillFailsExplicitly(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	b := box(t, geom.Vec3{X: 1, Y: 1, Z: 1e-10}, geom.Vec3{X: 3, Y: 3, Z: 2 + 1e-10})
	if _, e := Union(a, b, 1e-8); !errors.Is(e, ErrUnsupported) {
		t.Fatal("near-coplanar ambiguity accepted", e)
	}
	if _, e := Difference(a, b, 1e-8); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	// Containment is exact even when the inner box is closer than tolerance to a
	// boundary; it must not be mistaken for a disjoint zero-volume intersection.
	tiny := box(t, geom.Vec3{X: 1e-10, Y: 0.5, Z: 0.5}, geom.Vec3{X: 2e-10, Y: 1.5, Z: 1.5})
	removed, e := Difference(tiny, a, 1e-8)
	if e != nil || len(removed.Vertices()) != 0 {
		t.Fatal("near-boundary containment lost", removed.Topology(), e)
	}
}

func FuzzCoplanarBoxBooleans(f *testing.F) {
	f.Add(0.5, 0.75)
	f.Fuzz(func(t *testing.T, x, y float64) {
		if x < 0.1 || y < 0.1 || x > 1.9 || y > 1.9 || x != x || y != y {
			return
		}
		a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
		b := box(t, geom.Vec3{X: x, Y: y}, geom.Vec3{X: x + 2, Y: y + 2, Z: 2})
		intersection := 2 * (2 - x) * (2 - y)
		u, e := Union(a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		d, e := Difference(a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		near(t, u.Tessellate().SignedVolume(), 16-intersection)
		near(t, d.Tessellate().SignedVolume(), 8-intersection)
	})
}
func FuzzSharedFaceUnion(f *testing.F) {
	f.Add(0.5, 0.75)
	f.Fuzz(func(t *testing.T, y, z float64) {
		if y < 0.1 || z < 0.1 || y > 1.4 || z > 1.4 || y != y || z != z {
			return
		}
		a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
		b := box(t, geom.Vec3{X: 2, Y: y, Z: z}, geom.Vec3{X: 3, Y: y + 0.5, Z: z + 0.5})
		u, e := Union(a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		properties, e := u.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		near(t, properties.Volume, 8.25)
		near(t, properties.SurfaceArea, 26)
	})
}

func TestCoplanarSubdividedOperands(t *testing.T) {
	a := box(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	b := box(t, geom.Vec3{X: 1}, geom.Vec3{X: 3, Y: 2, Z: 2})
	ma, mb := a.Tessellate(), b.Tessellate()
	aa := solidFromTriangles(t, ma.Vertices, ma.Triangles)
	bb := solidFromTriangles(t, mb.Vertices, mb.Triangles)
	for _, pair := range [][2]brep.Solid{{a, bb}, {aa, b}, {aa, bb}} {
		u, e := Union(pair[0], pair[1], 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		d, e := Difference(pair[0], pair[1], 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		near(t, u.Tessellate().SignedVolume(), 12)
		near(t, d.Tessellate().SignedVolume(), 4)
	}
}

func TestConcaveFaceSubdivisionIsExplicitlyUnsupported(t *testing.T) {
	// A convex cube whose top is partitioned into an L-shaped face and a square.
	// Convex solid classification alone must not allow Sutherland-Hodgman clipping
	// to silently treat a concave individual boundary loop as convex.
	outer := []geom.Vec3{{}, {X: 1}, {X: 1, Y: 0.5}, {X: 1, Y: 1}, {X: 0.5, Y: 1}, {Y: 1}}
	input := []heal.Face{
		{Outer: []geom.Vec3{{Z: 1}, {X: 1, Z: 1}, {X: 1, Y: 0.5, Z: 1}, {X: 0.5, Y: 0.5, Z: 1}, {X: 0.5, Y: 1, Z: 1}, {Y: 1, Z: 1}}},
		{Outer: []geom.Vec3{{X: 0.5, Y: 0.5, Z: 1}, {X: 1, Y: 0.5, Z: 1}, {X: 1, Y: 1, Z: 1}, {X: 0.5, Y: 1, Z: 1}}},
	}
	for i, p := range outer {
		q := outer[(i+1)%len(outer)]
		input = append(input, heal.Face{Outer: []geom.Vec3{p, q, q.Add(geom.Vec3{Z: 1}), p.Add(geom.Vec3{Z: 1})}}, heal.Face{Outer: []geom.Vec3{{X: 0.5, Y: 0.5}, q, p}})
	}
	cube, _, e := heal.Sew(input, heal.Options{})
	if e != nil {
		t.Fatal(e)
	}
	near(t, cube.Tessellate().SignedVolume(), 1)
	other := box(t, geom.Vec3{X: 0.25, Y: 0.25, Z: 0.25}, geom.Vec3{X: 1.25, Y: 1.25, Z: 1.25})
	if _, e = Union(cube, other, 1e-9); !errors.Is(e, ErrUnsupported) {
		t.Fatal("concave face clipping must not return a nominal result", e)
	}
}

func TestCoplanarTriangularFaceAttachment(t *testing.T) {
	cube := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	tetra := solidFromTriangles(t, []geom.Vec3{{X: 1, Y: 0.25, Z: 0.25}, {X: 1, Y: 0.75, Z: 0.25}, {X: 1, Y: 0.25, Z: 0.75}, {X: 2, Y: 0.25, Z: 0.25}}, [][3]int{{1, 2, 3}, {0, 3, 2}, {0, 1, 3}, {0, 2, 1}})
	union, e := Union(cube, tetra, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, union.Tessellate().SignedVolume(), 1+1.0/24)
	difference, e := Difference(cube, tetra, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, difference.Tessellate().SignedVolume(), 1)
}
