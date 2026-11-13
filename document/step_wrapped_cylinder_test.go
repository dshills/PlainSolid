package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func wrappedCylinderDocument(t *testing.T) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/wrapped-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadWrappedCylindricalBodies(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.LengthUnit = .0254
	m.Bodies[0].Name = "Weighted cylinder"
	m.Bodies[0].Face.Name = "Wrapped skin"
	m.Bodies[0].CapNames = [2]string{"Lower disk", "Upper disk"}
	d, e := FromWrappedCylindricalSTEP(m, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestWrappedCylinderSTEPDocument(t *testing.T) {
	d := wrappedCylinderDocument(t)
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
	if e = s.WriteWrappedCylindricalSTEP(&out); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadWrappedCylindricalBodies(&out, step.Limits{}, step.WrappedSurfaceOptions{})
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
		if b.CapNames != ([2]string{"Lower disk", "Upper disk"}) {
			t.Fatal(b.CapNames)
		}
		for k, c := range [3]*big.Rat{b.Embedding.CenterX, b.Embedding.CenterY, new(big.Rat).Add(b.Embedding.LowerZ, big.NewRat(2, 1))} {
			if !c.IsInt() || c.Num().Int64() != want[i][k] {
				t.Fatal(i, b.Embedding)
			}
		}
	}
	n := s.Nodes["a"]
	n.Transform = geom.Affine{{0, -1, 0, 0}, {1, 0, 0, 0}, {0, 0, 1, 0}}
	s.Nodes["a"] = n
	var bad bytes.Buffer
	if e = s.WriteWrappedCylindricalSTEP(&bad); e == nil || bad.Len() != 0 {
		t.Fatal("rotation accepted", e)
	}
}

func TestOCCTWrappedCylinderDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("requires optional OCCT STEP oracle")
	}
	s, _ := wrappedCylinderDocument(t).Snapshot()
	s.LengthUnit = .001
	s.Nodes = map[string]Node{"a": {Name: "First", Asset: "body-000001", Transform: geom.Identity()}, "b": {Name: "Second", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{X: 16})}}
	var b bytes.Buffer
	if e := s.WriteWrappedCylindricalSTEP(&b); e != nil {
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
	for i, p := range []string{"../exchange/step/testdata/wrapped-cylinder.step", path} {
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

func TestWrappedCylinderDocumentReject(t *testing.T) {
	s, _ := wrappedCylinderDocument(t).Snapshot()
	node := s.Nodes["body-000001"]
	node.Transform = geom.Translation(geom.Vec3{X: math.Ldexp(1, -80)})
	s.Nodes["body-000001"] = node
	var out bytes.Buffer
	if e := s.WriteWrappedCylindricalSTEP(&out); e == nil || out.Len() != 0 {
		t.Fatal("accepted lossy translation", e)
	}
	toroid, _ := wrappedToroidDocument(t).Snapshot()
	if e := toroid.WriteWrappedCylindricalSTEP(&out); e == nil || out.Len() != 0 {
		t.Fatal("accepted unsupported asset", e)
	}
}
