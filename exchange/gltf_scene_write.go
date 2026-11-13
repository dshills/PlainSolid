package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"unicode/utf8"
)

type sceneWriteNode struct {
	Name     string    `json:"name,omitempty"`
	Mesh     *int      `json:"mesh,omitempty"`
	Children []int     `json:"children,omitempty"`
	Matrix   []float64 `json:"matrix"`
}
type sceneWritePrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    *int           `json:"indices,omitempty"`
	Mode       *int           `json:"mode,omitempty"`
}
type sceneWriteAccessor struct {
	View      *int      `json:"bufferView"`
	Offset    int       `json:"byteOffset,omitempty"`
	Component int       `json:"componentType"`
	Count     int       `json:"count"`
	Type      string    `json:"type"`
	Min       []float64 `json:"min,omitempty"`
	Max       []float64 `json:"max,omitempty"`
}
type sceneWriteMesh struct {
	Name       string                `json:"name,omitempty"`
	Primitives []sceneWritePrimitive `json:"primitives"`
}
type sceneWriteRoot struct {
	Name  string `json:"name,omitempty"`
	Nodes []int  `json:"nodes,omitempty"`
}
type sceneWriteDocument struct {
	Asset       gltfAsset            `json:"asset"`
	Scene       int                  `json:"scene"`
	Scenes      []sceneWriteRoot     `json:"scenes"`
	Nodes       []sceneWriteNode     `json:"nodes,omitempty"`
	Meshes      []sceneWriteMesh     `json:"meshes,omitempty"`
	Buffers     []gltfBuffer         `json:"buffers,omitempty"`
	BufferViews []readView           `json:"bufferViews,omitempty"`
	Accessors   []sceneWriteAccessor `json:"accessors,omitempty"`
}

// WriteGLTFScene preserves definitions, instances and names in an embedded glTF
// scene. Local determines node placement; World is derived and ignored. Writer
// resource limits are the default SceneLimits. Float32 overflow/collapse fails.
func WriteGLTFScene(w io.Writer, s MeshScene) error { return writeMeshScene(w, s, false) }

// WriteGLBScene writes the same scene as WriteGLTFScene in a GLB2 container.
func WriteGLBScene(w io.Writer, s MeshScene) error { return writeMeshScene(w, s, true) }
func writeMeshScene(w io.Writer, s MeshScene, glb bool) error {
	if w == nil {
		return errors.New("exchange: nil scene writer")
	}
	l, _ := sceneLimits(SceneLimits{})
	if !utf8.ValidString(s.Name) || len(s.Nodes) > l.MaxNodes || len(s.Meshes) > l.MaxMeshes {
		return errors.New("exchange: invalid scene name or definition limit")
	}
	d := sceneWriteDocument{Asset: gltfAsset{"2.0", "PlainSolid"}, Scenes: []sceneWriteRoot{{s.Name, append([]int(nil), s.Roots...)}}}
	for _, n := range s.Nodes {
		if !utf8.ValidString(n.Name) || n.Mesh < -1 || n.Mesh >= len(s.Meshes) {
			return errors.New("exchange: invalid scene node")
		}
		matrix := make([]float64, 16)
		matrix[15] = 1
		for row := 0; row < 3; row++ {
			for col := 0; col < 4; col++ {
				matrix[col*4+row] = n.Local[row][col]
			}
		}
		if _, e := sceneTransform(sceneInputNode{Matrix: matrix}); e != nil {
			return e
		}
		node := sceneWriteNode{Name: n.Name, Children: append([]int(nil), n.Children...), Matrix: matrix}
		if n.Mesh >= 0 {
			index := n.Mesh
			node.Mesh = &index
		}
		d.Nodes = append(d.Nodes, node)
	}
	var bin []byte
	vertices, triangles := 0, 0
	for _, definition := range s.Meshes {
		m := definition.Geometry
		if !utf8.ValidString(definition.Name) || len(m.Triangles) == 0 || len(m.Vertices) > l.MaxVertices-vertices || len(m.Triangles) > l.MaxTriangles-triangles {
			return errors.New("exchange: invalid mesh definition or total geometry limit")
		}
		vertices += len(m.Vertices)
		triangles += len(m.Triangles)
		// Reuse the qualified float32 conversion and indexed triangle writer. Each
		// temporary GLB is discarded after its aligned payload and views are merged.
		var encoded bytes.Buffer
		if e := WriteGLB(&encoded, m); e != nil {
			return e
		}
		raw := encoded.Bytes()
		n := int(binary.LittleEndian.Uint32(raw[12:]))
		var part readGLTF
		if e := json.Unmarshal(raw[20:20+n], &part); e != nil {
			return e
		}
		payload := raw[28+n:]
		if len(payload) > int(l.MaxBytes)-len(bin) {
			return errors.New("exchange: scene buffer limit")
		}
		base := len(bin)
		bin = append(bin, payload...)
		viewBase, accessorBase := len(d.BufferViews), len(d.Accessors)
		for _, v := range part.BufferViews {
			v.Offset += base
			d.BufferViews = append(d.BufferViews, v)
		}
		for _, a := range part.Accessors {
			view := *a.View + viewBase
			a.View = &view
			d.Accessors = append(d.Accessors, sceneWriteAccessor{a.View, a.Offset, a.Component, a.Count, a.Type, a.Min, a.Max})
		}
		p := part.Meshes[0].Primitives[0]
		p.Attributes = map[string]int{"POSITION": p.Attributes["POSITION"] + accessorBase}
		index := *p.Indices + accessorBase
		p.Indices = &index
		d.Meshes = append(d.Meshes, sceneWriteMesh{definition.Name, []sceneWritePrimitive{{p.Attributes, p.Indices, p.Mode}}})
	}
	if len(bin) > 0 {
		d.Buffers = []gltfBuffer{{ByteLength: len(bin)}}
	}
	// Validate staged output before touching the destination, including forest
	// ancestry and accumulated transform bounds after float32 conversion.
	js, e := json.Marshal(d)
	if e != nil {
		return e
	}
	if int64(len(js)) > l.MaxBytes {
		return errors.New("exchange: scene metadata byte limit")
	}
	if _, e = readScene(js, bin, true, l); e != nil {
		return e
	}
	if !glb {
		if len(bin) > 0 {
			d.Buffers[0].URI = "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(bin)
		}
		js, e = json.Marshal(d)
		if e != nil {
			return e
		}
		if int64(len(js))+1 > l.MaxBytes {
			return errors.New("exchange: scene JSON byte limit")
		}
		js = append(js, '\n')
		return writeSceneBytes(w, js)
	}
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	size := 20 + len(js)
	if len(bin) > 0 {
		size += 8 + len(bin)
	}
	if uint64(size) > math.MaxUint32 || int64(size) > l.MaxBytes {
		return errors.New("exchange: scene GLB byte limit")
	}
	output := make([]byte, size)
	copy(output, "glTF")
	binary.LittleEndian.PutUint32(output[4:], 2)
	binary.LittleEndian.PutUint32(output[8:], uint32(size))
	binary.LittleEndian.PutUint32(output[12:], uint32(len(js)))
	copy(output[16:], "JSON")
	copy(output[20:], js)
	if len(bin) > 0 {
		offset := 20 + len(js)
		binary.LittleEndian.PutUint32(output[offset:], uint32(len(bin)))
		copy(output[offset+4:], []byte{'B', 'I', 'N', 0})
		copy(output[offset+8:], bin)
	}
	return writeSceneBytes(w, output)
}
func writeSceneBytes(w io.Writer, b []byte) error {
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
