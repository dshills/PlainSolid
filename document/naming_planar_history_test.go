package document

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"plainsolid/csg/poly"
	"plainsolid/geom"
)

func TestPlanarFeatureFaceHistoryRegeneration(t *testing.T) {
	ctx := context.Background()
	state := State{LengthUnit: .001, Features: map[string]Feature{
		"stock":    {Kind: BoxFeature, Max: geom.Vec3{X: 4, Y: 4, Z: 4}},
		"tool":     {Kind: BoxFeature, Min: geom.Vec3{X: 1, Y: 1, Z: 1}, Max: geom.Vec3{X: 3, Y: 3, Z: 5}},
		"cut":      {Kind: BooleanFeature, Operation: poly.Difference, Input: "stock", Tool: "tool"},
		"moved":    {Kind: TransformFeature, Input: "cut", Transform: geom.Affine{{-1, 0, 0, 7}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
		"disabled": {Kind: PlanarOffsetFeature, Input: "stock", Distance: .25, Tolerance: 1e-9},
	}}
	d, err := New(state, 2)
	if err != nil {
		t.Fatal(err)
	}
	read := func(snapshot State, source, role, target string, status NamedFaceStatus, reversed bool) ResolvedPlanarFaceHistory {
		t.Helper()
		out, err := snapshot.ResolvePlanarFaceHistory(ctx, PlanarFaceHistory{Source: FeatureFace{source, role}, Target: target})
		if err != nil || out.Status != status {
			t.Fatal(source, role, target, status, out, err)
		}
		got, err := snapshot.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		faceCount := len(got.Planar[target].Faces())
		last := -1
		for _, f := range out.Faces {
			if f.Index <= last || f.Index >= faceCount || f.Reversed != reversed {
				t.Fatal("invalid boundary ownership", f, out)
			}
			last = f.Index
		}
		return out
	}
	ref := PlanarFaceHistory{Source: FeatureFace{"tool", "z-min"}, Target: "moved"}
	initial, _ := d.Snapshot()
	read(initial, "stock", "z-max", "stock", NamedFaceUnique, false)
	read(initial, "stock", "z-max", "moved", NamedFaceMultiple, false)
	read(initial, "tool", "z-max", "moved", NamedFaceDeleted, false)
	prior := read(initial, "tool", "z-min", "moved", NamedFaceMultiple, true)
	eval, err := initial.EvaluateFeatures(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range prior.Faces {
		p, err := eval.Planar["moved"].FacePlane(f.Index)
		if err != nil || p.Normal() != (geom.Vec3{Z: 1}) || p.SignedDistance(geom.Vec3{Z: 1}) != 0 {
			t.Fatal("a reversed cutter floor should face the positive Z void", p, err)
		}
	}
	for _, c := range []struct {
		target string
		want   error
	}{{"stock", ErrUnrelatedFeature}, {"disabled", geom.ErrUnrepresentable}} {
		ref := ref
		ref.Target = c.target
		if c.target == "disabled" {
			ref.Source.Feature = "stock"
		}
		if _, err := initial.ResolvePlanarFaceHistory(ctx, ref); !errors.Is(err, c.want) {
			t.Fatal("must distinguish unsupported from unrelated lineage", c, err)
		}
	}
	tx := d.Begin()
	tool := tx.State.Features["tool"]
	tool.Min.Z = 5
	tool.Max.Z = 7
	tx.State.Features["tool"] = tool
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	edited, _ := d.Snapshot()
	read(edited, "tool", "z-min", "moved", NamedFaceDeleted, false)
	if err := d.Undo(); err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := d.Write(&raw); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(raw.Bytes()), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := reopened.Snapshot()
	if recreated, err := snapshot.ResolvePlanarFaceHistory(ctx, ref); err != nil || !reflect.DeepEqual(recreated, prior) {
		t.Fatal("reopen should independently regenerate the original source reference", recreated, err)
	}
}
