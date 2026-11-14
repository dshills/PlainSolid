package draft

import (
	"math/big"
	"slices"
	"strings"

	"plainsolid/brep"
)

type insetAlgebraicBoundary struct {
	points  [][3]insetAlgebraic
	edges   []brep.Edge
	faces   []brep.Face
	planes  []insetRadicalPlane
	sources []int
}

type insetVertex struct {
	point    [3]insetAlgebraic
	supports []int
}

// Solve the independent plane triple with rational cofactors. The result is
// a convex-polyhedron vertex only if every shifted halfspace agrees.
func insetPlaneIntersection(planes []insetRadicalPlane, triple [3]int, f *insetRadicalField) (*insetVertex, error) {
	if !f.work.tick() {
		return nil, f.work.err
	}
	a, b, c := planes[triple[0]], planes[triple[1]], planes[triple[2]]
	cofactors := [3]point{cross(b.Normal, c.Normal), cross(c.Normal, a.Normal), cross(a.Normal, b.Normal)}
	det := dot(a.Normal, cofactors[0])
	f.work.check(det)
	if f.work.err != nil || det.Sign() == 0 {
		return nil, f.work.err
	}
	var vertex insetVertex
	for axis := range vertex.point {
		x := f.scaled(a.Offset, div(cofactors[0][axis], det))
		x = f.plus(x, f.scaled(b.Offset, div(cofactors[1][axis], det)))
		vertex.point[axis] = f.plus(x, f.scaled(c.Offset, div(cofactors[2][axis], det)))
	}
	vertex.supports = append(vertex.supports, triple[:]...)
	for i, plane := range planes {
		if i == triple[0] || i == triple[1] || i == triple[2] {
			continue
		}
		if !f.work.tick() {
			return nil, f.work.err
		}
		x := f.scaled(vertex.point[0], plane.Normal[0])
		for axis := 1; axis < 3; axis++ {
			x = f.plus(x, f.scaled(vertex.point[axis], plane.Normal[axis]))
		}
		sign := f.sign(f.minus(x, plane.Offset))
		if f.work.err != nil {
			return nil, f.work.err
		}
		if sign > 0 {
			return nil, nil
		}
		if sign == 0 {
			vertex.supports = append(vertex.supports, i)
		}
	}
	return &vertex, f.work.err
}

func insetConvexAlgebraicBoundary(planes []insetRadicalPlane, f *insetRadicalField) (insetAlgebraicBoundary, error) {
	var empty insetAlgebraicBoundary
	var boundary insetAlgebraicBoundary
	verticesByPlane := make([][]int, len(planes))
	seen := make(map[string]bool)
	for i := range planes {
		for j := i + 1; j < len(planes); j++ {
			for k := j + 1; k < len(planes); k++ {
				vertex, err := insetPlaneIntersection(planes, [3]int{i, j, k}, f)
				if err != nil {
					return empty, err
				}
				if vertex == nil {
					continue
				}
				key := insetVertexKey(vertex.point, len(f.radicals))
				if seen[key] {
					continue
				}
				seen[key] = true
				if len(boundary.points) == f.work.o.MaxVertices {
					return empty, ErrLimit
				}
				id := len(boundary.points)
				boundary.points = append(boundary.points, vertex.point)
				for _, p := range vertex.supports {
					verticesByPlane[p] = append(verticesByPlane[p], id)
				}
			}
		}
	}
	if !insetFullDimension(boundary.points, f) {
		return empty, f.work.err
	}
	shared := make(map[[2]int]int)
	used := make([]bool, len(boundary.points))
	for i, plane := range planes {
		ring, err := insetFacetHull(verticesByPlane[i], boundary.points, plane.Normal, f)
		if err != nil {
			return empty, err
		}
		if len(ring) < 3 {
			continue
		}
		var face brep.Face
		for j, start := range ring {
			end := ring[(j+1)%len(ring)]
			key := [2]int{min(start, end), max(start, end)}
			id, ok := shared[key]
			if !ok {
				id = len(boundary.edges)
				shared[key] = id
				boundary.edges = append(boundary.edges, brep.Edge{Start: key[0], End: key[1]})
			}
			face.Boundary = append(face.Boundary, brep.EdgeUse{Edge: id, Reversed: start > end})
			used[start] = true
		}
		boundary.faces = append(boundary.faces, face)
		boundary.planes = append(boundary.planes, plane)
		boundary.sources = append(boundary.sources, i)
	}
	for _, retained := range used {
		if !retained {
			return empty, ErrUnsupported
		}
	}
	return boundary, f.work.err
}

