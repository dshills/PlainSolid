package poly

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// CombineAxisAligned applies an exact coordinate-grid Boolean when every input
// triangle lies on an axis-aligned plane. It uses only represented input plane
// coordinates; no diagonal intersections or new rounded centroids are emitted.
// handled=false means the inputs leave this bounded domain. Classification uses
// exact rational winding and output undergoes the ordinary embedding checks.
func CombineAxisAligned(ctx context.Context, a, b brep.Solid, op Operation, o Options) (result brep.Solid, handled bool, err error) {
	return combineAxisAlignedBudget(ctx, a, b, op, o, [2]int{})
}
func combineAxisAlignedBudget(ctx context.Context, a, b brep.Solid, op Operation, o Options, usage [2]int) (result brep.Solid, handled bool, err error) {
	if ctx == nil || op > Difference {
		return brep.Solid{}, true, ErrInvalid
	}
	o, err = defaults(o)
	if err != nil {
		return brep.Solid{}, true, err
	}
	bu := &budget{ctx: ctx, o: o, work: usage[0], fragments: usage[1]}
	if !bu.tick(1) {
		return brep.Solid{}, true, bu.err
	}
	left, err := input(a, bu)
	if err != nil {
		return brep.Solid{}, true, err
	}
	right, err := input(b, bu)
	if err != nil {
		return brep.Solid{}, true, err
	}
	var coordinates [3][]float64
	unique := [3]map[float64]bool{{}, {}, {}}
	for _, operand := range [][]polygon{left, right} {
		for _, p := range operand {
			if !bu.tick(1) {
				return brep.Solid{}, true, bu.err
			}
			axis := -1
			for i, n := range p.p.n {
				if n.Sign() != 0 {
					if axis >= 0 {
						return brep.Solid{}, false, nil
					}
					axis = i
				}
			}
			if axis < 0 {
				return brep.Solid{}, true, ErrInvalid
			}
			value, exact := p.v[0][axis].Float64()
			if !exact {
				return brep.Solid{}, true, ErrPrecision
			}
			unique[axis][value] = true
		}
	}
	if len(left)+len(right) == 0 {
		return brep.Solid{}, true, nil
	}
	cells := 1
	for axis := range coordinates {
		for value := range unique[axis] {
			coordinates[axis] = append(coordinates[axis], value)
		}
		sort.Float64s(coordinates[axis])
		n := len(coordinates[axis]) - 1
		if n < 1 {
			return brep.Solid{}, true, ErrInvalid
		}
		if n > o.MaxDepth || n > (o.MaxFragments-bu.fragments)/cells {
			return brep.Solid{}, true, ErrLimit
		}
		cells *= n
	}
	bu.fragments += cells
	counts := [3]int{len(coordinates[0]) - 1, len(coordinates[1]) - 1, len(coordinates[2]) - 1}
	inside := make([]bool, cells)
	index := func(x, y, z int) int { return (x*counts[1]+y)*counts[2] + z }
	for x := 0; x < counts[0]; x++ {
		for y := 0; y < counts[1]; y++ {
			for z := 0; z < counts[2]; z++ {
				if !bu.tick(1) {
					return brep.Solid{}, true, bu.err
				}
				i := [3]int{x, y, z}
				center := point{}
				for axis := range center {
					center[axis] = div(add(new(big.Rat).SetFloat64(coordinates[axis][i[axis]]), new(big.Rat).SetFloat64(coordinates[axis][i[axis]+1])), big.NewRat(2, 1))
				}
				bu.point(center)
				if bu.err != nil {
					return brep.Solid{}, true, bu.err
				}
				aa, e := axisWinding(left, center, bu)
				if e != nil {
					return brep.Solid{}, true, e
				}
				bb, e := axisWinding(right, center, bu)
				if e != nil {
					return brep.Solid{}, true, e
				}
				value := false
				switch op {
				case Union:
					value = aa || bb
				case Intersection:
					value = aa && bb
				case Difference:
					value = aa && !bb
				}
				inside[index(x, y, z)] = value
			}
		}
	}
	var vertices []geom.Vec3
	var edges []brep.Edge
	var faces []brep.Face
	vertexIDs := map[[3]int]int{}
	edgeIDs := map[[2]int]int{}
	intern := func(p [3]int) (int, error) {
		if id, ok := vertexIDs[p]; ok {
			return id, nil
		}
		if len(vertices) >= o.MaxVertices {
			return 0, ErrLimit
		}
		id := len(vertices)
		v := geom.Vec3{X: coordinates[0][p[0]], Y: coordinates[1][p[1]], Z: coordinates[2][p[2]]}
		vertices = append(vertices, v)
		vertexIDs[p] = id
		return id, nil
	}
	for x := 0; x < counts[0]; x++ {
		for y := 0; y < counts[1]; y++ {
			for z := 0; z < counts[2]; z++ {
				if !inside[index(x, y, z)] {
					continue
				}
				cell := [3]int{x, y, z}
				for axis := 0; axis < 3; axis++ {
					for _, sign := range []int{-1, 1} {
						if !bu.tick(1) {
							return brep.Solid{}, true, bu.err
						}
						neighbor := cell
						neighbor[axis] += sign
						if neighbor[axis] >= 0 && neighbor[axis] < counts[axis] && inside[index(neighbor[0], neighbor[1], neighbor[2])] {
							continue
						}
						if len(faces) >= o.MaxOutputTriangles/2 {
							return brep.Solid{}, true, ErrLimit
						}
						u, v := (axis+1)%3, (axis+2)%3
						origin := cell
						if sign > 0 {
							origin[axis]++
						}
						corners := [4][3]int{origin, origin, origin, origin}
						corners[1][u]++
						corners[2][u]++
						corners[2][v]++
						corners[3][v]++
						if sign < 0 {
							corners[1], corners[3] = corners[3], corners[1]
						}
						ids := [4]int{}
						for j, p := range corners {
							id, e := intern(p)
							if e != nil {
								return brep.Solid{}, true, e
							}
							ids[j] = id
						}
						face := brep.Face{}
						for j, id := range ids {
							next := ids[(j+1)%4]
							key := [2]int{min(id, next), max(id, next)}
							edge, ok := edgeIDs[key]
							if !ok {
								edge = len(edges)
								edgeIDs[key] = edge
								edges = append(edges, brep.Edge{Start: key[0], End: key[1]})
							}
							face.Boundary = append(face.Boundary, brep.EdgeUse{Edge: edge, Reversed: id > next})
						}
						faces = append(faces, face)
					}
				}
			}
		}
	}
	if len(faces) == 0 {
		return brep.Solid{}, true, nil
	}
	result, err = brep.NewSolid(vertices, edges, faces)
	if err != nil {
		return brep.Solid{}, true, fmt.Errorf("axis-aligned result: %w", err)
	}
	mesh := result.Tessellate()
	if err = geometricCheck(mesh, bu); err != nil {
		return brep.Solid{}, true, err
	}
	polygons := make([]polygon, len(mesh.Triangles))
	for i, t := range mesh.Triangles {
		if !bu.tick(1) {
			return brep.Solid{}, true, bu.err
		}
		v := []point{vec(mesh.Vertices[t[0]]), vec(mesh.Vertices[t[1]]), vec(mesh.Vertices[t[2]])}
		polygons[i] = polygon{v: v, p: makePlane(v[0], v[1], v[2])}
	}
	if err = orientations(mesh, polygons, bu); err != nil {
		return brep.Solid{}, true, err
	}
	if bu.err != nil {
		return brep.Solid{}, true, bu.err
	}
	return result, true, nil
}

