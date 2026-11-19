package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func wrappedToroidalVoidFixture(t *testing.T) WrappedToroidalVoid {
	t.Helper()
	a, e := geom.NewCanonicalWrappedCircularToroid(context.Background(), geom.Vec3{X: 1, Y: 2, Z: 3}, 3, 1, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewCanonicalWrappedCircularToroid(context.Background(), geom.Vec3{X: 1, Y: 2, Z: 3}, 3, .5, .5, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return WrappedToroidalVoid{Name: "Hollow rational toroid", Outer: WrappedSplineFace{Name: "Exterior", Geometry: a}, Inner: WrappedSplineFace{Name: "Void", Geometry: b}, LengthUnit: .001}
}
func TestWrappedToroidalVoid(t *testing.T) {
	t.Parallel()
	m := wrappedToroidalVoidFixture(t)
	var b bytes.Buffer
	if e := WriteWrappedToroidalVoid(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedToroidalVoid(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.Outer.Name != m.Outer.Name || got.Inner.Name != m.Inner.Name || got.LengthUnit != m.LengthUnit || len(got.Solid.Topology().Shells) != 2 {
		t.Fatal(got)
	}
	mass, e := got.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(mass.Volume-4.5*math.Pi*math.Pi) > 1e-6 || math.Abs(mass.Area-18*math.Pi*math.Pi) > 1e-6 {
		t.Fatal(mass)
	}
	lines := strings.Split(b.String(), "\n")
	changed := false
	for i, line := range lines {
		if strings.Contains(line, "ORIENTED_CLOSED_SHELL") {
			lines[i] = strings.Replace(line, ",.F.)", ",.T.)", 1)
			changed = lines[i] != line
		}
	}
	if !changed {
		t.Fatal("stale void mutation")
	}
	if _, e = ReadWrappedToroidalVoid(strings.NewReader(strings.Join(lines, "\n")), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted positive void orientation")
	}
	m.Inner = m.Outer
	var bad bytes.Buffer
	if e = WriteWrappedToroidalVoid(&bad, m); e == nil || bad.Len() != 0 {
		t.Fatal("accepted touching shells", e)
	}
}
func TestWrappedToroidalVoidOCCT(t *testing.T) {
	t.Parallel()
	m := wrappedToroidalVoidFixture(t)
	var b bytes.Buffer
	if e := WriteWrappedToroidalVoid(&b, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/void.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-mass-precise", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(raw, &mass); e != nil {
		t.Fatal(e)
	}
	if math.Abs(mass.Volume-4.5*math.Pi*math.Pi) > 1e-6 || math.Abs(mass.Area-18*math.Pi*math.Pi) > 1e-6 || math.Abs(mass.Centroid[0]-1) > 1e-8 || math.Abs(mass.Centroid[1]-2) > 1e-8 || math.Abs(mass.Centroid[2]-3) > 1e-8 {
		t.Fatal(mass)
	}
}

func TestWrappedToroidalVoidStoredOCCT(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-toroidal-void.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedToroidalVoid(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Topology().Shells) != 2 {
		t.Fatal(m)
	}
}
func TestWrappedToroidalVoidFromOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-toroidal-void")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedToroidalVoid(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedToroidalVoid(&b, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/roundtrip.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	var masses [2]struct{ Area, Volume float64 }
	for i, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &masses[i]); e != nil {
			t.Fatal(e)
		}
	}
	if math.Abs(masses[0].Volume-masses[1].Volume) > 1e-8 || math.Abs(masses[0].Area-masses[1].Area) > 1e-8 {
		t.Fatal(masses)
	}
}
func TestWrappedToroidalVoidEllipticalProfiles(t *testing.T) {
	t.Parallel()
	m := wrappedToroidalVoidFixture(t)
	var e error
	m.Outer.Geometry, e = geom.NewCanonicalWrappedCircularToroid(context.Background(), geom.Vec3{X: 1, Y: 2, Z: 3}, 3, 1, 2, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Inner.Geometry, e = geom.NewCanonicalWrappedCircularToroid(context.Background(), geom.Vec3{X: 1, Y: 2, Z: 3}, 3, .5, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteWrappedToroidalVoid(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedToroidalVoid(&b, Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := got.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(mass.Volume-9*math.Pi*math.Pi) > 1e-6 {
		t.Fatal(mass, e)
	}
}
