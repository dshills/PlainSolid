package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestIndependentSplineFace(t *testing.T) {
	raw, e := os.ReadFile("testdata/rectangular-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Faces) != 1 {
		t.Fatal(m)
	}
	for _, uv := range [][2]float64{{0, 0}, {.3, .4}, {1, 1}} {
		p, e := m.Faces[0].Geometry.Eval(uv[0], uv[1])
		if e != nil || math.Abs(math.Hypot(p.X, p.Y)-1) > 1e-10 || math.Abs(p.Z-2*uv[1]) > 1e-10 {
			t.Fatal(p, e)
		}
	}
}

func TestSplineFaceRoundtrip(t *testing.T) {
	raw, e := os.ReadFile("testdata/rectangular-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteSplineFaces(&out, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSplineFaces(&out, Limits{})
	if e != nil || len(got.Faces) != 1 {
		t.Fatal(got, e)
	}
}
func TestOCCTSplineFace(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-surface")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteSplineFaces(&out, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "ours.step")
	if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	result := filepath.Join(t.TempDir(), "result.json")
	stepOracle(t, result, "step-surface", path)
	raw, e = os.ReadFile(result)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Faces int
		Area  float64
	}
	if e := json.Unmarshal(raw, &p); e != nil || p.Faces != 1 || math.Abs(p.Area-math.Pi) > 1e-6 {
		t.Fatal(p, e)
	}
}

func TestSplineFaceMalformed(t *testing.T) {
	raw, e := os.ReadFile("testdata/rectangular-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"pole", "weight", "uv", "connectivity", "orientation"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mutate := func(id int64, fn func(*Record)) { ent := f.Entities[id]; fn(&ent.Records[0]); f.Entities[id] = ent }
			switch kind {
			case "pole":
				mutate(51, func(r *Record) { r.Args[1].Items[0] = Value{Kind: Number, Text: "1.1"} })
			case "weight":
				ent := f.Entities[49]
				for i, r := range ent.Records {
					if r.Type == "RATIONAL_B_SPLINE_CURVE" {
						r.Args[0].Items[1] = Value{Kind: Number, Text: "0.8"}
						ent.Records[i] = r
					}
				}
				f.Entities[49] = ent
			case "uv":
				mutate(71, func(r *Record) { r.Args[1].Items[0] = Value{Kind: Number, Text: "0.9"} })
			case "connectivity":
				mutate(76, func(r *Record) { r.Args[1] = Value{Kind: Reference, Ref: 22} })
			case "orientation":
				mutate(75, func(r *Record) { r.Args[4].Text = "T" })
			}
			if _, e := decodeSplineFace(f, 17, 100); e == nil {
				t.Fatal("accepted malformed face")
			}
		})
	}
	if _, e := ReadSplineFaces(bytes.NewReader(raw), Limits{MaxVertices: 5}); e == nil {
		t.Fatal("control budget")
	}
	if _, e := ReadSplineFaces(bytes.NewReader(raw), Limits{MaxEdgeUses: 3}); e == nil {
		t.Fatal("edge budget")
	}
}
func TestSplineFaceGraphAndFold(t *testing.T) {
	graph, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, []float64{2, 2, 5, 5}, []float64{-3, -3, 1, 1}, []float64{1, 2, 1, 2})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteSplineFaces(&out, SplineFaceModel{[]RectangularSplineFace{{"graph", graph}}, .001}); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadSplineFaces(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
	poles := []geom.Vec3{}
	for _, x := range []float64{0, 2, -1, 1} {
		poles = append(poles, geom.Vec3{X: x}, geom.Vec3{X: x, Y: 1})
	}
	fold, e := geom.NewBSplineSurface(3, 1, 4, 2, poles, []float64{0, 0, 0, 0, 1, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e := WriteSplineFaces(&out, SplineFaceModel{[]RectangularSplineFace{{"fold", fold}}, .001}); e == nil {
		t.Fatal("fold accepted")
	}
	if e := WriteSplineFaces(shortWriter{}, SplineFaceModel{[]RectangularSplineFace{{"graph", graph}}, .001}); e == nil {
		t.Fatal("short writer")
	}
}
func FuzzReadSplineFaces(f *testing.F) {
	raw, e := os.ReadFile("testdata/rectangular-spline-face.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadSplineFaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 64, MaxFaces: 4, MaxSolids: 4, MaxEdgeUses: 32})
	})
}

func TestSplineFaceRefinedBoundary(t *testing.T) {
	raw, e := os.ReadFile("testdata/rectangular-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, elevate := range []bool{false, true} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		f.Entities[1000] = Entity{ID: 1000, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "1"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "1"}}}}}}}
		ent := f.Entities[27]
		r := ent.Records[0]
		r.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: 28}, {Kind: Reference, Ref: 1000}, {Kind: Reference, Ref: 29}}}
		if elevate {
			r.Args[1] = Value{Kind: Number, Text: "2"}
			r.Args[6] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "3"}, {Kind: Number, Text: "3"}}}
		} else {
			r.Args[6] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "2"}, {Kind: Number, Text: "1"}, {Kind: Number, Text: "2"}}}
			r.Args[7] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0"}, {Kind: Number, Text: "0.5"}, {Kind: Number, Text: "1"}}}
		}
		ent.Records[0] = r
		f.Entities[27] = ent
		if _, e := decodeSplineFace(f, 17, 100); e != nil {
			t.Fatal(elevate, e)
		}
	}
}
