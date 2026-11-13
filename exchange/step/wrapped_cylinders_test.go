package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestWrappedCylinderStoredOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || !m.Bodies[0].Embedding.Complete {
		t.Fatal(m)
	}
}

func TestWrappedCylinderRoundtripOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Bodies[0].Name = "Rational capped cylinder"
	var b bytes.Buffer
	if e = WriteWrappedCylindricalBodies(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedCylindricalBodies(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Bodies[0].Name != m.Bodies[0].Name || got.LengthUnit != m.LengthUnit {
		t.Fatal(got)
	}
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-cylinder")
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
	if masses[0].Volume <= 0 || math.Abs(masses[0].Volume-masses[1].Volume) > 1e-8 || math.Abs(masses[0].Area-masses[1].Area) > 1e-8 {
		t.Fatal(masses)
	}
}

func TestWrappedCylinderReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{
		{"#91 = CARTESIAN_POINT('',(2.,0.));", "#91 = CARTESIAN_POINT('',(2.000000000000001,0.));"},
		{"#141 = FACE_BOUND('',#142,.F.);", "#141 = FACE_BOUND('',#142,.T.);"},
		{"#147 = ORIENTED_EDGE('',*,*,#103,.T.);", "#147 = ORIENTED_EDGE('',*,*,#64,.T.);"},
		{"#140 = ADVANCED_FACE('',(#141),#84,.F.);", "#140 = ADVANCED_FACE('',(#141),#84,.T.);"},
	} {
		bad := strings.Replace(string(raw), change[0], change[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation", change[0])
		}
		if _, e = ReadWrappedCylindricalBodies(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("accepted invalid cap", change[0])
		}
	}
	if _, e = ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{MaxFaces: 2}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("face budget")
	}
	if _, e = ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{MaxOperations: 1}); e == nil {
		t.Fatal("arithmetic budget")
	}
}
