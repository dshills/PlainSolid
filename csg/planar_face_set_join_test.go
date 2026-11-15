package csg

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestJoinMatchingPlanarFacesFoldsAcrossOriginalEdge(t *testing.T) {
	ctx := context.Background()
	a, err := brep.Prism([]geom.Vec3{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	b, err := brep.Box(geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 2, Y: 2, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	s, h, err := JoinMatchingPlanarFaces(ctx, a, b, collision.PlanarFaceDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := h.Contact
	if len(p.Faces) != 2 || p.Faces[0].DropAxis == p.Faces[1].DropAxis || p.SignedVolume.RatString() != "4" {
		t.Fatalf("expected two perpendicular whole original faces: %+v", p)
	}
	m, err := s.MassProperties()
	if err != nil || len(s.Vertices()) != 14 || len(s.Edges()) != 22 || len(s.Faces()) != 10 ||
		math.Abs(m.Volume-4) > 1e-13 || math.Abs(m.SurfaceArea-16) > 1e-13 {
		t.Fatalf("folded join measures or topology: %+v + %dV/%dE/%dF, %v", m, len(s.Vertices()), len(s.Edges()), len(s.Faces()), err)
	}
	v0, v1 := geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: 1, Z: 1}
	for i, part := range [2]brep.Solid{a, b} {
		removed := 0
		for j, u := range h.Owners[i].Edges {
			if u.Edge != -1 {
				continue
			}
			removed++
			e, v := part.Edges()[j], part.Vertices()
			x, y := v[e.Start], v[e.End]
			if !(x == v0 && y == v1 || x == v1 && y == v0) {
				t.Fatal("deleted an exterior original edge", i, j, x, y)
			}
		}
		if removed != 1 {
			t.Fatal("lost the original internal seam identity", i, removed)
		}
	}
	loc, err := collision.NewPointLocator(ctx, s, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := loc.Locate(ctx, geom.Vec3{X: 1, Y: 1, Z: .5})
	if err != nil || r.Location != collision.Inside {
		t.Fatalf("folded interface still on boundary: %+v %v", r, err)
	}
	pairs := [][2]int{{p.Faces[1].FaceA, p.Faces[1].FaceB}, {p.Faces[0].FaceA, p.Faces[0].FaceB}}
	reverse, _, err := JoinPlanarFaceSet(ctx, a, b, pairs, collision.PlanarFaceContactOptions{})
	if err != nil || !reflect.DeepEqual(s, reverse) {
		t.Fatal("contact order changed original exterior topology", err)
	}
	for _, bad := range [][][2]int{pairs[:1], {pairs[0], pairs[0]}, nil} {
		out, h, err := JoinPlanarFaceSet(ctx, a, b, bad, collision.PlanarFaceContactOptions{})
		if !errors.Is(err, collision.ErrInvalid) || len(out.Faces()) != 0 || h.Contact.SignedVolume != nil {
			t.Fatalf("unproved set published a join: %+v %v", h, err)
		}
	}
}

func TestJoinMatchingPlanarFacesRemovesEnclosedOriginalVertices(t *testing.T) {
	ctx := context.Background()
	a := pocketJoinStock(t, true, false)
	b, err := brep.Box(geom.Vec3{X: 1, Y: 1, Z: .25}, geom.Vec3{X: 3, Y: 3, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	s, h, err := JoinMatchingPlanarFaces(ctx, a, b, collision.PlanarFaceDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := h.Contact
	m, err := s.MassProperties()
	if err != nil || len(p.Faces) != 5 || p.SignedVolume.RatString() != "16" || len(s.Vertices()) != 12 || len(s.Edges()) != 16 || len(s.Faces()) != 7 ||
		math.Abs(m.Volume-16) > 1e-13 || math.Abs(m.SurfaceArea-48) > 1e-13 {
		t.Fatalf("pocket join: %+v, %+v %v", p, m, err)
	}
	for i, part := range [2]brep.Solid{a, b} {
		var nums [2]int
		v := part.Vertices()
		for j, newID := range h.Owners[i].Vertices {
			removed := v[j].Z == .25
			if (newID < 0) != removed {
				t.Fatal("wrong deleted original vertex", i, j, v[j], newID)
			}
			if removed {
				nums[0]++
			}
		}
		for j, newID := range h.Owners[i].Edges {
			e := part.Edges()[j]
			removed := v[e.Start].Z == .25 || v[e.End].Z == .25
			if (newID.Edge < 0) != removed {
				t.Fatal("wrong deleted original edge", i, j, newID)
			}
			if removed {
				nums[1]++
			}
		}
		if nums != [2]int{4, 8} {
			t.Fatal("inner seam original history lost", i, nums)
		}
	}
}
