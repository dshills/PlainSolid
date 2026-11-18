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

func tubeModel(t *testing.T) TubeModel {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return TubeModel{LengthUnit: .001, Bodies: []TubeBody{{"tube", f, 3, 1, 4}}}
}
func checkTube(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-32*math.Pi) > 1e-7 || math.Abs(p.Area-48*math.Pi) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 5}) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestTubeRoundtrip(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteTubes(&b, tubeModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkTube(t, m)
	if _, e := ReadTrimmedBodies(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestOCCTTube(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tube.step")
	stepOracle(t, path, "write-step-tube")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkTube(t, m)
	var b bytes.Buffer
	if e = WriteTubes(&b, tubeModel(t)); e != nil {
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
	var p struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-32*math.Pi) > 1e-6 || math.Abs(p.Area-48*math.Pi) > 1e-6 {
		t.Fatal(p, e)
	}
}

func TestTubeMalformedGraphs(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer
	if e := WriteTubes(&raw, tubeModel(t)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"inner-sense", "missing-hole", "wrong-radius"} {
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
			if kind == "wrong-radius" && r.Type == "CYLINDRICAL_SURFACE" && r.Args[2].Text == "1." {
				r.Args[2].Text = "2."
				changed = true
			}
			if r.Type == "ADVANCED_FACE" {
				sid, _ := ref(r.Args[2])
				typ := f.Entities[sid].Records[0].Type
				if kind == "inner-sense" && typ == "CYLINDRICAL_SURFACE" && r.Args[3].Text == "F" {
					r.Args[3].Text = "T"
					changed = true
				}
				if kind == "missing-hole" && typ == "PLANE" {
					r.Args[1].Items = r.Args[1].Items[:1]
					changed = true
				}
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
func TestIndependentTubeFixture(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/tube.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkTube(t, m)
}
func TestTubeWriterInvalid(t *testing.T) {
	t.Parallel()
	for _, r := range []float64{0, 3, 4, math.NaN()} {
		m := tubeModel(t)
		m.Bodies[0].InnerRadius = r
		var b bytes.Buffer
		if e := WriteTubes(&b, m); e == nil || b.Len() != 0 {
			t.Fatal(r, e)
		}
	}
	if e := WriteTubes(failWriter{}, tubeModel(t)); e == nil {
		t.Fatal("writer failure")
	}
}

func FuzzTubeSTEP(f *testing.F) {
	raw, e := os.ReadFile("testdata/tube.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
