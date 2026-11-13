package solid

import (
	"fmt"
	"sort"

	"plainsolid/brep"
	"plainsolid/geom"
)

// FromOrthogonal converts an outward-oriented, axis-aligned planar B-rep to
// a regularized cell solid. It supports concavities, holes and cavity shells.
// Face vertices and edge directions must be exactly axis aligned. It verifies
// the oriented boundary against occupancy transitions on all three axes.
// No float64 midpoint or geometric tolerance is used for occupancy tests.
func FromOrthogonal(s brep.Solid, options Options) (Solid, error) {
	cellLimit, err := options.limit()
	if err != nil {
		return Solid{}, err
	}
	work := options.MaxWork
	if work == 0 {
		work = 20000000
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if len(vertices) == 0 {
		return Solid{}, nil
	}
	if len(vertices) > work || len(edges) > work || len(faces) > work {
		return Solid{}, ErrLimit
	}
	var coords [3][]float64
	for _, p := range vertices {
		for a, v := range [3]float64{p.X, p.Y, p.Z} {
			coords[a] = append(coords[a], v)
		}
	}
	var n [3]int
	total := 1
	for a, c := range coords {
		coords[a] = unique(c)
		n[a] = len(coords[a]) - 1
		if n[a] < 1 {
			return Solid{}, fmt.Errorf("solid: degenerate orthogonal grid")
		}
		if total > cellLimit/n[a] {
			return Solid{}, ErrLimit
		}
		total *= n[a]
	}
	if total > work/6 {
		return Solid{}, ErrLimit
	}
	work -= 6 * total
	vertex := make([][3]int, len(vertices))
	for i, p := range vertices {
		for a, v := range [3]float64{p.X, p.Y, p.Z} {
			vertex[i][a] = sort.SearchFloat64s(coords[a], v)
		}
	}
	for _, e := range edges {
		d := 0
		for k, a := range vertex[e.Start] {
			if a != vertex[e.End][k] {
				d++
			}
		}
		if d != 1 {
			return Solid{}, fmt.Errorf("solid: nonorthogonal edge")
		}
	}
	var boundary [3][]int8
	offset := func(a int, p [3]int) int { u, v := (a+1)%3, (a+2)%3; return (p[a]*n[u]+p[u])*n[v] + p[v] }
	for a := range boundary {
		u, v := (a+1)%3, (a+2)%3
		boundary[a] = make([]int8, (n[a]+1)*n[u]*n[v])
	}
	for fi, f := range faces {
		if work == 0 {
			return Solid{}, ErrLimit
		}
		work--
		rings := append([][]brep.EdgeUse{f.Boundary}, f.Holes...)
		points := make([][][3]int, len(rings))
		lo, hi := vertex[edges[f.Boundary[0].Edge].Start], vertex[edges[f.Boundary[0].Edge].Start]
		for j, ring := range rings {
			if len(ring) > work {
				return Solid{}, ErrLimit
			}
			work -= len(ring)
			for _, u := range ring {
				e := edges[u.Edge]
				i := e.Start
				if u.Reversed {
					i = e.End
				}
				p := vertex[i]
				points[j] = append(points[j], p)
				for k, a := range p {
					lo[k] = min(lo[k], a)
					hi[k] = max(hi[k], a)
				}
			}
		}
		axis := -1
		for i, a := range lo {
			if a == hi[i] {
				if axis != -1 {
					return Solid{}, fmt.Errorf("solid: degenerate face %d", fi)
				}
				axis = i
			}
		}
		if axis < 0 {
			return Solid{}, fmt.Errorf("solid: nonorthogonal face %d", fi)
		}
		u, v := (axis+1)%3, (axis+2)%3
		nu, nv := hi[u]-lo[u], hi[v]-lo[v]
		if nu+1 > work/nv {
			return Solid{}, ErrLimit
		}
		work -= (nu + 1) * nv
		deltas := make([]int, (nu+1)*nv)
		for _, ring := range points {
			for i, p := range ring {
				q := ring[(i+1)%len(ring)]
				if p[v] == q[v] {
					continue
				}
				from, to := min(p[v], q[v]), max(p[v], q[v])
				if to-from > work {
					return Solid{}, ErrLimit
				}
				work -= to - from
				sign := 1
				if q[v] > p[v] {
					sign = -1
				}
				for j := from; j < to; j++ {
					deltas[(p[u]-lo[u])*nv+j-lo[v]] += sign
				}
			}
		}
		for j := 0; j < nv; j++ {
			w := 0
			for i := 0; i <= nu; i++ {
				w += deltas[i*nv+j]
				if i == nu && w != 0 || w < -1 || w > 1 {
					return Solid{}, fmt.Errorf("solid: invalid face winding %d", fi)
				}
				if w == 0 {
					continue
				}
				p := lo
				p[u] += i
				p[v] += j
				idx := offset(axis, p)
				if boundary[axis][idx] != 0 {
					return Solid{}, fmt.Errorf("solid: overlapping coplanar faces")
				}
				boundary[axis][idx] = int8(w)
			}
		}
	}
	idx := func(p [3]int) int { return (p[0]*n[1]+p[1])*n[2] + p[2] }
	occ := make([]int8, total)
	for j := range n[1] {
		for k := range n[2] {
			var fill int8
			for i := 0; i <= n[0]; i++ {
				p := [3]int{i, j, k}
				fill -= boundary[0][offset(0, p)]
				if fill < 0 || fill > 1 || i == n[0] && fill != 0 {
					return Solid{}, fmt.Errorf("solid: invalid shell winding")
				}
				if i < n[0] {
					occ[idx(p)] = fill
				}
			}
		}
	}
	for a := 1; a < 3; a++ {
		u, v := (a+1)%3, (a+2)%3
		for i := range n[u] {
			for j := range n[v] {
				p := [3]int{}
				p[u], p[v] = i, j
				var prev int8
				for k := 0; k <= n[a]; k++ {
					p[a] = k
					var next int8
					if k < n[a] {
						next = occ[idx(p)]
					}
					if boundary[a][offset(a, p)] != prev-next {
						return Solid{}, fmt.Errorf("solid: boundary does not enclose consistent orthogonal cells")
					}
					prev = next
				}
			}
		}
	}
	out := Solid{}
	for i := range n[0] {
		for j := range n[1] {
			for k := range n[2] {
				if occ[idx([3]int{i, j, k})] == 0 {
					continue
				}
				b := geom.Box{Min: geom.Vec3{X: coords[0][i], Y: coords[1][j], Z: coords[2][k]}, Max: geom.Vec3{X: coords[0][i+1], Y: coords[1][j+1], Z: coords[2][k+1]}}
				if v := b.Volume(); v <= 0 || !finite(v) {
					return Solid{}, fmt.Errorf("solid: orthogonal cell volume outside range")
				}
				out.cells = append(out.cells, b)
			}
		}
	}
	if !finite(out.Volume()) {
		return Solid{}, fmt.Errorf("solid: volume overflow")
	}
	return out, nil
}

// BRep converts the regularized cell boundary to validated planar topology.
// Grid cuts are retained as coplanar quadrilateral faces. A point or edge
// contact that makes the boundary nonmanifold returns an error.
func (s Solid) BRep(options Options) (brep.Solid, error) {
	vertices, quads, err := s.boundaryQuads(options)
	if err != nil {
		return brep.Solid{}, err
	}
	index := map[[2]int]int{}
	var edges []brep.Edge
	faces := make([]brep.Face, len(quads))
	for i, t := range quads {
		for j, a := range t {
			b := t[(j+1)%4]
			k := [2]int{min(a, b), max(a, b)}
			id, ok := index[k]
			if !ok {
				id = len(edges)
				edges = append(edges, brep.Edge{Start: k[0], End: k[1]})
				index[k] = id
			}
			faces[i].Boundary = append(faces[i].Boundary, brep.EdgeUse{Edge: id, Reversed: a > b})
		}
	}
	return brep.NewSolid(vertices, edges, faces)
}
