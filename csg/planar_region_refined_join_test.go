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

func joinSeamEdge(t *testing.T, s brep.Solid) int {
	t.Helper()
	v := s.Vertices()
	for i, e := range s.Edges() {
		a, b := v[e.Start], v[e.End]
		if a.Y == 0 && b.Y == 0 && a.Z == 1 && b.Z == 1 {
			return i
		}
	}
	t.Fatal("missing original seam edge")
	return -1
}

func TestJoinRefinedPlanarRegionsOverlaysIndependentSeamStations(t *testing.T) {
	ctx := context.Background()
	base, fa := subdivideContactCap(t, 0, 1, nil)
	tool, fb := subdivideContactCap(t, 1, 0, nil)
	a, _, err := brep.SplitPlanarEdges(ctx, base, []brep.PlanarEdgeSplit{{Edge: joinSeamEdge(t, base), Points: []geom.Vec3{{X: .25, Z: 1}}}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := brep.SplitPlanarEdges(ctx, tool, []brep.PlanarEdgeSplit{{Edge: joinSeamEdge(t, tool), Points: []geom.Vec3{{X: .75, Z: 1}}}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	claimed := [2][]int{fa, fb}
	if s, _, err := JoinPlanarRegions(ctx, a, b, claimed, collision.PlanarFaceContactOptions{}); !errors.Is(err, collision.ErrInvalid) || len(s.Faces()) != 0 {
		t.Fatalf("unrefined interface unexpectedly ignored different seams: %v", err)
	}
	s, h, err := JoinRefinedPlanarRegions(ctx, a, b, claimed, PlanarRegionRefinementOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.MassProperties()
	if err != nil || len(h.Contact.Edges) != 6 || len(h.Contact.Vertices) != 6 || h.Contact.SignedVolume.RatString() != "2" ||
		len(s.Vertices()) != 14 || len(s.Edges()) != 22 || len(s.Faces()) != 10 || math.Abs(p.SurfaceArea-10) > 1e-13 {
		t.Fatalf("wrong exact refined join: %+v; %+v; %v", h.Contact, p, err)
	}
	re := s.Edges()
	rv := s.Vertices()
	owners := [2]map[int]bool{make(map[int]bool), make(map[int]bool)}
	for i, src := range [2]brep.Solid{a, b} {
		history := h.Owners[i]
		v := src.Vertices()
		changed := 0
		for j, e := range src.Edges() {
			x, y := v[e.Start], v[e.End]
			if x.Y != 0 || y.Y != 0 || x.Z != 1 || y.Z != 1 {
				continue
			}
			if len(history.Edges[j]) > 1 {
				changed++
			}
			for _, piece := range history.Edges[j] {
				te := re[piece.Result.Edge]
				tx, ty := rv[te.Start], rv[te.End]
				if piece.Result.Reversed {
					tx, ty = ty, tx
				}
				t0, _ := piece.Range[0].Float64()
				t1, _ := piece.Range[1].Float64()
				owners[i][piece.Result.Edge] = true
				if tx != x.Add(y.Sub(x).Scale(t0)) || ty != x.Add(y.Sub(x).Scale(t1)) {
					t.Fatalf("no exact original parent endpoint correspondence: part %d edge %d %+v", i, j, piece)
				}
			}
		}
		if changed != 1 || len(history.Vertices) != len(v) || len(history.Faces) != len(src.Faces()) || len(owners[i]) != 3 {
			t.Fatalf("original owner was not split and remapped locally: %d %+v", i, history)
		}
	}
	if !reflect.DeepEqual(owners[0], owners[1]) {
		t.Fatalf("refined seam lacks identical exterior edge ownership: %v", owners)
	}
	for _, opts := range []PlanarRegionRefinementOptions{{MaxStations: 1}, {MaxStationTests: 1}} {
		out, h, err := JoinRefinedPlanarRegions(ctx, a, b, claimed, opts)
		if !errors.Is(err, collision.ErrLimit) || len(out.Faces()) != 0 || h.Contact.SignedVolume != nil {
			t.Fatalf("budget-limited refinement published a partial union: %+v %v", h, err)
		}
	}
}
