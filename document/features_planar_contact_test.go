package document

import (
	"bytes"
	"context"
	"math"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestPlanarInsetFeatureRegeneratesOriginalNames(t *testing.T) {
	ctx := context.Background()
	s := State{LengthUnit: .001, Features: map[string]Feature{
		"host":   {Kind: BoxFeature, Max: geom.Vec3{X: 4, Y: 4, Z: 1}},
		"tool":   {Kind: BoxFeature, Min: geom.Vec3{X: 1, Y: 1, Z: 1}, Max: geom.Vec3{X: 3, Y: 3, Z: 2}},
		"joined": {Kind: PlanarInsetJoinFeature, Input: "host", Tool: "tool"},
		"moved":  {Kind: TransformFeature, Input: "joined", Transform: geom.Affine{{-1, 0, 0, 7}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
	}}
	doc, err := New(s, 3)
	if err != nil {
		t.Fatal(err)
	}
	face := PlanarFaceHistory{Source: FeatureFace{"host", "z-max"}, Target: "moved"}
	wire := PlanarEdgeHistory{Source: FeatureEdge{Feature: "tool", Roles: [2]string{"x-min", "z-min"}}, Target: "moved"}
	check := func(st State, wantX float64) (ResolvedPlanarFaceHistory, ResolvedPlanarEdgeHistory) {
		t.Helper()
		eval, err := st.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		shape := eval.Planar["moved"]
		owner, err := st.ResolvePlanarFaceHistory(ctx, face)
		if err != nil || owner.Status != NamedFaceUnique || len(owner.Faces) != 1 || owner.Faces[0].Reversed || len(shape.Faces()[owner.Faces[0].Index].Holes) != 1 {
			t.Fatalf("regenerated inset lost the named original host face: %+v %v", owner, err)
		}
		edge, err := st.ResolvePlanarEdgeHistory(ctx, wire)
		if err != nil || len(edge.Intervals) != 1 {
			t.Fatalf("regenerated inset lost the named tool rim: %+v %v", edge, err)
		}
		p := edge.Intervals[0]
		w := shape.Edges()[p.Index]
		a, b := shape.Vertices()[w.Start], shape.Vertices()[w.End]
		if a.X != wantX || b.X != wantX || a.Z != 1 || b.Z != 1 || p.OutputFrom.Sign() != 0 || p.OutputTo.RatString() != "1" ||
			!(p.From.RatString() == "0" && p.To.RatString() == "1" || p.From.RatString() == "1" && p.To.RatString() == "0") {
			t.Fatalf("tool footprint jumped source edges: %v %v %+v", a, b, p)
		}
		deleted, err := st.ResolvePlanarFaceHistory(ctx, PlanarFaceHistory{Source: FeatureFace{"tool", "z-min"}, Target: "moved"})
		if err != nil || deleted.Status != NamedFaceDeleted {
			t.Fatalf("internal original tool face survived: %+v %v", deleted, err)
		}
		m, err := shape.MassProperties()
		if err != nil || math.Abs(m.Volume-20) > 1e-13 || math.Abs(m.SurfaceArea-56) > 1e-13 {
			t.Fatalf("regenerated inset material changed: %+v %v", m, err)
		}
		return owner, edge
	}
	initial, _ := doc.Snapshot()
	oldFace, oldEdge := check(initial, 6)
	tx := doc.Begin()
	tool := tx.State.Features["tool"]
	tool.Min.X += .25
	tool.Max.X += .25
	tx.State.Features["tool"] = tool
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	edited, _ := doc.Snapshot()
	check(edited, 5.75)
	bad := doc.Begin()
	tool.Min.Z, tool.Max.Z = 3, 4
	bad.State.Features["tool"] = tool
	if err := bad.Commit(); err == nil {
		t.Fatal("document committed an inset with no contact")
	}
	if err := doc.Undo(); err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := doc.Write(&raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Read(bytes.NewReader(raw.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := reloaded.Snapshot()
	newFace, newEdge := check(restored, 6)
	if !reflect.DeepEqual(oldFace, newFace) || !reflect.DeepEqual(oldEdge, newEdge) {
		t.Fatalf("document reopen lost original face or edge lineage: %+v %+v", newFace, newEdge)
	}
}

func TestPlanarSeamFeatureKeepsBothOriginalEdgeOwners(t *testing.T) {
	ctx := context.Background()
	st := State{LengthUnit: .001, Features: map[string]Feature{
		"a":      {Kind: BoxFeature, Max: geom.Vec3{X: 1, Y: 1, Z: 1}},
		"b":      {Kind: BoxFeature, Min: geom.Vec3{Z: 1}, Max: geom.Vec3{X: 1, Y: 1, Z: 2}},
		"joined": {Kind: PlanarSeamJoinFeature, Input: "a", Tool: "b"},
	}}
	inputs := [2]FeatureEdge{{Feature: "a", Roles: [2]string{"y-min", "z-max"}}, {Feature: "b", Roles: [2]string{"y-min", "z-min"}}}
	var ids [2]int
	for i, source := range inputs {
		e, err := st.ResolvePlanarEdgeHistory(ctx, PlanarEdgeHistory{Source: source, Target: "joined"})
		if err != nil || len(e.Intervals) != 1 {
			t.Fatalf("shared edge lost an original document owner: %+v %v", e, err)
		}
		ids[i] = e.Intervals[0].Index
		f, err := st.ResolvePlanarFaceHistory(ctx, PlanarFaceHistory{Source: FeatureFace{source.Feature, source.Roles[1]}, Target: "joined"})
		if err != nil || f.Status != NamedFaceDeleted {
			t.Fatalf("original internal face not deleted: %+v %v", f, err)
		}
	}
	if ids[0] != ids[1] {
		t.Fatal("one seam edge no longer has both original document owners", ids)
	}
}
