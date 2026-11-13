package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func splineShellDocument(t *testing.T) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadSplineShells(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.LengthUnit = .0254
	m.Bodies[0].Name = "Warped prism"
	d, e := FromSplineShellSTEP(m, 1)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func placedSplineShellState(t *testing.T) State {
	t.Helper()
	s, _ := splineShellDocument(t).Snapshot()
	rotation := geom.Affine{{0, -1, 0, 0}, {1, 0, 0, 0}, {0, 0, 1, 0}}
	s.Nodes = map[string]Node{"a": {Name: "First prism", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{X: 10})}, "parent": {Name: "Group", Transform: geom.Translation(geom.Vec3{Y: 20})}, "b": {Name: "Second prism", Asset: "body-000001", Parent: "parent", Transform: rotation}}
	return s
}
func TestSplineShellSTEPDocumentRoundtrip(t *testing.T) {
	d := splineShellDocument(t)
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e := Read(&native, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	if len(s.Trimmed) != 1 || s.LengthUnit != .0254 || s.Nodes["body-000001"].Name != "Warped prism" || s.Labels["step:body-000001"].Attributes["step.body.name"].Text != "Warped prism" {
		t.Fatal(s)
	}
	s = placedSplineShellState(t)
	var out bytes.Buffer
	if e := s.WriteSplineShellSTEP(&out); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadSplineShells(&out, step.Limits{})
	if e != nil || len(m.Bodies) != 2 || m.LengthUnit != .0254 {
		t.Fatal(m, e)
	}
	transforms := map[string]geom.Affine{"First prism": s.Nodes["a"].Transform, "Second prism": s.Nodes["parent"].Transform.Mul(s.Nodes["b"].Transform)}
	for _, b := range m.Bodies {
		tr, ok := transforms[b.Name]
		if !ok {
			t.Fatal(b.Name)
		}
		points := map[geom.Vec3]bool{}
		for _, p := range s.Trimmed["body-000001"].Vertices() {
			points[tr.Apply(p)] = true
		}
		for _, p := range b.Solid.Vertices() {
			if !points[p] {
				t.Fatal(p)
			}
		}
	}
	if len(s.Trimmed) != 1 {
		t.Fatal("mutated shared asset")
	}
}
func TestSplineShellSTEPPlacementReject(t *testing.T) {
	for _, tr := range []geom.Affine{geom.Scaling(geom.Vec3{X: 2, Y: 1, Z: 1}), geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1}), geom.Translation(geom.Vec3{X: math.Ldexp(1, -54)})} {
		s := placedSplineShellState(t)
		n := s.Nodes["a"]
		n.Transform = tr
		s.Nodes["a"] = n
		var out bytes.Buffer
		if e := s.WriteSplineShellSTEP(&out); e == nil || out.Len() != 0 {
			t.Fatal(tr, e, out.Len())
		}
	}
}
func TestOCCTSplineShellDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("requires optional OCCT STEP oracle")
	}
	s := placedSplineShellState(t)
	var b bytes.Buffer
	if e := s.WriteSplineShellSTEP(&b); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "document.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if data, e := exec.CommandContext(ctx, oracle, out, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, data)
	}
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Volume   float64
		Centroid [3]float64
	}
	if e := json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-10*math.Pow(25.4, 3)) > 1e-5 || math.Abs(p.Centroid[0]-5*25.4) > 1e-7 || math.Abs(p.Centroid[1]-(10+16.0/15)*25.4) > 1e-7 || math.Abs(p.Centroid[2]-29.0/45*25.4) > 1e-7 {
		t.Fatal(p, e)
	}
}

func TestSplineShellSTEPFeatureRegeneration(t *testing.T) {
	s, _ := splineShellDocument(t).Snapshot()
	s.Nodes = map[string]Node{"result": {Name: "Regenerated prism", Asset: "moved", Transform: geom.Identity()}}
	for _, shift := range []float64{2, 7} {
		s.Features = map[string]Feature{"moved": {Kind: TransformFeature, Input: "body-000001", Transform: geom.Translation(geom.Vec3{X: shift})}}
		var out bytes.Buffer
		if e := s.WriteSplineShellSTEP(&out); e != nil {
			t.Fatal(e)
		}
		m, e := step.ReadSplineShells(&out, step.Limits{})
		if e != nil || len(m.Bodies) != 1 || m.Bodies[0].Name != "Regenerated prism" {
			t.Fatal(m, e)
		}
		min := math.Inf(1)
		for _, p := range m.Bodies[0].Solid.Vertices() {
			min = math.Min(min, p.X)
		}
		if min != shift {
			t.Fatal(min, shift)
		}
	}
}