func insetVertexKey(p [3]insetAlgebraic, terms int) string {
	var key strings.Builder
	for _, coordinate := range p {
		for _, coefficient := range coordinate[:terms] {
			if coefficient != nil {
				key.WriteString(coefficient.RatString())
			}
			key.WriteByte(',')
		}
	}
	return key.String()
}

func insetFullDimension(points [][3]insetAlgebraic, f *insetRadicalField) bool {
	if len(points) < 4 {
		return false
	}
	delta := func(p [3]insetAlgebraic) (out [3]insetAlgebraic) {
		for i := range p {
			out[i] = f.minus(p[i], points[0][i])
		}
		return
	}
	first := delta(points[1])
	var normal [3]insetAlgebraic
	independent := false
	for _, p := range points[2:] {
		if !f.work.tick() {
			return false
		}
		v := delta(p)
		for i := range normal {
			j, k := (i+1)%3, (i+2)%3
			normal[i] = f.minus(f.product(first[j], v[k]), f.product(first[k], v[j]))
			independent = independent || normal[i] != (insetAlgebraic{})
		}
		if independent {
			break
		}
	}
	if !independent {
		return false
	}
	for _, p := range points[2:] {
		if !f.work.tick() {
			return false
		}
		v := delta(p)
		var height insetAlgebraic
		for i := range normal {
			height = f.plus(height, f.product(normal[i], v[i]))
		}
		if height != (insetAlgebraic{}) {
			return true
		}
	}
	return false
}

func insetFacetHull(ids []int, points [][3]insetAlgebraic, normal [3]*big.Rat, f *insetRadicalField) ([]int, error) {
	if len(ids) < 3 {
		return nil, nil
	}
	axis := 0
	for i := 1; i < 3; i++ {
		if new(big.Rat).Abs(normal[i]).Cmp(new(big.Rat).Abs(normal[axis])) > 0 {
			axis = i
		}
	}
	x, y := (axis+1)%3, (axis+2)%3
	slices.SortFunc(ids, func(a, b int) int {
		if !f.work.tick() {
			return 0
		}
		if cmp := f.sign(f.minus(points[a][x], points[b][x])); cmp != 0 {
			return cmp
		}
		return f.sign(f.minus(points[a][y], points[b][y]))
	})
	turn := func(a, b, c int) int {
		if !f.work.tick() {
			return 0
		}
		abx, aby := f.minus(points[b][x], points[a][x]), f.minus(points[b][y], points[a][y])
		acx, acy := f.minus(points[c][x], points[a][x]), f.minus(points[c][y], points[a][y])
		return f.sign(f.minus(f.product(abx, acy), f.product(aby, acx)))
	}
	half := func() []int {
		var out []int
		for _, id := range ids {
			for len(out) >= 2 && turn(out[len(out)-2], out[len(out)-1], id) <= 0 {
				out = out[:len(out)-1]
			}
			out = append(out, id)
		}
		return out
	}
	lower := half()
	slices.Reverse(ids)
	upper := half()
	if f.work.err != nil {
		return nil, f.work.err
	}
	if len(lower)+len(upper) < 5 {
		return nil, nil
	}
	ring := append(lower[:len(lower)-1], upper[:len(upper)-1]...)
	if normal[axis].Sign() < 0 {
		slices.Reverse(ring)
	}
	return ring, nil
}
