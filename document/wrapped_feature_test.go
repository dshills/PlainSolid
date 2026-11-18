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

func TestWrappedCylinderFeaturesRegenerateAndUndo(t *testing.T) {
	slowtest.Skip(t)
	features := map[string]Feature{
		"stock": {Kind: WrappedCylinderFeature, Center: geom.Vec3{Z: -2}, Radius: 2, Offset: geom.Vec3{Z: 5}},
		"tool":  {Kind: WrappedCylinderFeature, Center: geom.Vec3{Z: 1}, Radius: 1, Offset: geom.Vec3{Z: 2}},
		"bore":  {Kind: WrappedBoreFeature, Input: "stock", Tool: "tool"},
		"draft": {Kind: WrappedDraftFeature, Input: "stock", Distance: -1},
	}
	d, e := New(State{LengthUnit: .001, Features: features}, 3)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = d.Write(&data); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&data, 0, 3)
	if e != nil {
		t.Fatal(e)
	}
	check := func(boreVolume float64) {
		t.Helper()
		s, _ := d.Snapshot()
		p, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		for id, want := range map[string]float64{"stock": 20 * math.Pi, "tool": 20*math.Pi - boreVolume,
			"bore": boreVolume, "draft": 35 * math.Pi / 3} {
			part := p.Trimmed[id]
			m, e := part.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 2e-6})
			if e != nil || math.Abs(m.Volume-want) > 1e-5 {
				t.Fatal(id, m, want, e)
			}
		}
		index, e := s.ResolveWrappedCylinderFace(FeatureFace{Feature: "bore", Role: "upper-cap"})
		if e != nil || index != 2 || len(p.Trimmed["bore"].Faces()[index].Holes) != 1 {
			t.Fatal(index, e)
		}
	}
	check(18 * math.Pi)
	tx := d.Begin()
	tool := tx.State.Features["tool"]
	tool.Radius = .5
	tx.State.Features["tool"] = tool
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(19.5 * math.Pi)
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(18 * math.Pi)
}

func TestWrappedStationLoftAndOffsetRecipes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stations := []trimmed.WrappedLoftStation{{Z: 0, Radius: 2}, {Z: 3, Radius: 1}, {Z: 5, Radius: 2}}
	d, e := New(State{LengthUnit: 1, Features: map[string]Feature{
		"stock":  {Kind: WrappedCylinderFeature, Radius: 1, Offset: geom.Vec3{Z: 4}},
		"draft":  {Kind: WrappedDraftFeature, Input: "stock", Distance: 3},
		"offset": {Kind: WrappedFrustumOffsetFeature, Input: "draft", Distance: 1.25},
		"loft":   {Kind: WrappedStationLoftFeature, Stations: stations},
		"moved":  {Kind: TransformFeature, Input: "loft", Transform: geom.Translation(geom.Vec3{Z: 10})},
	}}, 2)
	if e != nil {
		t.Fatal(e)
	}
	// Neither mutating the input slice nor editing a later snapshot may
	// alter the document's committed feature definition.
	stations[1].Radius = 8
	var b bytes.Buffer
	if e := d.Write(&b); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&b, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	check := func(volume float64) {
		t.Helper()
		state, _ := d.Snapshot()
		m, e := state.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		for name, want := range map[string]float64{"loft": volume, "moved": volume, "offset": 52 * math.Pi} {
			p, e := m.Trimmed[name].MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(p.Volume-want) > 1e-8 {
				t.Fatal(name, p, e)
			}
		}
		for role, want := range map[string]int{"wall:1": 1, "upper-cap": 3} {
			i, e := state.ResolveWrappedStationLoftFace(FeatureFace{Feature: "moved", Role: role})
			if e != nil || i != want {
				t.Fatal(role, i, e)
			}
		}
		if i, e := state.ResolveWrappedCylinderFace(FeatureFace{Feature: "offset", Role: "upper-cap"}); e != nil || i != 2 {
			t.Fatal(i, e)
		}
	}
	check(35 * math.Pi / 3)
	tx := d.Begin()
	loft := tx.State.Features["loft"]
	loft.Stations[1].Radius = 2
	tx.State.Features["loft"] = loft
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(20 * math.Pi)
	if e := d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(35 * math.Pi / 3)
}

