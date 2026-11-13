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

func hollowConeModel(t *testing.T) HollowConeModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return HollowConeModel{LengthUnit: .001, Bodies: []HollowConeBody{{"tube", f, 3, 2, 1, .5, 4}}}
}
func checkHollowCone(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-23*math.Pi) > 1e-7 || math.Abs(p.Area-(5*math.Sqrt(17)+1.5*math.Sqrt(16.25)+11.75)*math.Pi) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 328.0 / 69}) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestHollowConeRoundtrip(t *testing.T) {
	var b bytes.Buffer
	if e := WriteHollowCones(&b, hollowConeModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkHollowCone(t, m)
	if _, e := ReadTrimmedBodies(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestOCCTHollowCone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tube.step")
	stepOracle(t, path, "write-step-hollow-cone")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkHollowCone(t, m)
	var b bytes.Buffer
	if e = WriteHollowCones(&b, hollowConeModel(t)); e != nil {
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
	if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-23*math.Pi) > 1e-6 || math.Abs(p.Area-(5*math.Sqrt(17)+1.5*math.Sqrt(16.25)+11.75)*math.Pi) > 1e-6 || math.Abs(p.Centroid[0]-1) > 1e-8 || math.Abs(p.Centroid[1]-2) > 1e-8 || math.Abs(p.Centroid[2]-328.0/69) > 1e-8 {
		t.Fatal(p, e)
	}
}

func TestIndependentHollowCone(t *testing.T) {
	raw, e := os.ReadFile("testdata/hollow-cone.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkHollowCone(t, m)
}
func TestHollowConeInvalid(t *testing.T) {
	for _, kind := range []string{"crossing", "apex", "widening", "height", "nan"} {
		m := hollowConeModel(t)
		b := &m.Bodies[0]
		switch kind {
		case "crossing":
			b.InnerTopRadius = 2.5
		case "apex":
			b.InnerTopRadius = 0
		case "widening":
			b.OuterTopRadius = 4
		case "height":
			b.Height = -1
		case "nan":
			b.InnerBaseRadius = math.NaN()
		}
		var out bytes.Buffer
		if e := WriteHollowCones(&out, m); e == nil || out.Len() != 0 {
			t.Fatal(kind, e, out.Len())
		}
	}
	if e := WriteHollowCones(shortWriter{}, hollowConeModel(t)); e == nil {
		t.Fatal("short write accepted")
	}
	var raw bytes.Buffer
	if e := WriteHollowCones(&raw, hollowConeModel(t)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"inner-normal", "missing-hole", "slope"} {
		t.Run(kind, func(t *testing.T) {
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
				if kind == "slope" && r.Type == "CONICAL_SURFACE" {
					r.Args[3] = Value{Kind: Number, Text: "0.3"}
					changed = true
				}
				if r.Type == "ADVANCED_FACE" {
					sid, _ := ref(r.Args[2])
					typ := f.Entities[sid].Records[0].Type
					if kind == "inner-normal" && typ == "CONICAL_SURFACE" && r.Args[3].Text == "F" {
						r.Args[3].Text = "T"
						changed = true
					}
					if kind == "missing-hole" && typ == "PLANE" && len(r.Args[1].Items) == 2 {
						r.Args[1].Items = r.Args[1].Items[:1]
						changed = true
					}
				}
				ent.Records[0] = r
				f.Entities[id] = ent
			}
			if !changed || body == 0 {
				t.Fatal("mutation missing")
			}
			if _, e := analyticBody(f, body); e == nil {
				t.Fatal("accepted invalid graph")
			}
		})
	}
}
func FuzzHollowConeSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/hollow-cone.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 512, MaxFaces: 256, MaxSolids: 4, MaxEdgeUses: 2048})
	})
}

func TestHollowConeRotatedMultiple(t *testing.T) {
	m := hollowConeModel(t)
	fr, e := geom.NewFrame(geom.Vec3{X: -2, Y: 4, Z: 1}, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: -1, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	second := m.Bodies[0]
	second.Frame = fr
	second.Name = "second"
	m.Bodies = append(m.Bodies, second)
	var b bytes.Buffer
	if e := WriteHollowCones(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil || len(got.Bodies) != 2 {
		t.Fatal(got, e)
	}
	p, e := got.Bodies[1].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-23*math.Pi) > 1e-7 || p.Centroid.Distance(fr.ToWorld(geom.Vec3{Z: 121.0 / 69})) > 1e-8 {
		t.Fatal(p, e)
	}
}
