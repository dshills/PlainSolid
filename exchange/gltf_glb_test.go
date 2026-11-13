package exchange

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"plainsolid/geom"
	"plainsolid/mesh"
	"testing"
)

func independentGLB(t testing.TB) []byte {
	t.Helper()
	js := []byte(`{"asset":{"version":"2.0"},"scene":0,"scenes":[{"nodes":[0]}],"nodes":[{"mesh":0}],"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1}]}],"buffers":[{"byteLength":39}],"bufferViews":[{"buffer":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":3}],"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3","min":[0,0,0],"max":[1,1,0]},{"bufferView":1,"componentType":5121,"count":3,"type":"SCALAR"}]}`)
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	bin, e := hex.DecodeString("0000000000000000000000000000803f0000000000000000000000000000803f0000000000010200")
	if e != nil {
		t.Fatal(e)
	}
	out := make([]byte, 28+len(js)+len(bin))
	copy(out, "glTF")
	binary.LittleEndian.PutUint32(out[4:], 2)
	binary.LittleEndian.PutUint32(out[8:], uint32(len(out)))
	binary.LittleEndian.PutUint32(out[12:], uint32(len(js)))
	copy(out[16:], "JSON")
	copy(out[20:], js)
	offset := 20 + len(js)
	binary.LittleEndian.PutUint32(out[offset:], uint32(len(bin)))
	copy(out[offset+4:], []byte{'B', 'I', 'N', 0})
	copy(out[offset+8:], bin)
	return out
}
func TestGLBFixtureAndRoundtrip(t *testing.T) {
	raw := independentGLB(t)
	m, e := ReadGLB(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Vertices) != 3 || m.Vertices[1] != (geom.Vec3{X: 1}) || m.Triangles[0] != ([3]int{0, 1, 2}) {
		t.Fatal(m)
	}
	var b bytes.Buffer
	if e = WriteGLB(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadGLB(&b, Limits{})
	if e != nil || len(got.Triangles) != 1 || got.Vertices[2] != (geom.Vec3{Y: 1}) {
		t.Fatal(got, e)
	}
	if e = WriteGLB(&b, mesh.Mesh{}); e != nil {
		t.Fatal(e)
	}
	got, e = ReadGLB(&b, Limits{})
	if e != nil || len(got.Vertices) != 0 {
		t.Fatal(got, e)
	}
}
func TestGLBBoundsAndChunks(t *testing.T) {
	raw := independentGLB(t)
	for _, cut := range []int{0, 11, 19, len(raw) - 1} {
		if _, e := ReadGLB(bytes.NewReader(raw[:cut]), Limits{}); e == nil {
			t.Fatal(cut)
		}
	}
	for _, mutate := range []func([]byte){func(b []byte) { b[0] = 'x' }, func(b []byte) { b[4] = 1 }, func(b []byte) { b[8] = 0 }, func(b []byte) { b[12] = 255 }, func(b []byte) { b[16] = 'X' }, func(b []byte) { b[len(b)-1] = 1 }} {
		b := append([]byte(nil), raw...)
		mutate(b)
		if _, e := ReadGLB(bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("accepted malformed GLB")
		}
	}
	if _, e := ReadGLB(bytes.NewReader(raw), Limits{MaxBytes: int64(len(raw) - 1)}); e == nil {
		t.Fatal("byte limit")
	}
	if _, e := ReadGLB(bytes.NewReader(raw), Limits{MaxVertices: 2}); e == nil {
		t.Fatal("vertex limit")
	}
	m := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 1}, {Y: 1}}, Triangles: [][3]int{{0, 1, 2}}}
	for _, w := range []io.Writer{nil, plyFailWriter{}, plyShortWriter{}} {
		if e := WriteGLB(w, m); e == nil {
			t.Fatal("writer")
		}
	}
}
func FuzzReadGLB(f *testing.F) {
	f.Add(independentGLB(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		m, e := ReadGLB(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxVertices: 100, MaxTriangles: 100})
		if e == nil {
			if e = m.Validate(); e != nil {
				t.Fatal(e)
			}
		}
	})
}

func TestGLBDuplicateAndUnknownChunks(t *testing.T) {
	raw := independentGLB(t)
	for _, kind := range []uint32{0x4e4f534a, 0x004e4942, 0x12345678} {
		bad := append([]byte(nil), raw...)
		extra := make([]byte, 12)
		binary.LittleEndian.PutUint32(extra, 4)
		binary.LittleEndian.PutUint32(extra[4:], kind)
		bad = append(bad, extra...)
		binary.LittleEndian.PutUint32(bad[8:], uint32(len(bad)))
		if _, e := ReadGLB(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("extra chunk", kind)
		}
	}
	if _, e := ReadGLTF(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("JSON reader accepted GLB")
	}
}
