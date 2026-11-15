package document

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestPlanarEdgeSplitFeatureComposesExactTwoOwnerSeam(t *testing.T) {
	ctx := context.Background()
	st := State{LengthUnit: .001, Features: map[string]Feature{
		"a":         {Kind: BoxFeature, Max: geom.Vec3{X: 3, Y: 1, Z: 1}},
		"b":         {Kind: BoxFeature, Min: geom.Vec3{Z: 1}, Max: geom.Vec3{X: 3, Y: 1, Z: 2}},
		"a-station": {Kind: PlanarEdgeSplitFeature, Input: "a", PlanarEdgeSplit: &PlanarEdgeSplitRecipe{Roles: [2]string{"y-min", "z-max"}, Parameters: []string{"1/3"}}},
		"b-station": {Kind: PlanarEdgeSplitFeature, Input: "b", PlanarEdgeSplit: &PlanarEdgeSplitRecipe{Roles: [2]string{"y-min", "z-min"}, Parameters: []string{"2/3"}}},
		"joined":    {Kind: PlanarSeamJoinFeature, Input: "a-station", Tool: "b-station"},
	}}
	doc, err := New(st, 2)
	if err != nil {
		t.Fatal(err)
	}
	check := func(s State) [2]ResolvedPlanarEdgeHistory {
		t.Helper()
		eval, err := s.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := eval.Planar["joined"]
		if len(out.Vertices()) != 14 || len(out.Edges()) != 22 || len(out.Faces()) != 10 {
			t.Fatalf("different source splits were not reconciled: %+v", out.Topology())
		}
		res := [2]ResolvedPlanarEdgeHistory{}
		for i, id := range [2]string{"a", "b"} {
			role := [2]string{"z-max", "z-min"}[i]
			ref := PlanarEdgeHistory{Source: FeatureEdge{Feature: id, Roles: [2]string{"y-min", role}}, Target: "joined"}
			r, err := s.ResolvePlanarEdgeHistory(ctx, ref)
			if err != nil || len(r.Intervals) != 3 {
				t.Fatalf("source %s has incomplete split ancestry: %+v %v", id, r, err)
			}
			res[i] = r
			seen := map[string]bool{}
			for _, p := range r.Intervals {
				a, b := p.From, p.To
				if a.Cmp(b) > 0 {
					a, b = b, a
				}
				if p.OutputFrom.Sign() != 0 || p.OutputTo.RatString() != "1" {
					t.Fatal("result child is not fully owned", p)
				}
				seen[a.RatString()+":"+b.RatString()] = true
				e := out.Edges()[p.Index]
				v, w := out.Vertices()[e.Start], out.Vertices()[e.End]
				if v.Y != 0 || v.Z != 1 || w.Y != 0 || w.Z != 1 {
					t.Fatal("named source jumped to a different original seam", v, w)
				}
			}
			if !reflect.DeepEqual(seen, map[string]bool{"0:1/3": true, "1/3:2/3": true, "2/3:1": true}) {
				t.Fatalf("exact original fractions changed: %+v", seen)
			}
		}
		edges := func(h ResolvedPlanarEdgeHistory) map[int]bool {
			m := map[int]bool{}
			for _, p := range h.Intervals {
				m[p.Index] = true
			}
			return m
		}
		if !reflect.DeepEqual(edges(res[0]), edges(res[1])) {
			t.Fatal("both original owners must map to the same result edges")
		}
		return res
	}
	a, _ := doc.Snapshot()
	original := check(a)
	a.Features["a-station"].PlanarEdgeSplit.Parameters[0] = "1/2"
	again, _ := doc.Snapshot()
	if again.Features["a-station"].PlanarEdgeSplit.Parameters[0] != "1/3" {
		t.Fatal("snapshot mutated the authored exact parameter")
	}
	var raw bytes.Buffer
	if err := doc.Write(&raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Read(bytes.NewReader(raw.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := reloaded.Snapshot()
	if got := check(s); !reflect.DeepEqual(original, got) {
		t.Fatalf("reload changed exact split ownership: %+v", got)
	}
}

func TestPlanarEdgeSplitFeatureRejectsInexactAuthoredStation(t *testing.T) {
	for _, tc := range []struct {
		parameter string
		err       error
	}{
		{"1/3", geom.ErrUnrepresentable},
		{"2/4", geom.ErrInvalid},
		{"1", geom.ErrInvalid},
	} {
		s := State{LengthUnit: .001, Features: map[string]Feature{
			"base":  {Kind: BoxFeature, Max: geom.Vec3{X: 1, Y: 1, Z: 1}},
			"split": {Kind: PlanarEdgeSplitFeature, Input: "base", PlanarEdgeSplit: &PlanarEdgeSplitRecipe{Roles: [2]string{"y-min", "z-max"}, Parameters: []string{tc.parameter}}},
		}}
		if _, err := s.EvaluateFeatures(context.Background()); !errors.Is(err, tc.err) {
			t.Fatalf("authored parameter %q: got %v want %v", tc.parameter, err, tc.err)
		}
	}
}
