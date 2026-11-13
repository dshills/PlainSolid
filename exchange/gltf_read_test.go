package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"plainsolid/geom"
	"plainsolid/mesh"
)

func TestGLTFReadRoundtrip(t *testing.T) {
	s, e := mesh.Box(geom.Vec3{X: -1}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteGLTF(&b, s); e != nil {
		t.Fatal(e)
	}
	m, e := ReadGLTF(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(m, s) {
		t.Fatalf("roundtrip: %+v %v", m, e)
	}
	if _, e = ReadGLTF(bytes.NewReader(b.Bytes()), Limits{MaxTriangles: 1}); e == nil {
		t.Fatal("triangle limit ignored")
	}
	b.Reset()
	if e = WriteGLTF(&b, mesh.Mesh{}); e != nil {
		t.Fatal(e)
	}
	m, e = ReadGLTF(&b, Limits{})
	if e != nil || len(m.Triangles) != 0 {
		t.Fatal(e)
	}
}
func TestGLTFInterleavedFixture(t *testing.T) {
	data := make([]byte, 52)
	for i, p := range [][3]float32{{0, 0, 0}, {2, 0, 0}, {0, 3, 0}} {
		for j, x := range p {
			binary.LittleEndian.PutUint32(data[i*16+j*4:], math.Float32bits(x))
		}
	}
	copy(data[48:], []byte{0, 1, 2, 0})
	jsonData := map[string]any{
		"asset":       map[string]any{"version": "2.0"},
		"scenes":      []any{map[string]any{"nodes": []int{0}}},
		"nodes":       []any{map[string]int{"mesh": 0}},
		"meshes":      []any{map[string]any{"primitives": []any{map[string]any{"attributes": map[string]int{"POSITION": 0}, "indices": 1}}}},
		"buffers":     []any{map[string]any{"byteLength": 52, "uri": "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(data)}},
		"bufferViews": []any{map[string]int{"buffer": 0, "byteLength": 48, "byteStride": 16}, map[string]int{"buffer": 0, "byteOffset": 48, "byteLength": 3}},
		"accessors": []any{
			map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3", "min": []int{0, 0, 0}, "max": []int{2, 3, 0}},
			map[string]any{"bufferView": 1, "componentType": 5121, "count": 3, "type": "SCALAR"},
		},
	}
	read := func() (mesh.Mesh, error) {
		raw, e := json.Marshal(jsonData)
		if e != nil {
			t.Fatal(e)
		}
		return ReadGLTF(bytes.NewReader(raw), Limits{})
	}
	m, e := read()
	if e != nil || m.Area() != 3 {
		t.Fatalf("area %g %v", m.Area(), e)
	}
	a := jsonData["accessors"].([]any)[0].(map[string]any)
	a["count"] = int(^uint(0) >> 1)
	if _, e = read(); e == nil {
		t.Fatal("accepted overflowing accessor")
	}
	a["count"] = 3
	jsonData["buffers"].([]any)[0].(map[string]any)["uri"] = "file.bin"
	if _, e = read(); e == nil {
		t.Fatal("accepted external buffer")
	}
}
func FuzzReadGLTF(f *testing.F) {
	m, e := mesh.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		f.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteGLTF(&b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		m, e := ReadGLTF(bytes.NewReader(b), Limits{MaxBytes: 4096, MaxVertices: 32, MaxTriangles: 32})
		if e == nil && m.Validate() != nil {
			t.Fatal("invalid output")
		}
	})
}
