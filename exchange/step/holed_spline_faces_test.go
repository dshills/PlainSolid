package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func holedSplineFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/holed-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestHoledSplineFaceRoundtrip(t *testing.T) {
	raw := holedSplineFixture(t)
	m, e := ReadHoledSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil || len(m.Faces) != 1 {
		t.Fatal(e)
	}
	face := m.Faces[0]
	if face.Outer != (geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}) || face.Hole != (geom.ParameterRectangle{UMin: .3125, UMax: .75, VMin: .25, VMax: .75}) {
		t.Fatal(face)
	}
	m.Faces[0].Name = "Holed rational face"
	var b bytes.Buffer
	if e = WriteHoledSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	round, e := ReadHoledSplineFaces(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || len(round.Faces) != 1 || round.Faces[0].Name != m.Faces[0].Name || round.Faces[0].Outer != face.Outer || round.Faces[0].Hole != face.Hole || !reflect.DeepEqual(round.Faces[0].Geometry.Data(), face.Geometry.Data()) {
		t.Fatal(round, e)
	}
	second := m.Faces[0]
	second.Name = "Second face"
	m.Faces = append(m.Faces, second)
	var multi bytes.Buffer
	if e = WriteHoledSplineFaces(&multi, m); e != nil {
		t.Fatal(e)
	}
	multiple, e := ReadHoledSplineFaces(&multi, Limits{})
	if e != nil || len(multiple.Faces) != 2 || multiple.Faces[1].Name != "Second face" {
		t.Fatal(multiple, e)
	}
	if _, e = ReadBoundedSplineFaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old reader accepted hole")
	}
	if _, e = ReadBoundedSplineFaces(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("old reader accepted emitted hole")
	}
}
func TestHoledSplineFaceReject(t *testing.T) {
	raw := string(holedSplineFixture(t))
	for _, mutation := range [][2]string{{"#85 = FACE_BOUND('',#86,.F.)", "#85 = FACE_BOUND('',#86,.T.)"}, {"#17 = ADVANCED_FACE('',(#18,#85),#31,.T.)", "#17 = ADVANCED_FACE('',(#18,#85),#31,.F.)"}, {"#17 = ADVANCED_FACE('',(#18,#85),#31,.T.)", "#17 = ADVANCED_FACE('',(#18,#18),#31,.T.)"}, {"#87 = ORIENTED_EDGE('',*,*,#88,.F.)", "#87 = ORIENTED_EDGE('',*,*,#88,.T.)"}, {"#90 = CARTESIAN_POINT('',(20.,0.5,2.5))", "#90 = CARTESIAN_POINT('',(20.,0.5,2.5000000000000004))"}, {"#96 = CARTESIAN_POINT('',(20.,2.,10.))", "#96 = CARTESIAN_POINT('',(20.,2.,10.000000000000002))"}} {
		bad := strings.Replace(raw, mutation[0], mutation[1], 1)
		if bad == raw {
			t.Fatal("missed mutation", mutation)
		}
		if _, e := ReadHoledSplineFaces(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(mutation)
		}
	}
	m, e := ReadHoledSplineFaces(strings.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, hole := range []geom.ParameterRectangle{{UMin: 0, UMax: .75, VMin: .25, VMax: .75}, {UMin: -.1, UMax: .75, VMin: .25, VMax: .75}, {UMin: .25, UMax: .75, VMin: .25, VMax: .75}} {
		bad := m
		bad.Faces = append([]HoledSplineFace(nil), m.Faces...)
		bad.Faces[0].Hole = hole
		var b bytes.Buffer
		if e = WriteHoledSplineFaces(&b, bad); e == nil || b.Len() != 0 {
			t.Fatal(hole, e, b.Len())
		}
	}
	for _, l := range []Limits{{MaxEdgeUses: 7}, {MaxBytes: 100}, {MaxVertices: 3}, {MaxEntities: 10}} {
		if _, e = ReadHoledSplineFaces(strings.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func stepHoledArea(t *testing.T, path string) float64 {
	t.Helper()
	out := filepath.Join(t.TempDir(), "area.json")
	stepOracle(t, out, "step-surface", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(b, &r); e != nil || r.Faces != 1 {
		t.Fatal(r, e)
	}
	return r.Area
}
func TestOCCTHoledSplineFace(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-holed-spline-face")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadHoledSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteHoledSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	original, exported := stepHoledArea(t, source), stepHoledArea(t, path)
	if math.Abs(original-exported) > 1e-8 || math.Abs(original-342.9390573124855) > .003 {
		t.Fatal(original, exported)
	}
}
func FuzzHoledSplineFace(f *testing.F) {
	f.Add(holedSplineFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadHoledSplineFaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 256, MaxFaces: 8, MaxSolids: 8, MaxEdgeUses: 64})
	})
}
