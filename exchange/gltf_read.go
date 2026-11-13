package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"plainsolid/geom"
	"plainsolid/mesh"
)

type readView struct {
	Buffer int `json:"buffer"`
	Offset int `json:"byteOffset"`
	Length int `json:"byteLength"`
	Stride int `json:"byteStride"`
	Target int `json:"target"`
}
type readAccessor struct {
	View      *int   `json:"bufferView"`
	Offset    int    `json:"byteOffset"`
	Component int    `json:"componentType"`
	Count     int    `json:"count"`
	Type      string `json:"type"`
	Min, Max  []float64
}
type readPrimitive struct {
	Attributes    map[string]int `json:"attributes"`
	Indices, Mode *int
}
type readGLTF struct {
	Asset       gltfAsset
	Scene       int
	Scenes      []gltfScene
	Nodes       []struct{ Mesh *int }
	Meshes      []struct{ Primitives []readPrimitive }
	Buffers     []gltfBuffer
	BufferViews []readView
	Accessors   []readAccessor
}

type accessorData struct {
	data   []byte
	stride int
}

func getGLTFData(d readGLTF, buffer []byte, a readAccessor, width, alignment int, positions bool) (accessorData, error) {
	bad := func() (accessorData, error) {
		return accessorData{}, fmt.Errorf("exchange: glTF accessor outside valid buffer range or alignment")
	}
	if a.View == nil || *a.View < 0 || *a.View >= len(d.BufferViews) || a.Offset < 0 || a.Offset%alignment != 0 || a.Count <= 0 {
		return bad()
	}
	v := d.BufferViews[*a.View]
	if v.Buffer != 0 || v.Offset < 0 || v.Length <= 0 || v.Offset > len(buffer) || v.Length > len(buffer)-v.Offset || v.Offset%alignment != 0 || a.Offset > v.Length {
		return bad()
	}
	stride := v.Stride
	if positions {
		if (v.Target != 0 && v.Target != 34962) || stride != 0 && (stride < width || stride > 252 || stride%4 != 0) {
			return bad()
		}
	} else if v.Target != 0 && v.Target != 34963 || stride != 0 {
		return bad()
	}
	if stride == 0 {
		stride = width
	}
	if width > v.Length-a.Offset || a.Count-1 > (v.Length-a.Offset-width)/stride {
		return bad()
	}
	from := v.Offset + a.Offset
	to := from + (a.Count-1)*stride + width
	return accessorData{buffer[from:to], stride}, nil
}

