package step

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func orthogonalHoledModel(t testing.TB) OrthogonalHoledSplineFaceModel {
	t.Helper()
	g, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 4}, {X: 4}, {X: 4, Y: 4, Z: 1}}, []float64{0, 0, 4, 4}, []float64{0, 0, 4, 4}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return OrthogonalHoledSplineFaceModel{LengthUnit: 1, Faces: []OrthogonalHoledSplineFace{{Name: "concave holes", Geometry: g, Boundary: []geom.Vec2{{}, {X: 4}, {X: 4, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 4}, {Y: 4}}, Holes: [][]geom.Vec2{{{X: .5, Y: .5}, {X: 1.5, Y: .5}, {X: 1.5, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 1.5}, {X: .5, Y: 1.5}}, {{X: 2.5, Y: .5}, {X: 3.5, Y: .5}, {X: 3.5, Y: 1.5}, {X: 2.5, Y: 1.5}}}}}}
}
func TestIndependentOrthogonalHoledSplineFace(t *testing.T) {
	raw, e := os.ReadFile("testdata/orthogonal-holed-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalHoledSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Faces) != 1 || len(m.Faces[0].Boundary) != 6 || len(m.Faces[0].Holes) != 2 {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteOrthogonalHoledSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadOrthogonalHoledSplineFaces(&b, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func TestOrthogonalHoledRoundtrip(t *testing.T) {
	m := orthogonalHoledModel(t)
	var b bytes.Buffer
	if e := WriteOrthogonalHoledSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	got, e := ReadOrthogonalHoledSplineFaces(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Faces) != 1 || got.Faces[0].Name != m.Faces[0].Name || !reflect.DeepEqual(got.Faces[0].Boundary, m.Faces[0].Boundary) || !reflect.DeepEqual(got.Faces[0].Holes, m.Faces[0].Holes) || !reflect.DeepEqual(got.Faces[0].Geometry.Data(), m.Faces[0].Geometry.Data()) {
		t.Fatal(got)
	}
	if _, e = ReadOrthogonalSplineFaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old reader accepted holes")
	}
	for _, l := range []Limits{{MaxEdgeUses: 15}, {MaxVertices: 3}} {
		if _, e = ReadOrthogonalHoledSplineFaces(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, ent := range f.Entities {
		if ent.Records[0].Type == "FACE_BOUND" {
			ent.Records[0].Args[2].Text = "F"
			f.Entities[id] = ent
			break
		}
	}
	var fid int64
	for id, ent := range f.Entities {
		if ent.Records[0].Type == "ADVANCED_FACE" {
			fid = id
		}
	}
	if _, _, e = decodeOrthogonalHoledSplineFace(f, fid, 100); e == nil {
		t.Fatal("reversed source hole accepted")
	}
	// Reversing both the bound orientation and its physical edge traversal
	// preserves the same clockwise hole and must remain accepted.
	for _, ent := range f.Entities {
		if ent.Records[0].Type != "FACE_BOUND" || ent.Records[0].Args[2].Text != "F" {
			continue
		}
		lid, _ := ref(ent.Records[0].Args[1])
		loop, _ := simple(f, lid, "EDGE_LOOP", 2)
		ids, _ := listRefs(loop.Args[1], 4)
		for _, eid := range ids {
			use, _ := simple(f, eid, "ORIENTED_EDGE", 5)
			sense, _ := boolean(use.Args[4])
			if sense {
				use.Args[4].Text = "F"
			} else {
				use.Args[4].Text = "T"
			}
			f.Entities[eid] = Entity{ID: eid, Records: []Record{use}}
		}
		for i, j := 0, len(loop.Args[1].Items)-1; i < j; i, j = i+1, j-1 {
			loop.Args[1].Items[i], loop.Args[1].Items[j] = loop.Args[1].Items[j], loop.Args[1].Items[i]
		}
		f.Entities[lid] = Entity{ID: lid, Records: []Record{loop}}
	}
	if _, _, e = decodeOrthogonalHoledSplineFace(f, fid, 100); e != nil {
		t.Fatal("equivalent reversed source", e)
	}

}
func TestOrthogonalHoledRejection(t *testing.T) {
	square := func(x, y, size float64) []geom.Vec2 {
		return []geom.Vec2{{X: x, Y: y}, {X: x + size, Y: y}, {X: x + size, Y: y + size}, {X: x, Y: y + size}}
	}
	for _, kind := range []string{"notch", "outer-touch", "nested", "overlap", "hole-touch", "clockwise", "missing", "too-many"} {
		t.Run(kind, func(t *testing.T) {
			m := orthogonalHoledModel(t)
			f := &m.Faces[0]
			switch kind {
			case "notch":
				f.Holes = [][]geom.Vec2{square(2.5, 2.5, .5)}
			case "outer-touch":
				f.Holes = [][]geom.Vec2{square(0, .5, .5)}
			case "nested":
				f.Holes = [][]geom.Vec2{square(.5, .5, 1), square(.75, .75, .25)}
			case "overlap":
				f.Holes = [][]geom.Vec2{square(.5, .5, 1), square(1, 1, 1)}
			case "hole-touch":
				f.Holes = [][]geom.Vec2{square(.5, .5, .5), square(1, 1, .5)}
			case "clockwise":
				p := f.Holes[0]
				for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
					p[i], p[j] = p[j], p[i]
				}
			case "missing":
				f.Holes = nil
			case "too-many":
				f.Holes = make([][]geom.Vec2, MaxSplineFaceHoles+1)
			}
			var b bytes.Buffer
			if e := WriteOrthogonalHoledSplineFaces(&b, m); e == nil || b.Len() != 0 {
				t.Fatal("invalid face emitted", e)
			}
			// Stage individually valid loops without the public containment guard,
			// then exercise the independent reader's whole-face proof.
			if kind == "notch" || kind == "outer-touch" || kind == "nested" || kind == "overlap" || kind == "hole-touch" {
				flat := OrthogonalSplineFaceModel{LengthUnit: 1, Faces: []OrthogonalSplineFace{{Geometry: f.Geometry, Boundary: f.Boundary}}}
				for _, h := range f.Holes {
					flat.Faces = append(flat.Faces, OrthogonalSplineFace{Geometry: f.Geometry, Boundary: h})
				}
				if e := writeOrthogonalSplineFaces(&b, flat, []int{len(flat.Faces)}); e != nil {
					t.Fatal(e)
				}
				if _, e := ReadOrthogonalHoledSplineFaces(&b, Limits{}); e == nil {
					t.Fatal("invalid source containment accepted")
				}
			}
		})
	}
}
func TestOCCTOrthogonalHoledSplineFace(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-orthogonal-holed-spline-face")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalHoledSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Faces) != 1 || len(m.Faces[0].Boundary) != 6 || len(m.Faces[0].Holes) != 2 {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteOrthogonalHoledSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	output := filepath.Join(t.TempDir(), "ours.step")
	if e = os.WriteFile(output, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	a, c := stepHoledArea(t, source), stepHoledArea(t, output)
	if math.Abs(a-c) > 1e-8 {
		t.Fatal(a, c)
	}
}
func FuzzOrthogonalHoledSplineFace(f *testing.F) {
	m := orthogonalHoledModel(f)
	var b bytes.Buffer
	if e := WriteOrthogonalHoledSplineFaces(&b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadOrthogonalHoledSplineFaces(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 1000, MaxVertices: 128, MaxEdgeUses: 128})
	})
}
