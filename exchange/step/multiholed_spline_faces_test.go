package step

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func multiHoledSTEP(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/multiholed-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestMultiHoledSplineFaces(t *testing.T) {
	raw := multiHoledSTEP(t)
	m, e := ReadMultiHoledSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil || len(m.Faces) != 1 || len(m.Faces[0].Holes) != 2 {
		t.Fatal(m, e)
	}
	want := []geom.ParameterRectangle{{UMin: .3125, UMax: .75, VMin: .125, VMax: .375}, {UMin: .3125, UMax: .75, VMin: .625, VMax: .875}}
	if !reflect.DeepEqual(m.Faces[0].Holes, want) {
		t.Fatal(m.Faces[0])
	}
	var b bytes.Buffer
	if e = WriteMultiHoledSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	r, e := ReadMultiHoledSplineFaces(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(r.Faces[0].Holes, want) || !reflect.DeepEqual(r.Faces[0].Geometry.Data(), m.Faces[0].Geometry.Data()) {
		t.Fatal(r, e)
	}
	if _, e = ReadHoledSplineFaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("single-hole API widened")
	}
	if _, e = ReadBoundedSplineFaces(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("hole-free API widened")
	}
	for _, l := range []Limits{{MaxEdgeUses: 11}, {MaxVertices: 3}, {MaxEntities: 10}, {MaxBytes: 100}} {
		if _, e = ReadMultiHoledSplineFaces(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	for _, mutation := range [][2]string{{"#85 = FACE_BOUND('',#86,.F.)", "#85 = FACE_BOUND('',#85,.F.)"}, {"#85 = FACE_BOUND('',#86,.F.)", "#85 = FACE_BOUND('',#86,.T.)"}, {"(#18,#85,#147)", "(#18,#85,#85)"}, {"(20.,0.25,1.25)", "(20.,0.25,1.2500000000000002)"}} {
		bad := strings.Replace(string(raw), mutation[0], mutation[1], 1)
		if bad == string(raw) {
			t.Fatal("missed mutation")
		}
		if _, e = ReadMultiHoledSplineFaces(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal(mutation)
		}
	}
}
func TestMultiHoledSplineSeparation(t *testing.T) {
	raw := multiHoledSTEP(t)
	m, e := ReadMultiHoledSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	plane, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	first := geom.ParameterRectangle{UMin: .125, UMax: .5, VMin: .125, VMax: .5}
	for _, second := range []geom.ParameterRectangle{{UMin: .5, UMax: .75, VMin: .125, VMax: .5}, {UMin: .5, UMax: .75, VMin: .5, VMax: .75}, {UMin: .375, UMax: .75, VMin: .375, VMax: .75}, {UMin: .25, UMax: .375, VMin: .25, VMax: .375}, {UMin: .625, UMax: 1, VMin: .625, VMax: .875}} {
		bad := m
		bad.Faces = append([]MultiHoledSplineFace(nil), m.Faces...)
		bad.Faces[0].Holes = []geom.ParameterRectangle{first, second}
		var b bytes.Buffer
		if e := WriteMultiHoledSplineFaces(&b, bad); e == nil || b.Len() != 0 {
			t.Fatal(second, e)
		}
		// Build valid exact boundary geometry without the public containment
		// guard, so the independent reader must reject the invalid topology.
		unsafe := BoundedSplineFaceModel{LengthUnit: .001, Faces: []BoundedSplineFace{{Geometry: plane, Domain: geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}}}}
		if e := writeSplineRectangles(&b, unsafe, [][]geom.ParameterRectangle{{first, second}}); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadMultiHoledSplineFaces(&b, Limits{}); e == nil {
			t.Fatal("reader accepted contact/nesting", second)
		}

	}
	m.Faces[0].Holes = make([]geom.ParameterRectangle, MaxSplineFaceHoles+1)
	if e := WriteMultiHoledSplineFaces(&bytes.Buffer{}, m); e == nil {
		t.Fatal("hole count")
	}
}
func TestOCCTMultiHoledSplineFace(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-multiholed-spline-face")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadMultiHoledSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil || len(m.Faces[0].Holes) != 2 {
		t.Fatal(m, e)
	}
	var b bytes.Buffer
	if e = WriteMultiHoledSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	reference, got := stepHoledArea(t, source), stepHoledArea(t, path)
	if math.Abs(reference-got) > 1e-8 {
		t.Fatal(reference, got)
	}
}
func FuzzMultiHoledSplineFace(f *testing.F) {
	f.Add(multiHoledSTEP(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadMultiHoledSplineFaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 4096, MaxValues: 32768, MaxDepth: 24, MaxVertices: 256, MaxFaces: 8, MaxSolids: 8, MaxEdgeUses: 256})
	})
}
func BenchmarkMultiHoledSplineFace(b *testing.B) {
	raw := multiHoledSTEP(b)
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := ReadMultiHoledSplineFaces(bytes.NewReader(raw), Limits{}); e != nil {
			b.Fatal(e)
		}
	}
}
func BenchmarkPreflightSplineAssembly(b *testing.B) {
	raw, e := os.ReadFile("testdata/colored-spline-assembly.step")
	if e != nil {
		b.Fatal(e)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := Preflight(bytes.NewReader(raw), Limits{}); e != nil {
			b.Fatal(e)
		}
	}
}
