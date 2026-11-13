package occtcheck

import (
	"context"
	"testing"

	"plainsolid/brep"
	"plainsolid/csg/draft"
	"plainsolid/geom"
)

func TestOCCTOpposedEdgeHingeDraft(t *testing.T) {
	ref := runOracle(t, "feature")["opposed-edge-hinge-draft"]
	s, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	vertices, edges := s.Vertices(), s.Edges()
	var hinges []draft.FaceHinge
	for id, face := range s.Faces() {
		x := vertices[edges[face.Boundary[0].Edge].Start].X
		onX := true
		for _, u := range face.Boundary {
			onX = onX && vertices[edges[u.Edge].Start].X == x
		}
		if !onX {
			continue
		}
		for _, u := range face.Boundary {
			e := edges[u.Edge]
			a, b := vertices[e.Start], vertices[e.End]
			if a.Z != 0 || b.Z != 0 {
				continue
			}
			tan := .25
			if x == 4 {
				tan = -.125
			}
			if b.Y < a.Y {
				tan = -tan
			}
			hinges = append(hinges, draft.FaceHinge{Face: id, Edge: u.Edge, HalfAngleTangent: tan})
		}
	}
	if len(hinges) != 2 {
		t.Fatal("expected both original bottom hinge edges", hinges)
	}
	r, err := draft.HingeDraft(context.Background(), s, hinges, draft.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Hinges) != 2 || r.ExactVolume.RatString() != "1136/105" {
		t.Fatal("original hinge ancestry or exact volume missing", r.ExactVolume)
	}
	m, err := r.Solid.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	compare(t, ref, m.SurfaceArea, m.Volume, m.Centroid)
}
