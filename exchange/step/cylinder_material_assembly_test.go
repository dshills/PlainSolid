package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
	"time"
)

func cylinderMaterialFixture(t testing.TB) CylinderMaterialAssemblyModel {
	p := planarSplineFixture(t)
	m := CylinderMaterialAssemblyModel{Occurrences: p.Occurrences, LengthUnit: p.LengthUnit}
	for _, x := range p.Products {
		m.Products = append(m.Products, CylinderMaterialPart{ID: x.ID, Name: x.Name, Family: CylinderMaterialFamily(x.Family), Planar: x.Planar, Splines: x.Splines})
	}
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	m.Products = append(m.Products, CylinderMaterialPart{ID: "cylinder", Name: "Cylinder material", Family: CylinderMaterialCylinder, Cylinders: []CylinderBody{{Name: "Round stock", Frame: frame, Radius: 1, Height: 2}}})
	m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: "vertical", Name: "Cylinder vertical", Parent: "root", Child: "cylinder", Transform: geom.Translation(geom.Vec3{Y: -10})}, AssemblyInstance{ID: "horizontal", Name: "Cylinder horizontal", Parent: "root", Child: "cylinder", Transform: geom.Affine{{0, 0, 1, 20}, {1, 0, 0, 0}, {0, 1, 0, 0}}})
	return m
}
func TestCylinderMaterialAssembly(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteCylinderMaterialAssembly(&b, cylinderMaterialFixture(t)); e != nil {
		t.Fatal(e)
	}
	for _, raw := range [][]byte{b.Bytes(), cylinderMaterialSource(t)} {
		a, e := ReadCylinderMaterialAssembly(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(a.Products) != 4 || len(a.Assembly.Occurrences) != 5 {
			t.Fatal(a)
		}
		np, ns, nc := 0, 0, 0
		for _, p := range a.Products {
			np += len(p.Planar)
			ns += len(p.Splines)
			nc += len(p.Cylinders)
			if len(p.Cylinders) > 0 {
				if len(p.Analytic) != 1 || len(p.SourceBodyIDs) != 1 || p.Cylinders[0].Radius != 1 || p.Cylinders[0].Height != 2 {
					t.Fatal(p)
				}
			}
		}
		if np != 1 || ns != 1 || nc != 1 {
			t.Fatal(np, ns, nc)
		}
		if _, e = ReadPlanarSplineAssembly(bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("old family widened")
		}
		_ = cylinderMaterialReexport(t, a)
	}
}
func cylinderMaterialSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/cylinder-material-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func cylinderMaterialReexport(t testing.TB, a CylinderMaterialAssembly) []byte {
	t.Helper()
	m := CylinderMaterialAssemblyModel{LengthUnit: a.Assembly.LengthUnit}
	ids := map[int64]string{}
	for _, p := range a.Assembly.Products {
		ids[p.DefinitionID] = p.Identifier
		v := a.Products[p.DefinitionID]
		m.Products = append(m.Products, CylinderMaterialPart{ID: p.Identifier, Name: p.Name, Family: v.Family, Planar: v.Planar, Splines: v.Splines, Cylinders: v.Cylinders})
	}
	for _, o := range a.Assembly.Occurrences {
		m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: o.Identifier, Name: o.Name, Parent: ids[o.Parent], Child: ids[o.Child], Transform: o.Transform})
	}
	var b bytes.Buffer
	if e := WriteCylinderMaterialAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestCylinderMaterialAssemblyRejects(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteCylinderMaterialAssembly(&b, cylinderMaterialFixture(t)); e != nil {
		t.Fatal(e)
	}
	raw := b.Bytes()
	for _, l := range []Limits{{MaxSolids: 3}, {MaxFaces: 14}, {MaxVertices: 8}, {MaxEdgeUses: 59}, {MaxBytes: 100}} {
		a, e := ReadCylinderMaterialAssembly(bytes.NewReader(raw), l)
		if e == nil || a.Assembly.Source != nil {
			t.Fatal(l, e)
		}
	}
	for _, kind := range []string{"CONICAL_SURFACE", "SPHERICAL_SURFACE"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for id, en := range f.Entities {
			if len(en.Records) == 1 && en.Records[0].Type == "CYLINDRICAL_SURFACE" {
				en.Records[0].Type = kind
				f.Entities[id] = en
				found = true
				break
			}
		}
		if !found {
			t.Fatal("no cylinder")
		}
		if _, e = ReadCylinderMaterialAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal(kind)
		}
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "CYLINDRICAL_SURFACE" {
			en.Records[0].Args[2].Text = "2."
			f.Entities[id] = en
			break
		}
	}
	if _, e = ReadCylinderMaterialAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
		t.Fatal("radius disagrees with original rims")
	}
	for _, change := range []func(*CylinderMaterialAssemblyModel){func(m *CylinderMaterialAssemblyModel) { m.Products[3].Cylinders[0].Radius = -1 }, func(m *CylinderMaterialAssemblyModel) { m.Products[3].Family = CylinderMaterialPlanar }, func(m *CylinderMaterialAssemblyModel) { m.Products[3].Splines = m.Products[1].Splines }, func(m *CylinderMaterialAssemblyModel) { m.Occurrences[4].Transform[0][2] = -1 }} {
		m := cylinderMaterialFixture(t)
		change(&m)
		var out bytes.Buffer
		if e := WriteCylinderMaterialAssembly(&out, m); e == nil || out.Len() != 0 {
			t.Fatal("invalid writer", e)
		}
	}
}
func TestOCCTCylinderMaterialAssembly(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_CYLINDER_ASSEMBLY")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	source := cylinderMaterialSource(t)
	a, e := ReadCylinderMaterialAssembly(bytes.NewReader(source), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteCylinderMaterialAssembly(&b, cylinderMaterialFixture(t)); e != nil {
		t.Fatal(e)
	}
	for _, data := range [][]byte{source, b.Bytes(), cylinderMaterialReexport(t, a)} {
		path := filepath.Join(t.TempDir(), "mixed.step")
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(out))
		}
		at := bytes.LastIndex(out, []byte(`{"Faces"`))
		if at < 0 {
			t.Fatal(string(out))
		}
		var p struct {
			Faces, Solids, Definitions, Occurrences int
			Volume, Area                            float64
			Center                                  [3]float64
		}
		if e = json.Unmarshal(out[at:], &p); e != nil {
			t.Fatal(e)
		}
		v := 16 + 4*math.Pi
		want := [3]float64{(-7 + 42*math.Pi) / v, (350.0/3 - 20*math.Pi) / v, (139.0/9 + 2*math.Pi) / v}
		if p.Faces != 24 || p.Solids != 5 || p.Definitions != 3 || p.Occurrences != 5 || math.Abs(p.Volume-v) > 1e-8 || math.Abs(p.Area-(58.63229613162359+12*math.Pi)) > 1e-8 {
			t.Fatal(p)
		}
		for i, x := range want {
			if math.Abs(p.Center[i]-x) > 1e-8 {
				t.Fatal(p, want)
			}
		}
	}
}
func FuzzCylinderMaterialAssembly(f *testing.F) {
	var b bytes.Buffer
	if e := WriteCylinderMaterialAssembly(&b, cylinderMaterialFixture(f)); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadCylinderMaterialAssembly(bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 2048, MaxSolids: 8, MaxFaces: 64, MaxVertices: 1024, MaxEdgeUses: 512})
	})
}
