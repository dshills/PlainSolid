package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func splineAssemblyFixture(t testing.TB) SplineShellAssemblyModel {
	t.Helper()
	raw, e := os.ReadFile("testdata/spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return SplineShellAssemblyModel{LengthUnit: m.LengthUnit, Products: []SplineShellAssemblyPart{{ID: "root", Name: "Spline assembly"}, {ID: "part", Name: "Warped part", Bodies: m.Bodies}}, Occurrences: []AssemblyInstance{{ID: "left", Name: "Left instance", Parent: "root", Child: "part", Transform: geom.Translation(geom.Vec3{X: 10})}, {ID: "right", Name: "Right instance", Parent: "root", Child: "part", Transform: geom.Affine{{0, -1, 0, 0}, {1, 0, 0, 20}, {0, 0, 1, 0}}}}}
}
func TestSplineShellAssemblyRoundtrip(t *testing.T) {
	t.Parallel()
	m := splineAssemblyFixture(t)
	var b bytes.Buffer
	if e := WriteSplineShellAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadSplineShellAssembly(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || len(a.Assembly.Products) != 2 || len(a.Assembly.Occurrences) != 2 {
		t.Fatal(a, e)
	}
	count := 0
	for _, p := range a.Assembly.Products {
		bodies := a.ProductBodies[p.DefinitionID]
		count += len(bodies)
		if p.Identifier == "part" {
			if len(bodies) != 1 || len(bodies[0].Faces) != 6 {
				t.Fatal(bodies)
			}
			for i, face := range bodies[0].Faces {
				if !reflect.DeepEqual(face.Geometry.Data(), m.Products[1].Bodies[0].Faces[i].Geometry.Data()) {
					t.Fatal("local support changed")
				}
			}
		}
	}
	if count != 1 {
		t.Fatal("geometry duplicated per occurrence", count)
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
	if _, e := ReadSplineShells(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("shape-only API flattened occurrences")
	}
	// Independent local bodies can coexist; occurrence reuse remains explicit.
	second := m.Products[1].Bodies[0]
	second.Name = "Independent body"
	second.Faces = append([]BoundedSplineFace(nil), second.Faces...)
	for i, face := range second.Faces {
		p, e := geom.NewSurfacePatch(face.Geometry, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.Transformed(geom.Translation(geom.Vec3{X: 5}))
		if e != nil {
			t.Fatal(e)
		}
		second.Faces[i].Geometry, e = p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
	}
	m.Products[1].Bodies = append(m.Products[1].Bodies, second)
	b.Reset()
	if e := WriteSplineShellAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadSplineShellAssembly(bytes.NewReader(b.Bytes()), Limits{MaxVertices: 47}); e == nil {
		t.Fatal("aggregate control point budget reset across bodies")
	}
	a, e = ReadSplineShellAssembly(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count = 0
	for _, bs := range a.ProductBodies {
		count += len(bs)
	}
	if count != 2 {
		t.Fatal(count)
	}
}
func TestSplineShellAssemblyReject(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"scale", "cycle", "invalid-shell"} {
		t.Run(kind, func(t *testing.T) {
			m := splineAssemblyFixture(t)
			switch kind {
			case "scale":
				m.Occurrences[0].Transform[0][0] = 2
			case "cycle":
				m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: "cycle", Parent: "part", Child: "root", Transform: geom.Identity()})
			case "invalid-shell":
				m.Products[1].Bodies[0].Faces = m.Products[1].Bodies[0].Faces[:5]
			}
			var b bytes.Buffer
			if e := WriteSplineShellAssembly(&b, m); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
	m := splineAssemblyFixture(t)
	var b bytes.Buffer
	if e := WriteSplineShellAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxFaces: 5}, {MaxVertices: 23}, {MaxEdgeUses: 23}, {MaxSolids: 1}} {
		if _, e := ReadSplineShellAssembly(bytes.NewReader(b.Bytes()), l); e == nil {
			t.Fatal(l)
		}
	}
}
func checkOCCTSplineAssembly(t *testing.T, path string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "assembly.json")
	stepOracle(t, out, "step-assembly", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Volume      float64
		Centroid    [3]float64
		Products    []string
		Occurrences []struct {
			Name      string
			Transform [3][4]float64
		}
	}
	if e = json.Unmarshal(raw, &r); e != nil || len(r.Products) != 2 || len(r.Occurrences) != 2 || math.Abs(r.Volume-10) > 1e-7 {
		t.Fatal(r, e)
	}
	want := []float64{5, 10 + 16.0/15, 29.0 / 45}
	for i, v := range want {
		if math.Abs(r.Centroid[i]-v) > 1e-7 {
			t.Fatal(r)
		}
	}
	for _, name := range []string{"Spline assembly", "Warped part"} {
		found := false
		for _, p := range r.Products {
			found = found || p == name
		}
		if !found {
			t.Fatal(r)
		}
	}
	for _, expected := range splineAssemblyFixture(t).Occurrences {
		found := false
		for _, o := range r.Occurrences {
			if o.Name == expected.Name {
				found = true
				for i := 0; i < 3; i++ {
					for j := 0; j < 4; j++ {
						if math.Abs(o.Transform[i][j]-expected.Transform[i][j]) > 1e-10 {
							t.Fatal(o, expected)
						}
					}
				}
			}
		}
		if !found {
			t.Fatal(r)
		}
	}
}
func TestOCCTSplineShellAssembly(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-spline-shell-assembly")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadSplineShellAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, b := range a.ProductBodies {
		count += len(b)
	}
	if count != 1 || len(a.Assembly.Occurrences) != 2 {
		t.Fatal(a)
	}
	var b bytes.Buffer
	if e = WriteSplineShellAssembly(&b, splineAssemblyFixture(t)); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	checkOCCTSplineAssembly(t, source)
	checkOCCTSplineAssembly(t, path)
}

func TestIndependentSplineShellAssembly(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/spline-shell-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadSplineShellAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, bs := range a.ProductBodies {
		count += len(bs)
		for _, b := range bs {
			if len(b.Faces) != 6 {
				t.Fatal(b)
			}
		}
	}
	if count != 1 || len(a.Assembly.Occurrences) != 2 || len(a.Assembly.Products) != 2 {
		t.Fatal(a)
	}
	for _, p := range a.Assembly.Products {
		local, e := a.Assembly.SplineShellBodies(p.DefinitionID, Limits{})
		if e != nil || len(local) != len(a.ProductBodies[p.DefinitionID]) {
			t.Fatal(e)
		}
	}
}
func FuzzSplineShellAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/spline-shell-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadSplineShellAssembly(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 4096, MaxValues: 32768, MaxDepth: 24, MaxVertices: 256, MaxFaces: 32, MaxSolids: 8, MaxEdgeUses: 256})
	})
}
