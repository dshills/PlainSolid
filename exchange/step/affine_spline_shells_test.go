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

func checkAffineShear(t testing.TB, m AffineSplineShellModel) {
	t.Helper()
	mass, e := m.Bodies[0].Solid.MassProperties()
	wantArea := 24 + 24*math.Sqrt(2) + 24*math.Sqrt(3)
	if e != nil || math.Abs(mass.Volume-36) > 1e-10 || math.Abs(mass.SurfaceArea-wantArea) > 1e-10 || mass.Centroid.Sub(geom.Vec3{X: 31. / 3, Y: 85. / 6, Z: 14.5}).Length() > 1e-12 {
		t.Fatal(mass, e)
	}
}
func TestAffineSplineShellRoundtrip(t *testing.T) {
	t.Parallel()
	m := placedAffineShell(t, geom.Affine{{1, 1, 0, 7}, {0, 1, 1, 11}, {0, 0, 1, 13}})
	var b bytes.Buffer
	if e := WriteAffineSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	round, e := ReadAffineSplineShells(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkAffineShear(t, round)
	for i, f := range round.Bodies[0].Faces {
		if !reflect.DeepEqual(f.Geometry.Data(), m.Bodies[0].Faces[i].Geometry.Data()) || !reflect.DeepEqual(f.Boundary, m.Bodies[0].Faces[i].Boundary) {
			t.Fatal("source affine data changed")
		}
	}
	if _, e = ReadOrthogonalAffineSplineShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("orthogonal API broadened")
	}
	if _, e = ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("axis API broadened")
	}
}
func TestAffineSplineShellRejectCurvature(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadAffineSplineShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("curved support flattened")
	}
	m := placedAffineShell(t, geom.Affine{{1, 1, 0, 7}, {0, 1, 1, 11}, {0, 0, 1, 13}})
	d := m.Bodies[0].Faces[0].Geometry.Data()
	d.Poles[3].Z = math.Nextafter(d.Poles[3].Z, math.Inf(1))
	g, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	m.Bodies[0].Faces[0].Geometry = g
	if affineSplineSupports(m.Bodies[0].Faces, false) {
		t.Fatal("one-ULP nonaffine support accepted")
	}
	var out bytes.Buffer
	if e = WriteAffineSplineShells(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("invalid shape emitted", e)
	}
}
func TestOCCTAffineSplineShell(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-affine-spline-shell")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAffineSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkAffineShear(t, m)
	var b bytes.Buffer
	if e = WriteAffineSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{source, path} {
		out := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, out, "step-mass-precise", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Area, Volume float64 }
		if e = json.Unmarshal(data, &mass); e != nil || math.Abs(mass.Volume-36) > 1e-8 || math.Abs(mass.Area-(24+24*math.Sqrt(2)+24*math.Sqrt(3))) > 1e-8 {
			t.Fatal(mass, e)
		}
	}
}

func TestIndependentAffineSplineShell(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/affine-spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAffineSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkAffineShear(t, m)
	for _, l := range []Limits{{MaxFaces: 7}, {MaxVertices: 31}, {MaxEdgeUses: 35}} {
		if _, e := ReadAffineSplineShells(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func FuzzAffineSplineShell(f *testing.F) {
	raw, e := os.ReadFile("testdata/affine-spline-shell.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadAffineSplineShells(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxVertices: 128, MaxEdgeUses: 128})
	})
}
