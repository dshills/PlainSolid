package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func torusModel(t *testing.T) TorusModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return TorusModel{LengthUnit: .001, Bodies: []TorusBody{{Name: "torus", Frame: f, MajorRadius: 3, MinorRadius: 1}}}
}
func checkTorus(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-6*math.Pi*math.Pi) > 1e-6 || math.Abs(p.Area-12*math.Pi*math.Pi) > 1e-6 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 3}) > 1e-8 {
		t.Fatal(p)
	}
}
func TestTorusRoundtrip(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if e := WriteTori(&b, torusModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkTorus(t, m)
}
func TestOCCTTorusImport(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "torus.step")
	stepOracle(t, p, "write-step-torus")
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	checkTorus(t, m)
}
func TestOCCTTorusExport(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "torus.step")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteTori(f, torusModel(t))
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, p)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if math.Abs(got.Area-12*math.Pi*math.Pi) > 1e-6 || math.Abs(got.Volume-6*math.Pi*math.Pi) > 1e-6 || math.Abs(got.Centroid[0]-1) > 1e-8 || math.Abs(got.Centroid[1]-2) > 1e-8 || math.Abs(got.Centroid[2]-3) > 1e-8 {
		t.Fatal(got)
	}
}

func TestTorusSeamRejections(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer
	if e := WriteTori(&raw, torusModel(t)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"pcurve_offset", "circle_radius", "edge_sense", "use_sense", "master", "trim_count"} {
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
			r := &ent.Records[0]
			if r.Type == "MANIFOLD_SOLID_BREP" {
				body = id
			}
			if !changed {
				switch {
				case kind == "pcurve_offset" && r.Type == "CARTESIAN_POINT" && len(r.Args[1].Items) == 2:
					r.Args[1].Items[0] = Value{Kind: Number, Text: "0.25"}
					changed = true
				case kind == "circle_radius" && r.Type == "CIRCLE":
					r.Args[2] = Value{Kind: Number, Text: "4.5"}
					changed = true
				case kind == "edge_sense" && r.Type == "EDGE_CURVE":
					r.Args[4] = Value{Kind: Enumeration, Text: "F"}
					changed = true
				case kind == "use_sense" && r.Type == "ORIENTED_EDGE":
					r.Args[4] = Value{Kind: Enumeration, Text: "T"}
					changed = true
				case kind == "master" && r.Type == "SEAM_CURVE":
					r.Args[3] = Value{Kind: Enumeration, Text: "CURVE_3D"}
					changed = true
				case kind == "trim_count" && r.Type == "EDGE_LOOP":
					r.Args[1].Items = r.Args[1].Items[:3]
					changed = true
				}
			}
			f.Entities[id] = ent
		}
		if !changed || body == 0 {
			t.Fatal("mutation missed", kind)
		}
		if _, e = analyticBody(f, body); e == nil {
			t.Fatal("accepted", kind)
		}
	}
}
func TestTorusWriterFailures(t *testing.T) {
	t.Parallel()
	m := torusModel(t)
	for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
		if e := WriteTori(w, m); e == nil {
			t.Fatal("writer")
		}
	}
	m.Bodies[0].MinorRadius = 3
	if e := WriteTori(io.Discard, m); e == nil {
		t.Fatal("horn torus")
	}
}
func FuzzReadTorus(f *testing.F) {
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	var b bytes.Buffer
	_ = WriteTori(&b, TorusModel{LengthUnit: 1, Bodies: []TorusBody{{Frame: frame, MajorRadius: 3, MinorRadius: 1}}})
	f.Add(b.String())
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadAnalyticBodies(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxVertices: 100, MaxFaces: 100, MaxSolids: 10, MaxEdgeUses: 100})
	})
}
