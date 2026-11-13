package exchange

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func brepBox(t *testing.T) brep.Solid {
	t.Helper()
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestBRepRoundTrip(t *testing.T) {
	for _, s := range []brep.Solid{{}, brepBox(t)} {
		var b bytes.Buffer
		if e := WriteBRepJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		got, e := ReadBRepJSON(byteReader{&b}, BRepLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(s.Vertices(), got.Vertices()) || !reflect.DeepEqual(s.Edges(), got.Edges()) || !reflect.DeepEqual(s.Faces(), got.Faces()) {
			t.Fatal("entity graph changed")
		}
		if got.Topology() != s.Topology() {
			t.Fatal("topology changed")
		}
		if got.Tessellate().SignedVolume() != s.Tessellate().SignedVolume() {
			t.Fatal("geometry changed")
		}
	}
}
func TestBRepMalformedAndLimits(t *testing.T) {
	var b bytes.Buffer
	_ = WriteBRepJSON(&b, brepBox(t))
	valid := b.String()
	for _, s := range []string{"", strings.Replace(valid, `"version":1`, `"version":2`, 1), strings.Replace(valid, `"kind":"planar-brep"`, `"kind":"curved-brep"`, 1), valid + "{}", strings.Replace(valid, `"Start":0`, `"Start":-1`, 1), strings.Replace(valid, `"version":1`, `"unknown":1,"version":1`, 1)} {
		if _, e := ReadBRepJSON(strings.NewReader(s), BRepLimits{}); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	for _, l := range []BRepLimits{{MaxBytes: 10}, {MaxVertices: 2}, {MaxEdges: 2}, {MaxFaces: 2}, {MaxEdgeUses: 2}, {MaxBytes: -1}, {MaxVertices: -1}} {
		if _, e := ReadBRepJSON(strings.NewReader(valid), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestBRepIO(t *testing.T) {
	s := brepBox(t)
	if e := WriteBRepJSON(shortWriter{}, s); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if e := WriteBRepJSON(brokenWriter{}, s); !errors.Is(e, io.ErrClosedPipe) {
		t.Fatal(e)
	}
	if e := WriteBRepJSON(nil, s); e == nil {
		t.Fatal("nil writer")
	}
	if _, e := ReadBRepJSON(brokenReader{}, BRepLimits{}); !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Fatal(e)
	}
	if _, e := ReadBRepJSON(nil, BRepLimits{}); e == nil {
		t.Fatal("nil reader")
	}
}
func FuzzReadBRepJSON(f *testing.F) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		f.Fatal(e)
	}
	var b bytes.Buffer
	_ = WriteBRepJSON(&b, s)
	f.Add(b.String())
	f.Fuzz(func(t *testing.T, s string) {
		m, e := ReadBRepJSON(strings.NewReader(s), BRepLimits{MaxBytes: 1 << 16, MaxVertices: 100, MaxEdges: 100, MaxFaces: 100, MaxEdgeUses: 300})
		if e == nil {
			if e = m.Validate(); e != nil {
				t.Fatal(e)
			}
		}
	})
}

func TestBRepHoledFacePersistence(t *testing.T) {
	s, e := brep.PrismWithHoles([]geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}, [][]geom.Vec3{{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 2}}}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteBRepJSON(&b, s); e != nil {
		t.Fatal(e)
	}
	data := append([]byte(nil), b.Bytes()...)
	got, e := ReadBRepJSON(&b, BRepLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got.Faces(), s.Faces()) || got.Topology() != s.Topology() {
		t.Fatal("holes changed")
	}
	uses := 0
	for _, f := range s.Faces() {
		uses += len(f.Boundary)
		for _, h := range f.Holes {
			uses += len(h)
		}
	}
	if _, e = ReadBRepJSON(bytes.NewReader(data), BRepLimits{MaxEdgeUses: uses - 1}); e == nil {
		t.Fatal("hole uses omitted from limit")
	}
}
