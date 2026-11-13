package step

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func lensAssemblyFixture(t testing.TB) SplineLensAssemblyModel {
	body := splineLensFixture(t)
	return SplineLensAssemblyModel{LengthUnit: body.LengthUnit, Products: []SplineLensAssemblyPart{{ID: "root", Name: "Lens group"}, {ID: "lens", Name: "Shared local lens", Bodies: []SplineLensBody{body}}}, Occurrences: []AssemblyInstance{{ID: "a", Name: "Translated", Parent: "root", Child: "lens", Transform: geom.Translation(geom.Vec3{X: 4})}, {ID: "b", Name: "Rotated", Parent: "root", Child: "lens", Transform: geom.Affine{{0, 0, 1, 8}, {0, 1, 0, 0}, {-1, 0, 0, 4}}}}}
}
func TestSplineLensAssembly(t *testing.T) {
	m := lensAssemblyFixture(t)
	var b bytes.Buffer
	if e := WriteSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadSplineLensAssembly(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 || len(a.Assembly.Products) != 2 {
		t.Fatal(a.Assembly)
	}
	if a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("lost shared product")
	}
	count := 0
	for _, bodies := range a.ProductBodies {
		count += len(bodies)
	}
	if count != 1 {
		t.Fatal(count)
	}
	for _, occ := range a.Assembly.Occurrences {
		found := false
		for _, want := range m.Occurrences {
			found = found || occ.Transform == want.Transform
		}
		if !found {
			t.Fatal(occ)
		}
	}
	if _, e = ReadSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("standalone flattened assembly")
	}
}
func TestSplineLensAssemblyOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-spline-lens-assembly")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadSplineLensAssembly(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal(a.Assembly)
	}
	m := SplineLensAssemblyModel{LengthUnit: a.Assembly.LengthUnit}
	keys := map[int64]string{}
	for _, p := range a.Assembly.Products {
		key := fmt.Sprint(p.DefinitionID)
		keys[p.DefinitionID] = key
		m.Products = append(m.Products, SplineLensAssemblyPart{ID: key, Name: p.Name, Bodies: a.ProductBodies[p.DefinitionID]})
	}
	for _, occ := range a.Assembly.Occurrences {
		m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: fmt.Sprint(occ.ID), Name: occ.Name, Parent: keys[occ.Parent], Child: keys[occ.Child], Transform: occ.Transform})
	}
	var b bytes.Buffer
	if e = WriteSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/out.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	result := t.TempDir() + "/result.json"
	stepOracle(t, result, "step-assembly", path)
	data, e := os.ReadFile(result)
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
		want := geom.Translation(geom.Vec3{X: 4})
		if occ.Name == "Rotated" {
			want = geom.Affine{{0, 0, 1, 8}, {0, 1, 0, 0}, {-1, 0, 0, 4}}
		}
		if geom.Affine(occ.Transform) != want {
			t.Fatal(occ)
		}
	}
}
func TestSplineLensAssemblyRejections(t *testing.T) {
	for _, kind := range []string{"near-rigid", "reflection", "two-bodies"} {
		m := lensAssemblyFixture(t)
		switch kind {
		case "near-rigid":
			m.Occurrences[0].Transform[0][1] = 1e-12
		case "reflection":
			m.Occurrences[0].Transform[0][0] = -1
		case "two-bodies":
			m.Products[1].Bodies = append(m.Products[1].Bodies, m.Products[1].Bodies[0])
		}
		var b bytes.Buffer
		if e := WriteSplineLensAssembly(&b, m); e == nil || b.Len() != 0 {
			t.Fatal(kind, e)
		}
	}
	raw, e := os.ReadFile("testdata/spline-lens-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"extra-shell", "from-frame", "cycle"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range f.Order {
				switch kind {
				case "extra-shell":
					r, e := simple(f, id, "CLOSED_SHELL", 2)
					if e == nil {
						next := f.Order[len(f.Order)-1] + 1
						ovalSet(f, next, r)
						f.Order = append(f.Order, next)
						changed = true
					}
				case "from-frame":
					r, e := simple(f, id, "ITEM_DEFINED_TRANSFORMATION", 4)
					if e == nil {
						aid, _ := ref(r.Args[2])
						a, _ := simple(f, aid, "AXIS2_PLACEMENT_3D", 4)
						pid, _ := ref(a.Args[1])
						p, _ := simple(f, pid, "CARTESIAN_POINT", 2)
						p.Args[1].Items[0] = Value{Kind: Number, Text: "0.5"}
						ovalSet(f, pid, p)
						changed = true
					}
				case "cycle":
					r, e := simple(f, id, "PCURVE", 3)
					if e == nil {
						r.Args[2] = Value{Kind: Reference, Ref: id}
						ovalSet(f, id, r)
						changed = true
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Fatal("mutation missed")
			}
			if _, e = ReadSplineLensAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
				t.Fatal("invalid assembly accepted")
			}
		})
	}
	if _, e = ReadSplineLensAssembly(bytes.NewReader(raw), Limits{MaxFaces: 1}, CurvedOvalOptions{}); e == nil {
		t.Fatal("ignored face budget")
	}
}
func FuzzSplineLensAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/spline-lens-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadSplineLensAssembly(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 512, MaxValues: 12000, MaxFaces: 2, MaxSolids: 1, MaxVertices: 256, MaxEdgeUses: 8, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}

func TestSplineLensNestedAssembly(t *testing.T) {
	m := lensAssemblyFixture(t)
	m.Products = append(m.Products, SplineLensAssemblyPart{ID: "group", Name: "Repeated nested group"})
	for i := range m.Occurrences {
		m.Occurrences[i].Parent = "group"
	}
	m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: "g1", Parent: "root", Child: "group", Transform: geom.Identity()}, AssemblyInstance{ID: "g2", Parent: "root", Child: "group", Transform: geom.Translation(geom.Vec3{Y: 8})})
	var b bytes.Buffer
	if e := WriteSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadSplineLensAssembly(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Products) != 3 || len(a.Assembly.Occurrences) != 4 {
		t.Fatal(a.Assembly)
	}
	count := 0
	for _, bodies := range a.ProductBodies {
		count += len(bodies)
	}
	if count != 1 {
		t.Fatal("nested sharing lost")
	}
}
