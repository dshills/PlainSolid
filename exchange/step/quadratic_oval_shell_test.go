package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func ovalShellFixture(t *testing.T) QuadraticOvalOpenShell {
	m := quadraticFaceFixture(t)
	annulus := m.Faces[0]
	disk := QuadraticOvalFace{Name: "Disk", Geometry: annulus.Geometry, Outer: annulus.Holes[0]}
	return QuadraticOvalOpenShell{Annulus: annulus, Disk: disk, LengthUnit: .001}
}
func TestQuadraticOvalConnectedShell(t *testing.T) {
	m := ovalShellFixture(t)
	var b bytes.Buffer
	if e := WriteQuadraticOvalOpenShell(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadQuadraticOvalOpenShell(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.SourceSharedEdge == 0 || len(got.Annulus.SourceEdges) != 2 || len(got.Disk.SourceEdges) != 1 || got.Disk.SourceEdges[0] != got.SourceSharedEdge {
		t.Fatal(got)
	}
	if _, e = ReadQuadraticOvalFaces(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("standalone face reader flattened shell")
	}
	m.Disk.Outer.Center.X += .0625
	var bad bytes.Buffer
	if e = WriteQuadraticOvalOpenShell(&bad, m); e == nil || bad.Len() != 0 {
		t.Fatal("different disk accepted")
	}
	// A distinct source support chart is retained while the common model rim agrees.
	m = ovalShellFixture(t)
	d := m.Disk.Geometry.Data()
	for i := range d.Poles {
		d.Poles[i] = d.Poles[i].Scale(2)
	}
	m.Disk.Geometry, e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	m.Disk.Outer.Center = geom.Vec2{X: .25, Y: .25}
	m.Disk.Outer.Radii = geom.Vec2{X: .0625, Y: .0625}
	b.Reset()
	if e = WriteQuadraticOvalOpenShell(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadQuadraticOvalOpenShell(&b, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func TestOCCTQuadraticOvalConnectedShell(t *testing.T) {
	m := ovalShellFixture(t)
	var out bytes.Buffer
	if e := WriteQuadraticOvalOpenShell(&out, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/shell.step"
	if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	result := t.TempDir() + "/mass.json"
	stepOracle(t, result, "step-mass-precise", path)
	raw, e := os.ReadFile(result)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct{ Area float64 }
	if e = json.Unmarshal(raw, &mass); e != nil {
		t.Fatal(e)
	}
	m.Annulus.Holes = nil
	out.Reset()
	if e = WriteQuadraticOvalFaces(&out, QuadraticOvalFaceModel{LengthUnit: .001, Faces: []QuadraticOvalFace{m.Annulus}}); e != nil {
		t.Fatal(e)
	}
	single := t.TempDir() + "/single.step"
	if e = os.WriteFile(single, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	stepOracle(t, result, "step-mass-precise", single)
	raw, e = os.ReadFile(result)
	if e != nil {
		t.Fatal(e)
	}
	var expected struct{ Area float64 }
	if e = json.Unmarshal(raw, &expected); e != nil {
		t.Fatal(e)
	}
	if mass.Area <= 0 || math.Abs(mass.Area-expected.Area) > 1e-8 {
		t.Fatal(mass, expected)
	}
}

func TestIndependentQuadraticConnectedShell(t *testing.T) {
	raw, e := os.ReadFile("testdata/quadratic-connected-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadQuadraticOvalOpenShell(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Disk.Outer.Center != (geom.Vec2{X: .25, Y: .25}) || m.Annulus.Holes[0].Center != (geom.Vec2{X: .5, Y: .5}) {
		t.Fatal("charts lost")
	}
	var out bytes.Buffer
	if e = WriteQuadraticOvalOpenShell(&out, m); e != nil {
		t.Fatal(e)
	}
}

func TestQuadraticConnectedShellReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/quadratic-connected-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"same-use", "extra-shell", "wrong-owner", "unshared-rim"} {
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
				change := false
				switch {
				case kind == "same-use" && r.Type == "ORIENTED_EDGE" && r.Args[4].Text == "F":
					r.Args[4] = Value{Kind: Enumeration, Text: "T"}
					change = true
				case kind == "extra-shell" && r.Type == "OPEN_SHELL":
					en.ID = 999999
					f.Entities[en.ID] = en
					f.Order = append(f.Order, en.ID)
					change = true
				case kind == "wrong-owner" && r.Type == "SURFACE_CURVE" && len(r.Args[2].Items) == 2:
					r.Args[2] = Value{Kind: Aggregate, Items: []Value{r.Args[2].Items[0], r.Args[2].Items[0]}}
					change = true
				case kind == "unshared-rim" && r.Type == "ORIENTED_EDGE" && r.Args[4].Text == "F":
					eid := r.Args[3].Ref
					copy := f.Entities[eid]
					copy.ID = 999999
					f.Entities[copy.ID] = copy
					f.Order = append(f.Order, copy.ID)
					r.Args[3] = Value{Kind: Reference, Ref: copy.ID}
					change = true
				}
				if change {
					if kind != "extra-shell" {
						f.Entities[id] = Entity{ID: id, Records: []Record{r}}
					}
					break
				}
			}
			if _, e = ReadQuadraticOvalOpenShell(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("accepted", kind)
			}
		})
	}
}
func TestOCCTIndependentConnectedShell(t *testing.T) {
	source := "testdata/quadratic-connected-shell.step"
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-surface", source)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var q struct{ Faces int }
	if e = json.Unmarshal(raw, &q); e != nil || q.Faces != 2 {
		t.Fatal(q, e)
	}
	raw, e = os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadQuadraticOvalOpenShell(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteQuadraticOvalOpenShell(&b, m); e != nil {
		t.Fatal(e)
	}
	native := t.TempDir() + "/native.step"
	if e = os.WriteFile(native, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	areas := [2]float64{}
	for i, path := range []string{source, native} {
		stepOracle(t, out, "step-mass-precise", path)
		raw, e = os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var p struct{ Area float64 }
		if e = json.Unmarshal(raw, &p); e != nil {
			t.Fatal(e)
		}
		areas[i] = p.Area
	}
	if areas[0] <= 0 || math.Abs(areas[0]-areas[1]) > 1e-8 {
		t.Fatal(areas)
	}
}

func FuzzQuadraticOvalOpenShell(f *testing.F) {
	if raw, e := os.ReadFile("testdata/quadratic-connected-shell.step"); e == nil {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = ReadQuadraticOvalOpenShell(bytes.NewReader(raw), Limits{MaxBytes: 65536, MaxEntities: 300, MaxVertices: 200, MaxFaces: 4, MaxEdgeUses: 8, MaxDepth: 24})
	})
}
