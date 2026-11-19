package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func cavityModel(t *testing.T) CavityModel {
	outer, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 10, Y: 10, Z: 10})
	if e != nil {
		t.Fatal(e)
	}
	inner, e := brep.Box(geom.Vec3{X: 2, Y: 2, Z: 2}, geom.Vec3{X: 5, Y: 5, Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	return CavityModel{LengthUnit: .001, Bodies: []CavityBody{{Name: "cavity", Outer: outer, Voids: []OrientedVoidShell{{inner, true}}}}}
}
func checkCavity(t *testing.T, m CavityModel) {
	t.Helper()
	if len(m.Bodies) != 1 || len(m.Bodies[0].Voids) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties()
	if e != nil || math.Abs(p.Volume-973) > 1e-8 || math.Abs(p.SurfaceArea-654) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestCavityRoundtrip(t *testing.T) {
	t.Parallel()
	for _, reversed := range []bool{true} {
		m := cavityModel(t)
		if !reversed {
			m.Bodies[0].Voids[0].Shell = m.Bodies[0].Voids[0].Shell.Reversed()
			m.Bodies[0].Voids[0].Reversed = false
		}
		var b bytes.Buffer
		if e := WriteCavities(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadCavities(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkCavity(t, got)
		if got.Bodies[0].Voids[0].Reversed != reversed {
			t.Fatal("orientation lost")
		}
	}
}
func TestCavityRejects(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"orientation", "touch", "outside", "overlap"} {
		m := cavityModel(t)
		switch kind {
		case "orientation":
			m.Bodies[0].Voids[0].Reversed = false
		case "touch":
			s, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
			m.Bodies[0].Voids[0].Shell = s
		case "outside":
			s, _ := brep.Box(geom.Vec3{X: 20}, geom.Vec3{X: 21, Y: 1, Z: 1})
			m.Bodies[0].Voids[0].Shell = s
		case "overlap":
			m.Bodies[0].Voids = append(m.Bodies[0].Voids, m.Bodies[0].Voids[0])
		}
		var b bytes.Buffer
		if e := WriteCavities(&b, m); e == nil || b.Len() != 0 {
			t.Fatal(kind, e)
		}
	}
}
func TestOCCTCavities(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "reference.step")
	stepOracle(t, path, "write-step-cavity")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCavities(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkCavity(t, m)
	var b bytes.Buffer
	if e = WriteCavities(&b, cavityModel(t)); e != nil {
		t.Fatal(e)
	}
	ours := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, ours)
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var p struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-973) > 1e-7 || math.Abs(p.Area-654) > 1e-7 {
		t.Fatal(p, e)
	}
}

func TestIndependentCavityFixture(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/cavity.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadCavities(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkCavity(t, m)
	for _, l := range []Limits{{MaxVertices: 8}, {MaxFaces: 6}, {MaxEdgeUses: 24}} {
		if _, e := ReadCavities(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit", l)
		}
	}
}

func TestMultipleCavityShells(t *testing.T) {
	t.Parallel()
	m := cavityModel(t)
	second, e := brep.Box(geom.Vec3{X: 6, Y: 6, Z: 6}, geom.Vec3{X: 7, Y: 7, Z: 7})
	if e != nil {
		t.Fatal(e)
	}
	m.Bodies[0].Voids = append(m.Bodies[0].Voids, OrientedVoidShell{second, true})
	var b bytes.Buffer
	if e = WriteCavities(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadCavities(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := got.Bodies[0].Solid.MassProperties()
	if e != nil || math.Abs(p.Volume-972) > 1e-8 || got.Bodies[0].Solid.Topology().Shells != 3 {
		t.Fatal(p, e)
	}
}
func FuzzCavitySTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/cavity.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCavities(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
