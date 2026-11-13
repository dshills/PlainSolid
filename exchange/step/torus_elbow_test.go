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

func elbowModel(t *testing.T, angle float64) TorusElbowModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return TorusElbowModel{LengthUnit: .001, Bodies: []TorusElbowBody{{"elbow", f, 3, 1, angle}}}
}
func checkElbow(t *testing.T, m AnalyticModel, angle float64) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	want := geom.Vec3{X: 1 + (37./12)*math.Sin(angle)/angle, Y: 2 + (37./12)*(1-math.Cos(angle))/angle, Z: 3}
	if p.Centroid.Distance(want) > 1e-8 {
		t.Fatal(p.Centroid, want)
	}
	if e != nil || math.Abs(p.Volume-3*math.Pi*angle) > 1e-7 || math.Abs(p.Area-(6*math.Pi*angle+2*math.Pi)) > 1e-7 {
		t.Fatal(p, e)
	}
}
func TestTorusElbowRoundtrip(t *testing.T) {
	for _, angle := range []float64{math.Pi / 2, 1.3 * math.Pi} {
		var b bytes.Buffer
		if e := WriteTorusElbows(&b, elbowModel(t, angle)); e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkElbow(t, m, angle)
		if _, e := ReadTrimmedBodies(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
			t.Fatal(e)
		}
	}
}
func TestOCCTTorusElbow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.step")
	stepOracle(t, path, "write-step-torus-elbow")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkElbow(t, m, math.Pi/2)
	for _, angle := range []float64{math.Pi / 2, 1.3 * math.Pi} {
		var b bytes.Buffer
		if e = WriteTorusElbows(&b, elbowModel(t, angle)); e != nil {
			t.Fatal(e)
		}
		ours := filepath.Join(t.TempDir(), "ours.step")
		if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, out, "step-mass-precise", ours)
		raw, e = os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var p struct{ Volume, Area float64 }
		if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-3*math.Pi*angle) > 1e-6 || math.Abs(p.Area-(6*math.Pi*angle+2*math.Pi)) > 1e-6 {
			t.Fatal(angle, p, e)
		}
	}
}

func TestIndependentTorusElbow(t *testing.T) {
	raw, e := os.ReadFile("testdata/torus-elbow.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkElbow(t, m, math.Pi/2)
}
func TestTorusElbowRejectsGraphs(t *testing.T) {
	var raw bytes.Buffer
	if e := WriteTorusElbows(&raw, elbowModel(t, math.Pi/2)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"cap-orientation", "collapsed-seam", "circle-radius", "cap-pcurve-center"} {
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
			if kind == "cap-orientation" && r.Type == "ADVANCED_FACE" && r.Args[3].Text == "F" {
				r.Args[3].Text = "T"
				changed = true
			}
			if kind == "collapsed-seam" && r.Type == "EDGE_CURVE" && r.Args[1].Ref != r.Args[2].Ref {
				r.Args[2] = r.Args[1]
				changed = true
			}
			if kind == "circle-radius" && r.Type == "SURFACE_CURVE" {
				cid := r.Args[1].Ref
				c := f.Entities[cid]
				c.Records[0].Args[2].Text = "1.1"
				f.Entities[cid] = c
				changed = true
			}
			if kind == "cap-pcurve-center" && r.Type == "AXIS2_PLACEMENT_2D" {
				pid := r.Args[1].Ref
				p := f.Entities[pid]
				p.Records[0].Args[1].Items[0].Text = "0.1"
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
	for _, angle := range []float64{0, -1, 2 * math.Pi, math.NaN()} {
		var b bytes.Buffer
		if e := WriteTorusElbows(&b, elbowModel(t, angle)); e == nil || b.Len() != 0 {
			t.Fatal(angle, e)
		}
	}
	if e := WriteTorusElbows(failWriter{}, elbowModel(t, 1)); e == nil {
		t.Fatal("writer failure")
	}
}
func FuzzTorusElbowSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/torus-elbow.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
