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

func TestWrappedShoulderStoredOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-shoulder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedShoulder(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	top := m.Solid.Topology()
	if top.Faces != 5 || len(top.Shells) != 1 || top.EulerCharacteristic != 2 {
		t.Fatal(top)
	}
	var b bytes.Buffer
	if e = WriteWrappedShoulder(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedShoulder(&b, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
}

func TestWrappedShoulderLiveOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-shoulder")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedShoulder(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Name = "Stepped rational body"
	m.Larger.Name = "Large wall"
	m.Smaller.Name = "Small wall"
	m.LargerCapNames = [2]string{"Large end", "Shoulder"}
	m.SmallerEndName = "Small end"
	var b bytes.Buffer
	if e = WriteWrappedShoulder(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedShoulder(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.Larger.Name != m.Larger.Name || got.Smaller.Name != m.Smaller.Name || got.LargerCapNames != m.LargerCapNames || got.SmallerEndName != m.SmallerEndName || got.LengthUnit != m.LengthUnit {
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
func TestWrappedShoulderScope(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-shoulder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedShoulder(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{{"#210 = ADVANCED_FACE('',(#211),#179,.T.);", "#210 = ADVANCED_FACE('',(#211),#179,.F.);"}, {"#148 = FACE_BOUND('',#149,.T.);", "#148 = FACE_BOUND('',#149,.F.);"}} {
		bad := strings.Replace(string(raw), change[0], change[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation")
		}
		if _, e = ReadWrappedShoulder(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("accepted reversed wall/shoulder")
		}
	}
	if _, e = ReadWrappedBlindBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("blind API accepted shoulder")
	}
	for _, shift := range []float64{.25, -.25} {
		bad := m
		bad.Smaller.Geometry, _, e = m.Smaller.Geometry.Transformed(context.Background(), geom.Translation(geom.Vec3{Z: shift}), geom.WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteWrappedShoulder(&out, bad); e == nil || out.Len() != 0 {
			t.Fatal("accepted gap/overlap", shift, e)
		}
	}
	m.Smaller.Geometry, _, e = m.Smaller.Geometry.Transformed(context.Background(), geom.Translation(geom.Vec3{Z: -7.5}), geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteWrappedShoulder(&out, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedShoulder(&out, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal("widening shoulder", e)
	}
}
