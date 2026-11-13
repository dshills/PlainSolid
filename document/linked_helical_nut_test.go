package document

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestLinkedHelicalNutFollowsShaftEdits(t *testing.T) {
	ctx := context.Background()
	shaft := HelicalGrooveRecipe{
		Radial: geom.Vec3{X: 1}, Axis: geom.Vec3{Z: 1},
		OuterRadius: 2, Height: 8, Lead: 2, Start: .5, End: 2.5, Hand: geom.RightHanded, Starts: 2,
		Profile: []geom.Vec2{{X: 1.5, Y: -.125}, {X: 2, Y: -.125}, {X: 2, Y: .625}, {X: 1.5, Y: .625}},
	}
	nut := LinkedHelicalNutRecipe{Nut: HelicalGrooveRecipe{
		OuterRadius: 3, BoreRadius: 1.75, Height: 8,
		Profile: []geom.Vec2{{X: 1.75, Y: .5}, {X: 2.25, Y: .5}, {X: 2.25, Y: 1}, {X: 1.75, Y: 1}},
	}}
	d, e := New(State{LengthUnit: .001, Features: map[string]Feature{
		"shaft": {Kind: HelicalGrooveFeature, HelicalGroove: &shaft},
		"nut":   {Kind: LinkedHelicalNutFeature, Input: "shaft", LinkedHelicalNut: &nut},
	}}, 3)
	if e != nil {
		t.Fatal(e)
	}
	nut.Nut.Profile[0].Y = 99
	measure := func(d *Document, wantShift float64) {
		t.Helper()
		s, _ := d.Snapshot()
		if s.Features["nut"].LinkedHelicalNut.Nut.Profile[0].Y != .5 {
			t.Fatal("aliased nut profile")
		}
		m, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		for id, want := range map[string]float64{"shaft": 26.75 * math.Pi, "nut": 43.5 * math.Pi} {
			part := m.Trimmed[id]
			p, e := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			b, e2 := part.Bounds()
			if e != nil || e2 != nil || math.Abs(p.Volume-want) > 1e-6 || math.Abs(b.Min.Z-wantShift) > 1e-9 {
				t.Fatal(id, p.Volume, want, b, e, e2)
			}
		}
	}
	tx := d.Begin()
	tx.State.Features["shaft"].HelicalGroove.Origin.Z = 2
	tx.State.Features["shaft"].HelicalGroove.Hand = geom.LeftHanded
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	measure(d, 2)
	tx = d.Begin()
	delete(tx.State.Features, "shaft")
	if e := tx.Commit(); e == nil {
		t.Fatal("orphaned nut accepted")
	}
	tx.Rollback()
	tx = d.Begin()
	tx.State.Features["shaft"].HelicalGroove.Lead = 4
	if e := tx.Commit(); e == nil {
		t.Fatal("accepted mismatched flank spacing after shaft pitch edit")
	}
	tx.Rollback()
	measure(d, 2)
	var raw bytes.Buffer
	if e := d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	saved, e := Read(&raw, 0, 3)
	if e != nil {
		t.Fatal(e)
	}
	measure(saved, 2)
	if e := d.Undo(); e != nil {
		t.Fatal(e)
	}
	measure(d, 0)
}
