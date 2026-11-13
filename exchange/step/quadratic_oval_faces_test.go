package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func quadraticFaceFixture(t *testing.T) QuadraticOvalFaceModel {
	t.Helper()
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3}, {X: 2}, {X: 2, Y: 3}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	q := QuadraticOval{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: .25, Y: .25}, QuarterWeight: .75}
	h := q
	h.Radii = geom.Vec2{X: .125, Y: .125}
	return QuadraticOvalFaceModel{LengthUnit: .001, Faces: []QuadraticOvalFace{{Name: "Curved annulus", Geometry: s, Outer: q, Holes: []QuadraticOval{h}}}}
}
func TestQuadraticOvalSTEP(t *testing.T) {
	m := quadraticFaceFixture(t)
	var b bytes.Buffer
	if e := WriteQuadraticOvalFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadQuadraticOvalFaces(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Faces) != 1 || got.Faces[0].Name != m.Faces[0].Name || len(got.Faces[0].Holes) != 1 || got.Faces[0].Holes[0] != m.Faces[0].Holes[0] || len(got.Faces[0].SourceEdges) != 2 {
		t.Fatal(got)
	}
	for _, kind := range []string{"sense", "vertex", "owner", "master"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range f.Order {
				ent := f.Entities[id]
				if len(ent.Records) != 1 {
					continue
				}
				r := ent.Records[0]
				r.Args = append([]Value(nil), r.Args...)
				change := false
				switch {
				case kind == "sense" && r.Type == "ORIENTED_EDGE":
					r.Args[4] = Value{Kind: Enumeration, Text: "F"}
					change = true
				case kind == "vertex" && r.Type == "VERTEX_POINT":
					r.Args[1] = Value{Kind: Reference, Ref: 999999}
					change = true
				case kind == "master" && r.Type == "SURFACE_CURVE":
					r.Args[3] = Value{Kind: Enumeration, Text: "CURVE_3D"}
					change = true
				case kind == "owner" && r.Type == "EDGE_CURVE":
					ent.ID = 999999
					f.Entities[ent.ID] = ent
					f.Order = append(f.Order, ent.ID)
					change = true
				}
				if change {
					if kind != "owner" {
						f.Entities[id] = Entity{ID: id, Records: []Record{r}}
					}
					break
				}
			}
			if _, e = ReadQuadraticOvalFaces(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("malformed graph accepted")
			}
		})
	}
	if _, e = ReadQuadraticOvalFaces(bytes.NewReader(b.Bytes()), Limits{MaxEdgeUses: 1}); e == nil {
		t.Fatal("edge budget")
	}
}
func TestOCCTQuadraticOvalSTEP(t *testing.T) {
	var b bytes.Buffer
	if e := WriteQuadraticOvalFaces(&b, quadraticFaceFixture(t)); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/annulus.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-mass-precise", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	if m.Area <= 0 {
		t.Fatal(m)
	}
}

func TestIndependentQuadraticOvalSTEP(t *testing.T) {
	raw, e := os.ReadFile("testdata/quadratic-annulus.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadQuadraticOvalFaces(bytes.NewReader(raw), Limits{})
	if e != nil || len(m.Faces) != 1 || len(m.Faces[0].Holes) != 1 {
		t.Fatal(m, e)
	}
}

func TestOCCTQuarterOvalFixture(t *testing.T) {
	raw, e := os.ReadFile("testdata/quadratic-annulus-quarters.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedQuadraticOvalFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Faces) != 1 || len(m.Faces[0].SourceEdges) != 8 || len(m.Faces[0].Holes) != 1 {
		t.Fatal(m)
	}
	if _, e = ReadQuadraticOvalFaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old API accepted partition")
	}
	var out bytes.Buffer
	if e = WriteQuadraticOvalFaces(&out, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadQuadraticOvalFaces(&out, Limits{})
	if e != nil || got.Faces[0].Outer != m.Faces[0].Outer || got.Faces[0].Holes[0] != m.Faces[0].Holes[0] {
		t.Fatal(got, e)
	}
}

func TestOCCTQuadraticOvalBothDirections(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-quadratic-annulus")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedQuadraticOvalFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteQuadraticOvalFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	native := t.TempDir() + "/native.step"
	if e = os.WriteFile(native, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	var areas [3]float64
	for i, path := range []string{source, native, "testdata/quadratic-annulus.step"} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", path)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var mass struct{ Area float64 }
		if e = json.Unmarshal(raw, &mass); e != nil {
			t.Fatal(e)
		}
		areas[i] = mass.Area
	}
	if areas[0] <= 0 || math.Abs(areas[0]-areas[1]) > 1e-8 || math.Abs(areas[0]-areas[2]) > 1e-8 {
		t.Fatal(areas)
	}
}

func TestPartitionedOvalReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/quadratic-annulus-quarters.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"continuity", "pcurve-owner", "coefficient", "sense"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) != 1 {
					continue
				}
				r := en.Records[0]
				r.Args = append([]Value(nil), r.Args...)
				changed := false
				switch {
				case kind == "continuity" && r.Type == "EDGE_CURVE":
					r.Args[2] = r.Args[1]
					changed = true
				case kind == "pcurve-owner" && r.Type == "PCURVE":
					r.Args[1] = Value{Kind: Reference, Ref: 22}
					changed = true
				case kind == "sense" && r.Type == "ORIENTED_EDGE":
					r.Args[4] = Value{Kind: Enumeration, Text: "F"}
					changed = true
				case kind == "coefficient" && r.Type == "CARTESIAN_POINT" && id == 29:
					r.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Number, Text: "1.5000000000000002"}, {Kind: Number, Text: "2.25"}, {Kind: Number, Text: "0."}}}
					changed = true
				}
				if changed {
					f.Entities[id] = Entity{ID: id, Records: []Record{r}}
					break
				}
			}
			if _, e = ReadPartitionedQuadraticOvalFaces(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("accepted", kind)
			}
		})
	}
}

func FuzzQuadraticOvalFaces(f *testing.F) {
	for _, path := range []string{"testdata/quadratic-annulus.step", "testdata/quadratic-annulus-quarters.step"} {
		if raw, e := os.ReadFile(path); e == nil {
			f.Add(raw)
		}
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadPartitionedQuadraticOvalFaces(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 300, MaxVertices: 200, MaxFaces: 4, MaxEdgeUses: 32, MaxDepth: 16})
	})
}
