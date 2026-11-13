package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func writerScene(t *testing.T) MeshScene {
	d, bin := independentScene(t)
	s, e := ReadGLTFScene(bytes.NewReader(sceneBytes(t, d, bin, false)), SceneLimits{})
	if e != nil {
		t.Fatal(e)
	}
	s.Name = "Assembly"
	return s
}
func TestWriteMeshScenes(t *testing.T) {
	for _, glb := range []bool{false, true} {
		s := writerScene(t)
		var b, again bytes.Buffer
		write, read := WriteGLTFScene, ReadGLTFScene
		if glb {
			write, read = WriteGLBScene, ReadGLBScene
		}
		if e := write(&b, s); e != nil {
			t.Fatal(e)
		}
		if e := write(&again, s); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
			t.Fatal("non deterministic", e)
		}
		got, e := read(bytes.NewReader(b.Bytes()), SceneLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(s, got) {
			t.Fatal(s, got)
		}
		// Independently inspect schema field spellings, definition references and
		// buffer offsets instead of relying only on the case-insensitive decoder.
		raw := b.Bytes()
		var payload []byte
		if glb {
			n := int(binary.LittleEndian.Uint32(raw[12:]))
			payload = raw[28+n:]
			raw = raw[20 : 20+n]
		}
		var doc map[string]any
		if e = json.Unmarshal(raw, &doc); e != nil {
			t.Fatal(e)
		}
		nodes := doc["nodes"].([]any)
		if len(nodes) != 4 || nodes[1].(map[string]any)["mesh"] != float64(0) || nodes[2].(map[string]any)["mesh"] != float64(0) {
			t.Fatal(nodes)
		}
		meshes := doc["meshes"].([]any)
		if len(meshes) != 2 {
			t.Fatal(meshes)
		}
		p := meshes[1].(map[string]any)["primitives"].([]any)[0].(map[string]any)
		if p["indices"] != float64(3) {
			t.Fatal(p)
		}
		accessors := doc["accessors"].([]any)
		if accessors[0].(map[string]any)["min"] == nil || accessors[2].(map[string]any)["bufferView"] != float64(2) {
			t.Fatal(accessors)
		}
		if !glb {
			uri := doc["buffers"].([]any)[0].(map[string]any)["uri"].(string)
			payload, e = base64.StdEncoding.DecodeString(uri[len("data:application/octet-stream;base64,"):])
			if e != nil {
				t.Fatal(e)
			}
		}
		views := doc["bufferViews"].([]any)
		offset := int(views[2].(map[string]any)["byteOffset"].(float64))
		if offset != 48 || math.Float32frombits(binary.LittleEndian.Uint32(payload[offset+12:])) != 1 {
			t.Fatal(offset)
		}
	}
}
func TestSceneWriterInvalid(t *testing.T) {
	for _, kind := range []string{"cycle", "zero", "shear", "bad-mesh", "invalid-name", "overflow", "collapse"} {
		s := writerScene(t)
		switch kind {
		case "cycle":
			s.Nodes[1].Children = []int{0}
		case "zero":
			s.Nodes[1].Local = geom.Affine{}
		case "shear":
			s.Nodes[1].Local[0][1] = .5
		case "bad-mesh":
			s.Nodes[1].Mesh = 99
		case "invalid-name":
			s.Name = string([]byte{255})
		case "overflow":
			s.Meshes[0].Geometry.Vertices[1].X = 1e100
		case "collapse":
			s.Meshes[0].Geometry.Vertices[1].X = 1e-100
		}
		var b bytes.Buffer
		if e := WriteGLTFScene(&b, s); e == nil || b.Len() != 0 {
			t.Fatal(kind, e, b.Len())
		}
	}
	s := writerScene(t)
	if e := WriteGLTFScene(nil, s); e == nil {
		t.Fatal("nil writer")
	}
	if e := WriteGLBScene(plyFailWriter{}, s); e == nil {
		t.Fatal("writer error")
	}
	if e := WriteGLTFScene(plyShortWriter{}, s); e == nil {
		t.Fatal("short writer")
	}
}
func TestSceneWriterEmpty(t *testing.T) {
	var b bytes.Buffer
	if e := WriteGLBScene(&b, MeshScene{}); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadGLBScene(&b, SceneLimits{}); e != nil {
		t.Fatal(e)
	}
}
