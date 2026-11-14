package collision

import (
	"context"
	"math/rand"
	"plainsolid/geom"
	"plainsolid/mesh"
	"reflect"
	"testing"
)

func TestRuledContactExactDifferential(t *testing.T) {
	cases := []struct {
		name  string
		extra []geom.Vec3
		tri   [3]int
		safe  bool
	}{
		{"shared plane point", []geom.Vec3{{X: 1, Y: 1, Z: 1}, {X: 2, Y: 1, Z: 1}}, [3]int{0, 3, 4}, true},
		{"foreign plane point", []geom.Vec3{{}, {X: 1, Y: 1, Z: 1}, {X: 2, Y: 1, Z: 1}}, [3]int{3, 4, 5}, false},
		{"shared plane edge", []geom.Vec3{{X: 1, Y: 1, Z: 1}}, [3]int{0, 1, 3}, true},
		{"foreign plane edge", []geom.Vec3{{}, {X: 2}, {X: 1, Y: 1, Z: 1}}, [3]int{3, 4, 5}, false},
		{"crossing plane", []geom.Vec3{{X: 1, Y: 1, Z: 1}, {X: 1, Y: 1, Z: -1}}, [3]int{0, 3, 4}, false},
		{"disjoint extension", []geom.Vec3{{X: 3}, {X: 4}, {X: 3, Z: 1}}, [3]int{3, 4, 5}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := mesh.Mesh{Vertices: append([]geom.Vec3{{}, {X: 2}, {Y: 2}}, c.extra...), Triangles: [][3]int{{0, 1, 2}, c.tri}}
			got := ruledExpectedContact(m, 0, 1)
			if got != c.safe {
				t.Fatal(got, c.safe)
			}
			kind, ps := triangleIntersection(triangle(m, 0), triangle(m, 1))
			if got && kind != Disjoint && !expectedSharedContact(m, 0, 1, kind, ps) {
				t.Fatal("false contact certificate", kind)
			}
			a, e := SelfIntersections(m, Options{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := SelfIntersectionsBVHContext(context.Background(), m, Options{})
			if e != nil || !reflect.DeepEqual(a.Contacts, b.Contacts) {
				t.Fatal(a, b, e)
			}
		})
	}
	// A non-coordinate axial plane y=2x. Opposite-side shared diagonal is safe;
	// moving the fourth point onto the same side produces an actual overlap.
	for _, z := range []float64{-1, .5} {
		m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2, Y: 4}, {Z: 2}, {X: 1, Y: 2, Z: z}}, Triangles: [][3]int{{0, 1, 2}, {1, 0, 3}}}
		if got := ruledExpectedContact(m, 0, 1); got != (z < 0) {
			t.Fatal(z, got)
		}
		a, _ := SelfIntersections(m, Options{})
		b, e := SelfIntersectionsBVHContext(context.Background(), m, Options{})
		if e != nil || !reflect.DeepEqual(a.Contacts, b.Contacts) {
			t.Fatal(a, b, e)
		}
	}
}

func TestRuledContactRandomExactReference(t *testing.T) {
	rng := rand.New(rand.NewSource(83))
	for n := 0; n < 600; n++ {
		m := mesh.Mesh{Triangles: [][3]int{{0, 1, 2}, {3, 4, 5}}}
		for i := 0; i < 6; i++ {
			x, z := float64(rng.Intn(33)-16)/8, float64(rng.Intn(33)-16)/8
			p := [3]float64{x, 2 * x, z}
			shift := n % 3
			m.Vertices = append(m.Vertices, geom.Vec3{X: p[shift], Y: p[(shift+1)%3], Z: p[(shift+2)%3]})
		}
		if n%2 == 0 {
			m.Triangles[1][0] = 0
		}
		if n%3 == 0 {
			m.Triangles[1][1] = 1
		}
		a, b := triangle(m, 0), triangle(m, 1)
		if nonzeroAxis(triangleNormal(exactTriangle(a))) < 0 || nonzeroAxis(triangleNormal(exactTriangle(b))) < 0 {
			continue
		}
		if ruledExpectedContact(m, 0, 1) {
			kind, ps := triangleIntersection(a, b)
			if kind != Disjoint && !expectedSharedContact(m, 0, 1, kind, ps) {
				t.Fatal(n, kind, m)
			}
		}
	}
}
