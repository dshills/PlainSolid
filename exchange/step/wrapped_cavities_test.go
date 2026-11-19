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

func TestWrappedCavityStoredOCCT(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-cavity.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedCavity(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Solid.Topology().Shells) != 2 {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteWrappedCavity(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedCavity(&b, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
}

func TestWrappedCavityLiveOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-cavity")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedCavity(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Name = "Enclosed rational cavity"
	m.Outer.Name = "Outer wall"
	m.Inner.Name = "Void wall"
	m.OuterCapNames = [2]string{"Outer bottom", "Outer top"}
	m.InnerCapNames = [2]string{"Void bottom", "Void top"}
	var b bytes.Buffer
	if e = WriteWrappedCavity(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedCavity(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.Outer.Name != m.Outer.Name || got.Inner.Name != m.Inner.Name || got.OuterCapNames != m.OuterCapNames || got.InnerCapNames != m.InnerCapNames || got.LengthUnit != m.LengthUnit {
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
func TestWrappedCavityReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-cavity.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{{"#148 = ORIENTED_CLOSED_SHELL('',*,#149,.F.);", "#148 = ORIENTED_CLOSED_SHELL('',*,#149,.T.);"}, {"#15 = BREP_WITH_VOIDS('',#16,(#148));", "#15 = BREP_WITH_VOIDS('',#149,(#148));"}, {"#150 = ADVANCED_FACE('',(#151),#164,.T.);", "#150 = ADVANCED_FACE('',(#151),#164,.F.);"}} {
		bad := strings.Replace(string(raw), change[0], change[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation")
		}
		if _, e = ReadWrappedCavity(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("accepted wrong orientation/ownership")
		}
	}
	m, e := ReadWrappedCavity(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, offset := range []geom.Vec3{{Z: 1.25}, {Z: 2}, {X: 2}} {
		bad := m
		bad.Inner.Geometry, _, e = m.Inner.Geometry.Transformed(context.Background(), geom.Translation(offset), geom.WrappedSurfaceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteWrappedCavity(&out, bad); e == nil || out.Len() != 0 {
			t.Fatal("accepted touching/outside/noncoaxial void", offset, e)
		}
	}
	if _, e = ReadWrappedBlindBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("blind API accepted void shell")
	}
}

func FuzzWrappedCavity(f *testing.F) {
	raw, e := os.ReadFile("testdata/wrapped-cavity.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_, _ = ReadWrappedCavity(bytes.NewReader(data), Limits{MaxBytes: 1 << 20, MaxEntities: 2000, MaxValues: 30000, MaxDepth: 32, MaxVertices: 4096, MaxFaces: 64, MaxSolids: 8, MaxEdgeUses: 256}, WrappedSurfaceOptions{MaxSurfaces: 8, MaxPoles: 256, MaxOperations: 200000, MaxCoefficientBits: 4096})
	})
}
