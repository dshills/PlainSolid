package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func verifyAxisAlignedShell(t testing.TB, raw []byte) {
	t.Helper()
	m, e := ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || len(m.Bodies[0].Faces) != 8 {
		t.Fatal(m)
	}
	mass, e := m.Bodies[0].Solid.MassProperties()
	if e != nil || math.Abs(mass.Volume-36) > 1e-12 || math.Abs(mass.SurfaceArea-72) > 1e-12 || mass.Centroid.Sub(geom.Vec3{X: 5. / 3, Y: 5. / 3, Z: 1.5}).Length() > 1e-12 {
		t.Fatal(mass, e)
	}
	if _, e = ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("open API widened")
	}
	for _, l := range []Limits{{MaxFaces: 7}, {MaxEdgeUses: 35}, {MaxVertices: 31}} {
		if _, e = ReadAxisAlignedSplineShells(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestIndependentAxisAlignedSplineShell(t *testing.T) {
	raw, e := os.ReadFile("testdata/axis-aligned-spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	verifyAxisAlignedShell(t, raw)
}
func TestAxisAlignedSplineShellReject(t *testing.T) {
	for _, name := range []string{"spline-shell.step", "orthogonal-open-shell.step", "orthogonal-holed-open-shell.step"} {
		raw, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal(name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ReadAxisAlignedSplineShellsContext(ctx, bytes.NewReader(nil), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestAxisAlignedAffineSupport(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	for _, kind := range []string{"plane", "warped", "skew", "weights"} {
		p := []geom.Vec3{{}, {Y: 2}, {X: 2}, {X: 2, Y: 2}}
		var w []float64
		switch kind {
		case "warped":
			p[3].Z = math.SmallestNonzeroFloat64
		case "skew":
			p[2].Y = 1
			p[3].Y = 3
		case "weights":
			w = []float64{1, 1, 2, 2}
		}
		g, e := geom.NewBSplineSurface(1, 1, 2, 2, p, k, k, w)
		if e != nil {
			t.Fatal(e)
		}
		if axisAlignedAffineSupport(g) != (kind == "plane") {
			t.Fatal(kind)
		}
	}
}
func TestOCCTAxisAlignedSplineShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, path, "write-step-axis-aligned-spline-shell")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	verifyAxisAlignedShell(t, raw)
	m, e := ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var exported bytes.Buffer
	if e = WriteAdvanced(&exported, Model{LengthUnit: m.LengthUnit, Bodies: []Body{{Name: m.Bodies[0].Name, Solid: m.Bodies[0].Solid}}}); e != nil {
		t.Fatal(e)
	}
	native := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(native, exported.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, "step-mass-precise", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var record struct{ Volume, Area float64 }
	if e = json.Unmarshal(b, &record); e != nil || math.Abs(record.Volume-36) > 1e-9 || math.Abs(record.Area-72) > 1e-9 {
		t.Fatal(record, e)
	}
	stepOracle(t, out, "step-mass-precise", native)
	b, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &record); e != nil || math.Abs(record.Volume-36) > 1e-9 || math.Abs(record.Area-72) > 1e-9 {
		t.Fatal(record, e)
	}

}

func TestAxisAlignedShellRejectsInwardBoundary(t *testing.T) {
	raw, e := os.ReadFile("testdata/axis-aligned-spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b := m.Bodies[0]
	s := OrthogonalSplineOpenShell{Faces: b.Faces, FaceHoles: b.FaceHoles, FaceIDs: b.FaceIDs}
	for face := range b.Faces {
		for loop := 0; loop <= len(b.FaceHoles[face]); loop++ {
			part := []OrthogonalShellUse{}
			for _, u := range b.Uses {
				if u.Face == face && u.Loop == loop {
					u.Forward = !u.Forward
					part = append(part, u)
				}
			}
			for i := len(part) - 1; i >= 0; i-- {
				s.Uses = append(s.Uses, part[i])
			}
		}
	}
	if e := orthogonalShellIncidence(s, true); e != nil {
		t.Fatal("reversed topology should remain manifold", e)
	}
	if _, e := axisAlignedShellBoundary(context.Background(), f, s); e == nil {
		t.Fatal("inward boundary passed global orientation proof")
	}
}
func FuzzAxisAlignedSplineShell(f *testing.F) {
	raw, e := os.ReadFile("testdata/axis-aligned-spline-shell.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxVertices: 128, MaxEdgeUses: 128})
	})
}
