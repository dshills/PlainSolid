package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestReadWrappedTubeOCCTFixture(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-tube.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedTube(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Solid.Topology().Faces != 4 {
		t.Fatal(m)
	}
}

func TestWrappedTubeRoundtripOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-tube.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedTube(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Name = "Rational annular tube"
	m.Outer.Name = "Outer wall"
	m.Inner.Name = "Bore wall"
	m.CapNames = [2]string{"Lower annulus", "Upper annulus"}
	var b bytes.Buffer
	if e = WriteWrappedTube(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedTube(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Name != m.Name || got.Outer.Name != m.Outer.Name || got.Inner.Name != m.Inner.Name || got.CapNames != m.CapNames {
		t.Fatal(got)
	}
	path := t.TempDir() + "/tube.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-tube")
	var masses [2]struct{ Volume, Area float64 }
	for i, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(data, &masses[i]); e != nil {
			t.Fatal(e)
		}
	}
	if math.Abs(masses[0].Volume-masses[1].Volume) > 1e-8 || math.Abs(masses[0].Area-masses[1].Area) > 1e-8 {
		t.Fatal(masses)
	}
}
func TestWrappedTubeReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-tube.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{{"#266 = CARTESIAN_POINT('',(0.,0.5));", "#266 = CARTESIAN_POINT('',(0.,0.500000000000001));"}, {"#253 = ADVANCED_FACE('',(#254),#175,.F.);", "#253 = ADVANCED_FACE('',(#254),#175,.T.);"}, {"#146 = ORIENTED_EDGE('',*,*,#147,.F.);", "#146 = ORIENTED_EDGE('',*,*,#147,.T.);"}} {
		bad := strings.Replace(string(raw), change[0], change[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation")
		}
		if _, e = ReadWrappedTube(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("accepted malformed tube")
		}
	}
}
