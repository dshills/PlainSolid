package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"plainsolid/mesh"
)

func TestPLYRoundTrip(t *testing.T) {
	var b bytes.Buffer
	if e := WritePLY(&b, tetra()); e != nil {
		t.Fatal(e)
	}
	m, e := ReadPLY(byteReader{&b}, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if volume(m) != volume(tetra()) {
		t.Fatal(m)
	}
	for _, m := range []mesh.Mesh{{}, tetra()} {
		b.Reset()
		if e = WritePLY(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadPLY(&b, Limits{})
		if e != nil || len(got.Triangles) != len(m.Triangles) {
			t.Fatal(got, e)
		}
	}
}
func TestPLYMalformed(t *testing.T) {
	var b bytes.Buffer
	_ = WritePLY(&b, tetra())
	valid := b.String()
	for _, s := range []string{"", strings.Replace(valid, "ascii", "binary_little_endian", 1), strings.Replace(valid, "property double x", "property double w", 1), strings.Replace(valid, "element face 4", "element face -1", 1), strings.Replace(valid, "element vertex 4", "element vertex 99999999", 1), strings.Replace(valid, "3 0 2 1", "4 0 2 1", 1), strings.Replace(valid, "3 0 2 1", "3 0 2 -1", 1), strings.Replace(valid, "0 0 0", "NaN 0 0", 1), valid + "3 0 1 2\n", strings.Replace(valid, "end_header", "no_header", 1)} {
		if _, e := ReadPLY(strings.NewReader(s), Limits{}); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	for _, l := range []Limits{{MaxBytes: 10}, {MaxVertices: 2}, {MaxTriangles: 2}} {
		if _, e := ReadPLY(strings.NewReader(valid), l); e == nil {
			t.Fatal(l)
		}
	}
}
func TestPLYMetadata(t *testing.T) {
	var b bytes.Buffer
	_ = WritePLY(&b, tetra())
	s := strings.Replace(b.String(), "element vertex", "comment hello\nobj_info foo\nelement vertex", 1)
	if _, e := ReadPLY(strings.NewReader(s), Limits{}); e != nil {
		t.Fatal(e)
	}
}

// Decode into independent anonymous structs rather than reusing exporter types.
func TestGLTF(t *testing.T) {
	var b bytes.Buffer
	m := tetra()
	if e := WriteGLTF(&b, m); e != nil {
		t.Fatal(e)
	}
	var d struct {
		Asset  struct{ Version string }
		Scene  int
		Scenes []struct{ Nodes []int }
		Nodes  []struct{ Mesh int }
		Meshes []struct {
			Primitives []struct {
				Attributes    map[string]int
				Indices, Mode int
			}
		}
		Buffers []struct {
			URI        string
			ByteLength int
		}
		BufferViews []struct{ Buffer, ByteOffset, ByteLength, Target int }
		Accessors   []struct {
			BufferView, ComponentType, Count int
			Type                             string
			Min, Max                         []float64
		}
	}
	if e := json.Unmarshal(b.Bytes(), &d); e != nil {
		t.Fatal(e)
	}
	if d.Asset.Version != "2.0" || d.Scene != 0 || len(d.Scenes) != 1 || len(d.Scenes[0].Nodes) != 1 || d.Scenes[0].Nodes[0] != 0 || len(d.Nodes) != 1 || d.Nodes[0].Mesh != 0 || len(d.Meshes) != 1 {
		t.Fatal("scene structure")
	}
	p := d.Meshes[0].Primitives[0]
	if p.Mode != 4 || p.Attributes["POSITION"] != 0 || p.Indices != 1 {
		t.Fatal(p)
	}
	if len(d.Buffers) != 1 || len(d.BufferViews) != 2 || len(d.Accessors) != 2 {
		t.Fatal("buffer structure")
	}
	const prefix = "data:application/octet-stream;base64,"
	if !strings.HasPrefix(d.Buffers[0].URI, prefix) {
		t.Fatal("buffer URI")
	}
	data, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(d.Buffers[0].URI, prefix))
	if e != nil {
		t.Fatal(e)
	}
	if len(data) != d.Buffers[0].ByteLength || len(data) != 12*(len(m.Vertices)+len(m.Triangles)) {
		t.Fatal("buffer size")
	}
	a := d.Accessors[0]
	if a.ComponentType != 5126 || a.Type != "VEC3" || a.Count != 4 || a.BufferView != 0 || len(a.Min) != 3 || len(a.Max) != 3 {
		t.Fatal(a)
	}
	a = d.Accessors[1]
	if a.ComponentType != 5125 || a.Type != "SCALAR" || a.Count != 12 || a.BufferView != 1 {
		t.Fatal(a)
	}
	for i, v := range m.Vertices {
		for j, x := range []float64{v.X, v.Y, v.Z} {
			got := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*12+j*4:])))
			if got != x {
				t.Fatal(got, x)
			}
		}
	}
	off := d.BufferViews[1].ByteOffset
	if off != 48 || d.BufferViews[0].ByteOffset != 0 || d.BufferViews[0].ByteLength != 48 || d.BufferViews[1].ByteLength != 48 || d.BufferViews[0].Target != 34962 || d.BufferViews[1].Target != 34963 {
		t.Fatal(d.BufferViews)
	}
	for i, tr := range m.Triangles {
		for j, x := range tr {
			got := binary.LittleEndian.Uint32(data[off+i*12+j*4:])
			if int(got) != x {
				t.Fatal(got, x)
			}
		}
	}
	for j := 0; j < 3; j++ {
		if d.Accessors[0].Min[j] != 0 || d.Accessors[0].Max[j] != 1 {
			t.Fatal("bounds")
		}
	}
}
func TestGLTFEmptyAndInvalid(t *testing.T) {
	var b bytes.Buffer
	if e := WriteGLTF(&b, mesh.Mesh{}); e != nil {
		t.Fatal(e)
	}
	var d map[string]any
	if e := json.Unmarshal(b.Bytes(), &d); e != nil {
		t.Fatal(e)
	}
	if _, ok := d["buffers"]; ok {
		t.Fatal("empty has buffers")
	}
	m := tetra()
	m.Vertices[1].X = 1e40
	if e := WriteGLTF(io.Discard, m); e == nil {
		t.Fatal("overflow")
	}
	m = tetra()
	for i := range m.Vertices {
		m.Vertices[i].X += 1e10
	}
	if e := WriteGLTF(io.Discard, m); e == nil {
		t.Fatal("collapsed")
	}
	m = tetra()
	m.Triangles[0][0] = -1
	if e := WriteGLTF(io.Discard, m); e == nil {
		t.Fatal("invalid index")
	}
}
func TestNewCodecIO(t *testing.T) {
	for _, w := range []func(io.Writer, mesh.Mesh) error{WritePLY, WriteGLTF} {
		if e := w(shortWriter{}, tetra()); !errors.Is(e, io.ErrShortWrite) {
			t.Fatal(e)
		}
		if e := w(brokenWriter{}, tetra()); !errors.Is(e, io.ErrClosedPipe) {
			t.Fatal(e)
		}
		if e := w(nil, tetra()); e == nil {
			t.Fatal("nil writer")
		}
	}
	if _, e := ReadPLY(brokenReader{}, Limits{}); !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Fatal(e)
	}
	if _, e := ReadPLY(nil, Limits{}); e == nil {
		t.Fatal("nil reader")
	}
}
func FuzzReadPLY(f *testing.F) {
	var b bytes.Buffer
	_ = WritePLY(&b, tetra())
	f.Add(b.String())
	f.Fuzz(func(t *testing.T, s string) {
		m, e := ReadPLY(strings.NewReader(s), Limits{MaxBytes: 1 << 16, MaxVertices: 2000, MaxTriangles: 1000})
		if e == nil {
			if e = m.Validate(); e != nil {
				t.Fatal(e)
			}
		}
	})
}
