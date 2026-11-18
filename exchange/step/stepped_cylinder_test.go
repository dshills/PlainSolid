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

func steppedModel(t *testing.T) SteppedCylinderModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return SteppedCylinderModel{LengthUnit: .001, Bodies: []SteppedCylinderBody{{"shaft", f, 3, 1, 4, 2}}}
}
func checkStepped(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-38*math.Pi) > 1e-7 || math.Abs(p.Area-46*math.Pi) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 98. / 19}) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestSteppedCylinderRoundtrip(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteSteppedCylinders(&b, steppedModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkStepped(t, m)
}
func TestOCCTSteppedCylinder(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "reference.step")
	stepOracle(t, path, "write-step-stepped-cylinder")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkStepped(t, m)
	var b bytes.Buffer
	if e = WriteSteppedCylinders(&b, steppedModel(t)); e != nil {
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
	if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-38*math.Pi) > 1e-6 || math.Abs(p.Area-46*math.Pi) > 1e-6 || math.Abs(p.Centroid[2]-98./19) > 1e-8 {
		t.Fatal(p, e)
	}
}

func TestSteppedCylinderCoverage(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer
	if e := WriteSteppedCylinders(&raw, steppedModel(t)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"missing-shoulder-hole", "shoulder-orientation", "shifted-coverage"} {
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
				if kind == "missing-shoulder-hole" {
					r.Args[1].Items = r.Args[1].Items[:1]
					changed = true
				}
				if kind == "shoulder-orientation" {
					r.Args[3].Text = "F"
					changed = true
				}
			}
			if kind == "shifted-coverage" && r.Type == "CYLINDRICAL_SURFACE" && r.Args[2].Text == "1." {
				placement := f.Entities[r.Args[1].Ref]
				pointID := placement.Records[0].Args[1].Ref
				point := f.Entities[pointID]
				point.Records[0].Args[1].Items[2].Text = "7.25"
				f.Entities[pointID] = point
				changed = true
			}
			ent.Records[0] = r
			f.Entities[id] = ent
		}
		if !changed || body == 0 {
			t.Fatal("stale mutation", kind)
		}
		if _, e := analyticBody(f, body); e == nil {
			t.Fatal("accepted incomplete coverage", kind)
		}
	}
}
func TestIndependentSteppedCylinder(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/stepped-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkStepped(t, m)
}
func TestSteppedCylinderWriterInvalid(t *testing.T) {
	t.Parallel()
	for _, r := range []float64{0, 3, 4, math.NaN()} {
		m := steppedModel(t)
		m.Bodies[0].UpperRadius = r
		var b bytes.Buffer
		if e := WriteSteppedCylinders(&b, m); e == nil || b.Len() != 0 {
			t.Fatal(r, e)
		}
	}
	if e := WriteSteppedCylinders(failWriter{}, steppedModel(t)); e == nil {
		t.Fatal("writer error")
	}
}
func FuzzSteppedCylinderSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/stepped-cylinder.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
