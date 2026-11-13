package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestSideOpenDualDiskFeatureTracksStockAndPersists(t *testing.T) {
	ctx := context.Background()
	box := Feature{Kind: BoxFeature, Min: geom.Vec3{Y: 4}, Max: geom.Vec3{X: 10, Y: 10, Z: 6}}
	recipe := SideOpenDualDiskRecipe{Origin: geom.Vec3{Y: 4}, U: geom.Vec3{X: 1}, Inward: geom.Vec3{Z: 1},
		Discs: [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5}, Radius: 2.5}}, Depth: 3}
	s := State{LengthUnit: .001,
		Features: map[string]Feature{"stock": box, "pockets": {Kind: SideOpenDualDiskCutFeature, Input: "stock", SideOpenDualDisk: &recipe}},
		Nodes:    map[string]Node{"part": {Name: "Part", Asset: "pockets", Transform: geom.Identity()}},
	}
	d, e := New(s, 4)
	if e != nil {
		t.Fatal(e)
	}
	check := func(d *Document, height, depth float64) {
		t.Helper()
		s, _ := d.Snapshot()
		model, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		b := model.Trimmed["pockets"]
		if len(b.Topology().Shells) != 1 || model.Nodes["part"].Asset != "pockets" {
			t.Fatal("materialization lost stable asset ownership")
		}
		mass, e := b.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		want := 60*height - depth*(6.25*(math.Pi-math.Acos(.6))+3)
		if e != nil || math.Abs(mass.Volume-want) > 3e-7 {
			t.Fatal("regenerated analytic volume", mass.Volume, want, e)
		}
	}
	// Snapshot and constructor input both own their recipe data.
	recipe.Depth = 1
	snap, _ := d.Snapshot()
	snap.Features["pockets"].SideOpenDualDisk.Through = true
	check(d, 6, 3)

	tx := d.Begin()
	box.Max.Z = 9
	tx.State.Features["stock"] = box
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(d, 9, 3)
	tx = d.Begin()
	through := tx.State.Features["pockets"].SideOpenDualDisk
	through.Through, through.Depth = true, 0
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(d, 9, 9)
	if e := d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(d, 9, 3)
	if e := d.Redo(); e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e := d.Write(&buf); e != nil {
		t.Fatal(e)
	}
	reloaded, e := Read(&buf, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	check(reloaded, 9, 9)

	tx = reloaded.Begin()
	bad := tx.State.Features["pockets"].SideOpenDualDisk
	bad.Discs[1].Center.X = 6 // irrational seam after edit
	if e := tx.Commit(); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("unrepresented crossing edit should fail atomically", e)
	}
	check(reloaded, 9, 9)

	tx = reloaded.Begin()
	box.Min.Z = 1
	tx.State.Features["stock"] = box
	if e := tx.Commit(); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("entry cap was silently reassigned", e)
	}
}
