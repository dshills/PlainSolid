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
	"reflect"
	"testing"
	"time"
)

func lensDocument(t testing.TB) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestSplineLensDocumentChain(t *testing.T) {
	d := lensDocument(t)
	s, _ := d.Snapshot()
	s.LengthUnit = .0254
	n := s.Nodes["body-000001"]
	n.Name = "Named exact lens"
	s.Nodes["body-000001"] = n
	label := s.Labels["step:body-000001"]
	label.Attributes["step.lens.upper.name"] = Attribute{Kind: "string", Text: "upper original"}
	label.Attributes["step.lens.lower.name"] = Attribute{Kind: "string", Text: "lower original"}
	s.Labels["step:body-000001"] = label
	original := label.Attributes["step.lens.provenance"].Text
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
	if s.Labels["step:body-000001"].Attributes["step.lens.provenance"].Text != original {
		t.Fatal("source roles changed")
	}
	var out bytes.Buffer
	if e = s.WriteSplineLensSTEP(&out); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadSplineLensBody(bytes.NewReader(out.Bytes()), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Name != n.Name || m.FaceNames != ([2]string{"upper original", "lower original"}) || m.LengthUnit != .0254 {
		t.Fatal(m)
	}
	d, e = FromSplineLensSTEP(m, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	out.Reset()
	if e = s.WriteSplineLensSTEP(&out); e != nil {
		t.Fatal(e)
	}
}
func TestSplineLensDocumentRejectsEdits(t *testing.T) {
	for _, kind := range []string{"source", "radius", "geometry", "placement", "instances"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := lensDocument(t).Snapshot()
			label := s.Labels["step:body-000001"]
			switch kind {
			case "source":
				a := label.Attributes["step.lens.provenance"]
				var p lensProvenance
				if e := json.Unmarshal([]byte(a.Text), &p); e != nil {
					t.Fatal(e)
				}
				p.Edges[0]++
				raw, _ := json.Marshal(p)
				a.Text = string(raw)
				label.Attributes["step.lens.provenance"] = a
				s.Labels["step:body-000001"] = label
			case "radius":
				label.Attributes["step.lens.radius.x"] = Attribute{Kind: "number", Number: .125}
				s.Labels["step:body-000001"] = label
			case "geometry":
				v, e := s.Trimmed["body-000001"].Transformed(geom.Translation(geom.Vec3{Z: 1}))
				if e != nil {
					t.Fatal(e)
				}
				s.Trimmed["body-000001"] = v
			case "placement":
				n := s.Nodes["body-000001"]
				n.Transform = geom.Translation(geom.Vec3{Z: 1})
				s.Nodes["body-000001"] = n
			case "instances":
				s.Nodes["copy"] = s.Nodes["body-000001"]
			}
			var b bytes.Buffer
			if e := s.WriteSplineLensSTEP(&b); e == nil || b.Len() != 0 {
				t.Fatal("unsupported edit exported", e)
			}
		})
	}
}
func TestOCCTSplineLensDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional STEP oracle")
	}
	source := t.TempDir() + "/source.step"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	data, e := exec.CommandContext(ctx, oracle, source, "write-step-spline-lens").CombinedOutput()
	cancel()
	if e != nil {
		t.Fatalf("%v: %s", e, data)
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
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
	s, _ := d.Snapshot()
	var exported bytes.Buffer
	if e = s.WriteSplineLensSTEP(&exported); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/document.step"
	if e = os.WriteFile(path, exported.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	data, e = exec.CommandContext(ctx, oracle, out, "step-mass-precise", path).CombinedOutput()
	cancel()
	if e != nil {
		t.Fatalf("%v: %s", e, data)
	}
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &mass); e != nil {
		t.Fatal(e)
	}
	if math.Abs(mass.Volume-math.Pi/512) > 1e-10 || math.Abs(mass.Area-4*math.Pi/3*(math.Pow(1+1./16, 1.5)-1)) > 1e-10 {
		t.Fatal(mass)
	}
}
func TestShearedSplineLensDocument(t *testing.T) {
	raw, e := os.ReadFile("../exchange/step/testdata/sheared-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
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
	s, _ := d.Snapshot()
	var out bytes.Buffer
	if e = s.WriteSplineLensSTEP(&out); e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadSplineLensBody(&out, step.Limits{}, step.CurvedOvalOptions{})
	if e != nil || m.Shear != -.125 {
		t.Fatal(m.Shear, e)
	}
	label := s.Labels["step:body-000001"]
	delete(label.Attributes, "step.lens.shear")
	s.Labels["step:body-000001"] = label
	out.Reset()
	if e = s.WriteSplineLensSTEP(&out); e == nil || out.Len() != 0 {
		t.Fatal("lost shear silently accepted", e)
	}
	legacy, _ := lensDocument(t).Snapshot()
	label = legacy.Labels["step:body-000001"]
	delete(label.Attributes, "step.lens.shear")
	legacy.Labels["step:body-000001"] = label
	if e = legacy.WriteSplineLensSTEP(&out); e != nil {
		t.Fatal("legacy zero-shear rejected", e)
	}
}
func TestRefinedSplineLensDocument(t *testing.T) {
	raw, e := os.ReadFile("../exchange/step/testdata/refined-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	source, e := step.ReadSplineLensBody(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
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
	s, _ := d.Snapshot()
	var out bytes.Buffer
	if e = s.WriteSplineLensSTEP(&out); e != nil {
		t.Fatal(e)
	}
	got, e := step.ReadSplineLensBody(&out, step.Limits{}, step.CurvedOvalOptions{})
	if e != nil || !reflect.DeepEqual(source.Lower.Data(), got.Lower.Data()) || !reflect.DeepEqual(source.Upper.Data(), got.Upper.Data()) {
		t.Fatal("native persistence altered basis", e)
	}
}
