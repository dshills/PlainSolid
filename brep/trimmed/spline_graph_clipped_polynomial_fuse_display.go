package trimmed

import (
	"cmp"
	"context"
	"math"
	"slices"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

type fuseDisplayPoint struct {
	uv geom.Vec2
	id int
}

type fuseDisplayRow struct {
	points []fuseDisplayPoint
	// traceEnds indexes the ends of the two outer traces in the
	// previous and next slabs, respectively.
	traceEnds [2][]int
}

type fuseDisplaySample struct {
	id, station int
	parameter   float64
}

type fuseDisplaySide struct {
	trimSide
	start, end float64
}

func fuseDisplayCoordinate(p geom.Vec3, axis int) float64 {
	return [2]float64{p.X, p.Y}[axis]
}

// The clipped Fuse proves affine XY charts and polynomial traces
// in a common free coordinate. Slicing both owners at the same stations
// keeps thin contact regions from being filled by nearly parallel ears.
func (s Solid) tessellateClippedFuse(ctx context.Context, axis int, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if len(s.vertices) > o.MaxTriangles {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	levels := make([]float64, len(s.vertices))
	for i, v := range s.vertices {
		levels[i] = fuseDisplayCoordinate(v, axis)
	}
	slices.Sort(levels)
	levels = slices.Compact(levels)
	if len(levels) < 2 {
		return mesh.Mesh{}, errTrimFold
	}
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		// Preserve every native station. New cuts are shared by all
		// traces that cross them, including edges with different ends.
		stations := []float64{levels[0]}
		width := (levels[len(levels)-1] - levels[0]) / float64(n)
		if !finite(width) || width <= 0 {
			return mesh.Mesh{}, errTrimFold
		}
		for i, end := range levels[1:] {
			start := levels[i]
			count := max(1, int(math.Ceil((end-start)/width)))
			for j := 1; j < count; j++ {
				q := start + (end-start)*(float64(j)/float64(count))
				if q <= stations[len(stations)-1] || q >= end {
					return mesh.Mesh{}, errTrimFold
				}
				stations = append(stations, q)
			}
			stations = append(stations, end)
		}
		m, err := s.clippedFuseDisplayCandidate(ctx, axis, n, stations, o)
		if err != errRefine && err != errTrimFold {
			return m, err
		}
	}
	return mesh.Mesh{}, parametric.ErrBudget
}

func (s Solid) clippedFuseDisplayEdges(ctx context.Context, axis, n int, stations []float64, o parametric.TessellationOptions,
	m *mesh.Mesh) ([][]fuseDisplaySample, error) {
	edges := make([][]fuseDisplaySample, len(s.edges))
	for i, edge := range s.edges {
		a, b := fuseDisplayCoordinate(s.vertices[edge.Start], axis), fuseDisplayCoordinate(s.vertices[edge.End], axis)
		start, _ := slices.BinarySearch(stations, a)
		end, _ := slices.BinarySearch(stations, b)
		count, step := n, 0
		if start < end {
			count, step = end-start, 1
		} else if end < start {
			count, step = start-end, -1
		}
		if count > o.MaxSubdivisions || count-1 > o.MaxTriangles-len(m.Vertices) {
			return nil, parametric.ErrBudget
		}
		points := make([]fuseDisplaySample, count+1)
		points[0], points[count] = fuseDisplaySample{edge.Start, start, 0}, fuseDisplaySample{edge.End, end, 1}
		for j := 1; j < count; j++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			station := start + step*j
			parameter := float64(j) / float64(count)
			if step != 0 {
				parameter = (stations[station] - a) / (b - a)
			}
			if parameter <= points[j-1].parameter || parameter >= 1 {
				return nil, errTrimFold
			}
			p, e := s.onEdge(i, parameter)
			if e != nil {
				return nil, e
			}
			if !p.IsFinite() || math.Abs(fuseDisplayCoordinate(p, axis)-stations[station]) > s.tolerance {
				return nil, errTrimFold
			}
			points[j] = fuseDisplaySample{len(m.Vertices), station, parameter}
			m.Vertices = append(m.Vertices, p)
		}
		edges[i] = points
	}
	return edges, nil
}