// ReadGLTF reads a self-contained glTF 2.0 JSON triangle scene containing one
// identity mesh instance, one primitive and one embedded base64 buffer. It
// supports float32 positions and uint8/16/32 or implicit triangle indices.
// Other scene features, extensions, external buffers and GLB are rejected.
// Units and coordinate orientation are returned unchanged.
func ReadGLTF(r io.Reader, l Limits) (mesh.Mesh, error) {
	b, l, err := input(r, l)
	if err != nil {
		return mesh.Mesh{}, err
	}
	return readGLTFJSON(b, nil, false, l)
}
func readGLTFJSON(b, bin []byte, glb bool, l Limits) (mesh.Mesh, error) {
	var err error
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var d readGLTF
	if err = dec.Decode(&d); err != nil {
		return mesh.Mesh{}, err
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return mesh.Mesh{}, fmt.Errorf("exchange: trailing glTF data")
	}
	if d.Asset.Version != "2.0" || len(d.Scenes) != 1 || d.Scene != 0 {
		return mesh.Mesh{}, fmt.Errorf("exchange: unsupported glTF version or scene")
	}
	nodes := d.Scenes[0].Nodes
	if len(nodes) == 0 && len(d.Nodes) == 0 && len(d.Meshes) == 0 && len(d.Buffers) == 0 && len(d.BufferViews) == 0 && len(d.Accessors) == 0 {
		if glb && len(bin) > 0 {
			return mesh.Mesh{}, fmt.Errorf("exchange: unexpected GLB buffer")
		}
		return mesh.Mesh{}, nil
	}
	if len(nodes) != 1 || nodes[0] != 0 || len(d.Nodes) != 1 || d.Nodes[0].Mesh == nil || *d.Nodes[0].Mesh != 0 ||
		len(d.Meshes) != 1 || len(d.Meshes[0].Primitives) != 1 || len(d.Buffers) != 1 {
		return mesh.Mesh{}, fmt.Errorf("exchange: unsupported glTF scene layout")
	}
	p := d.Meshes[0].Primitives[0]
	pos, ok := p.Attributes["POSITION"]
	if !ok || len(p.Attributes) != 1 || p.Mode != nil && *p.Mode != 4 || pos < 0 || pos >= len(d.Accessors) {
		return mesh.Mesh{}, fmt.Errorf("exchange: unsupported glTF attributes or primitive")
	}
	a := d.Accessors[pos]
	if a.Component != 5126 || a.Type != "VEC3" || a.Count <= 0 || a.Count > l.MaxVertices || len(a.Min) != 3 || len(a.Max) != 3 {
		return mesh.Mesh{}, fmt.Errorf("exchange: invalid glTF position accessor")
	}
	var raw []byte
	if glb {
		if d.Buffers[0].URI != "" || d.Buffers[0].ByteLength <= 0 || d.Buffers[0].ByteLength > len(bin) || len(bin)-d.Buffers[0].ByteLength > 3 {
			return mesh.Mesh{}, fmt.Errorf("exchange: invalid GLB buffer")
		}
		raw = bin[:d.Buffers[0].ByteLength]
		for _, v := range bin[len(raw):] {
			if v != 0 {
				return mesh.Mesh{}, fmt.Errorf("exchange: nonzero GLB padding")
			}
		}
	} else {
		const prefix = "data:application/octet-stream;base64,"
		if !strings.HasPrefix(d.Buffers[0].URI, prefix) || d.Buffers[0].ByteLength <= 0 || int64(d.Buffers[0].ByteLength) > l.MaxBytes {
			return mesh.Mesh{}, fmt.Errorf("exchange: unsupported glTF buffer")
		}
		raw, err = base64.StdEncoding.Strict().DecodeString(d.Buffers[0].URI[len(prefix):])
		if err != nil || len(raw) != d.Buffers[0].ByteLength {
			return mesh.Mesh{}, fmt.Errorf("exchange: invalid embedded glTF buffer")
		}
	}
	positions, err := getGLTFData(d, raw, a, 12, 4, true)
	if err != nil {
		return mesh.Mesh{}, err
	}
	count := a.Count
	var index accessorData
	var component int
	if p.Indices != nil {
		i := *p.Indices
		if i < 0 || i >= len(d.Accessors) {
			return mesh.Mesh{}, fmt.Errorf("exchange: invalid glTF index accessor")
		}
		x := d.Accessors[i]
		component = map[int]int{5121: 1, 5123: 2, 5125: 4}[x.Component]
		if x.Type != "SCALAR" || component == 0 {
			return mesh.Mesh{}, fmt.Errorf("exchange: unsupported glTF index type")
		}
		count = x.Count
		index, err = getGLTFData(d, raw, x, component, component, false)
		if err != nil {
			return mesh.Mesh{}, err
		}
	}
	if count < 3 || count%3 != 0 || count/3 > l.MaxTriangles {
		return mesh.Mesh{}, fmt.Errorf("exchange: invalid glTF triangle count or limit")
	}
	m := mesh.Mesh{Vertices: make([]geom.Vec3, a.Count), Triangles: make([][3]int, count/3)}
	lo, hi := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}, [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for i := range m.Vertices {
		var xyz [3]float64
		for j := range xyz {
			xyz[j] = float64(math.Float32frombits(binary.LittleEndian.Uint32(positions.data[i*positions.stride+j*4:])))
			lo[j], hi[j] = math.Min(lo[j], xyz[j]), math.Max(hi[j], xyz[j])
		}
		m.Vertices[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	for i := range lo {
		if lo[i] != float64(float32(a.Min[i])) || hi[i] != float64(float32(a.Max[i])) {
			return mesh.Mesh{}, fmt.Errorf("exchange: glTF position bounds disagree with buffer")
		}
	}
	for i := 0; i < count; i++ {
		x := uint32(i)
		switch component {
		case 1:
			x = uint32(index.data[i])
		case 2:
			x = uint32(binary.LittleEndian.Uint16(index.data[i*2:]))
		case 4:
			x = binary.LittleEndian.Uint32(index.data[i*4:])
		}
		if uint64(x) >= uint64(a.Count) || component == 1 && x == 255 || component == 2 && x == 65535 || component == 4 && x == math.MaxUint32 {
			return mesh.Mesh{}, fmt.Errorf("exchange: invalid glTF vertex index")
		}
		m.Triangles[i/3][i%3] = int(x)
	}
	if err = m.Validate(); err != nil {
		return mesh.Mesh{}, err
	}
	return m, nil
}
