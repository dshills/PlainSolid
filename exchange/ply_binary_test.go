package exchange

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"plainsolid/geom"
	"plainsolid/mesh"
	"strings"
	"testing"
)

func independentBinaryPLY(t testing.TB, big bool) []byte {
	t.Helper()
	format, body := "binary_little_endian", "0000000000000000000000000000803f0000000000000000000000000000803f0000000003000000000100000002000000"
	if big {
		format = "binary_big_endian"
		body = "0000000000000000000000003f8000000000000000000000000000003f8000000000000003000000000000000100000002"
	}
	data, e := hex.DecodeString(body)
	if e != nil {
		t.Fatal(e)
	}
	return append([]byte(fmt.Sprintf("ply\nformat %s 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nelement face 1\nproperty list uchar int vertex_indices\nend_header\n", format)), data...)
}
func TestBinaryPLYFixtures(t *testing.T) {
	for _, big := range []bool{false, true} {
		raw := independentBinaryPLY(t, big)
		m, e := ReadPLY(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if m.Vertices[1] != (geom.Vec3{X: 1}) || m.Vertices[2] != (geom.Vec3{Y: 1}) || m.Triangles[0] != ([3]int{0, 1, 2}) {
			t.Fatal(m)
		}
		var b bytes.Buffer
		if e = WriteBinaryPLY(&b, m, big); e != nil {
			t.Fatal(e)
		}
		got, e := ReadPLY(&b, Limits{})
		if e != nil || got.Vertices[1] != m.Vertices[1] || got.Triangles[0] != m.Triangles[0] {
			t.Fatal(got, e)
		}
	}
}
func TestBinaryPLYFailures(t *testing.T) {
	raw := independentBinaryPLY(t, false)
	header := bytes.Index(raw, []byte("end_header\n")) + len("end_header\n")
	for _, offset := range []int{header, header + 35, len(raw) - 1} {
		if _, e := ReadPLY(bytes.NewReader(raw[:offset]), Limits{}); e == nil {
			t.Fatal("truncated", offset)
		}
	}
	for _, mutation := range []func([]byte){func(b []byte) { b[header+36] = 4 }, func(b []byte) { b[header+40] = 255 }, func(b []byte) { b[header+2] = 128; b[header+3] = 127 }} {
		bad := append([]byte(nil), raw...)
		mutation(bad)
		if _, e := ReadPLY(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("malformed")
		}
	}
	if _, e := ReadPLY(bytes.NewReader(append(raw, 0)), Limits{}); e == nil {
		t.Fatal("trailing")
	}
	if _, e := ReadPLY(bytes.NewReader(raw), Limits{MaxVertices: 2}); e == nil {
		t.Fatal("limit")
	}
	m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 1}, {Y: 1}}, Triangles: [][3]int{{0, 1, 2}}}
	for _, w := range []io.Writer{nil, plyFailWriter{}, plyShortWriter{}} {
		if e := WriteBinaryPLY(w, m, false); e == nil {
			t.Fatal("writer")
		}
	}
	ascii := "ply\nformat ascii 1.0\ncomment format binary_little_endian 1.0\nelement vertex 3\nproperty float x\nproperty float y\nproperty float z\nelement face 1\nproperty list uchar int vertex_indices\nend_header\n0 0 0\n1 0 0\n0 1 0\n3 0 1 2\n"
	if _, e := ReadPLY(strings.NewReader(ascii), Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadPLY(strings.NewReader(strings.Replace(ascii, "1 0 0", "1e100 0 0", 1)), Limits{}); e == nil {
		t.Fatal("float range")
	}
}

type plyFailWriter struct{}

func (plyFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type plyShortWriter struct{}

func (plyShortWriter) Write(p []byte) (int, error) { return len(p) / 2, nil }
func FuzzBinaryPLY(f *testing.F) {
	f.Add(independentBinaryPLY(f, false))
	f.Add(independentBinaryPLY(f, true))
	f.Fuzz(func(t *testing.T, b []byte) {
		m, e := ReadPLY(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxVertices: 100, MaxTriangles: 100})
		if e == nil {
			if e = m.Validate(); e != nil {
				t.Fatal(e)
			}
		}
	})
}
