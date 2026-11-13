package trimmed

import (
	"context"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestValidateContinuousEdgeAgreement(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	full, e := s.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil || !full.Complete || len(full.Uses) != 24 {
		t.Fatal(full, e)
	}
	edges, vertices := s.Edges(), s.Vertices()
	id := 0
	p, e := geom.NewBezier([]geom.Vec3{vertices[edges[id].Start], vertices[edges[id].End]}, nil)
	if e != nil {
		t.Fatal(e)
	}
	edges[id].Curve = p
	edges[id].Range.Min, edges[id].Range.Max = 0, 1
	s, e = NewSolid(vertices, edges, s.Faces(), 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, x := range r.Uses {
		if x.Edge == id {
			n++
			if x.Report.Status != edgecheck.Certified {
				t.Fatal(x)
			}
		}
	}
	if n != 2 {
		t.Fatal(n)
	}
}
