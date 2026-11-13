package exchange

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"strings"
)

// SceneLimits bounds total definition geometry and the node hierarchy.
// Shared mesh instances count once toward geometry limits.
type SceneLimits struct {
	Limits
	MaxNodes, MaxMeshes, MaxDepth int
}
type SceneMesh struct {
	Name     string
	Geometry mesh.Mesh
}

// SceneNode retains source indices. Mesh=-1 means a transform-only node.
type SceneNode struct {
	Name         string
	Mesh         int
	Children     []int
	Local, World geom.Affine
}
type MeshScene struct {
	Name   string
	Meshes []SceneMesh
	Nodes  []SceneNode
	Roots  []int
}
type sceneInputNode struct {
	Name                                 string
	Mesh                                 *int
	Children                             []int
	Matrix, Translation, Rotation, Scale []float64
}
type sceneInput struct {
	Asset  gltfAsset
	Scene  int
	Scenes []struct {
		Name  string
		Nodes []int
	}
	Nodes  []sceneInputNode
	Meshes []struct {
		Name       string
		Primitives []readPrimitive
	}
	Buffers     []gltfBuffer
	BufferViews []readView
	Accessors   []readAccessor
}

func sceneLimits(l SceneLimits) (SceneLimits, error) {
	var e error
	l.Limits, e = l.Limits.normalized()
	if e != nil {
		return l, e
	}
	if l.MaxNodes == 0 {
		l.MaxNodes = 100000
	}
	if l.MaxMeshes == 0 {
		l.MaxMeshes = 10000
	}
	if l.MaxDepth == 0 {
		l.MaxDepth = 256
	}
	if l.MaxNodes < 1 || l.MaxMeshes < 1 || l.MaxDepth < 1 || l.MaxDepth > 4096 {
		return l, errors.New("exchange: invalid scene limits")
	}
	return l, nil
}

// ReadGLTFScene reads one embedded-buffer triangle scene with multiple mesh
// definitions and rigid or nonuniform TRS nodes, preserving shared instances.
// Each mesh has one POSITION-only triangle primitive. Extensions, materials,
// animations, cameras, skins, unused nodes and multiply-parented nodes fail.
func ReadGLTFScene(r io.Reader, l SceneLimits) (MeshScene, error) {
	l, e := sceneLimits(l)
	if e != nil {
		return MeshScene{}, e
	}
	b, _, e := input(r, l.Limits)
	if e != nil {
		return MeshScene{}, e
	}
	return readScene(b, nil, false, l)
}

