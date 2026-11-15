package collision

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestProjectedContactMatchesExactIntersection(t *testing.T) {
	cases := []struct {
		name  string
		extra []geom.Vec3
		ids   [3]int
		safe  bool
	}{
		{"separated sloped facets", []geom.Vec3{{X: 1.5, Y: 1.5}, {X: 2.5, Y: 1.5, Z: 2}, {X: 1.5, Y: 2.5, Z: 4}}, [3]int{3, 4, 5}, true},
		{"noncoplanar shared edge", []geom.Vec3{{X: 1, Y: -1, Z: 3}}, [3]int{1, 0, 3}, true},
		{"noncoplanar shared vertex", []geom.Vec3{{X: -1, Y: -1}, {X: -1, Z: 3}}, [3]int{0, 3, 4}, true},
		{"unindexed coincident vertex", []geom.Vec3{{}, {X: -1, Y: -1}, {X: -1, Z: 3}}, [3]int{3, 4, 5}, false},
		{"coplanar overlap", []geom.Vec3{{X: .5, Y: .25, Z: 1}}, [3]int{0, 1, 3}, false},
		{"proper crossing", []geom.Vec3{{X: .25, Y: .25, Z: 4}, {X: .25, Y: .25, Z: -4}, {X: 1.5, Y: .5, Z: 1}}, [3]int{3, 4, 5}, false},
		{"duplicate triangle", nil, [3]int{2, 1, 0}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, exponent := range []int{0, -500, 500} {
				m := mesh.Mesh{
					Vertices:  append([]geom.Vec3{{}, {X: 2, Z: 2}, {Y: 2, Z: 4}}, c.extra...),
					Triangles: [][3]int{{0, 1, 2}, c.ids},
				}
				for i, p := range m.Vertices {
					m.Vertices[i] = p.Scale(math.Ldexp(1, exponent))
				}
				if got := projectedExpectedContact(m, 0, 1); got != c.safe {
					t.Fatalf("projection certificate at exponent %d = %t, want %t", exponent, got, c.safe)
				}
				oracle, err := SelfIntersections(m, Options{})
				if err != nil {
					t.Fatal(err)
				}
				got, err := SelfIntersectionsBVHContext(context.Background(), m, Options{MaxPairs: 2})
				if err != nil || !got.Complete || got.BroadPhasePairs != 2 || got.NarrowPhasePairs != 1 ||
					!reflect.DeepEqual(got.Contacts, oracle.Contacts) {
					t.Fatalf("indexed scan changed decisions or work counts: %+v, oracle %+v, %v", got, oracle, err)
				}
				partial, err := SelfIntersectionsBVHContext(context.Background(), m, Options{MaxPairs: 1})
				if !errors.Is(err, ErrLimit) || partial.Complete || partial.BroadPhasePairs != 1 ||
					partial.NarrowPhasePairs != 0 || len(partial.Contacts) != 0 {
					t.Fatalf("projection certificate bypassed the work limit: %+v, %v", partial, err)
				}
			}
		})
	}
}

func TestProjectedContactPreservesPartialContactReport(t *testing.T) {
	m := mesh.Mesh{
		Vertices: []geom.Vec3{
			{}, {X: 2, Z: 2}, {Y: 2, Z: 4},
			{X: 1.5, Y: 1.5}, {X: 2.5, Y: 1.5, Z: 2}, {X: 1.5, Y: 2.5, Z: 4},
		},
		Triangles: [][3]int{{0, 1, 2}, {3, 4, 5}, {0, 1, 2}, {0, 1, 2}},
	}
	complete, err := SelfIntersectionsBVHContext(context.Background(), m, Options{})
	if err != nil || !complete.Complete || len(complete.Contacts) != 3 {
		t.Fatalf("complete report: %+v, %v", complete, err)
	}
	limited, err := SelfIntersectionsBVHContext(context.Background(), m, Options{MaxContacts: 1})
	if !errors.Is(err, ErrLimit) || limited.Complete || limited.BroadPhasePairs != 4 ||
		limited.NarrowPhasePairs != 3 || !reflect.DeepEqual(limited.Contacts, complete.Contacts[:1]) {
		t.Fatalf("contact budget and first retained witness: %+v, %v", limited, err)
	}
}
