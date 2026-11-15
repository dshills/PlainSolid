package csg

import (
	"context"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func subdivideContactCap(t *testing.T, z float64, cap int, center *geom.Vec3) (brep.Solid, []int) {
	t.Helper()
	s, err := brep.Prism([]geom.Vec3{{Z: z}, {X: 1, Z: z}, {X: 1, Y: 1, Z: z}, {Y: 1, Z: z}}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	if center == nil {
		return s, []int{cap}
	}
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	wire := f[cap].Boundary
	centerID := len(v)
	v = append(v, *center)
	spokes := make([]int, len(wire))
	for i, u := range wire {
		vertex := e[u.Edge].Start
		if u.Reversed {
			vertex = e[u.Edge].End
		}
		spokes[i] = len(e)
		e = append(e, brep.Edge{Start: vertex, End: centerID})
	}
	ids := make([]int, len(wire))
	for i, u := range wire {
		triangle := brep.Face{Boundary: []brep.EdgeUse{u, {Edge: spokes[(i+1)%len(wire)]}, {Edge: spokes[i], Reversed: true}}}
		if i == 0 {
			ids[i] = cap
			f[cap] = triangle
		} else {
			ids[i] = len(f)
			f = append(f, triangle)
		}
	}
	s, err = brep.NewSolid(v, e, f)
	if err != nil {
		t.Fatal(err)
	}
	return s, ids
}

func TestJoinPlanarRegionsKeepsSeamAcrossIndependentSubdivisions(t *testing.T) {
	ctx := context.Background()
	a, af := subdivideContactCap(t, 0, 1, &geom.Vec3{X: .5, Y: .5, Z: 1})
	for _, center := range []*geom.Vec3{nil, {X: .25, Y: .75, Z: 1}} {
		b, bf := subdivideContactCap(t, 1, 0, center)
		faces := [2][]int{af, bf}
		s, h, err := JoinPlanarRegions(ctx, a, b, faces, collision.PlanarFaceContactOptions{})
		if err != nil {
			t.Fatal(err)
		}
		p := h.Contact
		m, err := s.MassProperties()
		if err != nil || p.SignedVolume.RatString() != "2" || len(p.Edges) != 4 || len(p.Vertices) != 4 ||
			len(s.Vertices()) != 12 || len(s.Edges()) != 20 || len(s.Faces()) != 10 || math.Abs(m.SurfaceArea-10) > 1e-13 {
			t.Fatalf("split interface disturbed exterior: %+v, %+v %v", p, m, err)
		}
		for i, part := range [2]brep.Solid{a, b} {
			area := new(big.Rat)
			for j, q := range p.TwiceProjectedAreas[i] {
				if p.DropAxes[i][j] != 2 || q.Sign() <= 0 {
					t.Fatal("bad planar source coverage", i, j, q)
				}
				area.Add(area, q)
			}
			if area.RatString() != "2" {
				t.Fatal("incomplete source region coverage", i, area)
			}
			owner := h.Owners[i]
			for j, face := range owner.Faces {
				removed := false
				for _, id := range faces[i] {
					removed = removed || id == j
				}
				if removed != (face < 0) {
					t.Fatal("wrong deleted face", i, j, face)
				}
			}
			if len(faces[i]) == 4 {
				insideID := len(part.Vertices()) - 1
				if owner.Vertices[insideID] != -1 {
					t.Fatal("retained center of an internal face division", i)
				}
				for j, edge := range part.Edges() {
					spoke := edge.Start == insideID || edge.End == insideID
					if spoke != (owner.Edges[j].Edge < 0) {
						t.Fatal("wrong deleted source subdivision edge", i, j)
					}
				}
			}
		}
		discovered, proof, err := JoinCoincidentPlanarRegions(ctx, a, b, collision.PlanarFaceContactOptions{})
		if err != nil || !reflect.DeepEqual(s, discovered) || !reflect.DeepEqual(faces, proof.Contact.Faces) ||
			proof.Contact.BroadPairs != p.BroadPairs {
			t.Fatalf("one discovery sweep did not reproduce the original interface: %+v %v", proof, err)
		}
		out, g, err := JoinPlanarRegions(ctx, a, b, [2][]int{af[:3], bf}, collision.PlanarFaceContactOptions{})
		if !errors.Is(err, collision.ErrInvalid) || len(out.Vertices()) != 0 || g.Contact.SignedVolume != nil {
			t.Fatalf("partly declared interface returned a union: %+v %v", g, err)
		}
	}
}

func TestDiscoverPlanarRegionsRejectsPartialAndLowerDimensionalContact(t *testing.T) {
	ctx := context.Background()
	a, _ := subdivideContactCap(t, 0, 1, &geom.Vec3{X: .5, Y: .5, Z: 1})
	for _, lo := range []geom.Vec3{{X: .25, Z: 1}, {X: 1, Y: 1}, {X: .5, Y: .5, Z: .5}, {Z: 3}} {
		b, err := brep.Box(lo, lo.Add(geom.Vec3{X: 1, Y: 1, Z: 1}))
		if err != nil {
			t.Fatal(err)
		}
		s, h, err := JoinCoincidentPlanarRegions(ctx, a, b, collision.PlanarFaceContactOptions{})
		if !errors.Is(err, collision.ErrInvalid) || len(s.Faces()) != 0 || h.Contact.SignedVolume != nil {
			t.Fatalf("unproved original interface at %v: %+v %v", lo, h, err)
		}
	}
	b, _ := subdivideContactCap(t, 1, 0, nil)
	s, h, err := JoinCoincidentPlanarRegions(ctx, a, b, collision.PlanarFaceContactOptions{Cross: collision.Options{MaxContacts: 1}})
	if !errors.Is(err, collision.ErrLimit) || len(s.Faces()) != 0 || h.Contact.SignedVolume != nil {
		t.Fatalf("uncompleted exact discovery published material: %+v %v", h, err)
	}
}