// A ray along +X hits each actual support plane once even where its projection
// lies on an internal triangulation diagonal. Equal-plane signs must agree.
func axisWinding(polygons []polygon, p point, bu *budget) (bool, error) {
	crossings := map[string]int{}
	for _, f := range polygons {
		if !bu.tick(1) {
			return false, bu.err
		}
		sign := f.p.n[0].Sign()
		if sign == 0 || f.v[0][0].Cmp(p[0]) <= 0 {
			continue
		}
		positive, negative := false, false
		for i, a := range f.v {
			b := f.v[(i+1)%len(f.v)]
			turn := sub(mul(sub(b[1], a[1]), sub(p[2], a[2])), mul(sub(b[2], a[2]), sub(p[1], a[1])))
			bu.check(turn)
			positive = positive || turn.Sign() > 0
			negative = negative || turn.Sign() < 0
		}
		if bu.err != nil {
			return false, bu.err
		}
		if positive && negative {
			continue
		}
		key := f.v[0][0].RatString()
		if prior, ok := crossings[key]; ok && prior != sign {
			return false, ErrInvalid
		}
		crossings[key] = sign
	}
	winding := 0
	for _, sign := range crossings {
		winding += sign
	}
	if winding != 0 && winding != 1 {
		return false, ErrInvalid
	}
	return winding == 1, nil
}
