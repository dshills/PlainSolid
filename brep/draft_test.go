package brep

import (
	"math"
	"testing"

	"plainsolid/geom"
)

func TestDraftPrism(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	holes := [][]geom.Vec3{{{X: 1, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 3}, {X: 1, Y: 3}}}
	for _, tc := range []struct{ factor, height float64 }{{.5, 3}, {1.5, -3}, {1, 3}} {
		s, e := DraftPrism(outer, holes, geom.Vec3{X: 2, Y: 2}, geom.Vec3{X: 1, Z: tc.height}, tc.factor)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties()
		w := 12 * math.Abs(tc.height) * (1 + tc.factor + tc.factor*tc.factor) / 3
		if e != nil || math.Abs(p.Volume-w) > 1e-9 || s.Topology().EulerCharacteristic != 0 {
			t.Fatalf("got %v %v, want %g", p, e, w)
		}
	}
	if _, e := DraftPrism(outer, nil, geom.Vec3{Z: 1}, geom.Vec3{Z: 2}, .8); e == nil {
		t.Fatal("accepted off-plane center")
	}
}

func TestTriangleFaceMapping(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	holes := [][]geom.Vec3{{{X: 1, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 3}, {X: 1, Y: 3}}}
	s, e := PrismWithHoles(outer, holes, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []Solid{s, s.Reversed()} {
		m := s.Tessellate()
		ids := s.TriangleFaces()
		if len(ids) != len(m.Triangles) {
			t.Fatal(len(ids), len(m.Triangles))
		}
		for i, tri := range m.Triangles {
			p, e := s.FacePlane(ids[i])
			if e != nil {
				t.Fatal(e)
			}
			for _, j := range tri {
				if math.Abs(p.SignedDistance(m.Vertices[j])) > 1e-12 {
					t.Fatal("triangle mapped to unrelated face")
				}
			}
		}
	}
}