// ReadGLBScene applies ReadGLTFScene's scene subset to a GLB2 container.
func ReadGLBScene(r io.Reader, l SceneLimits) (MeshScene, error) {
	l, e := sceneLimits(l)
	if e != nil {
		return MeshScene{}, e
	}
	b, _, e := input(r, l.Limits)
	if e != nil {
		return MeshScene{}, e
	}
	fail := func() (MeshScene, error) { return MeshScene{}, errors.New("exchange: malformed scene GLB") }
	if len(b) < 20 || !bytes.Equal(b[:4], []byte("glTF")) || binary.LittleEndian.Uint32(b[4:]) != 2 || uint64(binary.LittleEndian.Uint32(b[8:])) != uint64(len(b)) {
		return fail()
	}
	pos := 12
	var js, bin []byte
	for pos < len(b) {
		if len(b)-pos < 8 {
			return fail()
		}
		n := uint64(binary.LittleEndian.Uint32(b[pos:]))
		kind := binary.LittleEndian.Uint32(b[pos+4:])
		pos += 8
		if n%4 != 0 || n > uint64(len(b)-pos) {
			return fail()
		}
		chunk := b[pos : pos+int(n)]
		pos += int(n)
		if js == nil {
			if kind != 0x4e4f534a || n == 0 {
				return fail()
			}
			js = chunk
		} else if bin == nil && kind == 0x004e4942 && n > 0 {
			bin = chunk
		} else {
			return fail()
		}
	}
	return readScene(js, bin, true, l)
}
func readScene(raw, bin []byte, glb bool, l SceneLimits) (MeshScene, error) {
	fail := func() (MeshScene, error) {
		return MeshScene{}, errors.New("exchange: invalid or unsupported glTF scene")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var d sceneInput
	if e := dec.Decode(&d); e != nil {
		return MeshScene{}, e
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return fail()
	}
	if d.Asset.Version != "2.0" || d.Scene != 0 || len(d.Scenes) != 1 || len(d.Nodes) > l.MaxNodes || len(d.Meshes) > l.MaxMeshes {
		return fail()
	}
	out := MeshScene{Name: d.Scenes[0].Name, Roots: append([]int(nil), d.Scenes[0].Nodes...), Nodes: make([]SceneNode, len(d.Nodes))}
	parents := make([]int, len(d.Nodes))
	for i := range parents {
		parents[i] = -1
	}
	for i, n := range d.Nodes {
		local, e := sceneTransform(n)
		if e != nil {
			return MeshScene{}, e
		}
		mi := -1
		if n.Mesh != nil {
			mi = *n.Mesh
			if mi < 0 || mi >= len(d.Meshes) {
				return fail()
			}
		}
		out.Nodes[i] = SceneNode{Name: n.Name, Mesh: mi, Children: append([]int(nil), n.Children...), Local: local}
		for _, c := range n.Children {
			if c < 0 || c >= len(d.Nodes) || parents[c] != -1 || c == i {
				return fail()
			}
			parents[c] = i
		}
	}
	visited := make([]bool, len(d.Nodes))
	type pending struct {
		id, depth int
		world     geom.Affine
	}
	stack := []pending{}
	for _, root := range out.Roots {
		if root < 0 || root >= len(d.Nodes) || parents[root] != -1 || visited[root] {
			return fail()
		}
		visited[root] = true
		stack = append(stack, pending{root, 1, geom.Identity()})
	}
	count := 0
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x.depth > l.MaxDepth {
			return fail()
		}
		count++
		n := &out.Nodes[x.id]
		n.World = x.world.Mul(n.Local)
		if _, e := n.World.Inverse(); e != nil {
			return fail()
		}
		for _, child := range n.Children {
			if visited[child] {
				return fail()
			}
			visited[child] = true
			stack = append(stack, pending{child, x.depth + 1, n.World})
		}
	}
	if count != len(d.Nodes) {
		return fail()
	}
	if len(d.Meshes) == 0 {
		if len(d.Buffers) != 0 || len(d.BufferViews) != 0 || len(d.Accessors) != 0 || len(bin) != 0 {
			return fail()
		}
		return out, nil
	}
	if len(d.Buffers) != 1 || d.Buffers[0].ByteLength <= 0 {
		return fail()
	}
	size := d.Buffers[0].ByteLength
	if glb {
		if d.Buffers[0].URI != "" || size > len(bin) || len(bin)-size > 3 {
			return fail()
		}
		for _, v := range bin[size:] {
			if v != 0 {
				return fail()
			}
		}
		bin = bin[:size]
	} else {
		const prefix = "data:application/octet-stream;base64,"
		if !strings.HasPrefix(d.Buffers[0].URI, prefix) || int64(size) > l.MaxBytes {
			return fail()
		}
		var e error
		bin, e = base64.StdEncoding.Strict().DecodeString(d.Buffers[0].URI[len(prefix):])
		if e != nil || len(bin) != size {
			return fail()
		}
	}
	vertices, triangles := 0, 0
	for _, definition := range d.Meshes {
		if len(definition.Primitives) != 1 {
			return fail()
		}
		p := definition.Primitives[0]
		pos, ok := p.Attributes["POSITION"]
		if !ok || len(p.Attributes) != 1 {
			return fail()
		}
		// Reuse the existing accessor decoder with compact remapped accessors/views,
		// avoiding repeated base64 decoding and whole-document copies per definition.
		sub := readGLTF{Asset: d.Asset, Scenes: []gltfScene{{Nodes: []int{0}}}, Buffers: []gltfBuffer{{ByteLength: len(bin)}}}
		zero := 0
		sub.Nodes = append(sub.Nodes, struct{ Mesh *int }{&zero})
		add := func(index int) (int, error) {
			if index < 0 || index >= len(d.Accessors) {
				return 0, errors.New("exchange: scene accessor index")
			}
			a := d.Accessors[index]
			if a.View == nil || *a.View < 0 || *a.View >= len(d.BufferViews) {
				return 0, errors.New("exchange: scene buffer view")
			}
			view := len(sub.BufferViews)
			sub.BufferViews = append(sub.BufferViews, d.BufferViews[*a.View])
			a.View = &view
			index = len(sub.Accessors)
			sub.Accessors = append(sub.Accessors, a)
			return index, nil
		}
		index, e := add(pos)
		if e != nil {
			return MeshScene{}, e
		}
		p.Attributes = map[string]int{"POSITION": index}
		if p.Indices != nil {
			index, e = add(*p.Indices)
			if e != nil {
				return MeshScene{}, e
			}
			p.Indices = &index
		}
		sub.Meshes = append(sub.Meshes, struct{ Primitives []readPrimitive }{[]readPrimitive{p}})
		encoded, e := json.Marshal(sub)
		if e != nil {
			return MeshScene{}, e
		}
		remaining := l.Limits
		remaining.MaxVertices -= vertices
		remaining.MaxTriangles -= triangles
		if remaining.MaxVertices <= 0 || remaining.MaxTriangles <= 0 {
			return fail()
		}
		m, e := readGLTFJSON(encoded, bin, true, remaining)
		if e != nil {
			return MeshScene{}, e
		}
		vertices += len(m.Vertices)
		triangles += len(m.Triangles)
		out.Meshes = append(out.Meshes, SceneMesh{definition.Name, m})
	}
	bounds := make([]geom.Box, len(out.Meshes))
	for i, m := range out.Meshes {
		bounds[i], _ = m.Geometry.Bounds()
	}
	for _, node := range out.Nodes {
		if node.Mesh < 0 {
			continue
		}
		b := bounds[node.Mesh]
		for mask := 0; mask < 8; mask++ {
			p := b.Min
			if mask&1 != 0 {
				p.X = b.Max.X
			}
			if mask&2 != 0 {
				p.Y = b.Max.Y
			}
			if mask&4 != 0 {
				p.Z = b.Max.Z
			}
			if !node.World.Apply(p).IsFinite() {
				return fail()
			}
		}
	}
	return out, nil
}
func sceneTransform(n sceneInputNode) (geom.Affine, error) {
	fail := func() (geom.Affine, error) { return geom.Affine{}, errors.New("exchange: invalid glTF node transform") }
	m := geom.Identity()
	if n.Matrix != nil {
		if len(n.Matrix) != 16 || n.Translation != nil || n.Rotation != nil || n.Scale != nil {
			return fail()
		}
		if n.Matrix[3] != 0 || n.Matrix[7] != 0 || n.Matrix[11] != 0 || n.Matrix[15] != 1 {
			return fail()
		}
		for row := 0; row < 3; row++ {
			for col := 0; col < 4; col++ {
				m[row][col] = n.Matrix[col*4+row]
			}
		}
		// glTF node matrices must be decomposable to TRS (no local shear).
		cols := [3]geom.Vec3{}
		for j := range cols {
			v := geom.Vec3{X: m[0][j], Y: m[1][j], Z: m[2][j]}
			u, e := v.Unit()
			if e != nil {
				return fail()
			}
			cols[j] = u
		}
		if math.Abs(cols[0].Dot(cols[1])) > 1e-9 || math.Abs(cols[0].Dot(cols[2])) > 1e-9 || math.Abs(cols[1].Dot(cols[2])) > 1e-9 {
			return fail()
		}
	} else {
		if n.Translation != nil {
			if len(n.Translation) != 3 {
				return fail()
			}
			for i := 0; i < 3; i++ {
				m[i][3] = n.Translation[i]
			}
		}
		if n.Rotation != nil {
			if len(n.Rotation) != 4 {
				return fail()
			}
			q := n.Rotation
			norm := math.Hypot(math.Hypot(q[0], q[1]), math.Hypot(q[2], q[3]))
			if math.Abs(norm-1) > 1e-6 {
				return fail()
			}
			x, y, z, w := q[0]/norm, q[1]/norm, q[2]/norm, q[3]/norm
			m[0][0], m[0][1], m[0][2] = 1-2*(y*y+z*z), 2*(x*y-z*w), 2*(x*z+y*w)
			m[1][0], m[1][1], m[1][2] = 2*(x*y+z*w), 1-2*(x*x+z*z), 2*(y*z-x*w)
			m[2][0], m[2][1], m[2][2] = 2*(x*z-y*w), 2*(y*z+x*w), 1-2*(x*x+y*y)
		}
		if n.Scale != nil {
			if len(n.Scale) != 3 {
				return fail()
			}
			for row := 0; row < 3; row++ {
				for col := 0; col < 3; col++ {
					m[row][col] *= n.Scale[col]
				}
			}
		}
	}
	if _, e := m.Inverse(); e != nil {
		return fail()
	}
	return m, nil
}
