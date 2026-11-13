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

func checkCylinder(t *testing.T, m AnalyticModel) {
	t.Helper()
	if len(m.Bodies) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(p.Volume-16*math.Pi) > 1e-6 || math.Abs(p.Area-24*math.Pi) > 1e-6 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 5}) > 1e-8 {
		t.Fatal(p)
	}
}
func TestOCCTCylinderImport(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cylinder.step")
	stepOracle(t, p, "write-step-cylinder")
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	checkCylinder(t, m)
}

func cylinderModel(t *testing.T) CylinderModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return CylinderModel{LengthUnit: .001, Bodies: []CylinderBody{{Name: "cylinder", Frame: f, Radius: 2, Height: 4}}}
}
func TestCylinderRoundtrip(t *testing.T) {
	var b bytes.Buffer
	if e := WriteCylinders(&b, cylinderModel(t)); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	checkCylinder(t, m)
}
func TestOCCTCylinderExport(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cylinder.step")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	e = WriteCylinders(f, cylinderModel(t))
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
	if math.Abs(got.Area-24*math.Pi) > 1e-6 || math.Abs(got.Volume-16*math.Pi) > 1e-6 || math.Abs(got.Centroid[0]-1) > 1e-8 || math.Abs(got.Centroid[1]-2) > 1e-8 || math.Abs(got.Centroid[2]-5) > 1e-8 {
		t.Fatal(got)
	}
}
func TestCylinderMalformedGraphs(t *testing.T) {
	var raw bytes.Buffer
	if e := WriteCylinders(&raw, cylinderModel(t)); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"circle_radius", "cap_uv", "seam_sense", "face_sense", "loop_count", "cylinder_radius"} {
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
				case kind == "circle_radius" && r.Type == "CIRCLE":
					r.Args[2] = Value{Kind: Number, Text: "2.5"}
					changed = true
				case kind == "cap_uv" && r.Type == "AXIS2_PLACEMENT_2D":
					r.Args[2] = Value{Kind: Omitted}
					changed = true
				case kind == "seam_sense" && r.Type == "EDGE_CURVE":
					r.Args[4] = Value{Kind: Enumeration, Text: "F"}
					changed = true
				case kind == "face_sense" && r.Type == "ADVANCED_FACE":
					r.Args[3] = Value{Kind: Enumeration, Text: "F"}
					changed = true
				case kind == "loop_count" && r.Type == "EDGE_LOOP":
					r.Args[1].Items = r.Args[1].Items[:1]
					changed = true
				case kind == "cylinder_radius" && r.Type == "CYLINDRICAL_SURFACE":
					r.Args[2] = Value{Kind: Number, Text: "1."}
					changed = true
				}
			}
			f.Entities[id] = ent
		}
		if !changed || body == 0 {
			t.Fatal(kind)
		}
		if _, e = analyticBody(f, body); e == nil {
			t.Fatal("accepted", kind)
		}
	}
	for _, l := range []Limits{{MaxFaces: 2}, {MaxEdgeUses: 11}, {MaxVertices: 1}} {
		if _, e := ReadAnalyticBodies(bytes.NewReader(raw.Bytes()), l); e == nil {
			t.Fatal("limit", l)
		}
	}
	for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
		if e := WriteCylinders(w, cylinderModel(t)); e == nil {
			t.Fatal("writer")
		}
	}
}
func FuzzReadCylinder(f *testing.F) {
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	var b bytes.Buffer
	_ = WriteCylinders(&b, CylinderModel{LengthUnit: 1, Bodies: []CylinderBody{{Frame: frame, Radius: 2, Height: 4}}})
	f.Add(b.String())
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadAnalyticBodies(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxVertices: 100, MaxFaces: 100, MaxSolids: 10, MaxEdgeUses: 100})
	})
}

func TestCylinderRotatedFrame(t *testing.T) {
	frame, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1, Y: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	model := CylinderModel{LengthUnit: 1, Bodies: []CylinderBody{{Frame: frame, Radius: 2, Height: 4}}}
	var b bytes.Buffer
	if e = WriteCylinders(&b, model); e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-16*math.Pi) > 1e-6 || p.Centroid.Distance(frame.Origin().Add(frame.ZAxis().Scale(2))) > 1e-8 {
		t.Fatal(p, e)
	}
}
