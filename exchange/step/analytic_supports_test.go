package step

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func mixedSupportFixture() AnalyticSupportModel {
	p := planeSupportFixture().Surfaces[0]
	c := cylinderSupportFixture().Surfaces[0]
	s := sphereSupportFixture().Surfaces[0]
	t := torusSupportFixture().Surfaces[0]
	return AnalyticSupportModel{Name: "mixed", SetName: "supports", LengthUnit: .3048, Surfaces: []AnalyticSupport{{Torus: &t}, {Plane: &p}, {Sphere: &s}, {Cylinder: &c}}}
}
func TestAnalyticSupportsRoundtrip(t *testing.T) {
	t.Parallel()
	m := mixedSupportFixture()
	var b bytes.Buffer
	if e := WriteAnalyticSupports(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticSupports(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, m) {
		t.Fatal(got, m)
	}
	var out bytes.Buffer
	if e = WriteAnalyticSupports(&out, got); e != nil || !bytes.Equal(out.Bytes(), b.Bytes()) {
		t.Fatal("canonical", e)
	}
	if _, e = ReadTorusSupports(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("homogeneous accepted mixed")
	}
	if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("solid accepted")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 5}, {MaxValues: 10}} {
		if _, e = ReadAnalyticSupports(bytes.NewReader(b.Bytes()), l); e == nil {
			t.Fatal("limit")
		}
	}
	got.Surfaces[0].Torus.MajorRadius = 99
	if m.Surfaces[0].Torus.MajorRadius == 99 {
		t.Fatal("borrowed input")
	}
}
func TestAnalyticSupportsOwnership(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	_ = WriteAnalyticSupports(&b, mixedSupportFixture())
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	_, set, _, roots, e := analyticSupportRoots(f)
	if e != nil {
		t.Fatal(e)
	}
	keep := map[int64]bool{}
	for id := range f.Entities {
		keep[id] = true
	}
	bad := append([]int64(nil), roots...)
	bad[1] = bad[0]
	if _, e = ReadAnalyticSupports(bytes.NewReader(analyticSupportSubset(f, keep, set, bad)), Limits{}); e == nil {
		t.Fatal("repeated root")
	}
	first := f.Entities[roots[0]].Records[0].Args[1]
	x := f.Entities[roots[1]]
	x.Records[0].Args[1] = first
	f.Entities[roots[1]] = x
	raw := analyticSupportSubset(f, keep, set, roots)
	if _, e = ReadAnalyticSupports(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("shared placement")
	}
	text := b.String()
	for _, mut := range [][2]string{{"TOROIDAL_SURFACE(", "UNSUPPORTED_SURFACE("}, {"SI_UNIT($,.RADIAN.)", "SI_UNIT(.MILLI.,.RADIAN.)"}, {"DATA;", "DATA;\n#999=CARTESIAN_POINT('',(0.,0.,0.));"}} {
		if !strings.Contains(text, mut[0]) {
			t.Fatal("stale mutation")
		}
		got, e := ReadAnalyticSupports(strings.NewReader(strings.Replace(text, mut[0], mut[1], 1)), Limits{})
		if e == nil || len(got.Surfaces) != 0 {
			t.Fatal("invalid graph accepted")
		}
	}
}
func TestAnalyticSupportsAtomicWriter(t *testing.T) {
	t.Parallel()
	m := mixedSupportFixture()
	m.Surfaces[3].Plane = m.Surfaces[1].Plane
	var b bytes.Buffer
	if e := WriteAnalyticSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("ambiguous variant")
	}
	m = mixedSupportFixture()
	m.Surfaces[3].Cylinder.Radius = -1
	if e := WriteAnalyticSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("partial invalid output")
	}
}
func FuzzAnalyticSupports(f *testing.F) {
	var b bytes.Buffer
	_ = WriteAnalyticSupports(&b, mixedSupportFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticSupports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxEntities: 64, MaxValues: 512, MaxDepth: 8})
	})
}

type mixedSupportShortWriter struct{}

func (mixedSupportShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestAnalyticSupportsBoundaries(t *testing.T) {
	t.Parallel()
	m := mixedSupportFixture()
	p := m.Surfaces[1].Plane
	m.Surfaces = nil
	for i := 0; i < 64; i++ {
		m.Surfaces = append(m.Surfaces, AnalyticSupport{Plane: p})
	}
	var b bytes.Buffer
	if e := WriteAnalyticSupports(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticSupports(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || len(got.Surfaces) != 64 {
		t.Fatal(len(got.Surfaces), e)
	}
	if e = WriteAnalyticSupports(mixedSupportShortWriter{}, mixedSupportFixture()); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m.Surfaces = append(m.Surfaces, AnalyticSupport{Plane: p})
	b.Reset()
	if e = WriteAnalyticSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("root limit")
	}
}
