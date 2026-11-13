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

func zoneModel(t *testing.T) SphericalZoneModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return SphericalZoneModel{LengthUnit: .001, Bodies: []SphericalZoneBody{{"zone", f, 2, -math.Pi / 6, math.Pi / 6}}}
}
func checkZone(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-22*math.Pi/3) > 1e-7 || math.Abs(p.Area-14*math.Pi) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 3}) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestSphericalZoneRoundtrip(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSphericalZones(&b, zoneModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkZone(t, m)
	if _, e := ReadTrimmedBodies(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestOCCTSphericalZone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.step")
	stepOracle(t, path, "write-step-spherical-zone")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkZone(t, m)
	var b bytes.Buffer
	if e = WriteSphericalZones(&b, zoneModel(t)); e != nil {
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
	var p struct {
		Volume, Area float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-22*math.Pi/3) > 1e-6 || math.Abs(p.Area-14*math.Pi) > 1e-6 || math.Abs(p.Centroid[2]-3) > 1e-8 {
		t.Fatal(p, e)
	}
}

func TestSphericalZoneMalformed(t *testing.T) {
	var raw bytes.Buffer
	if e := WriteSphericalZones(&raw, zoneModel(t)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"collapsed-seam", "cap-orientation", "rim-radius"} {
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
			if kind == "collapsed-seam" && r.Type == "EDGE_CURVE" && r.Args[1].Ref != r.Args[2].Ref {
				r.Args[2] = r.Args[1]
				changed = true
			}
			if kind == "cap-orientation" && r.Type == "ADVANCED_FACE" && r.Args[3].Text == "F" {
				r.Args[3].Text = "T"
				changed = true
			}
			if kind == "rim-radius" && r.Type == "SURFACE_CURVE" {
				cid := r.Args[1].Ref
				circle := f.Entities[cid]
				circle.Records[0].Args[2].Text = "1.9"
				f.Entities[cid] = circle
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
	for _, pair := range [][2]float64{{0, 0}, {1, -1}, {-math.Pi / 2, 0}, {0, math.Pi / 2}, {math.NaN(), 0}} {
		m := zoneModel(t)
		m.Bodies[0].LowerLatitude, m.Bodies[0].UpperLatitude = pair[0], pair[1]
		var b bytes.Buffer
		if e := WriteSphericalZones(&b, m); e == nil || b.Len() != 0 {
			t.Fatal(pair, e)
		}
	}
	if e := WriteSphericalZones(failWriter{}, zoneModel(t)); e == nil {
		t.Fatal("writer failure")
	}
}
func TestIndependentSphericalZone(t *testing.T) {
	raw, e := os.ReadFile("testdata/spherical-zone.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkZone(t, m)
	for _, l := range []Limits{{MaxVertices: 1}, {MaxFaces: 1}, {MaxEdgeUses: 1}} {
		if _, e := ReadAnalyticBodies(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func FuzzSphericalZoneSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/spherical-zone.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024, MaxValues: 8192, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}

func TestSphericalZoneAsymmetricRotated(t *testing.T) {
	m := zoneModel(t)
	frame, e := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 5}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	m.Bodies[0].Frame = frame
	m.Bodies[0].LowerLatitude = 0
	var b bytes.Buffer
	if e = WriteSphericalZones(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := got.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	want := frame.ToWorld(geom.Vec3{Z: 21. / 44})
	if e != nil || math.Abs(p.Volume-11*math.Pi/3) > 1e-7 || math.Abs(p.Area-11*math.Pi) > 1e-7 || p.Centroid.Distance(want) > 1e-8 {
		t.Fatal(p, want, e)
	}
}
