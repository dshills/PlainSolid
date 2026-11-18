package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func planarSplineFixture(t testing.TB) PlanarSplineAssemblyModel {
	t.Helper()
	s := splineAssemblyFixture(t)
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	return PlanarSplineAssemblyModel{LengthUnit: .001, Products: []PlanarSplinePart{{ID: "root", Name: "Mixed assembly", Family: PlanarSplineNode}, {ID: "curve", Name: "Warped material", Family: PlanarSplineSpline, Splines: s.Products[1].Bodies}, {ID: "box", Name: "Planar material", Family: PlanarSplinePlanar, Planar: []Body{{Name: "Box", Solid: b}}}}, Occurrences: []AssemblyInstance{{ID: "left", Name: "Curved left", Parent: "root", Child: "curve", Transform: geom.Translation(geom.Vec3{X: 10})}, {ID: "right", Name: "Curved right", Parent: "root", Child: "curve", Transform: geom.Affine{{0, -1, 0, 0}, {1, 0, 0, 20}, {0, 0, 1, 0}}}, {ID: "stock", Name: "Planar instance", Parent: "root", Child: "box", Transform: geom.Translation(geom.Vec3{X: -10})}}}
}
func TestPlanarSplineAssembly(t *testing.T) {
	t.Parallel()
	m := planarSplineFixture(t)
	var b bytes.Buffer
	if e := WritePlanarSplineAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadPlanarSplineAssembly(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 3 || len(a.Assembly.Occurrences) != 3 {
		t.Fatal(a)
	}
	np, ns := 0, 0
	for _, p := range a.Products {
		np += len(p.Planar)
		ns += len(p.Splines)
		if len(p.Splines) > 0 {
			for i, f := range p.Splines[0].Faces {
				if !reflect.DeepEqual(f.Geometry.Data(), m.Products[1].Splines[0].Faces[i].Geometry.Data()) {
					t.Fatal("original chart changed")
				}
			}
		}
	}
	if np != 1 || ns != 1 {
		t.Fatal(np, ns)
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
	if _, e = ReadSplineShellAssembly(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("homogeneous spline reader widened")
	}
	raw, e := os.ReadFile("testdata/planar-spline-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	external, e := ReadPlanarSplineAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal("independent OCCT source", e)
	}
	if len(external.Products) != 3 || len(external.Assembly.Occurrences) != 3 {
		t.Fatal(external)
	}
}
func TestPlanarSplineAssemblyRejects(t *testing.T) {
	t.Parallel()
	m := planarSplineFixture(t)
	var b bytes.Buffer
	if e := WritePlanarSplineAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	for _, l := range []Limits{{MaxSolids: 1}, {MaxFaces: 11}, {MaxVertices: 8}, {MaxEdgeUses: 24}, {MaxBytes: 100}} {
		v, e := ReadPlanarSplineAssembly(bytes.NewReader(raw), l)
		if e == nil || v.Assembly.Source != nil {
			t.Fatal(l, e)
		}
	}
	for _, change := range []func(*PlanarSplineAssemblyModel){func(m *PlanarSplineAssemblyModel) { m.Products[1].Family = PlanarSplinePlanar }, func(m *PlanarSplineAssemblyModel) { m.Products[2].Planar = nil }, func(m *PlanarSplineAssemblyModel) { m.Occurrences[0].Transform[0][0] = 1.000000000000001 }, func(m *PlanarSplineAssemblyModel) { m.Occurrences[0].Transform[0][0] = -1 }, func(m *PlanarSplineAssemblyModel) { m.Occurrences[0].Child = "missing" }} {
		n := planarSplineFixture(t)
		change(&n)
		var out bytes.Buffer
		if e := WritePlanarSplineAssembly(&out, n); e == nil || out.Len() != 0 {
			t.Fatal("invalid writer", e)
		}
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "ADVANCED_FACE" {
			copyEntity := en
			copyEntity.ID = id + 100000
			f.Entities[copyEntity.ID] = copyEntity
			break
		}
	}
	v, e := ReadPlanarSplineAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{})
	if e == nil || len(v.Products) != 0 {
		t.Fatal("unowned face", e)
	}
}
func TestPlanarSplineAssemblyRejectsSharedDefinitionGeometry(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WritePlanarSplineAssembly(&b, planarSplineFixture(t)); e != nil {
		t.Fatal(e)
	}
	a, e := ReadAssembly(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var ids []int64
	var reps []int64
	for _, p := range a.Products {
		items, _, e := a.productItems(p.DefinitionID)
		if e != nil {
			t.Fatal(e)
		}
		if len(items) > 0 {
			ids = append(ids, items[0])
			reps = append(reps, p.Representations[0])
		}
	}
	if len(ids) != 2 {
		t.Fatal(ids)
	}
	ent := a.Source.Entities[reps[1]]
	changed := false
	for i, v := range ent.Records[0].Args[1].Items {
		if v.Kind == Reference && v.Ref == ids[1] {
			ent.Records[0].Args[1].Items[i].Ref = ids[0]
			changed = true
		}
	}
	if !changed {
		t.Fatal("no represented body")
	}
	a.Source.Entities[reps[1]] = ent
	out, e := ReadPlanarSplineAssembly(bytes.NewReader(serializeWrappedProofFile(a.Source)), Limits{})
	if e == nil || out.Assembly.Source != nil {
		t.Fatal("cross-product body sharing", e)
	}
}

func planarSplineReexport(t testing.TB, a PlanarSplineAssembly) []byte {
	t.Helper()
	m := PlanarSplineAssemblyModel{LengthUnit: a.Assembly.LengthUnit}
	ids := map[int64]string{}
	for _, p := range a.Assembly.Products {
		ids[p.DefinitionID] = p.Identifier
		b := a.Products[p.DefinitionID]
		m.Products = append(m.Products, PlanarSplinePart{ID: p.Identifier, Name: p.Name, Family: b.Family, Planar: b.Planar, Splines: b.Splines})
	}
	for _, o := range a.Assembly.Occurrences {
		m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: o.Identifier, Name: o.Name, Parent: ids[o.Parent], Child: ids[o.Child], Transform: o.Transform})
	}
	var out bytes.Buffer
	if e := WritePlanarSplineAssembly(&out, m); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func mixedAssemblyOracle(t *testing.T, bin, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(b))
	}
	at := bytes.LastIndex(b, []byte(`{"Faces"`))
	if at < 0 {
		t.Fatal(string(b))
	}
	var p struct {
		Faces, Solids, Definitions, Occurrences int
		Volume, Area                            float64
		Center                                  [3]float64
	}
	if e = json.Unmarshal(b[at:], &p); e != nil {
		t.Fatal(e)
	}
	if p.Faces != 18 || p.Solids != 3 || p.Definitions != 2 || p.Occurrences != 3 || math.Abs(p.Volume-16) > 1e-8 || math.Abs(p.Area-58.63229613162359) > 1e-8 || math.Abs(p.Center[0]+7.0/16) > 1e-8 || math.Abs(p.Center[1]-175.0/24) > 1e-8 || math.Abs(p.Center[2]-139.0/144) > 1e-8 {
		t.Fatal(p)
	}
}
func TestOCCTPlanarSplineAssembly(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_MIXED_ASSEMBLY")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	mixedAssemblyOracle(t, bin, "testdata/planar-spline-assembly.step")
	var b bytes.Buffer
	if e := WritePlanarSplineAssembly(&b, planarSplineFixture(t)); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	mixedAssemblyOracle(t, bin, path)
	raw, e := os.ReadFile("testdata/planar-spline-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadPlanarSplineAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	round := planarSplineReexport(t, a)
	if e = os.WriteFile(path, round, 0600); e != nil {
		t.Fatal(e)
	}
	mixedAssemblyOracle(t, bin, path)

}
func FuzzPlanarSplineAssembly(f *testing.F) {
	var b bytes.Buffer
	if e := WritePlanarSplineAssembly(&b, planarSplineFixture(f)); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			t.Skip()
		}
		_, _ = ReadPlanarSplineAssembly(bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 2048, MaxSolids: 8, MaxFaces: 64, MaxVertices: 1024, MaxEdgeUses: 512})
	})
}
