package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/brep/parametric"
	"strings"
	"testing"
)

func TestWrappedFrustaStoredOCCT(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"wrapped-frustum", "wrapped-circular-frustum"} {
		t.Run(kind, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/" + kind + ".step")
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadWrappedFrusta(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(m.Bodies) != 1 || !m.Bodies[0].Embedding.Complete || m.Bodies[0].Embedding.Circular != (kind == "wrapped-circular-frustum") {
				t.Fatal(m)
			}
			if _, e = ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
				t.Fatal("cylinder API accepted taper")
			}
			m.Bodies[0].Name = "Drafted rational body"
			m.Bodies[0].Face.Name = "Tapered wall"
			m.Bodies[0].CapNames = [2]string{"Large end", "Small end"}
			var out bytes.Buffer
			if e = WriteWrappedFrusta(&out, m); e != nil {
				t.Fatal(e)
			}
			got, e := ReadWrappedFrusta(&out, Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if got.Bodies[0].Name != m.Bodies[0].Name || got.Bodies[0].Face.Name != m.Bodies[0].Face.Name || got.Bodies[0].CapNames != m.Bodies[0].CapNames || got.LengthUnit != m.LengthUnit {
				t.Fatal(got)
			}
			if kind == "wrapped-circular-frustum" {
				mass, e := got.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil {
					t.Fatal(e)
				}
				if math.Abs(mass.Volume-35*math.Pi/3) > 1e-6 || math.Abs(mass.Area-(3*math.Pi*math.Sqrt(26)+5*math.Pi)) > 1e-6 || math.Abs(mass.Centroid.Z+1.0/28) > 1e-7 {
					t.Fatal(mass)
				}
			}
		})
	}
}

func TestWrappedFrustaLiveOCCT(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"write-step-wrapped-frustum", "write-step-wrapped-circular-frustum"} {
		t.Run(mode, func(t *testing.T) {
			source := t.TempDir() + "/source.step"
			stepOracle(t, source, mode)
			raw, e := os.ReadFile(source)
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadWrappedFrusta(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			var b bytes.Buffer
			if e = WriteWrappedFrusta(&b, m); e != nil {
				t.Fatal(e)
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
			if math.Abs(masses[0].Area-masses[1].Area) > 1e-8 || math.Abs(masses[0].Volume-masses[1].Volume) > 1e-8 {
				t.Fatal(masses)
			}
			if mode == "write-step-wrapped-circular-frustum" && (math.Abs(masses[0].Volume-35*math.Pi/3) > 1e-6 || math.Abs(masses[0].Area-(3*math.Pi*math.Sqrt(26)+5*math.Pi)) > 1e-6 || math.Abs(masses[0].Centroid[2]+1.0/28) > 1e-7) {
				t.Fatal(masses)
			}
		})
	}
}

func TestWrappedFrustaReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/wrapped-frustum.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{{"#35 = CARTESIAN_POINT('',(1.,1.,3.));", "#35 = CARTESIAN_POINT('',(1.000000000000001,1.,3.));"}, {"#141 = FACE_BOUND('',#142,.F.);", "#141 = FACE_BOUND('',#142,.T.);"}} {
		bad := strings.Replace(string(raw), change[0], change[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation")
		}
		if _, e = ReadWrappedFrusta(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("accepted malformed frustum")
		}
	}
	m, e := ReadWrappedFrusta(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	old := WrappedCylindricalModel{LengthUnit: m.LengthUnit, Bodies: []WrappedCylindricalBody{{Face: m.Bodies[0].Face}}}
	if e = WriteWrappedCylindricalBodies(&out, old); e == nil || out.Len() != 0 {
		t.Fatal("old writer accepted taper", e)
	}
}
