package document

import (
	"bytes"
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestHelicalPartRecipesPersistAndRegenerate(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a := HelicalGrooveRecipe{
		Origin: geom.Vec3{X: 1, Z: 2}, Radial: geom.Vec3{X: 1}, Axis: geom.Vec3{Z: 1},
		OuterRadius: 2, Height: 8, Lead: 2, Start: .5, End: 2.5, Hand: geom.RightHanded, Starts: 2,
		Profile: []geom.Vec2{{X: 1.5, Y: -.125}, {X: 2, Y: -.125}, {X: 2, Y: .625}, {X: 1.5, Y: .625}},
	}
	b := a
	b.OuterRadius, b.BoreRadius = 3, 1.75
	b.Profile = []geom.Vec2{{X: 1.75, Y: .5}, {X: 2.25, Y: .5}, {X: 2.25, Y: 1}, {X: 1.75, Y: 1}}
	link := HelicalPairPartRecipe{Shaft: a, Nut: b, SelectNut: true}
	d, e := New(State{LengthUnit: .001, Features: map[string]Feature{
		"shaft": {Kind: HelicalGrooveFeature, HelicalGroove: &a},
		"nut":   {Kind: HelicalPairPartFeature, HelicalPair: &link},
	}}, 3)
	if e != nil {
		t.Fatal(e)
	}
	a.Profile[0].Y = 99
	b.Profile[0].Y = 99
	check := func(d *Document, hand geom.HelixHandedness) {
		t.Helper()
		s, _ := d.Snapshot()
		if s.Features["shaft"].HelicalGroove.Hand != hand || s.Features["nut"].HelicalPair.Nut.Hand != hand ||
			s.Features["shaft"].HelicalGroove.Profile[0].Y != -.125 || s.Features["nut"].HelicalPair.Nut.Profile[0].Y != .5 {
			t.Fatal("wrong recipe snapshot")
		}
		result, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		for id, want := range map[string]float64{"shaft": 26.75 * math.Pi, "nut": 43.5 * math.Pi} {
			body := result.Trimmed[id]
			m, e := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil || math.Abs(m.Volume-want) > 1e-6 || body.Topology().InnerWires != 2 {
				t.Fatal(id, m.Volume, want, e)
			}
		}
	}
	tx := d.Begin()
	tx.State.Features["shaft"].HelicalGroove.Hand = geom.LeftHanded
	tx.State.Features["nut"].HelicalPair.Shaft.Hand = geom.LeftHanded
	tx.State.Features["nut"].HelicalPair.Nut.Hand = geom.LeftHanded
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var file bytes.Buffer
	if e := d.Write(&file); e != nil {
		t.Fatal(e)
	}
	copy, e := Read(&file, 0, 3)
	if e != nil {
		t.Fatal(e)
	}
	check(copy, geom.LeftHanded)
	if e := d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(d, geom.RightHanded)
	tx = d.Begin()
	tx.State.Features["nut"].HelicalPair.Nut.Profile[0].Y = .625
	tx.State.Features["nut"].HelicalPair.Nut.Profile[1].Y = .625
	if e := tx.Commit(); e == nil {
		t.Fatal("accepted a zero-clearance flank")
	}
	tx.Rollback()
	tx = d.Begin()
	tx.State.Features["nut"].HelicalPair.Reliefs.NutLower = trimmed.HelicalReliefSpec{Radius: 2.125, Depth: .5}
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var finished bytes.Buffer
	if e := d.Write(&finished); e != nil {
		t.Fatal(e)
	}
	loaded, e := Read(&finished, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	state, _ := loaded.Snapshot()
	if state.Features["nut"].HelicalPair.Reliefs.NutLower != (trimmed.HelicalReliefSpec{Radius: 2.125, Depth: .5}) {
		t.Fatal("lost relief recipe")
	}
	result, e := state.EvaluateFeatures(ctx)
	if e != nil {
		t.Fatal(e)
	}
	m, e := result.Trimmed["nut"].MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6, MaxSubdivisions: 128})
	if want := (43.5 - 93./128) * math.Pi; e != nil || math.Abs(m.Volume-want) > 1e-6 {
		t.Fatal(m, want, e)
	}
}
