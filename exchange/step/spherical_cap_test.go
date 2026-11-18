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

func capModel(t *testing.T, latitude float64) SphericalCapModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return SphericalCapModel{LengthUnit: .001, Bodies: []SphericalCapBody{{"cap", f, 2, latitude}}}
}
func checkCap(t *testing.T, m AnalyticModel, latitude float64) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	h := 2 * (1 - math.Sin(latitude))
	volume := math.Pi * h * h * (2 - h/3)
	area := 4*math.Pi*h + 4*math.Pi*math.Pow(math.Cos(latitude), 2)
	if e != nil || math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
		t.Fatal(p, e, volume, area)
	}
	if latitude == 0 && p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 3.75}) > 1e-8 {
		t.Fatal(p)
	}
}
func TestSphericalCapRoundtrip(t *testing.T) {
	t.Parallel()
	for _, latitude := range []float64{0, math.Pi / 6, -math.Pi / 6} {
		var b bytes.Buffer
		if e := WriteSphericalCaps(&b, capModel(t, latitude)); e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkCap(t, m, latitude)
	}
}
func TestOCCTSphericalCap(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "hemisphere.step")
	stepOracle(t, path, "write-step-hemisphere")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkCap(t, m, 0)
	for _, latitude := range []float64{0, math.Pi / 6, -math.Pi / 6} {
		var b bytes.Buffer
		if e = WriteSphericalCaps(&b, capModel(t, latitude)); e != nil {
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
		h := 2 * (1 - math.Sin(latitude))
		volume := math.Pi * h * h * (2 - h/3)
		area := 4*math.Pi*h + 4*math.Pi*math.Pow(math.Cos(latitude), 2)
		if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-volume) > 1e-6 || math.Abs(p.Area-area) > 1e-6 {
			t.Fatal(latitude, p, e, volume, area)
		}
	}
}

func TestIndependentHemisphere(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/hemisphere.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkCap(t, m, 0)
}
func TestSphericalCapInvalidGraphs(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer
	if e := WriteSphericalCaps(&raw, capModel(t, 0)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"pole", "cap-sense", "rim-radius", "seam-phase"} {
		f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		body := int64(0)
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
			if kind == "cap-sense" && r.Type == "ADVANCED_FACE" && r.Args[3].Text == "F" {
				r.Args[3].Text = "T"
				changed = true
			}
			if kind == "rim-radius" && r.Type == "SURFACE_CURVE" {
				cid := r.Args[1].Ref
				circle := f.Entities[cid]
				circle.Records[0].Args[2].Text = "2.1"
				f.Entities[cid] = circle
				changed = true
			}
			if kind == "pole" && r.Type == "VERTEX_POINT" {
				pid := r.Args[1].Ref
				p := f.Entities[pid]
				z := p.Records[0].Args[1].Items[2]
				if z.Text == "5." {
					p.Records[0].Args[1].Items[2].Text = "5.1"
					f.Entities[pid] = p
					changed = true
				}
			}
			if kind == "seam-phase" && r.Type == "SEAM_CURVE" {
				pc := f.Entities[r.Args[2].Items[0].Ref].Records[0]
				rep := f.Entities[pc.Args[2].Ref].Records[0]
				line := f.Entities[rep.Args[1].Items[0].Ref].Records[0]
				pid := line.Args[1].Ref
				p := f.Entities[pid]
				p.Records[0].Args[1].Items[1].Text = "0.1"
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
func TestSphericalCapWriterInvalid(t *testing.T) {
	t.Parallel()
	for _, latitude := range []float64{math.Pi / 2, -math.Pi / 2, math.NaN(), math.Inf(1)} {
		var b bytes.Buffer
		if e := WriteSphericalCaps(&b, capModel(t, latitude)); e == nil || b.Len() != 0 {
			t.Fatal(latitude, e)
		}
	}
	if e := WriteSphericalCaps(failWriter{}, capModel(t, 0)); e == nil {
		t.Fatal("writer failure")
	}
}
func FuzzSphericalCapSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/hemisphere.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
