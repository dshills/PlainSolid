package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestWrappedToroidalBodies(t *testing.T) {
	m := WrappedToroidalModel{LengthUnit: .001, Bodies: []WrappedToroidalBody{{Name: "weighted toroid", Face: WrappedSplineFace{Name: "periodic skin", Geometry: wrappedToroidForSTEP(t)}}}}
	var b bytes.Buffer
	if e := WriteWrappedToroidalBodies(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadWrappedToroidalBodies(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Bodies) != 1 || got.LengthUnit != m.LengthUnit || got.Bodies[0].Name != m.Bodies[0].Name || got.Bodies[0].Face.Name != "periodic skin" || !got.Bodies[0].Embedding.Outward || !got.Bodies[0].Embedding.Complete {
		t.Fatalf("%+v", got)
	}
	top := got.Bodies[0].Solid.Topology()
	if top.Vertices != 1 || top.Edges != 2 || top.Faces != 1 || len(top.Shells) != 1 {
		t.Fatal(top)
	}
	if _, e = ReadWrappedSplineFaces(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("surface API accepted material")
	}
	bad := strings.Replace(b.String(), "CLOSED_SHELL(", "OPEN_SHELL(", 1)
	if _, e = ReadWrappedToroidalBodies(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted open shell")
	}
	lines := strings.Split(b.String(), "\n")
	changed := false
	for i, line := range lines {
		if strings.Contains(line, "=ADVANCED_FACE(") {
			lines[i] = strings.Replace(line, ",.T.);", ",.F.);", 1)
			changed = lines[i] != line
		}
	}
	if !changed {
		t.Fatal("stale orientation mutation")
	}
	if _, e = ReadWrappedToroidalBodies(strings.NewReader(strings.Join(lines, "\n")), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted inward face")
	}
	// A perfectly valid seam-bearing cylinder is not a toroidal material body.
	m.Bodies[0].Face.Geometry = wrappedCylinder(t)
	var rejected bytes.Buffer
	if e = WriteWrappedToroidalBodies(&rejected, m); e == nil || rejected.Len() != 0 {
		t.Fatal("cylinder promotion", e)
	}
}

func TestWrappedToroidalBodiesOCCT(t *testing.T) {
	m := WrappedToroidalModel{LengthUnit: .001, Bodies: []WrappedToroidalBody{{Name: "weighted toroid", Face: WrappedSplineFace{Geometry: wrappedToroidForSTEP(t)}}}}
	var b bytes.Buffer
	if e := WriteWrappedToroidalBodies(&b, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/toroid.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-mass-precise", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct{ Area, Volume float64 }
	if e = json.Unmarshal(raw, &mass); e != nil {
		t.Fatal(e)
	}
	// Dyadic quarter weights are close to circular weights, but the source is
	// the exact represented rational tensor shape, not an ideal analytic torus.
	if math.Abs(mass.Volume-6*math.Pi*math.Pi) > 1e-5 || math.Abs(mass.Area-12*math.Pi*math.Pi) > 1e-5 {
		t.Fatal(mass)
	}
}

func TestWrappedToroidalBodiesFromOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-wrapped-toroid")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedToroidalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || !m.Bodies[0].Embedding.Complete {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteWrappedToroidalBodies(&b, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/roundtrip.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	var measures [2]struct{ Area, Volume float64 }
	for i, p := range []string{source, path} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &measures[i]); e != nil {
			t.Fatal(e)
		}
	}
	if measures[0].Volume <= 0 || math.Abs(measures[0].Volume-measures[1].Volume) > 1e-8 || math.Abs(measures[0].Area-measures[1].Area) > 1e-8 {
		t.Fatal(measures)
	}
}

func TestWrappedToroidalStoredOCCT(t *testing.T) {
	raw, e := os.ReadFile("testdata/wrapped-toroid.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedToroidalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || !m.Bodies[0].Embedding.Complete || !m.Bodies[0].Embedding.Outward {
		t.Fatal(m)
	}
}