func (s Solid) clippedFuseDisplayCandidate(ctx context.Context, axis, n int, stations []float64, o parametric.TessellationOptions) (mesh.Mesh, error) {
	m := mesh.Mesh{Vertices: s.Vertices()}
	edges, e := s.clippedFuseDisplayEdges(ctx, axis, n, stations, o, &m)
	if e != nil {
		return mesh.Mesh{}, e
	}
	for _, face := range s.faces {
		if len(face.Holes) != 0 {
			return mesh.Mesh{}, errTrimFold
		}
		boundary := make(map[[2]int]fuseDisplaySide)
		ring := make([]fuseDisplayPoint, 0, len(face.Outer))
		rows := make([]fuseDisplayRow, len(stations))
		_, wall := face.Surface.(splineGraphAffineWall)
		for _, use := range face.Outer {
			points := edges[use.Edge]
			start, end, step := 0, len(points)-1, 1
			if use.Reversed {
				start, end, step = end, start, -1
			}
			side := trimSide{use.Edge, use}
			for j := start; j != end; j += step {
				if err := ctx.Err(); err != nil {
					return mesh.Mesh{}, err
				}
				a, b := points[j], points[j+step]
				uv := use.UVStart
				if j != start {
					uv, e = side.point(a.parameter)
					if e != nil {
						return mesh.Mesh{}, e
					}
				}
				p, e := face.Surface.Eval(uv.X, uv.Y)
				if e != nil || !closePoint(p, m.Vertices[a.id], s.tolerance) {
					return mesh.Mesh{}, errTrimFold
				}
				point := fuseDisplayPoint{uv, a.id}
				boundary[[2]int{a.id, b.id}] = fuseDisplaySide{side, a.parameter, b.parameter}
				if wall {
					ring = append(ring, point)
				} else {
					rows[a.station].points = append(rows[a.station].points, point)
					if a.station != b.station {
						left, right := a, b
						if a.station > b.station {
							left, right = b, a
						}
						leftEnds := &rows[left.station].traceEnds[1]
						rightEnds := &rows[right.station].traceEnds[0]
						*leftEnds = append(*leftEnds, left.id)
						*rightEnds = append(*rightEnds, right.id)
					}
				}
			}
		}
		emit := func(points [3]fuseDisplayPoint) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(m.Triangles) == o.MaxTriangles {
				return parametric.ErrBudget
			}
			turn, e := geom.Orient2D(points[0].uv, points[1].uv, points[2].uv)
			if e != nil || turn == 0 {
				return errTrimFold
			}
			if turn < 0 {
				points[1], points[2] = points[2], points[1]
			}
			var tri samples
			var xyz [3]geom.Vec3
			for j, p := range points {
				tri.ids[j], tri.uv[j], xyz[j] = p.id, p.uv, m.Vertices[p.id]
				tri.edge[j] = internalSide()
				key := [2]int{p.id, points[(j+1)%3].id}
				if side, ok := boundary[key]; ok {
					tri.edge[j], tri.start[j], tri.end[j] = side.trimSide, side.start, side.end
					delete(boundary, key)
				}
			}
			if e = check(face.Surface, tri, xyz, o.Deflection, o.Angle); e != nil {
				return e
			}
			if face.Reversed {
				tri.ids[1], tri.ids[2] = tri.ids[2], tri.ids[1]
			}
			m.Triangles = append(m.Triangles, tri.ids)
			return nil
		}
		if wall {
			e = emitFuseDisplayWall(ctx, ring, emit)
		} else {
			e = emitFuseDisplayCap(ctx, face.Surface, rows, axis, n, &m, o.MaxTriangles, boundary, emit)
		}
		if e != nil {
			return mesh.Mesh{}, e
		}
		if len(boundary) != 0 {
			return mesh.Mesh{}, errTrimFold
		}
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, errTrimFold
	}
	return m, nil
}

func emitFuseDisplayWall(ctx context.Context, ring []fuseDisplayPoint, emit func([3]fuseDisplayPoint) error) error {
	uv := make([]geom.Vec3, len(ring))
	points := make(map[geom.Vec2]fuseDisplayPoint, len(ring))
	for i, p := range ring {
		uv[i] = geom.Vec3{X: p.uv.X, Y: p.uv.Y}
		points[p.uv] = p
	}
	triangles, e := triangulateUVBounded(ctx, [][]geom.Vec3{uv}, brep.MaxHoledFaceVertices)
	if e != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errTrimFold
	}
	for _, tri := range triangles {
		if e := emit([3]fuseDisplayPoint{points[tri[0]], points[tri[1]], points[tri[2]]}); e != nil {
			return e
		}
	}
	return nil
}

