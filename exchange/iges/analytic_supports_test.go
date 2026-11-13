package iges

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func mixedSupportFixture() AnalyticSupportModel {
	p := planeFixture().Surfaces[0]
	c := cylinderFixture().Surfaces[0]
	s := sphereFixture().Surfaces[0]
	t := torusFixture().Surfaces[0]
	return AnalyticSupportModel{LengthUnit: .3048, Surfaces: []AnalyticSupport{{Torus: &t}, {Plane: &p}, {Sphere: &s}, {Cylinder: &c}}}
}
func TestAnalyticSupportsRoundtrip(t *testing.T) {
	m := mixedSupportFixture()
	var b bytes.Buffer
	if e := WriteAnalyticSupports(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticSupports(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(got, m) {
		t.Fatal(got, e)
	}
	var c bytes.Buffer
	if e = WriteAnalyticSupports(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
		t.Fatal("canonical", e)
	}
	if _, e = ReadToroidalSupports(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("homogeneous reader accepted mixed")
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxEntities: 4}, {MaxValues: 5}, {MaxRecords: 4}} {
		if _, e = ReadAnalyticSupports(bytes.NewReader(b.Bytes()), l); e == nil {
			t.Fatal("limits")
		}
	}
	got.Surfaces[0].Torus.MajorRadius = 99
	if m.Surfaces[0].Torus.MajorRadius == 99 {
		t.Fatal("alias")
	}
}
func TestAnalyticSupportsOwnership(t *testing.T) {
	var b bytes.Buffer
	_ = WriteAnalyticSupports(&b, mixedSupportFixture())
	d, e := readRingSupportRecords(bytes.NewReader(b.Bytes()), Limits{}, false, true, true, true, true)
	if e != nil {
		t.Fatal(e)
	}
	var roots []int
	for i, x := range d.entities {
		if analyticIGESReferenceIndices(x.kind) != nil {
			roots = append(roots, i)
		}
	}
	first := d.entities[roots[0]].values[2].text
	second := &d.entities[roots[1]]
	second.values = append([]parameter(nil), second.values...)
	second.values[2].text = first
	a := analyticIGESAuthor()
	keep := map[int]bool{}
	for _, r := range d.entities {
		keep[r.id] = true
	}
	if e = analyticIGESAppend(&a, d, keep); e != nil {
		t.Fatal(e)
	}
	bad, e := a.file(d.unit)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := ReadAnalyticSupports(bytes.NewReader(bad), Limits{}); e == nil || len(got.Surfaces) != 0 {
		t.Fatal("shared axis")
	}
	// Source scalar text must not gain validity merely through record staging.
	d, e = readRingSupportRecords(bytes.NewReader(b.Bytes()), Limits{}, false, true, true, true, true)
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range d.entities {
		if x.kind == 116 {
			d.entities[i].values[1] = parameter{text: "0", stringValue: true}
			break
		}
	}
	a = analyticIGESAuthor()
	if e = analyticIGESAppend(&a, d, keep); e == nil {
		t.Fatal("Hollerith numeric accepted")
	}
	//194 has a different exact parameter contract and cannot join this family.
	text := b.String()
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if len(line) == 80 && line[72] == 'D' && strings.TrimSpace(line[:8]) == "198" {
			lines[i] = "     194" + line[8:]
		}
	}
	if _, e = ReadAnalyticSupports(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
		t.Fatal("cone mixed")
	}
}

type mixedSupportShortWriter struct{}

func (mixedSupportShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestAnalyticSupportsBoundsAndAtomicity(t *testing.T) {
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
		t.Fatal(e, len(got.Surfaces))
	}
	m.Surfaces = append(m.Surfaces, AnalyticSupport{Plane: p})
	b.Reset()
	if e = WriteAnalyticSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("limit")
	}
	m = mixedSupportFixture()
	m.Surfaces[3].Plane = p
	if e = WriteAnalyticSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("ambiguous variant")
	}
	if e = WriteAnalyticSupports(mixedSupportShortWriter{}, mixedSupportFixture()); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
func FuzzAnalyticSupports(f *testing.F) {
	var b bytes.Buffer
	_ = WriteAnalyticSupports(&b, mixedSupportFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticSupports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxEntities: 64, MaxRecords: 128, MaxValues: 512})
	})
}
