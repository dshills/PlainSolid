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

func wrappedToroidDocument(t *testing.T) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/wrapped-toroid.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadWrappedToroidalBodies(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.LengthUnit = .0254
	m.Bodies[0].Name = "Weighted toroid"
	m.Bodies[0].Face.Name = "Wrapped skin"
	d, e := FromWrappedToroidalSTEP(m, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestWrappedToroidSTEPDocument(t *testing.T) {
	d := wrappedToroidDocument(t)
	s, _ := d.Snapshot()
	s.Nodes = map[string]Node{"parent": {Name: "Group", Transform: geom.Translation(geom.Vec3{Y: 16})}, "a": {Name: "Left", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{X: 16})}, "b": {Name: "Right", Parent: "parent", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{Z: 8})}}
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
	if len(s.Trimmed) != 1 || s.Nodes["a"].Asset != s.Nodes["b"].Asset || s.Nodes["b"].Parent != "parent" || s.LengthUnit != .0254 {
		t.Fatal("native shared state")
	}
	var out bytes.Buffer
	if e = s.WriteWrappedToroidalSTEP(&out); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadWrappedToroidalBodies(&out, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 2 || m.LengthUnit != .0254 || m.Bodies[0].Name != "Left" || m.Bodies[1].Name != "Right" {
		t.Fatal(m)
	}
	want := [2][3]int64{{16, 0, 0}, {0, 16, 8}}
	for i, b := range m.Bodies {
		if b.Face.Name != "Wrapped skin" {
			t.Fatal(b.Face.Name)
		}
		for k, c := range b.Embedding.Center {
			if !c.IsInt() || c.Num().Int64() != want[i][k] {
				t.Fatal(i, b.Embedding.Center)
			}
		}
	}
	n := s.Nodes["a"]
	n.Transform = geom.Affine{{0, -1, 0, 0}, {1, 0, 0, 0}, {0, 0, 1, 0}}
	s.Nodes["a"] = n
	var bad bytes.Buffer
	if e = s.WriteWrappedToroidalSTEP(&bad); e == nil || bad.Len() != 0 {
		t.Fatal("rotation accepted", e)
	}
}

func TestOCCTWrappedToroidDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("requires optional OCCT STEP oracle")
	}
	s, _ := wrappedToroidDocument(t).Snapshot()
	s.LengthUnit = .001
	s.Nodes = map[string]Node{"a": {Name: "First", Asset: "body-000001", Transform: geom.Identity()}, "b": {Name: "Second", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{X: 16})}}
	var b bytes.Buffer
	if e := s.WriteWrappedToroidalSTEP(&b); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/document.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	var masses [2]struct {
		Volume   float64
		Centroid [3]float64
	}
	for i, p := range []string{"../exchange/step/testdata/wrapped-toroid.step", path} {
		out := t.TempDir() + "/mass.json"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		data, e := exec.CommandContext(ctx, oracle, out, "step-mass-precise", p).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatalf("%v: %s", e, data)
		}
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &masses[i]); e != nil {
			t.Fatal(e)
		}
	}
	if masses[0].Volume <= 0 || math.Abs(masses[1].Volume-2*masses[0].Volume) > 1e-8 || math.Abs(masses[1].Centroid[0]-8) > 1e-8 {
		t.Fatal(masses)
	}
}
