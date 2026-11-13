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

func mixedAxialModel(t *testing.T) MixedAxialModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return MixedAxialModel{LengthUnit: .001, Bodies: []MixedAxialBody{{"tube", f, []AxialSection{{3, 3, 2}, {2, 1, 2}, {.5, .5, 1}}}}}
}
func checkMixedAxial(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-275.0/12*math.Pi) > 1e-7 || math.Abs(p.Area-(28+3*math.Sqrt(5))*math.Pi) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 3 + 771.0/550}) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestMixedAxialRoundtrip(t *testing.T) {
	var b bytes.Buffer
	if e := WriteMixedAxialProfiles(&b, mixedAxialModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkMixedAxial(t, m)
	if _, e := ReadTrimmedBodies(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestOCCTMixedAxial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tube.step")
	stepOracle(t, path, "write-step-mixed-axial")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkMixedAxial(t, m)
	var b bytes.Buffer
	if e = WriteMixedAxialProfiles(&b, mixedAxialModel(t)); e != nil {
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
	if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-275.0/12*math.Pi) > 1e-6 || math.Abs(p.Area-(28+3*math.Sqrt(5))*math.Pi) > 1e-6 || math.Abs(p.Centroid[0]-1) > 1e-8 || math.Abs(p.Centroid[1]-2) > 1e-8 || math.Abs(p.Centroid[2]-(3+771.0/550)) > 1e-8 {
		t.Fatal(p, e)
	}
}

func TestIndependentMixedAxial(t *testing.T) {
	raw, e := os.ReadFile("testdata/mixed-axial.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkMixedAxial(t, m)
}
func TestMixedAxialInvalid(t *testing.T) {
	for _, kind := range []string{"no-shoulder", "apex", "widening", "nan", "short"} {
		m := mixedAxialModel(t)
		switch kind {
		case "no-shoulder":
			m.Bodies[0].Sections[1].BaseRadius = 3
		case "apex":
			m.Bodies[0].Sections[1].TopRadius = 0
		case "widening":
			m.Bodies[0].Sections[1].TopRadius = 4
		case "nan":
			m.Bodies[0].Sections[1].Height = math.NaN()
		case "short":
			m.Bodies[0].Sections = m.Bodies[0].Sections[:2]
		}
		var out bytes.Buffer
		if e := WriteMixedAxialProfiles(&out, m); e == nil || out.Len() != 0 {
			t.Fatal(kind, e)
		}
	}
	raw, e := os.ReadFile("testdata/mixed-axial.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"slope", "shoulder", "missing-bound"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
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
				if kind == "slope" && r.Type == "CONICAL_SURFACE" {
					r.Args[3] = Value{Kind: Number, Text: "0.2"}
					changed = true
				}
				if r.Type == "ADVANCED_FACE" && len(r.Args[1].Items) == 2 {
					if kind == "shoulder" {
						r.Args[3].Text = "F"
						changed = true
					}
					if kind == "missing-bound" {
						r.Args[1].Items = r.Args[1].Items[:1]
						changed = true
					}
				}
				ent.Records[0] = r
				f.Entities[id] = ent
			}
			if !changed || body == 0 {
				t.Fatal("missing mutation")
			}
			if _, e := analyticBody(f, body); e == nil {
				t.Fatal("accepted invalid graph")
			}
		})
	}
}
func FuzzMixedAxialSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/mixed-axial.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 512, MaxFaces: 256, MaxSolids: 4, MaxEdgeUses: 2048})
	})
}

func TestMixedAxialWideningShoulder(t *testing.T) {
	m := mixedAxialModel(t)
	m.Bodies[0].Sections = []AxialSection{{4, 4, 1}, {3, 1, 1}, {2, 2, 1}}
	var b bytes.Buffer
	if e := WriteMixedAxialProfiles(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := got.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-73.0/3*math.Pi) > 1e-7 || math.Abs(p.Area-(42+4*math.Sqrt(5))*math.Pi) > 1e-7 {
		t.Fatal(p, e)
	}
}
