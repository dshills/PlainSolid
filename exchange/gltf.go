package exchange

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"

	"plainsolid/geom"
	"plainsolid/mesh"
)

type gltfAsset struct {
	Version   string `json:"version"`
	Generator string `json:"generator"`
}
type gltfBuffer struct {
	URI        string `json:"uri,omitempty"`
	ByteLength int    `json:"byteLength"`
}
type gltfView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	Target     int `json:"target"`
}
type gltfAccessor struct {
	BufferView    int       `json:"bufferView"`
	ComponentType int       `json:"componentType"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	Min           []float64 `json:"min,omitempty"`
	Max           []float64 `json:"max,omitempty"`
}
type gltfPrimitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
	Mode       int            `json:"mode"`
}
type gltfMesh struct {
	Primitives []gltfPrimitive `json:"primitives"`
}
type gltfNode struct {
	Mesh int `json:"mesh"`
}
type gltfScene struct {
	Nodes []int `json:"nodes,omitempty"`
}
type gltfDocument struct {
	Asset       gltfAsset      `json:"asset"`
	Scene       int            `json:"scene"`
	Scenes      []gltfScene    `json:"scenes"`
	Nodes       []gltfNode     `json:"nodes,omitempty"`
	Meshes      []gltfMesh     `json:"meshes,omitempty"`
	Buffers     []gltfBuffer   `json:"buffers,omitempty"`
	BufferViews []gltfView     `json:"bufferViews,omitempty"`
	Accessors   []gltfAccessor `json:"accessors,omitempty"`
}

// WriteGLTF writes a self-contained glTF 2.0 JSON scene with one triangle mesh
// and an embedded base64 buffer. Positions are float32 and indices are uint32;
// normals, materials and CAD topology are not exported. Coordinates are written
// unchanged: callers must convert their units to meters and choose orientation
// for glTF's right-handed, Y-up coordinate convention. Float32 overflow or
// triangle collapse is rejected. An empty triangle mesh writes an empty scene.
// Memory usage is proportional to the mesh and its base64/JSON representation.
func WriteGLTF(w io.Writer, m mesh.Mesh) error { return writeGLTF(w, m, false) }
func writeGLTF(w io.Writer, m mesh.Mesh, glb bool) error {
	if w == nil {
		return errors.New("exchange: nil writer")
	}
	if e := m.Validate(); e != nil {
		return e
	}
	var binaryData []byte
	d := gltfDocument{Asset: gltfAsset{"2.0", "PlainSolid"}, Scenes: []gltfScene{{}}}
	if len(m.Triangles) > 0 {
		maxInt := int(^uint(0) >> 1)
		if uint64(len(m.Vertices)) > math.MaxUint32 || len(m.Vertices) > maxInt/12 || len(m.Triangles) > (maxInt-len(m.Vertices)*12)/12 {
			return errors.New("exchange: glTF buffer too large")
		}
		vertices := make([]geom.Vec3, len(m.Vertices))
		min, max := geom.Vec3{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}, geom.Vec3{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
		for i, v := range m.Vertices {
			p := geom.Vec3{X: float64(float32(v.X)), Y: float64(float32(v.Y)), Z: float64(float32(v.Z))}
			if !p.IsFinite() {
				return errors.New("exchange: glTF float32 overflow")
			}
			vertices[i] = p
			min = geom.Vec3{X: math.Min(min.X, p.X), Y: math.Min(min.Y, p.Y), Z: math.Min(min.Z, p.Z)}
			max = geom.Vec3{X: math.Max(max.X, p.X), Y: math.Max(max.Y, p.Y), Z: math.Max(max.Z, p.Z)}
		}
		rounded := mesh.Mesh{Vertices: vertices, Triangles: m.Triangles}
		if e := rounded.Validate(); e != nil {
			return fmt.Errorf("exchange: glTF precision: %w", e)
		}
		positionBytes := len(vertices) * 12
		indexBytes := len(m.Triangles) * 12
		data := make([]byte, positionBytes+indexBytes)
		for i, p := range vertices {
			for j, v := range []float64{p.X, p.Y, p.Z} {
				binary.LittleEndian.PutUint32(data[i*12+j*4:], math.Float32bits(float32(v)))
			}
		}
		for i, t := range m.Triangles {
			for j, v := range t {
				binary.LittleEndian.PutUint32(data[positionBytes+i*12+j*4:], uint32(v))
			}
		}
		d.Scenes[0].Nodes = []int{0}
		d.Nodes = []gltfNode{{0}}
		d.Meshes = []gltfMesh{{[]gltfPrimitive{{map[string]int{"POSITION": 0}, 1, 4}}}}
		if glb {
			binaryData = data
			d.Buffers = []gltfBuffer{{"", len(data)}}
		} else {
			d.Buffers = []gltfBuffer{{"data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(data), len(data)}}
		}
		d.BufferViews = []gltfView{{0, 0, positionBytes, 34962}, {0, positionBytes, indexBytes, 34963}}
		d.Accessors = []gltfAccessor{{0, 5126, len(vertices), "VEC3", []float64{min.X, min.Y, min.Z}, []float64{max.X, max.Y, max.Z}}, {1, 5125, len(m.Triangles) * 3, "SCALAR", nil, nil}}
	}
	if glb {
		return writeGLBChunks(w, d, binaryData)
	}
	b := bufio.NewWriter(w)
	if e := json.NewEncoder(b).Encode(d); e != nil {
		return e
	}
	return b.Flush()
}
