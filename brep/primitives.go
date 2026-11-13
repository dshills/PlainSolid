package brep

import (
	"fmt"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Prism extrudes a simple planar polygon by offset, retaining two polygon caps
// and quadrilateral side faces. The input may use either winding. Face holes and
// adjacent collinear vertices are unsupported; see mesh.Extrude for tolerance.
func Prism(polygon []geom.Vec3, offset geom.Vec3) (Solid, error) {
	m, err := mesh.Extrude(polygon, offset)
	if err != nil {
		return Solid{}, err
	}
	n := len(polygon)
	polygons := make([][]int, 0, n+2)
	bottom, top := make([]int, n), make([]int, n)
	for i := 0; i < n; i++ {
		bottom[i] = n - 1 - i
		top[i] = n + i
	}
	polygons = append(polygons, bottom, top)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		polygons = append(polygons, []int{i, j, j + n, i + n})
	}
	return fromPolygons(m.Vertices, polygons)
}

// Box constructs six planar faces sharing twelve edges and eight vertices.
func Box(min, max geom.Vec3) (Solid, error) {
	m, err := mesh.Box(min, max)
	if err != nil {
		return Solid{}, err
	}
	return fromPolygons(m.Vertices, [][]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}})
}

func fromPolygons(vertices []geom.Vec3, polygons [][]int) (Solid, error) {
	var edges []Edge
	faces := make([]Face, len(polygons))
	lookup := make(map[[2]int]int)
	for f, p := range polygons {
		for i, a := range p {
			b := p[(i+1)%len(p)]
			lo, hi := a, b
			if lo > hi {
				lo, hi = hi, lo
			}
			key := [2]int{lo, hi}
			id, ok := lookup[key]
			if !ok {
				id = len(edges)
				lookup[key] = id
				edges = append(edges, Edge{Start: lo, End: hi})
			}
			faces[f].Boundary = append(faces[f].Boundary, EdgeUse{Edge: id, Reversed: a > b})
		}
	}
	return NewSolid(vertices, edges, faces)
}

// PrismWithHoles extrudes a planar outer polygon with through-holes. Outer and
// inner input windings are normalized automatically. Holes must be strictly
// contained, disjoint, and nonnested; touching loops and consecutive collinear
// vertices are rejected. Holed caps support at most MaxHoledFaceVertices total
// boundary vertices and use a scale-relative 1e-12 geometric resolution.
func PrismWithHoles(outer []geom.Vec3, holes [][]geom.Vec3, offset geom.Vec3) (Solid, error) {
	if len(holes) == 0 {
		return Prism(outer, offset)
	}
	count := len(outer)
	for _, hole := range holes {
		count += len(hole)
	}
	if count > MaxHoledFaceVertices {
		return Solid{}, fmt.Errorf("brep: holed prism exceeds %d vertex work limit", MaxHoledFaceVertices)
	}
	base, err := mesh.Extrude(outer, offset)
	if err != nil {
		return Solid{}, err
	}
	normalized := append([]geom.Vec3(nil), base.Vertices[:len(outer)]...)
	normal := geom.Vec3{}
	origin := normalized[0]
	scale := 0.0
	for _, v := range normalized {
		scale = math.Max(scale, v.Sub(origin).Length())
	}
	for i, v := range normalized {
		normal = normal.Add(v.Sub(origin).Scale(1 / scale).Cross(normalized[(i+1)%len(normalized)].Sub(origin).Scale(1 / scale)))
	}
	normalizedHoles := make([][]geom.Vec3, len(holes))
	for i, hole := range holes {
		copy := append([]geom.Vec3(nil), hole...)
		n := geom.Vec3{}
		for j, v := range copy {
			n = n.Add(v.Sub(origin).Scale(1 / scale).Cross(copy[(j+1)%len(copy)].Sub(origin).Scale(1 / scale)))
		}
		if n.Dot(normal) > 0 {
			for j, k := 0, len(copy)-1; j < k; j, k = j+1, k-1 {
				copy[j], copy[k] = copy[k], copy[j]
			}
		}
		normalizedHoles[i] = copy
	}
	if _, err = triangulateRings(normalized, normalizedHoles); err != nil {
		return Solid{}, err
	}
	rings := append([][]geom.Vec3{normalized}, normalizedHoles...)
	vertices := make([]geom.Vec3, 0, 2*count)
	var indices [][]int
	for _, ring := range rings {
		ids := make([]int, len(ring))
		for i, v := range ring {
			ids[i] = len(vertices)
			vertices = append(vertices, v)
		}
		indices = append(indices, ids)
	}
	for i := 0; i < count; i++ {
		vertices = append(vertices, vertices[i].Add(offset))
	}
	bottom, top := make([][]int, len(indices)), make([][]int, len(indices))
	var sides [][][]int
	for r, ids := range indices {
		bottom[r] = append([]int(nil), ids...)
		reverseIDs(bottom[r])
		top[r] = make([]int, len(ids))
		for i, id := range ids {
			top[r][i] = id + count
			j := ids[(i+1)%len(ids)]
			sides = append(sides, [][]int{{id, j, j + count, id + count}})
		}
	}
	polygons := append([][][]int{bottom, top}, sides...)
	return fromPolygonFaces(vertices, polygons)
}

func fromPolygonFaces(vertices []geom.Vec3, polygons [][][]int) (Solid, error) {
	var edges []Edge
	faces := make([]Face, len(polygons))
	lookup := make(map[[2]int]int)
	for f, rings := range polygons {
		for r, p := range rings {
			uses := make([]EdgeUse, len(p))
			for i, a := range p {
				b := p[(i+1)%len(p)]
				lo, hi := a, b
				if lo > hi {
					lo, hi = hi, lo
				}
				key := [2]int{lo, hi}
				id, ok := lookup[key]
				if !ok {
					id = len(edges)
					lookup[key] = id
					edges = append(edges, Edge{Start: lo, End: hi})
				}
				uses[i] = EdgeUse{Edge: id, Reversed: a > b}
			}
			if r == 0 {
				faces[f].Boundary = uses
			} else {
				faces[f].Holes = append(faces[f].Holes, uses)
			}
		}
	}
	return NewSolid(vertices, edges, faces)
}