func emitFuseDisplayCap(ctx context.Context, surface geom.Surface, rows []fuseDisplayRow, axis, n int,
	m *mesh.Mesh, maxTriangles int, boundary map[[2]int]fuseDisplaySide, emit func([3]fuseDisplayPoint) error) error {
	first := slices.IndexFunc(rows, func(row fuseDisplayRow) bool { return len(row.points) != 0 })
	if first < 0 {
		return errTrimFold
	}
	rows = rows[first:]
	for len(rows[len(rows)-1].points) == 0 {
		rows = rows[:len(rows)-1]
	}
	transverse := func(p fuseDisplayPoint) float64 {
		return fuseDisplayCoordinate(m.Vertices[p.id], 1-axis)
	}
	var previous []fuseDisplayPoint
	for i, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		sides, e := splitFuseDisplayRow(surface, row, transverse, n, maxTriangles, m, boundary)
		if e != nil {
			return e
		}
		if (i == 0) != (len(sides[0]) == 0) || (i == len(rows)-1) != (len(sides[1]) == 0) {
			return errTrimFold
		}
		if previous != nil {
			if e := emitFuseDisplayStrip(previous, sides[0], transverse, emit); e != nil {
				return e
			}
		}
		previous = sides[1]
	}
	return nil
}

// A stock can start or end halfway across this row. Its exposed native
// segments belong to one slab; the remaining interval is shared by both.
func splitFuseDisplayRow(surface geom.Surface, row fuseDisplayRow, transverse func(fuseDisplayPoint) float64, n, maxTriangles int,
	m *mesh.Mesh, boundary map[[2]int]fuseDisplaySide) ([2][]fuseDisplayPoint, error) {
	var sides [2][]fuseDisplayPoint
	points := row.points
	slices.SortFunc(points, func(a, b fuseDisplayPoint) int { return cmp.Compare(transverse(a), transverse(b)) })
	for j := 1; j < len(points); j++ {
		if transverse(points[j-1]) >= transverse(points[j]) {
			return sides, errTrimFold
		}
	}
	bounds := [2][2]int{{-1, -1}, {-1, -1}}
	for i, ends := range row.traceEnds {
		if len(ends) == 0 {
			continue
		}
		if len(ends) != 2 {
			return sides, errTrimFold
		}
		var at [2]int
		for j, id := range ends {
			at[j] = slices.IndexFunc(points, func(p fuseDisplayPoint) bool { return p.id == id })
			if at[j] < 0 {
				return sides, errTrimFold
			}
		}
		bounds[i] = [2]int{min(at[0], at[1]), max(at[0], at[1])}
	}
	for i, a := range points {
		for j, span := range bounds {
			if span[0] <= i && i <= span[1] {
				sides[j] = append(sides[j], a)
			}
		}
		if i == len(points)-1 {
			break
		}
		b := points[i+1]
		before := bounds[0][0] <= i && i < bounds[0][1]
		after := bounds[1][0] <= i && i < bounds[1][1]
		if (!before && !after) || hasFuseDisplaySide(boundary, a, b) != (before != after) {
			return sides, errTrimFold
		}
		if before && after {
			if n-1 > maxTriangles-len(m.Vertices) {
				return sides, parametric.ErrBudget
			}
			for j := 1; j < n; j++ {
				uv := mix(a.uv, b.uv, float64(j)/float64(n))
				v, e := surface.Eval(uv.X, uv.Y)
				if e != nil || !v.IsFinite() {
					return sides, errTrimFold
				}
				p := fuseDisplayPoint{uv, len(m.Vertices)}
				m.Vertices = append(m.Vertices, v)
				sides[0] = append(sides[0], p)
				sides[1] = append(sides[1], p)
			}
		}
	}
	return sides, nil
}

func hasFuseDisplaySide(boundary map[[2]int]fuseDisplaySide, a, b fuseDisplayPoint) bool {
	_, forward := boundary[[2]int{a.id, b.id}]
	_, backward := boundary[[2]int{b.id, a.id}]
	return forward || backward
}

func emitFuseDisplayStrip(a, b []fuseDisplayPoint, coordinate func(fuseDisplayPoint) float64, emit func([3]fuseDisplayPoint) error) error {
	if len(a)+len(b) < 3 {
		return errTrimFold
	}
	amin, awidth := coordinate(a[0]), coordinate(a[len(a)-1])-coordinate(a[0])
	bmin, bwidth := coordinate(b[0]), coordinate(b[len(b)-1])-coordinate(b[0])
	i, j := 0, 0
	for i < len(a)-1 || j < len(b)-1 {
		var tri [3]fuseDisplayPoint
		// Match physical fractions along each row. Matching indices
		// would skew the diagonal where a wall adds a short segment.
		if j == len(b)-1 || i < len(a)-1 && (coordinate(a[i+1])-amin)*bwidth <= (coordinate(b[j+1])-bmin)*awidth {
			tri = [3]fuseDisplayPoint{a[i], a[i+1], b[j]}
			i++
		} else {
			tri = [3]fuseDisplayPoint{a[i], b[j], b[j+1]}
			j++
		}
		if e := emit(tri); e != nil {
			return e
		}
	}
	return nil
}
