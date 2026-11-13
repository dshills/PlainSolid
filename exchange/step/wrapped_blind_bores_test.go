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

func TestWrappedBlindBoreStoredOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-blind-bore.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedBlindBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Solid.Topology().Faces != 5 {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteWrappedBlindBore(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedBlindBore(&b, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
}

func TestWrappedBlindBoreLiveOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-blind-bore")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedBlindBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Name = "Blind bore"
	m.Outer.Name = "Outer wall"
	m.Inner.Name = "Bore wall"
	m.CapNames = [2]string{"Bottom", "Opening"}
	m.FloorName = "Internal floor"
	var b bytes.Buffer
	if e = WriteWrappedBlindBore(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedBlindBore(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.FloorName != m.FloorName || got.CapNames != m.CapNames || got.Inner.Name != m.Inner.Name || got.Outer.Name != m.Outer.Name {
		t.Fatal(got)
	}
	path := t.TempDir() + "/roundtrip.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	var masses [2]struct {
		Area, Volume float64
		Centroid     [3]float64
	}
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
	native, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(masses[0].Area-masses[1].Area) > 1e-8 || math.Abs(masses[0].Volume-masses[1].Volume) > 1e-8 || math.Abs(native.Volume-masses[0].Volume) > 1e-6 || math.Abs(native.Area-masses[0].Area) > 1e-6 || native.Centroid.Distance(geom.Vec3{X: masses[0].Centroid[0], Y: masses[0].Centroid[1], Z: masses[0].Centroid[2]}) > 1e-7 {
		t.Fatal(masses, native)
	}
}
func TestWrappedBlindBoreScope(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-blind-bore.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedBlindBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedTube(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("tube accepted blind bore")
	}
	tube, e := os.ReadFile("testdata/wrapped-tube.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedBlindBore(bytes.NewReader(tube), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("blind API accepted through bore")
	}
	bad := strings.Replace(string(raw), "#281 = ADVANCED_FACE('',(#282),#262,.T.);", "#281 = ADVANCED_FACE('',(#282),#262,.F.);", 1)
	if bad == string(raw) {
		t.Fatal("stale floor mutation")
	}
	if _, e = ReadWrappedBlindBore(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted inward floor")
	}
	original := m.Inner.Geometry
	m.Inner.Geometry, _, e = original.Transformed(context.Background(), geom.Translation(geom.Vec3{Z: -2.5}), geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.FloorName = "Upper floor"
	var b bytes.Buffer
	if e = WriteWrappedBlindBore(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedBlindBore(&b, Limits{}, WrappedSurfaceOptions{})
	if e != nil || got.FloorName != m.FloorName {
		t.Fatal(got, e)
	}
	m.Inner.Geometry, _, e = original.Transformed(context.Background(), geom.Translation(geom.Vec3{Z: -1}), geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteWrappedBlindBore(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("accepted cavity", e)
	}
}
