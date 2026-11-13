package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func axialModel(t *testing.T) AxialProfileModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return AxialProfileModel{LengthUnit: .001, Bodies: []AxialProfileBody{{"shaft", f, []CylinderSection{{3, 4}, {1, 2}, {.5, 1}}}}}
}
func checkAxial(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-38.25*math.Pi) > 1e-7 || math.Abs(p.Area-47*math.Pi) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 198.375 / 38.25}) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestAxialProfileRoundtrip(t *testing.T) {
	var b bytes.Buffer
	if e := WriteAxialProfiles(&b, axialModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkAxial(t, m)
}
func TestOCCTAxialProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.step")
	stepOracle(t, path, "write-step-axial-profile")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkAxial(t, m)
	var b bytes.Buffer
	if e = WriteAxialProfiles(&b, axialModel(t)); e != nil {
		t.Fatal(e)
	}
	ours := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, ours)
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Volume, Area float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-38.25*math.Pi) > 1e-6 || math.Abs(p.Area-47*math.Pi) > 1e-6 || math.Abs(p.Centroid[2]-198.375/38.25) > 1e-8 {
		t.Fatal(p, e)
	}
}

func TestAxialProfileLongerAndInvalid(t *testing.T) {
	m := axialModel(t)
	m.Bodies[0].Sections = append(m.Bodies[0].Sections, CylinderSection{.25, .5})
	var b bytes.Buffer
	if e := WriteAxialProfiles(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := got.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-38.28125*math.Pi) > 1e-7 || math.Abs(p.Area-47.25*math.Pi) > 1e-7 {
		t.Fatal(p, e)
	}
	for _, kind := range []string{"equal-radius", "zero-radius", "negative-height", "few-sections", "NaN"} {
		m := axialModel(t)
		switch kind {
		case "equal-radius":
			m.Bodies[0].Sections[2].Radius = 1
		case "zero-radius":
			m.Bodies[0].Sections[2].Radius = 0
		case "negative-height":
			m.Bodies[0].Sections[1].Height = -1
		case "few-sections":
			m.Bodies[0].Sections = m.Bodies[0].Sections[:2]
		case "NaN":
			m.Bodies[0].Sections[2].Radius = math.NaN()
		}
		var b bytes.Buffer
		if e := WriteAxialProfiles(&b, m); e == nil || b.Len() != 0 {
			t.Fatal(kind, e)
		}
	}
	if e := WriteAxialProfiles(failWriter{}, axialModel(t)); e == nil {
		t.Fatal("writer error")
	}
}
func TestAxialProfileCoverage(t *testing.T) {
	var raw bytes.Buffer
	if e := WriteAxialProfiles(&raw, axialModel(t)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"shoulder-hole", "cap-sense", "axial-gap"} {
		f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		body := int64(0)
		changed := false
		for _, id := range f.Order {
			ent := f.Entities[id]
			if len(ent.Records) != 1 {
				continue
			}
			r := ent.Records[0]
			if r.Type == "MANIFOLD_SOLID_BREP" {
				body = id
			}
			if changed {
				continue
			}
			if r.Type == "ADVANCED_FACE" && len(r.Args[1].Items) == 2 {
				if kind == "shoulder-hole" {
					r.Args[1].Items = r.Args[1].Items[:1]
					changed = true
				}
				if kind == "cap-sense" {
					r.Args[3].Text = "F"
					changed = true
				}
			}
			if kind == "axial-gap" && r.Type == "CYLINDRICAL_SURFACE" && r.Args[2].Text == "0.5" {
				placement := f.Entities[r.Args[1].Ref]
				pid := placement.Records[0].Args[1].Ref
				p := f.Entities[pid]
				p.Records[0].Args[1].Items[2].Text = "9.25"
				f.Entities[pid] = p
				changed = true
			}
			ent.Records[0] = r
			f.Entities[id] = ent
		}
		if !changed || body == 0 {
			t.Fatal("stale mutation", kind)
		}
		if _, e := analyticBody(f, body); e == nil {
			t.Fatal("accepted", kind)
		}
	}
}
func TestIndependentAxialProfile(t *testing.T) {
	raw, e := os.ReadFile("testdata/axial-profile.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkAxial(t, m)
}
func FuzzAxialProfileSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/axial-profile.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 512, MaxFaces: 256, MaxSolids: 4, MaxEdgeUses: 2048})
	})
}
