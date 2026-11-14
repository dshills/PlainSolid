package collision

import (
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Memo entries own all exact numbers. Predicate arithmetic never mutates its
// arguments. The bounded prefix avoids an unbounded retained big-number cache
// for large meshes; uncached IDs retain exactly the same arithmetic.
const exactTriangleMemoLimit = 4096

type preparedExactTriangle struct {
	vertices [3]point
	normal   point
	offset   scalar
}

func prepareExactTriangle(t [3]geom.Vec3) preparedExactTriangle {
	vertices := exactTriangle(t)
	normal := triangleNormal(vertices)
	return preparedExactTriangle{vertices, normal, dot(normal, vertices[0])}
}

type exactTriangleMemo struct {
	mesh   mesh.Mesh
	values []*preparedExactTriangle
}

func newExactTriangleMemo(m mesh.Mesh) exactTriangleMemo {
	n := len(m.Triangles)
	if n > exactTriangleMemoLimit {
		n = exactTriangleMemoLimit
	}
	return exactTriangleMemo{mesh: m, values: make([]*preparedExactTriangle, n)}
}
func (m *exactTriangleMemo) get(id int) preparedExactTriangle {
	if id >= len(m.values) {
		return prepareExactTriangle(triangle(m.mesh, id))
	}
	if m.values[id] == nil {
		v := prepareExactTriangle(triangle(m.mesh, id))
		m.values[id] = &v
	}
	return *m.values[id]
}
