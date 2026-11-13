package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func mixedWrappedDocument(t *testing.T) *Document {
	t.Helper()
	m := step.WrappedMaterialModel{LengthUnit: .001}
	for _, name := range []string{"wrapped-toroid", "wrapped-cylinder", "wrapped-station-loft"} {
		raw, e := os.ReadFile("../exchange/step/testdata/" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		one, e := step.ReadWrappedMaterials(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		m.Bodies = append(m.Bodies, one.Bodies...)
	}
	d, e := FromWrappedMaterialsSTEP(m, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func mixedPlacedState(t *testing.T) State {
	s, _ := mixedWrappedDocument(t).Snapshot()
	s.Nodes = map[string]Node{
		"group": {Name: "Translated group", Transform: geom.Translation(geom.Vec3{Y: 16})},
		"a":     {Name: "Toroid", Asset: "body-000001", Transform: geom.Identity()},
		"b":     {Name: "Cylinder", Asset: "body-000002", Transform: geom.Translation(geom.Vec3{X: 16})},
		"c":     {Name: "Loft one", Asset: "body-000003", Transform: geom.Translation(geom.Vec3{X: 32})},
		"d":     {Name: "Loft two", Parent: "group", Asset: "body-000003", Transform: geom.Translation(geom.Vec3{Z: 8})},
	}
	return s
}
func TestMixedWrappedDocument(t *testing.T) {
	s := mixedPlacedState(t)
	d, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if len(s.Trimmed) != 3 || s.Nodes["c"].Asset != s.Nodes["d"].Asset || s.Nodes["d"].Parent != "group" {
		t.Fatal("sharing/hierarchy lost")
	}
	var out bytes.Buffer
	if e = s.WriteWrappedMaterialsSTEP(&out); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadWrappedMaterials(&out, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 4 || m.LengthUnit != .001 || m.Bodies[0].Toroid == nil || m.Bodies[1].Cylinder == nil || m.Bodies[2].StationLoft == nil || m.Bodies[3].StationLoft == nil {
		t.Fatal("family/unit loss")
	}
	a, b := m.Bodies[2].StationLoft, m.Bodies[3].StationLoft
	if a.Name != "Loft one" || b.Name != "Loft two" || a.Center.X != 32 || b.Center.Y != 16 || b.Stations[0].Z != 8 {
		t.Fatal("placement/name loss", a, b)
	}
	label := s.Labels["step:body-000003"]
	label.TargetID = "body-000002"
	s.Labels["step:body-000003"] = label
	var bad bytes.Buffer
	if e = s.WriteWrappedMaterialsSTEP(&bad); e == nil || bad.Len() != 0 {
		t.Fatal("stale target accepted", e)
	}
}
func TestMixedWrappedDocumentReject(t *testing.T) {
	for _, which := range []string{"rotation", "lossy", "family", "asset"} {
		t.Run(which, func(t *testing.T) {
			s := mixedPlacedState(t)
			n := s.Nodes["c"]
			switch which {
			case "rotation":
				n.Transform = geom.Affine{{0, -1, 0, 0}, {1, 0, 0, 0}, {0, 0, 1, 0}}
			case "lossy":
				n.Transform = geom.Translation(geom.Vec3{X: math.Ldexp(1, -80)})
			case "family":
				l := s.Labels["step:body-000003"]
				l.Attributes["step.wrapped.family"] = Attribute{Kind: "string", Text: "tube"}
				s.Labels["step:body-000003"] = l
			case "asset":
				s.Trimmed["body-000003"] = s.Trimmed["body-000002"]
			}
			s.Nodes["c"] = n
			var b bytes.Buffer
			if e := s.WriteWrappedMaterialsSTEP(&b); e == nil || b.Len() != 0 {
				t.Fatal("invalid state accepted", e)
			}
		})
	}
}
func TestOCCTMixedWrappedDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	s := mixedPlacedState(t)
	var b bytes.Buffer
	if e := s.WriteWrappedMaterialsSTEP(&b); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/mixed.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	var wantA, wantV float64
	for i, p := range []string{"../exchange/step/testdata/wrapped-toroid.step", "../exchange/step/testdata/wrapped-cylinder.step", "../exchange/step/testdata/wrapped-station-loft.step", path} {
		out := t.TempDir() + "/mass.json"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		data, e := exec.CommandContext(ctx, oracle, out, "step-mass-precise", p).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatalf("%v %s", e, data)
		}
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var m struct{ Area, Volume float64 }
		if e = json.Unmarshal(raw, &m); e != nil {
			t.Fatal(e)
		}
		if i < 3 {
			factor := 1.
			if i == 2 {
				factor = 2
			}
			wantA += factor * m.Area
			wantV += factor * m.Volume
		} else if math.Abs(m.Area-wantA) > 1e-7 || math.Abs(m.Volume-wantV) > 1e-7 {
			t.Fatal("independent mass mismatch", m, wantA, wantV)
		}
	}
}

func TestMixedWrappedAllFamiliesImport(t *testing.T) {
	model := step.WrappedMaterialModel{LengthUnit: .0254}
	for _, name := range []string{"wrapped-toroid", "wrapped-cylinder", "wrapped-frustum", "wrapped-tube", "wrapped-blind-bore", "wrapped-shoulder", "wrapped-cavity", "wrapped-toroidal-void", "wrapped-station-loft"} {
		raw, e := os.ReadFile("../exchange/step/testdata/" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		one, e := step.ReadWrappedMaterials(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		model.Bodies = append(model.Bodies, one.Bodies...)
	}
	d, e := FromWrappedMaterialsSTEP(model, 1)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&native, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	if len(s.Trimmed) != 9 || len(s.Nodes) != 9 || s.LengthUnit != .0254 {
		t.Fatal("import lost bodies/units")
	}
	families := map[string]bool{}
	for _, l := range s.Labels {
		families[l.Attributes["step.wrapped.family"].Text] = true
	}
	if len(families) != 9 {
		t.Fatal("family labels", families)
	}
	var bad bytes.Buffer
	if e = s.WriteWrappedMaterialsSTEP(&bad); e != nil {
		t.Fatal(e)
	}
	restored, e := step.ReadWrappedMaterials(&bad, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil || len(restored.Bodies) != 9 {
		t.Fatal("nine-family export", e)
	}
}
