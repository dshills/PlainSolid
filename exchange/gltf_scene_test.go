package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"plainsolid/geom"
	"testing"
)

func independentScene(t testing.TB) (map[string]any, []byte) {
	b := independentGLB(t)
	n := int(binary.LittleEndian.Uint32(b[12:]))
	var d map[string]any
	if e := json.Unmarshal(b[20:20+n], &d); e != nil {
		t.Fatal(e)
	}
	bin := append([]byte(nil), b[28+n:]...)
	d["meshes"] = []any{map[string]any{"name": "triangle", "primitives": []any{map[string]any{"attributes": map[string]any{"POSITION": 0}, "indices": 1}}}, map[string]any{"name": "second", "primitives": []any{map[string]any{"attributes": map[string]any{"POSITION": 0}, "indices": 1}}}}
	d["nodes"] = []any{map[string]any{"name": "parent", "translation": []float64{10, 20, 30}, "children": []int{1, 2, 3}}, map[string]any{"mesh": 0, "translation": []float64{1, 0, 0}}, map[string]any{"mesh": 0, "rotation": []float64{0, 0, math.Sqrt(.5), math.Sqrt(.5)}, "scale": []float64{2, 3, 1}}, map[string]any{"mesh": 1, "matrix": []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 5, 1}}}
	return d, bin
}
func sceneBytes(t testing.TB, d map[string]any, bin []byte, glb bool) []byte {
	t.Helper()
	if !glb {
		d["buffers"] = []any{map[string]any{"byteLength": 39, "uri": "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(bin[:39])}}
	} else {
		d["buffers"] = []any{map[string]any{"byteLength": 39}}
	}
	js, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	if !glb {
		return js
	}
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	b := make([]byte, 28+len(js)+len(bin))
	copy(b, "glTF")
	binary.LittleEndian.PutUint32(b[4:], 2)
	binary.LittleEndian.PutUint32(b[8:], uint32(len(b)))
	binary.LittleEndian.PutUint32(b[12:], uint32(len(js)))
	copy(b[16:], "JSON")
	copy(b[20:], js)
	p := 20 + len(js)
	binary.LittleEndian.PutUint32(b[p:], uint32(len(bin)))
	copy(b[p+4:], []byte{'B', 'I', 'N', 0})
	copy(b[p+8:], bin)
	return b
}
func TestSceneHierarchyAndInstances(t *testing.T) {
	for _, glb := range []bool{false, true} {
		d, bin := independentScene(t)
		raw := sceneBytes(t, d, bin, glb)
		read := ReadGLTFScene
		if glb {
			read = ReadGLBScene
		}
		s, e := read(bytes.NewReader(raw), SceneLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Meshes) != 2 || len(s.Nodes) != 4 || s.Meshes[0].Name != "triangle" || s.Nodes[1].Mesh != s.Nodes[2].Mesh {
			t.Fatal(s)
		}
		for i, want := range map[int]geom.Vec3{1: {X: 12, Y: 20, Z: 30}, 2: {X: 10, Y: 22, Z: 30}, 3: {X: 11, Y: 20, Z: 35}} {
			got := s.Nodes[i].World.Apply(geom.Vec3{X: 1})
			if got.Distance(want) > 1e-12 {
				t.Fatal(i, got, want)
			}
		}
		if glb {
			if _, e := ReadGLB(bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("old reader expanded")
			}
		} else {
			if _, e := ReadGLTF(bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("old reader expanded")
			}
		}
	}
}
func TestSceneMalformedAndLimits(t *testing.T) {
	for _, kind := range []string{"cycle", "shared", "duplicate-root", "unreachable", "mesh", "quaternion", "matrix-trs", "shear", "singular", "overflow"} {
		d, bin := independentScene(t)
		nodes := d["nodes"].([]any)
		n := nodes[1].(map[string]any)
		switch kind {
		case "cycle":
			n["children"] = []int{0}
		case "shared":
			n["children"] = []int{2}
		case "duplicate-root":
			d["scenes"] = []any{map[string]any{"nodes": []int{0, 0}}}
		case "unreachable":
			nodes[0].(map[string]any)["children"] = []int{1, 2}
		case "mesh":
			n["mesh"] = 99
		case "quaternion":
			n["rotation"] = []float64{0, 0, 0, 0}
		case "matrix-trs":
			n["matrix"] = make([]float64, 16)
		case "shear":
			delete(n, "translation")
			n["matrix"] = []float64{1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
		case "singular":
			n["scale"] = []float64{0, 1, 1}
		case "overflow":
			n["scale"] = []float64{1e308, 1e308, 1e308}
			nodes[0].(map[string]any)["scale"] = []float64{2, 2, 2}
		}
		if _, e := ReadGLTFScene(bytes.NewReader(sceneBytes(t, d, bin, false)), SceneLimits{}); e == nil {
			t.Fatal(kind)
		}
	}
	d, bin := independentScene(t)
	raw := sceneBytes(t, d, bin, false)
	for _, l := range []SceneLimits{{MaxNodes: 3}, {MaxMeshes: 1}, {MaxDepth: 1}, {Limits: Limits{MaxVertices: 5}}, {Limits: Limits{MaxTriangles: 1}}, {Limits: Limits{MaxBytes: 10}}} {
		if _, e := ReadGLTFScene(bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
}
func FuzzGLTFScene(f *testing.F) {
	d, bin := independentScene(f)
	f.Add(sceneBytes(f, d, bin, false))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadGLTFScene(bytes.NewReader(b), SceneLimits{Limits: Limits{MaxBytes: 65536, MaxVertices: 512, MaxTriangles: 512}, MaxNodes: 128, MaxMeshes: 16, MaxDepth: 16})
	})
}

func TestGLBSceneMalformedContainer(t *testing.T) {
	d, bin := independentScene(t)
	raw := sceneBytes(t, d, bin, true)
	for _, cut := range []int{0, 12, 19, len(raw) - 1} {
		if _, e := ReadGLBScene(bytes.NewReader(raw[:cut]), SceneLimits{}); e == nil {
			t.Fatal(cut)
		}
	}
	bad := append([]byte(nil), raw...)
	binary.LittleEndian.PutUint32(bad[12:], math.MaxUint32)
	if _, e := ReadGLBScene(bytes.NewReader(bad), SceneLimits{}); e == nil {
		t.Fatal("chunk overflow")
	}
	bad = append([]byte(nil), raw...)
	bad[len(bad)-1] = 1
	if _, e := ReadGLBScene(bytes.NewReader(bad), SceneLimits{}); e == nil {
		t.Fatal("padding")
	}
	if _, e := ReadGLBScene(nil, SceneLimits{}); e == nil {
		t.Fatal("nil reader")
	}
}
func TestSceneNegativeScale(t *testing.T) {
	d, bin := independentScene(t)
	d["nodes"].([]any)[1].(map[string]any)["scale"] = []float64{-1, 2, 3}
	s, e := ReadGLTFScene(bytes.NewReader(sceneBytes(t, d, bin, false)), SceneLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if s.Nodes[1].World.Determinant() >= 0 || s.Nodes[1].World.Apply(geom.Vec3{X: 1}).Distance(geom.Vec3{X: 10, Y: 20, Z: 30}) > 1e-12 {
		t.Fatal(s.Nodes[1])
	}
}
