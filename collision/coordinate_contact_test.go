package collision

import (
	"context"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"reflect"
	"testing"
)

func TestCoordinatePlaneExpectedContact(t *testing.T) {
	fixtures := []struct {
		name   string
		points []geom.Vec3
		tri    [3]int
		safe   bool
	}{
		{"shared edge opposite", []geom.Vec3{{X: 1, Y: -1}}, [3]int{1, 0, 3}, true},
		{"shared edge overlap", []geom.Vec3{{X: 1, Y: .25}}, [3]int{0, 1, 3}, false},
		{"shared vertex", []geom.Vec3{{X: -1, Y: -.5}, {X: -.5, Y: -1}}, [3]int{0, 3, 4}, true},
		{"shared vertex collinear extensions", []geom.Vec3{{X: -1}, {Y: -1}}, [3]int{0, 3, 4}, false},
		{"duplicate position", []geom.Vec3{{}, {X: -1}, {Y: -1}}, [3]int{3, 4, 5}, false},
		{"edge interior touch", []geom.Vec3{{X: 1}, {X: .5, Y: -1}, {X: 1.5, Y: -1}}, [3]int{3, 4, 5}, false},
		{"collinear extension", []geom.Vec3{{X: 3}, {X: 4}, {X: 3, Y: -1}}, [3]int{3, 4, 5}, true},
		{"same triangle", nil, [3]int{2, 1, 0}, false},
		{"crossing", []geom.Vec3{{X: 1, Y: -1}, {X: 1, Y: 1}, {X: 3, Y: 1}}, [3]int{3, 4, 5}, false},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			for _, exponent := range []int{0, -500, 500} {
				m := mesh.Mesh{Vertices: append([]geom.Vec3{{}, {X: 2}, {Y: 2}}, f.points...), Triangles: [][3]int{{0, 1, 2}, f.tri}}
				for i, p := range m.Vertices {
					m.Vertices[i] = p.Scale(math.Ldexp(1, exponent))
				}
				got := coordinatePlaneExpectedContact(m, 0, 1)
				if got != f.safe {
					t.Fatal(exponent, got, f.safe)
				}
				kind, points := triangleIntersection(triangle(m, 0), triangle(m, 1))
				want := kind == Disjoint || expectedSharedContact(m, 0, 1, kind, points)
				if got && !want {
					t.Fatal("false geometric certificate", kind)
				}
				if exponent == 0 {
					a, e := SelfIntersections(m, Options{})
					if e != nil {
						t.Fatal(e)
					}
					b, e := SelfIntersectionsBVHContext(context.Background(), m, Options{})
					if e != nil {
						t.Fatal(e)
					}
					if !reflect.DeepEqual(a.Contacts, b.Contacts) || a.Complete != b.Complete {
						t.Fatal(a, b)
					}
				}
			}
		})
	}
}
