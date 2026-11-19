package step

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func nestedAssemblyModel(t *testing.T) AssemblyModel {
	t.Helper()
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	rotation, e := geom.Rotation(geom.Vec3{Z: 1}, math.Pi/2)
	if e != nil {
		t.Fatal(e)
	}
	return AssemblyModel{LengthUnit: .001, Products: []AssemblyPart{{ID: "root", Name: "Machine"}, {ID: "module", Name: "Module"}, {ID: "part", Name: "Bracket", Bodies: []Body{{Name: "box", Solid: box}}}}, Occurrences: []AssemblyInstance{{ID: "m", Name: "Module occurrence", Parent: "root", Child: "module", Transform: geom.Translation(geom.Vec3{Z: 5})}, {ID: "a", Name: "Left bracket", Parent: "module", Child: "part", Transform: geom.Translation(geom.Vec3{X: 10})}, {ID: "b", Name: "Right bracket", Parent: "module", Child: "part", Transform: geom.Translation(geom.Vec3{Y: 20}).Mul(rotation)}}}
}
func TestWriteAssemblyRoundtrip(t *testing.T) {
	t.Parallel()
	m := nestedAssemblyModel(t)
	var b, again bytes.Buffer
	if e := WriteAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	if e := WriteAssembly(&again, m); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("nondeterministic", e)
	}
	a, e := ReadAssembly(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 3 || len(a.Occurrences) != 3 || len(a.Roots) != 1 {
		t.Fatal(a)
	}
	for _, o := range a.Occurrences {
		found := false
		for _, want := range m.Occurrences {
			if o.Identifier == want.ID {
				found = true
				for _, p := range []geom.Vec3{{}, {X: 1, Y: 2, Z: 3}} {
					if o.Transform.Apply(p).Distance(want.Transform.Apply(p)) > 1e-10 {
						t.Fatal(o, want)
					}
				}
			}
		}
		if !found {
			t.Fatal(o)
		}
	}
	for _, p := range a.Products {
		if p.Identifier == "part" {
			b, e := a.PlanarBodies(p.DefinitionID, Limits{})
			if e != nil || len(b) != 1 {
				t.Fatal(b, e)
			}
			props, e := b[0].Solid.MassProperties()
			if e != nil || math.Abs(props.Volume-6) > 1e-10 {
				t.Fatal(props, e)
			}
		}
	}
}
func TestWriteAssemblyFailures(t *testing.T) {
	t.Parallel()
	for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
		if e := WriteAssembly(w, nestedAssemblyModel(t)); e == nil {
			t.Fatal("writer")
		}
	}
	for _, kind := range []string{"cycle", "missing", "scale", "reflect", "id", "empty"} {
		m := nestedAssemblyModel(t)
		switch kind {
		case "cycle":
			m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: "cycle", Parent: "part", Child: "root", Transform: geom.Identity()})
		case "missing":
			m.Occurrences[0].Child = "absent"
		case "scale":
			m.Occurrences[0].Transform = geom.Scaling(geom.Vec3{X: 2, Y: 1, Z: 1})
		case "reflect":
			m.Occurrences[0].Transform = geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1})
		case "id":
			m.Products[1].ID = "root"
		case "empty":
			m.Products[2].Bodies = nil
		}
		if e := WriteAssembly(io.Discard, m); e == nil {
			t.Fatal(kind)
		}
	}
}
func TestOCCTAssemblyExport(t *testing.T) {
	t.Parallel()
	m := nestedAssemblyModel(t)
	path := filepath.Join(t.TempDir(), "assembly.step")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteAssembly(f, m)
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	out := filepath.Join(t.TempDir(), "assembly.json")
	stepOracle(t, out, "step-assembly", path)
	data, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Volume      float64
		Centroid    [3]float64
		Roots       int
		Products    []string
		Occurrences []struct {
			Name      string
			Transform geom.Affine
		}
	}
	if e = json.Unmarshal(data, &got); e != nil {
		t.Fatal(e)
	}
	if got.Roots != 1 || len(got.Products) != 3 || len(got.Occurrences) != 3 || math.Abs(got.Volume-12) > 1e-8 || math.Abs(got.Centroid[0]-4.75) > 1e-8 || math.Abs(got.Centroid[1]-10.75) > 1e-8 || math.Abs(got.Centroid[2]-6.5) > 1e-8 {
		t.Fatal(got)
	}
	for _, p := range m.Products {
		found := false
		for _, name := range got.Products {
			if name == p.Name {
				found = true
			}
		}
		if !found {
			t.Fatal("missing product name", p.Name, got)
		}
	}
	for _, want := range m.Occurrences {
		found := false
		for _, o := range got.Occurrences {
			if o.Name == want.Name {
				found = true
				for _, p := range []geom.Vec3{{}, {X: 1, Y: 2, Z: 3}} {
					if o.Transform.Apply(p).Distance(want.Transform.Apply(p)) > 1e-9 {
						t.Fatal(o, want)
					}
				}
			}
		}
		if !found {
			t.Fatal("missing occurrence", want.Name, got)
		}
	}
}

func TestAssemblyMultipleLocalBodies(t *testing.T) {
	t.Parallel()
	m := nestedAssemblyModel(t)
	other, e := brep.Box(geom.Vec3{X: 5}, geom.Vec3{X: 6, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	m.Products[2].Bodies = append(m.Products[2].Bodies, Body{Name: "second", Solid: other})
	var b bytes.Buffer
	if e = WriteAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadAssembly(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range a.Products {
		if p.Identifier != "part" {
			continue
		}
		bodies, e := a.PlanarBodies(p.DefinitionID, Limits{})
		if e != nil || len(bodies) != 2 {
			t.Fatal(bodies, e)
		}
		p0, _ := bodies[0].Solid.MassProperties()
		p1, _ := bodies[1].Solid.MassProperties()
		if math.Abs(p0.Volume-6) > 1e-10 || math.Abs(p1.Volume-6) > 1e-10 || math.Abs(p1.Centroid.X-5.5) > 1e-10 {
			t.Fatal(p0, p1)
		}
	}
}
