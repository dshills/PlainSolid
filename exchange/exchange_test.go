package exchange

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"plainsolid/geom"
	"plainsolid/mesh"
)

func tetra() mesh.Mesh {
	return mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 1}, {Y: 1}, {Z: 1}}, Triangles: [][3]int{{0, 2, 1}, {0, 1, 3}, {0, 3, 2}, {1, 2, 3}}}
}
func TestRoundTrips(t *testing.T) {
	for _, c := range []struct {
		name  string
		write func(io.Writer, mesh.Mesh) error
		read  func(io.Reader, Limits) (mesh.Mesh, error)
	}{{"obj", WriteOBJ, ReadOBJ}, {"binary STL", WriteSTL, ReadSTL}, {"ASCII STL", WriteASCIISTL, ReadSTL}, {"JSON", WriteJSON, ReadJSON}} {
		t.Run(c.name, func(t *testing.T) {
			var b bytes.Buffer
			if e := c.write(&b, tetra()); e != nil {
				t.Fatal(e)
			}
			m, e := c.read(&b, Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if len(m.Vertices) != 4 || len(m.Triangles) != 4 {
				t.Fatal(m)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			if math.Abs(volume(m)-1.0/6) > 1e-14 {
				t.Fatal(volume(m))
			}
		})
	}
}
func volume(m mesh.Mesh) float64 {
	v := 0.
	for _, f := range m.Triangles {
		v += m.Vertices[f[0]].Dot(m.Vertices[f[1]].Cross(m.Vertices[f[2]])) / 6
	}
	return v
}
func TestOBJIndices(t *testing.T) {
	m, e := ReadOBJ(strings.NewReader("v 0 0 0\nv 1 0 0\nv 0 1 0\nvt 0 0\nvn 0 0 1\nf -3/1/1 -2/1/1 -1/1/1\n"), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Triangles[0] != [3]int{0, 1, 2} {
		t.Fatal(m)
	}
}
func TestMalformed(t *testing.T) {
	obj := []string{"v NaN 0 0", "v 0 0 0\nf 1 2 3", "v 0 0 0\nf 0 0 0", "curv 0 1 1 2", "v 0 0 0\nf 1 1 1 1", "v 0 0 0\nf 1/1 1/1 1/1"}
	for _, s := range obj {
		if _, e := ReadOBJ(strings.NewReader(s), Limits{}); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	for _, s := range []string{"", "solid x", "solid x\nendsolid x\ngarbage", "solid x\nfacet normal 0 0 1\nouter loop\nvertex 0 0 0\nendloop\nendfacet\nendsolid"} {
		if _, e := ReadSTL(strings.NewReader(s), Limits{}); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	for _, s := range []string{`{"version":2,"mesh":{}}`, `{"version":1,"mesh":{},"extra":3}`, `{"version":1,"mesh":{}}{}`} {
		if _, e := ReadJSON(strings.NewReader(s), Limits{}); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
func TestLimits(t *testing.T) {
	var b bytes.Buffer
	_ = WriteOBJ(&b, tetra())
	for _, l := range []Limits{{MaxBytes: 4}, {MaxVertices: 2}, {MaxTriangles: 2}, {MaxBytes: -1}, {MaxBytes: 1 << 31}} {
		if _, e := ReadOBJ(bytes.NewReader(b.Bytes()), l); e == nil {
			t.Fatal(l)
		}
	}
	b.Reset()
	_ = WriteSTL(&b, tetra())
	if _, e := ReadSTL(&b, Limits{MaxTriangles: 2}); e == nil {
		t.Fatal("triangle limit")
	}
}
func TestBinarySolidHeader(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSTL(&b, tetra()); e != nil {
		t.Fatal(e)
	}
	data := b.Bytes()
	copy(data, "solid misleading")
	if _, e := ReadSTL(bytes.NewReader(data), Limits{}); e != nil {
		t.Fatal(e)
	}
	binary.LittleEndian.PutUint32(data[84+12:], math.Float32bits(float32(math.NaN())))
	if _, e := ReadSTL(bytes.NewReader(data), Limits{}); e == nil {
		t.Fatal("accepted NaN")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestIOErrors(t *testing.T) {
	for _, f := range []func(io.Writer, mesh.Mesh) error{WriteSTL, WriteASCIISTL, WriteOBJ, WriteJSON} {
		if e := f(brokenWriter{}, tetra()); !errors.Is(e, io.ErrClosedPipe) {
			t.Fatal(e)
		}
		if e := f(nil, tetra()); e == nil {
			t.Fatal("nil writer")
		}
	}
	for _, f := range []func(io.Reader, Limits) (mesh.Mesh, error){ReadSTL, ReadOBJ, ReadJSON} {
		if _, e := f(brokenReader{}, Limits{}); !errors.Is(e, io.ErrUnexpectedEOF) {
			t.Fatal(e)
		}
		if _, e := f(nil, Limits{}); e == nil {
			t.Fatal("nil reader")
		}
	}
}
func TestSTLPrecision(t *testing.T) {
	m := tetra()
	for i := range m.Vertices {
		m.Vertices[i].X += 1e10
	}
	if e := WriteSTL(io.Discard, m); e == nil {
		t.Fatal("accepted collapsed triangle")
	}
	m = tetra()
	m.Vertices[1].X = 1e40
	if e := WriteSTL(io.Discard, m); e == nil {
		t.Fatal("accepted overflow")
	}
}
func FuzzReadSTL(f *testing.F) {
	var b bytes.Buffer
	_ = WriteSTL(&b, tetra())
	f.Add(b.Bytes())
	f.Add([]byte("solid empty\nendsolid empty\n"))
	f.Fuzz(func(t *testing.T, b []byte) {
		m, e := ReadSTL(bytes.NewReader(b), Limits{MaxBytes: 1 << 16, MaxVertices: 2000, MaxTriangles: 1000})
		if e == nil {
			if e = m.Validate(); e != nil {
				t.Fatal(e)
			}
		}
	})
}
func FuzzReadOBJ(f *testing.F) {
	f.Add("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 3\n")
	f.Fuzz(func(t *testing.T, s string) {
		m, e := ReadOBJ(strings.NewReader(s), Limits{MaxBytes: 1 << 16, MaxVertices: 2000, MaxTriangles: 1000})
		if e == nil {
			if e = m.Validate(); e != nil {
				t.Fatal(e)
			}
		}
	})
}
func FuzzReadJSON(f *testing.F) {
	f.Add(`{"version":1,"mesh":{"Vertices":[],"Triangles":[]}}`)
	f.Fuzz(func(t *testing.T, s string) {
		m, e := ReadJSON(strings.NewReader(s), Limits{MaxBytes: 1 << 16, MaxVertices: 2000, MaxTriangles: 1000})
		if e == nil {
			if e = m.Validate(); e != nil {
				t.Fatal(e)
			}
		}
	})
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) / 2, nil }

type byteReader struct{ r io.Reader }

func (r byteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.r.Read(p)
}
func TestShortIO(t *testing.T) {
	for _, c := range []struct {
		write func(io.Writer, mesh.Mesh) error
		read  func(io.Reader, Limits) (mesh.Mesh, error)
	}{{WriteOBJ, ReadOBJ}, {WriteSTL, ReadSTL}, {WriteASCIISTL, ReadSTL}, {WriteJSON, ReadJSON}} {
		if e := c.write(shortWriter{}, tetra()); !errors.Is(e, io.ErrShortWrite) {
			t.Fatalf("short writer: %v", e)
		}
		var b bytes.Buffer
		if e := c.write(&b, tetra()); e != nil {
			t.Fatal(e)
		}
		if _, e := c.read(byteReader{&b}, Limits{}); e != nil {
			t.Fatal(e)
		}
	}
}
