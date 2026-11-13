package occtcheck

import (
	"context"
	"plainsolid/brep"
	"plainsolid/csg/draft"
	"plainsolid/geom"
	"testing"
)

func TestOCCTMultipleSupportRemoval(t *testing.T) {
	refs := runOracle(t, "draft")
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	vs := s.Vertices()
	var cuts []draft.EdgeChamfer
	var removed []int
	for i, edge := range s.Edges() {
		a, b := vs[edge.Start], vs[edge.End]
		if a.X != b.X || a.Y != b.Y {
			continue
		}
		face := -1
		for j, f := range s.Faces() {
			for _, u := range f.Boundary {
				if u.Edge == i {
					face = j
				}
			}
		}
		if a.X == 0 {
			removed = append(removed, len(cuts))
		}
		cuts = append(cuts, draft.EdgeChamfer{Edge: i, Face: face, Setback: .125, OtherSetback: .125})
	}
	ch, e := draft.Chamfer(context.Background(), s, cuts, false, draft.Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"defeature-two", "defeature-all"} {
		t.Run(name, func(t *testing.T) {
			faces := ch.EdgeFaces
			if name == "defeature-two" {
				faces = []int{ch.EdgeFaces[removed[0]], ch.EdgeFaces[removed[1]]}
			}
			r, e := draft.RemoveSupports(context.Background(), ch.Solid, faces, draft.Options{})
			if e != nil {
				t.Fatal(e)
			}
			m, e := r.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			compare(t, refs[name], m.SurfaceArea, m.Volume, m.Centroid)
		})
	}
}
