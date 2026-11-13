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

func lensAssemblyDocument(t testing.TB) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/spline-lens-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadSplineLensAssemblySTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, STEPAssemblyOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func lensAssemblyDocumentRoundtrip(t testing.TB) []byte {
	t.Helper()
	d := lensAssemblyDocument(t)
	s, _ := d.Snapshot()
	if len(s.Trimmed) != 1 {
		t.Fatal("lost shared asset")
	}
	references := 0
	rotated := false
	for _, n := range s.Nodes {
		if n.Asset != "" {
			references++
		}
		if n.Name == "Rotated" {
			rotated = n.Transform == (geom.Affine{{0, 0, 1, 8}, {0, 1, 0, 0}, {-1, 0, 0, 4}})
		}
	}
	if references != 2 || !rotated {
		t.Fatal(s.Nodes)
	}
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e := Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	var out bytes.Buffer
	if e = s.WriteSplineLensAssemblySTEP(&out); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadSplineLensAssembly(bytes.NewReader(out.Bytes()), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal(a.Assembly)
	}
	count := 0
	for _, b := range a.ProductBodies {
		count += len(b)
	}
	if count != 1 {
		t.Fatal(count)
	}
	return out.Bytes()
}
func TestSplineLensAssemblyDocument(t *testing.T) { lensAssemblyDocumentRoundtrip(t) }
func TestSplineLensAssemblyDocumentRejectsEdits(t *testing.T) {
	for _, kind := range []string{"member", "source", "near-rigid", "unused"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := lensAssemblyDocument(t).Snapshot()
			switch kind {
			case "unused":
				for _, solid := range s.Trimmed {
					s.Trimmed["unused"] = solid
					break
				}
			case "member":
				for id, n := range s.Nodes {
					if n.Asset != "" {
						n.Transform = geom.Translation(geom.Vec3{X: 1})
						s.Nodes[id] = n
						break
					}
				}
			case "source":
				for key, label := range s.Labels {
					if a, ok := label.Attributes["step.source.body.id"]; ok {
						a.Integer++
						label.Attributes["step.source.body.id"] = a
						s.Labels[key] = label
						break
					}
				}
			case "near-rigid":
				for id, n := range s.Nodes {
					if n.Name == "Rotated" {
						n.Transform[0][0] = 1e-12
						s.Nodes[id] = n
					}
				}
			}
			var b bytes.Buffer
			if e := s.WriteSplineLensAssemblySTEP(&b); e == nil || b.Len() != 0 {
				t.Fatal("unsupported edit emitted", e)
			}
		})
	}
}
func TestOCCTSplineLensAssemblyDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional STEP oracle")
	}
	raw := lensAssemblyDocumentRoundtrip(t)
	path := t.TempDir() + "/document.step"
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/report.json"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	data, e := exec.CommandContext(ctx, oracle, out, "step-assembly", path).CombinedOutput()
	cancel()
	if e != nil {
		t.Fatalf("%v: %s", e, data)
	}
	data, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Products    []string
		Occurrences []struct {
			Name      string
			Transform [3][4]float64
		}
		Volume float64
	}
	if e = json.Unmarshal(data, &got); e != nil {
		t.Fatal(e)
	}
	if len(got.Products) != 2 || len(got.Occurrences) != 2 || math.Abs(got.Volume-math.Pi/256) > 1e-10 {
		t.Fatal(got)
	}
	for _, occ := range got.Occurrences {
		if occ.Name == "Rotated" && geom.Affine(occ.Transform) != (geom.Affine{{0, 0, 1, 8}, {0, 1, 0, 0}, {-1, 0, 0, 4}}) {
			t.Fatal(occ)
		}
	}
}
func TestShearedSplineLensAssemblyDocument(t *testing.T) {
	raw, e := os.ReadFile("../exchange/step/testdata/sheared-spline-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	body, e := step.ReadSplineLensBody(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	model := step.SplineLensAssemblyModel{LengthUnit: body.LengthUnit, Products: []step.SplineLensAssemblyPart{{ID: "root", Name: "Group"}, {ID: "lens", Name: "Sheared local lens", Bodies: []step.SplineLensBody{body}}}, Occurrences: []step.AssemblyInstance{{ID: "a", Parent: "root", Child: "lens", Transform: geom.Identity()}, {ID: "b", Parent: "root", Child: "lens", Transform: geom.Affine{{0, 0, 1, 8}, {0, 1, 0, 0}, {-1, 0, 0, 4}}}}}
	var source bytes.Buffer
	if e = step.WriteSplineLensAssembly(&source, model); e != nil {
		t.Fatal(e)
	}
	d, e := ReadSplineLensAssemblySTEP(&source, step.Limits{}, step.CurvedOvalOptions{}, STEPAssemblyOptions{}, 2)
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
	if e = s.WriteSplineLensAssemblySTEP(&out); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadSplineLensAssembly(&out, step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, bodies := range a.ProductBodies {
		for _, b := range bodies {
			if b.Shear != -.125 {
				t.Fatal("lost shear")
			}
			count++
		}
	}
	if count != 1 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("lost sharing")
	}
}
