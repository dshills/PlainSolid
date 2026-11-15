package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func convexArcFixture(t *testing.T) (ConvexArcExtrusionModel, trimmed.Solid) {
	t.Helper()
	s, e := brep.Prism([]geom.Vec3{{}, {X: 4}, {Y: 3}}, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	edge := -1
	v := s.Vertices()
	for i, q := range s.Edges() {
		if v[q.Start].X == 4 && v[q.End].X == 4 {
			edge = i
			break
		}
	}
	native, h, e := trimmed.FilletConvexPrismEdges(context.Background(), s, []trimmed.ConvexPrismEdgeRadius{{Edge: edge, Radius: 5.0 / 16}}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	return ConvexArcExtrusionModel{Body: ConvexArcExtrusionBody{Name: "original 3-4-5 fillet", Frame: h.Section.Frame, Height: h.Section.Height, Segments: h.Section.Segments}, LengthUnit: .001}, native
}
func TestConvexArcExtrusionSTEP(t *testing.T) {
	m, original := convexArcFixture(t)
	for _, normal := range []geom.Vec3{{Z: 1}, {X: -1}} {
		m.Body.Frame, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: 1}, normal)
		if _, e := BuildConvexArcExtrusion(m.Body); e != nil {
			t.Fatal("build", e)
		}
		var raw bytes.Buffer
		if e := writeConvexArcExtrusion(&raw, m); e != nil {
			t.Fatal("raw", e)
		}
		var b bytes.Buffer
		if e := WriteConvexArcExtrusion(&b, m); e != nil {
			t.Fatal("write", e)
		}
		got, e := ReadConvexArcExtrusion(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal("read", e)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatal("recipe", got, m)
		}
		var again bytes.Buffer
		if e = WriteConvexArcExtrusion(&again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
			t.Fatal("canonical", e)
		}
		built, e := BuildConvexArcExtrusion(got.Body)
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, built.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		props, e := reload.MassProperties(context.Background(), parametric.IntegrationOptions{})
		want, e2 := original.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || e2 != nil || math.Abs(props.Volume-want.Volume) > 1e-8 || math.Abs(props.Area-want.Area) > 1e-8 {
			t.Fatal(props, want, e, e2)
		}
	}
}

func TestConvexArcExtrusionReject(t *testing.T) {
	m, _ := convexArcFixture(t)
	var good bytes.Buffer
	if e := WriteConvexArcExtrusion(&good, m); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"CYLINDRICAL_SURFACE", "PCURVE", "EDGE_CURVE", "PLANE", "PRODUCT", "orphan"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(good.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range f.Order {
				r := &f.Entities[id].Records[0]
				if r.Type != kind {
					continue
				}
				switch kind {
				case "CYLINDRICAL_SURFACE":
					r.Args[2].Text = "2."
				case "PCURVE":
					r.Args[1].Ref = r.Args[2].Ref
				case "EDGE_CURVE":
					r.Args[4].Text = "F"
				case "PRODUCT":
					r.Args[1].Text = "changed metadata"
				case "PLANE":
					r.Type = "CYLINDRICAL_SURFACE"
				}
				break
			}
			if kind == "orphan" {
				id := f.Order[len(f.Order)-1] + 1
				f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "DIRECTION", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "1."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}}}}}}
				f.Order = append(f.Order, id)
			}
			raw, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadConvexArcExtrusion(bytes.NewReader(raw), Limits{})
			if e == nil || got.Body.Segments != nil {
				t.Fatal("accepted tamper", got, e)
			}
		})
	}
	if _, e := ReadConvexArcExtrusion(bytes.NewReader(good.Bytes()), Limits{MaxBytes: 64}); e == nil {
		t.Fatal("limit")
	}
	shifted := m
	shifted.Body.Frame, _ = geom.NewFrame(geom.Vec3{X: 8, Y: 16, Z: 32}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1})
	var rejected bytes.Buffer
	if e := WriteConvexArcExtrusion(&rejected, shifted); e == nil || rejected.Len() != 0 {
		t.Fatal("unrepresentable native placement must not round", e)
	}
	m.Body.Segments[1].Sweep = -m.Body.Segments[1].Sweep
	var out bytes.Buffer
	if e := WriteConvexArcExtrusion(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("atomic invalid profile", e)
	}
}
func TestConvexArcExtrusionLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CONVEX_ARC_STEP")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_CONVEX_ARC_STEP to the pinned read comparator")
	}
	m, _ := convexArcFixture(t)
	for _, normal := range []geom.Vec3{{Z: 1}, {X: -1}} {
		m.Body.Frame, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: 1}, normal)
		var b bytes.Buffer
		if e := WriteConvexArcExtrusion(&b, m); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "fillet.step")
		if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out, e := exec.Command(bin, "read", path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(out))
		}
		at := bytes.LastIndex(out, []byte("{\"Volume\""))
		if at < 0 {
			t.Fatal(string(out))
		}
		var q struct {
			Volume, Area  float64
			Faces, Solids int
		}
		if e = json.Unmarshal(bytes.TrimSpace(out[at:]), &q); e != nil {
			t.Fatal(e, string(out))
		}
		r := 5.0 / 16
		w := math.Atan2(3, -4)
		area := 6 - r*r*(3-w/2)
		perimeter := 12 - 6*r + r*w
		if q.Solids != 1 || q.Faces != 9 || math.Abs(q.Volume-5*area) > 1e-8 || math.Abs(q.Area-(2*area+5*perimeter)) > 1e-8 {
			t.Fatal(q, area, perimeter)
		}
	}
}

func TestConvexArcExtrusionTwoFillets(t *testing.T) {
	stock, e := brep.Prism([]geom.Vec3{{}, {X: 4}, {Y: 3}}, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	v := stock.Vertices()
	var req []trimmed.ConvexPrismEdgeRadius
	for i, q := range stock.Edges() {
		a, b := v[q.Start], v[q.End]
		if a.X == b.X && a.Y == b.Y && a.Y == 0 {
			radius := .25
			if a.X == 4 {
				radius = 5.0 / 16
			}
			req = append(req, trimmed.ConvexPrismEdgeRadius{Edge: i, Radius: radius})
		}
	}
	_, h, e := trimmed.FilletConvexPrismEdges(context.Background(), stock, req, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m := ConvexArcExtrusionModel{LengthUnit: .001, Body: ConvexArcExtrusionBody{Name: "two independently retained fillets", Frame: h.Section.Frame, Height: h.Section.Height, Segments: h.Section.Segments}}
	var b bytes.Buffer
	if e = WriteConvexArcExtrusion(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadConvexArcExtrusion(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(m, got) {
		t.Fatal(e, got)
	}
}
func FuzzConvexArcExtrusion(f *testing.F) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	q := QuarterSectorExtrusionBody{Frame: frame, Radius: 2, Height: 3}
	segments, _ := q.Segments()
	m := ConvexArcExtrusionModel{LengthUnit: .001, Body: ConvexArcExtrusionBody{Name: "seed", Frame: frame, Height: 3, Segments: segments}}
	var b bytes.Buffer
	if e := WriteConvexArcExtrusion(&b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		m, e := ReadConvexArcExtrusion(bytes.NewReader(raw), Limits{MaxBytes: 1 << 20, MaxEntities: 8192, MaxValues: 65536})
		if e != nil && m.Body.Segments != nil {
			t.Fatal("partial failure")
		}
		if e == nil {
			if _, e = BuildConvexArcExtrusion(m.Body); e != nil {
				t.Fatal(e)
			}
		}
	})
}
