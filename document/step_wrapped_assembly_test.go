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

func wrappedDocumentAssembly(t *testing.T) []byte {
	t.Helper()
	bodies := []step.WrappedMaterialBody{}
	for _, name := range []string{"wrapped-cylinder", "wrapped-station-loft"} {
		raw, e := os.ReadFile("../exchange/step/testdata/" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		m, e := step.ReadWrappedMaterials(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bodies = append(bodies, m.Bodies...)
	}
	m := step.WrappedMaterialAssemblyModel{LengthUnit: .0254, Products: []step.WrappedMaterialAssemblyPart{{ID: "root", Name: "Assembly"}, {ID: "part", Name: "Mixed part", Bodies: bodies}}, Occurrences: []step.AssemblyInstance{{ID: "left", Name: "Left", Parent: "root", Child: "part", Transform: geom.Identity()}, {ID: "right", Name: "Right", Parent: "root", Child: "part", Transform: geom.Affine{{0, -1, 0, 16}, {1, 0, 0, 0}, {0, 0, 1, 0}}}}}
	var b bytes.Buffer
	if e := step.WriteWrappedMaterialAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestWrappedMaterialAssemblyDocument(t *testing.T) {
	d, e := ReadWrappedMaterialAssemblySTEP(bytes.NewReader(wrappedDocumentAssembly(t)), step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{}, 2)
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
	if len(s.Trimmed) != 2 || len(s.Nodes) != 7 || s.LengthUnit != .0254 {
		t.Fatal("native identity lost", len(s.Trimmed), len(s.Nodes))
	}
	var out bytes.Buffer
	if e = s.WriteWrappedMaterialAssemblySTEP(&out); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadWrappedMaterialAssembly(&out, step.Limits{}, step.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Products) != 2 || len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("product sharing")
	}
	found := false
	for _, p := range a.Assembly.Products {
		if p.Identifier == "part" {
			found = true
			if p.Name != "Mixed part" || len(a.ProductBodies[p.DefinitionID]) != 2 {
				t.Fatal(p)
			}
		}
	}
	if !found {
		t.Fatal("product identifier")
	}
	for _, o := range a.Assembly.Occurrences {
		if o.Name == "Right" && o.Transform != (geom.Affine{{0, -1, 0, 16}, {1, 0, 0, 0}, {0, 0, 1, 0}}) {
			t.Fatal("rotation lost")
		}
	}
	for id, n := range s.Nodes {
		if n.Asset != "" {
			n.Transform = geom.Translation(geom.Vec3{X: 1})
			s.Nodes[id] = n
			break
		}
	}
	var bad bytes.Buffer
	if e = s.WriteWrappedMaterialAssemblySTEP(&bad); e == nil || bad.Len() != 0 {
		t.Fatal("edited member accepted", e)
	}
}
func TestWrappedMaterialAssemblyDocumentProvenance(t *testing.T) {
	raw := wrappedDocumentAssembly(t)
	for _, kind := range []string{"stale", "inconsistent", "missing"} {
		t.Run(kind, func(t *testing.T) {
			d, e := ReadWrappedMaterialAssemblySTEP(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{}, 1)
			if e != nil {
				t.Fatal(e)
			}
			s, _ := d.Snapshot()
			for id, n := range s.Nodes {
				if n.Asset == "" && n.Parent != "" {
					key := "step.product:" + id
					l := s.Labels[key]
					switch kind {
					case "stale":
						l.TargetID = n.Parent
					case "inconsistent":
						l.Attributes["step.product.name"] = Attribute{Kind: "string", Text: "different"}
					case "missing":
						delete(s.Labels, key)
					}
					if kind != "missing" {
						s.Labels[key] = l
					}
					break
				}
			}
			var out bytes.Buffer
			if e = s.WriteWrappedMaterialAssemblySTEP(&out); e == nil || out.Len() != 0 {
				t.Fatal("stale structure accepted", e)
			}
		})
	}
	if _, e := ReadWrappedMaterialAssemblySTEP(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{MaxNodes: 3}, 1); e == nil {
		t.Fatal("expansion budget")
	}
}

func TestOCCTWrappedMaterialAssemblyDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	raw := wrappedDocumentAssembly(t)
	d, e := ReadWrappedMaterialAssemblySTEP(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	var out bytes.Buffer
	if e = s.WriteWrappedMaterialAssemblySTEP(&out); e != nil {
		t.Fatal(e)
	}
	var volumes [2]float64
	for i, data := range [][]byte{raw, out.Bytes()} {
		path := t.TempDir() + "/assembly.step"
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
		result := t.TempDir() + "/result.json"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		log, e := exec.CommandContext(ctx, oracle, result, "step-assembly", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatalf("%v %s", e, log)
		}
		data, e = os.ReadFile(result)
		if e != nil {
			t.Fatal(e)
		}
		var r struct {
			Volume      float64
			Products    []string
			Occurrences []struct {
				Name      string
				Transform [3][4]float64
			}
		}
		if e = json.Unmarshal(data, &r); e != nil {
			t.Fatal(e)
		}
		if len(r.Products) != 2 || len(r.Occurrences) != 2 {
			t.Fatal(r)
		}
		seen := false
		for _, o := range r.Occurrences {
			if o.Name == "Right" {
				seen = true
				if o.Transform != ([3][4]float64{{0, -1, 0, 16 * 25.4}, {1, 0, 0, 0}, {0, 0, 1, 0}}) {
					t.Fatal(o)
				}
			}
		}
		if !seen {
			t.Fatal("occurrence name")
		}
		volumes[i] = r.Volume
	}
	if volumes[0] <= 0 || math.Abs(volumes[0]-volumes[1]) > 1e-5 {
		t.Fatal(volumes)
	}
}

func TestWrappedAssemblyStaleBodyID(t *testing.T) {
	d, e := ReadWrappedMaterialAssemblySTEP(bytes.NewReader(wrappedDocumentAssembly(t)), step.Limits{}, step.WrappedSurfaceOptions{}, STEPAssemblyOptions{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	for key, l := range s.Labels {
		if l.Target == AssetLabel {
			a := l.Attributes["step.source.body.id"]
			a.Integer++
			l.Attributes["step.source.body.id"] = a
			s.Labels[key] = l
			break
		}
	}
	var out bytes.Buffer
	if e = s.WriteWrappedMaterialAssemblySTEP(&out); e == nil || out.Len() != 0 {
		t.Fatal("stale body provenance accepted", e)
	}
}
