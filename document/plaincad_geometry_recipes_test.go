package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestPlanarToFaceRecipeRebuildAndRecovery(t *testing.T) {
	ctx := context.Background()
	box := Feature{Kind: BoxFeature, Max: geom.Vec3{X: 30, Y: 20, Z: 10}}
	tool := Feature{Kind: PlanarToFaceFeature, Input: "stock", EntryRole: "z-max", Direction: geom.Vec3{Z: 1},
		Profile: []geom.Vec3{{}, {X: 10}, {X: 10, Y: 5}, {Y: 5}}}
	doc, e := New(State{LengthUnit: 1, Features: map[string]Feature{"stock": box, "tool": tool},
		Nodes: map[string]Node{"tool": {Asset: "tool", Transform: geom.Identity()}}}, 4)
	if e != nil {
		t.Fatal(e)
	}
	check := func(d *Document, h float64) {
		t.Helper()
		s, _ := d.Snapshot()
		evaluated, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		p := evaluated.Planar["tool"]
		m, e := p.MassProperties()
		b, e2 := p.Bounds()
		if e != nil || e2 != nil || math.Abs(m.Volume-50*h) > 1e-9 || b.Max.Z != h {
			t.Fatal(m, b, e, e2)
		}
	}
	check(doc, 10)
	tx := doc.Begin()
	box.Max.Z = 20
	tx.State.Features["stock"] = box
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(doc, 20)
	tx = doc.Begin()
	delete(tx.State.Features, "stock")
	if e := tx.Commit(); e == nil {
		t.Fatal("removed target accepted")
	}
	check(doc, 20)
	tx = doc.Begin()
	box.Max.X = 8
	tx.State.Features["stock"] = box
	if e := tx.Commit(); !errors.Is(e, brep.ErrTerminationUncovered) {
		t.Fatal("expected finite target to lose coverage", e)
	}
	tx.Rollback()
	if e := doc.Undo(); e != nil {
		t.Fatal(e)
	}
	check(doc, 10)
	if e := doc.Redo(); e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e := doc.Write(&buf); e != nil {
		t.Fatal(e)
	}
	copy, e := Read(&buf, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	check(copy, 20)
}

func TestPlanarRevolveRecipeAxisEdit(t *testing.T) {
	ctx := context.Background()
	recipe := Feature{Kind: PlanarRevolveFeature, Distance: math.Pi / 2, Direction: geom.Vec3{Y: 1},
		Profile: []geom.Vec3{{}, {X: 20}, {X: 20, Y: 10}, {Y: 10}}}
	doc, e := New(State{LengthUnit: 1, Features: map[string]Feature{"revolve": recipe}}, 2)
	if e != nil {
		t.Fatal(e)
	}
	check := func(d *Document, want float64) {
		t.Helper()
		s, _ := d.Snapshot()
		result, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		m, e := result.Trimmed["revolve"].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(m.Volume-want) > 1e-8 {
			t.Fatal(m, want, e)
		}
	}
	check(doc, 1000*math.Pi)
	tx := doc.Begin()
	recipe.Direction = geom.Vec3{X: 1}
	recipe.Distance = -math.Pi / 2
	tx.State.Features["revolve"] = recipe
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(doc, 500*math.Pi)
	tx = doc.Begin()
	recipe.Distance = 2*math.Pi + .01
	tx.State.Features["revolve"] = recipe
	if e := tx.Commit(); e == nil {
		t.Fatal("accepted angle greater than full turn")
	}
	check(doc, 500*math.Pi)
}

func TestLongCapChainRecipeIsolationAndEdits(t *testing.T) {
	ctx := context.Background()
	p := LongCapChainRecipe{Length: 20, Width: 10, Height: 10, Radius: 1, TopSetback: 1, SideSetback: 1,
		Bore:   trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1},
		Pocket: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}},
		Boss: trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}},
			Height: 3, Upper: true}}
	doc, e := New(State{LengthUnit: 1, Features: map[string]Feature{"part": {Kind: LongCapChainFeature, LongCapChain: &p}},
		Nodes: map[string]Node{"placed": {Asset: "part", Transform: geom.Translation(geom.Vec3{X: 1})}}}, 3)
	if e != nil {
		t.Fatal(e)
	}
	p.Height = 999
	check := func(d *Document, depth, radius float64) {
		t.Helper()
		s, _ := d.Snapshot()
		if s.Features["part"].LongCapChain.Height != depth {
			t.Fatal("recipe state was aliased")
		}
		v, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		m, e := v.Trimmed["part"].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		want := 200*depth - 20*radius*radius*(1-math.Pi/4) - 10 - depth*math.Pi - 9*depth + 12
		if e != nil || math.Abs(m.Volume-want) > 1e-7 {
			t.Fatal(m, want, e)
		}
	}
	check(doc, 10, 1)
	tx := doc.Begin()
	tx.State.Features["part"].LongCapChain.Height = 12
	tx.State.Features["part"].LongCapChain.Radius = 2
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(doc, 12, 2)
	tx = doc.Begin()
	tx.State.Features["part"].LongCapChain.Radius = 6
	if e := tx.Commit(); e == nil {
		t.Fatal("accepted intrusion into bore clearance")
	}
	check(doc, 12, 2)
	var buf bytes.Buffer
	if e := doc.Write(&buf); e != nil {
		t.Fatal(e)
	}
	copy, e := Read(&buf, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	check(copy, 12, 2)
	if e := doc.Undo(); e != nil {
		t.Fatal(e)
	}
	check(doc, 10, 1)
}
