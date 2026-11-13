package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func coloredAssemblyFixture(t *testing.T) step.ColoredAssembly {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/colored-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadColoredAssembly(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestColoredSTEPAssemblyDocumentRoundtrip(t *testing.T) {
	a := coloredAssemblyFixture(t)
	d, e := FromColoredSTEPAssembly(a, STEPAssemblyOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	if len(s.Planar) != 1 || len(s.Nodes) != 3 {
		t.Fatal(s)
	}
	var out bytes.Buffer
	if e := s.WriteColoredSTEPAssembly(&out, "root"); e != nil {
		t.Fatal(e)
	}
	got, e := step.ReadColoredAssembly(&out, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	colored := 0
	for _, bodies := range got.ProductBodies {
		for _, b := range bodies {
			if b.SurfaceColor == nil || len(b.FaceColors) != 1 {
				t.Fatal(b)
			}
			if math.Abs(b.SurfaceColor.R-.2) > 1e-6 {
				t.Fatal(b)
			}
			colored++
			p, e := b.Body.Solid.MassProperties()
			if e != nil || math.Abs(p.Volume-6) > 1e-9 {
				t.Fatal(p, e)
			}
		}
	}
	if colored != 1 || len(got.Assembly.Occurrences) != 3 {
		t.Fatal(got)
	}
	d2, e := FromColoredSTEPAssembly(got, STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	s2, _ := d2.Snapshot()
	if len(s2.Planar) != 1 {
		t.Fatal(s2)
	}
}
func TestColoredSTEPAssemblyRegeneration(t *testing.T) {
	face := 0
	s := State{LengthUnit: .001, Features: map[string]Feature{"box": {Kind: BoxFeature, Max: geom.Vec3{X: 1, Y: 2, Z: 3}}}, Nodes: map[string]Node{"a": {Asset: "box", Transform: geom.Identity()}, "b": {Asset: "box", Transform: geom.Translation(geom.Vec3{X: 10})}}, Labels: map[string]Label{"face": {Target: FaceLabel, TargetID: "box", Face: &face, Attributes: colorAttributes(step.RGB{R: 1})}}}
	for _, width := range []float64{1, 4} {
		s.Features["box"] = Feature{Kind: BoxFeature, Max: geom.Vec3{X: width, Y: 2, Z: 3}}
		var b bytes.Buffer
		if e := s.WriteColoredSTEPAssembly(&b, "regenerated"); e != nil {
			t.Fatal(e)
		}
		a, e := step.ReadColoredAssembly(&b, step.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, bs := range a.ProductBodies {
			for _, body := range bs {
				n++
				if body.FaceColors[0] != (step.RGB{R: 1}) {
					t.Fatal(body)
				}
				p, e := body.Body.Solid.MassProperties()
				if e != nil || math.Abs(p.Volume-6*width) > 1e-9 {
					t.Fatal(p, e)
				}
			}
		}
		if n != 1 {
			t.Fatal(n)
		}
	}
}
func TestColoredSTEPAssemblyReject(t *testing.T) {
	for _, kind := range []string{"missing-product", "different-body", "face-index"} {
		a := coloredAssemblyFixture(t)
		for id, bs := range a.ProductBodies {
			if len(bs) == 0 {
				continue
			}
			switch kind {
			case "missing-product":
				delete(a.ProductBodies, id)
			case "different-body":
				other, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 5, Y: 5, Z: 5})
				bs[0].Body.Solid = other
				a.ProductBodies[id] = bs
			case "face-index":
				bs[0].FaceColors[99] = step.RGB{R: 1}
			}
			break
		}
		if _, e := FromColoredSTEPAssembly(a, STEPAssemblyOptions{}, 1); e == nil {
			t.Fatal(kind)
		}
	}
	d, e := FromColoredSTEPAssembly(coloredAssemblyFixture(t), STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	for id, l := range s.Labels {
		if l.Target == FaceLabel {
			delete(l.Attributes, "step.srgb.blue")
			s.Labels[id] = l
			break
		}
	}
	var out bytes.Buffer
	if e := s.WriteColoredSTEPAssembly(&out, "invalid"); e == nil || out.Len() != 0 {
		t.Fatal(e, out.Len())
	}
}

func TestOCCTColoredSTEPDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("requires optional OCCT STEP oracle")
	}
	d, e := FromColoredSTEPAssembly(coloredAssemblyFixture(t), STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	var b bytes.Buffer
	if e := s.WriteColoredSTEPAssembly(&b, "root"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "document.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "result.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if data, e := exec.CommandContext(ctx, oracle, out, "step-colored-assembly", path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, data)
	}
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Volume                 float64
		BodyColors, FaceColors [][3]float64
	}
	if e := json.Unmarshal(raw, &result); e != nil || math.Abs(result.Volume-12) > 1e-8 || len(result.BodyColors) != 1 || len(result.FaceColors) != 1 {
		t.Fatal(result, e)
	}
	if math.Abs(result.BodyColors[0][0]-.2) > 1e-6 || math.Abs(result.FaceColors[0][0]-.8) > 1e-6 {
		t.Fatal(result)
	}
}