func TestWrappedCylinderFilletFaceRoles(t *testing.T) {
	ctx := context.Background()
	s := State{LengthUnit: 1, Features: map[string]Feature{
		"stock": {Kind: WrappedCylinderFeature, Radius: 3, Offset: geom.Vec3{Z: 5}},
		"round": {Kind: WrappedCylinderFilletFeature, Input: "stock", Radius: .5, Distance: 1},
	}}
	for _, change := range []bool{false, true} {
		if change {
			f := s.Features["round"]
			f.Radius = 0
			s.Features["round"] = f
		}
		p, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		m := p.Trimmed["round"]
		j, e := s.ResolveWrappedCylinderFace(FeatureFace{Feature: "round", Role: "upper-blend"})
		want := 2
		if change {
			want = 1
		}
		if e != nil || j != want {
			t.Fatal(j, e)
		}
		if x, ok := m.Faces()[j].Surface.(*geom.WrappedSurfacePatch); !ok || x.Domain().VMax != 1 {
			t.Fatal("upper blend role selected the wrong support")
		}
		k, e := s.ResolveWrappedCylinderFace(FeatureFace{Feature: "round", Role: "upper-cap"})
		if e != nil || k != m.Topology().Faces-1 {
			t.Fatal(k, e)
		}
		_, e = s.ResolveWrappedCylinderFace(FeatureFace{Feature: "round", Role: "lower-blend"})
		if (e != nil) != change {
			t.Fatal("removed lower blend must not retain a stale role", e)
		}
	}
}

func TestWrappedCylinderOffsetRecipePersistenceAndRoles(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	d, e := New(State{LengthUnit: 1, Features: map[string]Feature{
		"stock":  {Kind: WrappedCylinderFeature, Radius: 3, Offset: geom.Vec3{Z: 5}},
		"offset": {Kind: WrappedCylinderOffsetFeature, Input: "stock", Distance: .25},
	}}, 3)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&raw, 0, 3)
	if e != nil {
		t.Fatal(e)
	}
	check := func(round bool) {
		t.Helper()
		s, _ := d.Snapshot()
		ref := func(role string) (int, error) {
			return s.ResolveWrappedCylinderFace(FeatureFace{Feature: "offset", Role: role})
		}
		p, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		m := p.Trimmed["offset"]
		k, e := ref("upper-cap")
		if e != nil || k != m.Topology().Faces-1 {
			t.Fatal(k, m.Topology(), e)
		}
		j, e := ref("lower-blend")
		if (e == nil) != round || round && j != 1 {
			t.Fatal(j, e)
		}
	}
	check(true)
	tx := d.Begin()
	f := tx.State.Features["offset"]
	f.Distance = -.25
	tx.State.Features["offset"] = f
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(false)
	if e := d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(true)
}

func TestWrappedStationBoreRecipePersistence(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	station := func(r, peak float64) Feature {
		return Feature{Kind: WrappedStationLoftFeature, Stations: []trimmed.WrappedLoftStation{
			{Z: 0, Radius: r}, {Z: 2, Radius: peak}, {Z: 4, Radius: r},
		}}
	}
	d, e := New(State{LengthUnit: 1, Features: map[string]Feature{
		"outer": station(3, 4), "inner": station(1, 2),
		"bore":  {Kind: WrappedStationBoreFeature, Input: "outer", Tool: "inner"},
		"moved": {Kind: TransformFeature, Input: "bore", Transform: geom.Translation(geom.Vec3{Z: 8})},
	}}, 3)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e := d.Write(&data); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&data, 0, 3)
	if e != nil {
		t.Fatal(e)
	}
	check := func(want float64) {
		t.Helper()
		s, _ := d.Snapshot()
		state, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		bore := state.Trimmed["bore"]
		proof, e := bore.ValidateWrappedCircularStationBore(ctx, geom.SurfacePatchOptions{})
		if e != nil || bore.Topology().InnerWires != 2 {
			t.Fatal(proof, e)
		}
		for role, id := range map[string]int{"upper-cap": proof.Caps[1], "inner-wall:1": proof.InnerWalls[1], "outer-wall:0": proof.OuterWalls[0]} {
			got, e := s.ResolveWrappedStationBoreFace(FeatureFace{Feature: "moved", Role: role})
			if e != nil || got != id {
				t.Fatal(role, got, e)
			}
		}
		p, e := state.Trimmed["moved"].MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-want) > 1e-8 || math.Abs(p.Centroid.Z-10) > 1e-8 {
			t.Fatal(p, e)
		}
	}
	check(40 * math.Pi)
	tx := d.Begin()
	tx.State.Features["inner"] = station(1, 1)
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(136 * math.Pi / 3)
	if e := d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(40 * math.Pi)
}
