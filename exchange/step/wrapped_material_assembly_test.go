package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func wrappedAssemblyFixture(t *testing.T) WrappedMaterialAssemblyModel {
	t.Helper()
	bodies := []WrappedMaterialBody{}
	for _, name := range []string{"wrapped-cylinder", "wrapped-toroidal-void", "wrapped-station-loft"} {
		raw, e := os.ReadFile("testdata/" + name + ".step")
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadWrappedMaterials(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bodies = append(bodies, m.Bodies...)
	}
	return WrappedMaterialAssemblyModel{LengthUnit: .001, Products: []WrappedMaterialAssemblyPart{{ID: "root", Name: "Mixed assembly"}, {ID: "part", Name: "Shared materials", Bodies: bodies}}, Occurrences: []AssemblyInstance{{ID: "first", Name: "First", Parent: "root", Child: "part", Transform: geom.Identity()}, {ID: "second", Name: "Second", Parent: "root", Child: "part", Transform: geom.Translation(geom.Vec3{X: 16})}}}
}
func TestWrappedMaterialAssembly(t *testing.T) {
	m := wrappedAssemblyFixture(t)
	var b bytes.Buffer
	if e := WriteWrappedMaterialAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadWrappedMaterialAssembly(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Products) != 2 || len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("shared graph", a.Assembly)
	}
	local := a.ProductBodies[a.Assembly.Occurrences[0].Child]
	if len(local) != 3 || local[0].Cylinder == nil || local[1].ToroidalVoid == nil || local[2].StationLoft == nil {
		t.Fatal("material identity")
	}
	if local[2].StationLoft.Center != (geom.Vec2{}) {
		t.Fatal("local geometry moved")
	}
	for _, o := range a.Assembly.Occurrences {
		found := false
		for _, want := range m.Occurrences {
			if o.Identifier == want.ID {
				found = true
				if o.Name != want.Name || o.Transform != want.Transform {
					t.Fatal(o, want)
				}
			}
		}
		if !found {
			t.Fatal(o)
		}
	}
	if _, e = ReadWrappedMaterials(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("shape API flattened")
	}
	if _, e = ReadWrappedMaterialAssembly(bytes.NewReader(b.Bytes()), Limits{MaxSolids: 2}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("cumulative body budget")
	}
	// An extra unreachable topology entity must not disappear during isolation.
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var clone Entity
	for _, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "EDGE_CURVE" {
			clone = en
			break
		}
	}
	clone.ID = 999999
	f.Entities[clone.ID] = clone
	f.Order = append(f.Order, clone.ID)
	if _, e = ReadWrappedMaterialAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("unowned edge accepted")
	}
}
func TestOCCTWrappedMaterialAssembly(t *testing.T) {
	var b bytes.Buffer
	if e := WriteWrappedMaterialAssembly(&b, wrappedAssemblyFixture(t)); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/mixed.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/assembly.json"
	stepOracle(t, out, "step-assembly", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Volume      float64
		FreeShapes  int
		Products    []string
		Occurrences []struct {
			Name      string
			Transform [3][4]float64
		}
	}
	if e = json.Unmarshal(raw, &r); e != nil {
		t.Fatal(e)
	}
	if len(r.Occurrences) != 2 || len(r.Products) != 2 {
		t.Fatal(string(raw))
	}
	names := map[string]bool{}
	for _, name := range r.Products {
		names[name] = true
	}
	if !names["Mixed assembly"] || !names["Shared materials"] {
		t.Fatal(r.Products)
	}
	seen := map[string]bool{}
	for _, o := range r.Occurrences {
		seen[o.Name] = true
		want := geom.Identity()
		if o.Name == "Second" {
			want = geom.Translation(geom.Vec3{X: 16})
		}
		if o.Transform != [3][4]float64(want) {
			t.Fatal(o, want)
		}
	}
	if !seen["First"] || !seen["Second"] {
		t.Fatal(r.Occurrences)
	}

}

func TestOCCTWrappedMaterialAssemblyMass(t *testing.T) {
	var b bytes.Buffer
	if e := WriteWrappedMaterialAssembly(&b, wrappedAssemblyFixture(t)); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/mixed.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	sum := 0.
	for i, p := range []string{"testdata/wrapped-cylinder.step", "testdata/wrapped-toroidal-void.step", "testdata/wrapped-station-loft.step", path} {
		out := t.TempDir() + "/mass.json"
		mode := "step-mass-precise"
		if i == 3 {
			mode = "step-assembly"
		}
		stepOracle(t, out, mode, p)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var m struct{ Volume float64 }
		if e = json.Unmarshal(raw, &m); e != nil {
			t.Fatal(e)
		}
		if i < 3 {
			sum += 2 * m.Volume
		} else if math.Abs(m.Volume-sum) > 1e-5 {
			t.Fatal(m.Volume, sum)
		}
	}
}
