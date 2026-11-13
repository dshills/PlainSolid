package document

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func sketchState(kind FeatureKind) State {
	d := geom.Vec3{Z: 1}
	if kind == PlanarPocketFeature {
		d.Z = -1
	}
	return State{LengthUnit: 1, Features: map[string]Feature{"base": {Kind: BoxFeature, Max: geom.Vec3{X: 4, Y: 4, Z: 4}}, "cut": {Kind: kind, Input: "base", EntryRole: "z-max", Profile: []geom.Vec3{{X: 1, Y: 1, Z: 4}, {X: 3, Y: 1, Z: 4}, {X: 3, Y: 3, Z: 4}, {X: 1, Y: 3, Z: 4}}, Offset: d}}, Nodes: map[string]Node{"part": {Asset: "cut", Transform: geom.Identity()}}}
}
func checkSketchVolume(t *testing.T, s State, want float64) {
	t.Helper()
	r, e := s.EvaluateFeatures(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Planar["cut"].MassProperties()
	if e != nil || math.Abs(m.Volume-want) > 1e-10 {
		t.Fatal(e, m)
	}
}
func TestSketchFeatureRegenerationAndPersistence(t *testing.T) {
	for _, kind := range []FeatureKind{PlanarProtrusionFeature, PlanarPocketFeature} {
		s := sketchState(kind)
		want := 68.
		if kind == PlanarPocketFeature {
			want = 60
		}
		checkSketchVolume(t, s, want)
		d, e := New(s, 3)
		if e != nil {
			t.Fatal(e)
		}
		var buf bytes.Buffer
		if e = d.Write(&buf); e != nil {
			t.Fatal(e)
		}
		loaded, e := Read(&buf, 0, 3)
		if e != nil {
			t.Fatal(e)
		}
		snap, _ := loaded.Snapshot()
		checkSketchVolume(t, snap, want)
		tx := d.Begin()
		f := tx.State.Features["cut"]
		f.Offset.Z *= 2
		tx.State.Features["cut"] = f
		if e = tx.Commit(); e != nil {
			t.Fatal(e)
		}
		snap, _ = d.Snapshot()
		checkSketchVolume(t, snap, 2*want-64)
		if e = d.Undo(); e != nil {
			t.Fatal(e)
		}
		snap, _ = d.Snapshot()
		checkSketchVolume(t, snap, want)
		if e = d.Redo(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestSketchFeatureNamedTransformedInput(t *testing.T) {
	s := sketchState(PlanarProtrusionFeature)
	s.Features["moved"] = Feature{Kind: TransformFeature, Input: "base", Transform: geom.Translation(geom.Vec3{X: 8})}
	f := s.Features["cut"]
	f.Input = "moved"
	for i := range f.Profile {
		f.Profile[i].X += 8
	}
	s.Features["cut"] = f
	checkSketchVolume(t, s, 68)
	if _, e := s.ResolveBoxFace(FeatureFace{"cut", "z-max"}); e == nil {
		t.Fatal("triangulated output must not inherit box indices")
	}
}
func TestSketchFeatureFailuresAtomic(t *testing.T) {
	for _, failure := range []string{"role", "through", "hole", "cycle"} {
		s := sketchState(PlanarPocketFeature)
		f := s.Features["cut"]
		switch failure {
		case "role":
			f.EntryRole = "bad"
		case "through":
			f.Offset.Z = -4
		case "hole":
			f.Holes = [][]geom.Vec3{f.Profile}
		case "cycle":
			f.Input = "cut"
		}
		s.Features["cut"] = f
		r, e := s.EvaluateFeatures(context.Background())
		if e == nil || len(r.Planar) != 0 {
			t.Fatal(failure, e)
		}
		if len(s.Features) != 2 || len(s.Planar) != 0 {
			t.Fatal("input mutated")
		}
	}
}
func TestNamedThroughPocket(t *testing.T) {
	s := sketchState(PlanarPocketFeature)
	f := s.Features["cut"]
	f.Kind = PlanarThroughPocketFeature
	f.ExitRole = "z-min"
	f.Offset = geom.Vec3{X: .5, Z: -4}
	s.Features["cut"] = f
	checkSketchVolume(t, s, 48)
	f.ExitRole = "x-min"
	s.Features["cut"] = f
	if _, e := s.EvaluateFeatures(context.Background()); e == nil {
		t.Fatal("nonparallel exit")
	}
}
