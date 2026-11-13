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

func checkConical(t *testing.T, m AnalyticModel, top float64) {
	t.Helper()
	if len(m.Bodies) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	area := math.Pi*(2+top)*math.Hypot(4, 2-top) + math.Pi*(4+top*top)
	volume := 4 * math.Pi * (4 + 2*top + top*top) / 3
	z := 3 + 4*(4+4*top+3*top*top)/(4*(4+2*top+top*top))
	if math.Abs(p.Volume-volume) > 1e-6 || math.Abs(p.Area-area) > 1e-6 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: z}) > 1e-8 {
		t.Fatal(p, area, volume, z)
	}
}
func TestOCCTConicalImport(t *testing.T) {
	for _, mode := range []string{"write-step-cone", "write-step-frustum"} {
		p := filepath.Join(t.TempDir(), "cone.step")
		stepOracle(t, p, mode)
		f, e := os.Open(p)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(f, Limits{})
		_ = f.Close()
		if e != nil {
			t.Fatal(mode, e)
		}
		top := 0.0
		if mode == "write-step-frustum" {
			top = 1
		}
		checkConical(t, m, top)
	}
}

func coneModel(t *testing.T, top float64) ConeModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return ConeModel{LengthUnit: .001, Bodies: []ConeBody{{Name: "cone", Frame: f, BaseRadius: 2, TopRadius: top, Height: 4}}}
}
func TestConicalRoundtrip(t *testing.T) {
	for _, top := range []float64{0, 1} {
		var b bytes.Buffer
		if e := WriteCones(&b, coneModel(t, top)); e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(&b, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkConical(t, m, top)
	}
}
func TestOCCTConicalExport(t *testing.T) {
	for _, top := range []float64{0, 1} {
		p := filepath.Join(t.TempDir(), "cone.step")
		f, e := os.Create(p)
		if e != nil {
			t.Fatal(e)
		}
		e = WriteCones(f, coneModel(t, top))
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
		area := math.Pi*(2+top)*math.Hypot(4, 2-top) + math.Pi*(4+top*top)
		volume := 4 * math.Pi * (4 + 2*top + top*top) / 3
		z := 3 + 4*(4+4*top+3*top*top)/(4*(4+2*top+top*top))
		if math.Abs(got.Area-area) > 1e-6 || math.Abs(got.Volume-volume) > 1e-6 || math.Abs(got.Centroid[0]-1) > 1e-8 || math.Abs(got.Centroid[1]-2) > 1e-8 || math.Abs(got.Centroid[2]-z) > 1e-8 {
			t.Fatal(got)
		}
	}
}
func TestConicalFailures(t *testing.T) {
	for _, top := range []float64{0, 1} {
		var raw bytes.Buffer
		if e := WriteCones(&raw, coneModel(t, top)); e != nil {
			t.Fatal(e)
		}
		for _, kind := range []string{"angle", "generator", "uv_direction", "rim"} {
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
					case kind == "angle" && r.Type == "CONICAL_SURFACE":
						r.Args[3] = Value{Kind: Number, Text: "0.1"}
						changed = true
					case kind == "generator" && r.Type == "SEAM_CURVE":
						r.Args[3] = Value{Kind: Enumeration, Text: "CURVE_3D"}
						changed = true
					case kind == "uv_direction" && r.Type == "DIRECTION" && len(r.Args[1].Items) == 2:
						r.Args[1].Items[0] = Value{Kind: Number, Text: "1."}
						changed = true
					case kind == "rim" && r.Type == "CIRCLE":
						r.Args[2] = Value{Kind: Number, Text: "3."}
						changed = true
					}
				}
				f.Entities[id] = ent
			}
			if !changed {
				t.Fatal(kind)
			}
			if _, e = analyticBody(f, body); e == nil {
				t.Fatal("accepted", kind)
			}
		}
		for _, w := range []io.Writer{nil, failWriter{}, shortWriter{}} {
			if e := WriteCones(w, coneModel(t, top)); e == nil {
				t.Fatal("writer")
			}
		}
	}
}
func FuzzReadConical(f *testing.F) {
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, top := range []float64{0, 1} {
		var b bytes.Buffer
		_ = WriteCones(&b, ConeModel{LengthUnit: 1, Bodies: []ConeBody{{Frame: frame, BaseRadius: 2, TopRadius: top, Height: 4}}})
		f.Add(b.String())
	}
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ReadAnalyticBodies(strings.NewReader(s), Limits{MaxBytes: 65536, MaxEntities: 300, MaxValues: 3000, MaxDepth: 12, MaxVertices: 100, MaxFaces: 100, MaxSolids: 10, MaxEdgeUses: 100})
	})
}

func TestConicalRotatedAndSurfaceParameters(t *testing.T) {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1, Y: -1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, top := range []float64{0, 1} {
		model := coneModel(t, top)
		model.Bodies[0].Frame = f
		var b bytes.Buffer
		if e = WriteCones(&b, model); e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		z := 4 * (4 + 4*top + 3*top*top) / (4 * (4 + 2*top + top*top))
		if e != nil || p.Centroid.Distance(f.Origin().Add(f.ZAxis().Scale(z))) > 1e-8 {
			t.Fatal(p, e)
		}
		parsed, e := Parse(&b, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		for id, ent := range parsed.Entities {
			if len(ent.Records) != 1 || ent.Records[0].Type != "CONICAL_SURFACE" {
				continue
			}
			surface, e := DecodeAnalyticSurface(parsed, id)
			if e != nil {
				t.Fatal(e)
			}
			p, e := surface.Eval(0, -2)
			want := f.Origin().Add(f.ZAxis().Scale(2)).Add(f.XAxis().Scale(1 + top/2))
			if e != nil || p.Distance(want) > 1e-10 {
				t.Fatal(p, want, e)
			}
		}
	}
	var zero ConicalSurface
	if _, e = zero.Eval(0, 0); e == nil {
		t.Fatal("zero cone")
	}
}
