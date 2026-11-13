// Package unify removes internal edges between exactly coplanar, equally
// oriented faces of an embedded planar solid. It never welds coordinates or
// uses a planarity tolerance.
package unify

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"sort"
)

var ErrInvalid = errors.New("unify: invalid input or options")
var ErrLimit = errors.New("unify: resource limit exceeded")
var ErrUnsupported = errors.New("unify: resulting junction cannot be represented")

// Options separately bounds the unification phase and each exact input/output
// validation pass. Zero defaults: 4096 faces, 8192 triangles, 2000000 work units,
// 100000 collision comparisons. Validation's collision pass is not internally
// cancellable; other loops check context. Face holes retain BRep's vertex limit.
type Options struct{ MaxFaces, MaxTriangles, MaxWork, MaxCollisionPairs int }

// Result maps source polygon faces to deterministic unified faces. Groups are
// ordered by their lowest source face index; NewToOld lists ascending indices.
// VertexMap maps original vertices to output indices (-1 means removed).
// Only coordinates from the input are used. All result slices are caller-owned.
type Result struct {
	Solid                         brep.Solid
	OldToNew                      []int
	NewToOld                      [][]int
	VertexMap                     []int
	RemovedEdges, RemovedVertices int
}
type q = *big.Rat
type point [3]q
type plane struct {
	n   point
	d   q
	key string
}

func sub(a, b q) q          { return new(big.Rat).Sub(a, b) }
func add(a, b q) q          { return new(big.Rat).Add(a, b) }
func mul(a, b q) q          { return new(big.Rat).Mul(a, b) }
func diff(a, b point) point { return point{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func cross(a, b point) point {
	return point{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func dot(a, b point) q  { return add(add(mul(a[0], b[0]), mul(a[1], b[1])), mul(a[2], b[2])) }
func zero(p point) bool { return p[0].Sign() == 0 && p[1].Sign() == 0 && p[2].Sign() == 0 }
func makePlane(a, b, c point) plane {
	n := cross(diff(b, a), diff(c, a))
	max := new(big.Rat)
	for _, v := range n {
		a := new(big.Rat).Abs(v)
		if a.Cmp(max) > 0 {
			max = a
		}
	}
	if max.Sign() == 0 {
		return plane{}
	}
	for i := range n {
		n[i] = new(big.Rat).Quo(n[i], max)
	}
	d := dot(n, a)
	return plane{n, d, n[0].RatString() + "," + n[1].RatString() + "," + n[2].RatString() + "," + d.RatString()}
}

type budget struct {
	ctx  context.Context
	left int
}

func (b *budget) tick(n int) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	if n > b.left {
		return ErrLimit
	}
	b.left -= n
	return nil
}
func loops(f brep.Face) [][]brep.EdgeUse { return append([][]brep.EdgeUse{f.Boundary}, f.Holes...) }
func indices(w []brep.EdgeUse, es []brep.Edge) []int {
	v := make([]int, len(w))
	for i, u := range w {
		e := es[u.Edge]
		v[i] = e.Start
		if u.Reversed {
			v[i] = e.End
		}
	}
	return v
}
func validate(ctx context.Context, s brep.Solid, o Options) error {
	_, e := collision.NewPointLocator(ctx, s, collision.LocationOptions{MaxTriangles: o.MaxTriangles, MaxWork: o.MaxWork, MaxCollisionPairs: o.MaxCollisionPairs})
	if e != nil {
		if errors.Is(e, collision.ErrLimit) {
			return fmt.Errorf("%w: %v", ErrLimit, e)
		}
		if errors.Is(e, collision.ErrInvalid) {
			return fmt.Errorf("%w: %v", ErrInvalid, e)
		}
		return e
	}
	return nil
}

// Unify merges edge-connected faces on the same exact oriented plane. Boundary
// loops preserve holes. Globally degree-two collinear feature vertices are
// removed consistently from both incident faces. Higher-valence collinear
// junctions, branched loops, or multiple outer boundaries in one connected group
// return ErrUnsupported rather than dropping required edge subdivisions.
// Exactly nonplanar represented faces and invalid material shells are rejected.
func Unify(ctx context.Context, s brep.Solid, o Options) (Result, error) {
	empty := Result{}
	if ctx == nil || o.MaxFaces < 0 || o.MaxTriangles < 0 || o.MaxTriangles > collision.MaxTriangles || o.MaxWork < 0 || o.MaxCollisionPairs < 0 {
		return empty, ErrInvalid
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 4096
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 8192
	}
	if o.MaxWork == 0 {
		o.MaxWork = 2000000
	}
	if o.MaxCollisionPairs == 0 {
		o.MaxCollisionPairs = 100000
	}
	b := &budget{ctx: ctx, left: o.MaxWork}
	if e := b.tick(1); e != nil {
		return empty, e
	}
	fs, es, vs := s.Faces(), s.Edges(), s.Vertices()
	if len(fs) > o.MaxFaces {
		return empty, ErrLimit
	}
	if e := validate(ctx, s, o); e != nil {
		return empty, e
	}
	if len(fs) == 0 {
		return Result{Solid: s}, nil
	}
	pv := make([]point, len(vs))
	for i, v := range vs {
		if e := b.tick(1); e != nil {
			return empty, e
		}
		pv[i] = point{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
	}
	planes := make([]plane, len(fs))
	owner := make([][]int, len(es))
	parents := make([]int, len(fs))
	var find func(int) int
	find = func(i int) int {
		for parents[i] != i {
			parents[i] = parents[parents[i]]
			i = parents[i]
		}
		return i
	}
	for i, f := range fs {
		parents[i] = i
		v := indices(f.Boundary, es)
		p := makePlane(pv[v[0]], pv[v[1]], pv[v[2]])
		if p.d == nil {
			return empty, fmt.Errorf("%w: degenerate source face", ErrInvalid)
		}
		planes[i] = p
		for _, w := range loops(f) {
			for _, u := range w {
				if e := b.tick(1); e != nil {
					return empty, e
				}
				e := es[u.Edge]
				if dot(p.n, pv[e.Start]).Cmp(p.d) != 0 || dot(p.n, pv[e.End]).Cmp(p.d) != 0 {
					return empty, fmt.Errorf("%w: face %d is not exactly planar", ErrInvalid, i)
				}
				owner[u.Edge] = append(owner[u.Edge], i)
			}
		}
	}
	for _, faces := range owner {
		if e := b.tick(1); e != nil {
			return empty, e
		}
		if len(faces) != 2 {
			return empty, ErrInvalid
		}
		a, c := faces[0], faces[1]
		if planes[a].key == planes[c].key {
			ra, rc := find(a), find(c)
			if ra > rc {
				ra, rc = rc, ra
			}
			parents[rc] = ra
		}
	}
	groupID := map[int]int{}
	r := Result{OldToNew: make([]int, len(fs)), VertexMap: make([]int, len(vs))}
	for i := range r.VertexMap {
		r.VertexMap[i] = -1
	}
	for i := range fs {
		root := find(i)
		g, ok := groupID[root]
		if !ok {
			g = len(r.NewToOld)
			groupID[root] = g
			r.NewToOld = append(r.NewToOld, nil)
		}
		r.OldToNew[i] = g
		r.NewToOld[g] = append(r.NewToOld[g], i)
	}
	outgoing := make([]map[int]int, len(r.NewToOld))
	incoming := make([]map[int]int, len(r.NewToOld))
	neighbors := make([]map[int]bool, len(vs))
	for i := range outgoing {
		outgoing[i] = map[int]int{}
		incoming[i] = map[int]int{}
	}
	for i, f := range fs {
		g := r.OldToNew[i]
		for _, w := range loops(f) {
			for _, u := range w {
				if e := b.tick(1); e != nil {
					return empty, e
				}
				own := owner[u.Edge]
				if r.OldToNew[own[0]] == r.OldToNew[own[1]] {
					continue
				}
				edge := es[u.Edge]
				a, c := edge.Start, edge.End
				if u.Reversed {
					a, c = c, a
				}
				if _, ok := outgoing[g][a]; ok {
					return empty, fmt.Errorf("%w: branched boundary", ErrUnsupported)
				}
				if _, ok := incoming[g][c]; ok {
					return empty, fmt.Errorf("%w: branched boundary", ErrUnsupported)
				}
				outgoing[g][a] = c
				incoming[g][c] = a
				if neighbors[a] == nil {
					neighbors[a] = map[int]bool{}
				}
				if neighbors[c] == nil {
					neighbors[c] = map[int]bool{}
				}
				neighbors[a][c] = true
				neighbors[c][a] = true
			}
		}
	}
	remove := make([]bool, len(vs))
	for i, n := range neighbors {
		if len(n) != 2 {
			continue
		}
		var ends []int
		for j := range n {
			ends = append(ends, j)
		}
		remove[i] = zero(cross(diff(pv[ends[0]], pv[i]), diff(pv[ends[1]], pv[i])))
	}
	faceLoops := make([][][]int, len(outgoing))
	for g, edges := range outgoing {
		var starts []int
		for v := range edges {
			starts = append(starts, v)
		}
		sort.Ints(starts)
		visited := map[int]bool{}
		var outer []int
		var holes [][]int
		normal := planes[r.NewToOld[g][0]].n
		for _, start := range starts {
			if visited[start] {
				continue
			}
			var loop []int
			at := start
			for {
				if e := b.tick(1); e != nil {
					return empty, e
				}
				if visited[at] {
					if at != start {
						return empty, ErrUnsupported
					}
					break
				}
				visited[at] = true
				if !remove[at] {
					loop = append(loop, at)
				}
				next, ok := edges[at]
				if !ok {
					return empty, ErrUnsupported
				}
				at = next
			}
			if len(loop) < 3 {
				return empty, ErrUnsupported
			}
			area := new(big.Rat)
			origin := pv[loop[0]]
			for i, v := range loop {
				prev, next := loop[(i+len(loop)-1)%len(loop)], loop[(i+1)%len(loop)]
				if zero(cross(diff(pv[v], pv[prev]), diff(pv[next], pv[v]))) {
					return empty, fmt.Errorf("%w: required collinear junction", ErrUnsupported)
				}
				area.Add(area, dot(cross(diff(pv[v], origin), diff(pv[next], origin)), normal))
			}
			switch area.Sign() {
			case 1:
				if outer != nil {
					return empty, fmt.Errorf("%w: multiple outer loops", ErrUnsupported)
				}
				outer = loop
			case -1:
				holes = append(holes, loop)
			default:
				return empty, ErrUnsupported
			}
		}
		if outer == nil {
			return empty, ErrUnsupported
		}
		faceLoops[g] = append([][]int{outer}, holes...)
	}
	used := make([]bool, len(vs))
	for _, ls := range faceLoops {
		for _, loop := range ls {
			for _, v := range loop {
				used[v] = true
			}
		}
	}
	var vertices []geom.Vec3
	for i, v := range vs {
		if used[i] {
			r.VertexMap[i] = len(vertices)
			vertices = append(vertices, v)
		}
	}
	var edges []brep.Edge
	edgeIDs := map[[2]int]int{}
	faces := make([]brep.Face, len(faceLoops))
	for i, ls := range faceLoops {
		for j, loop := range ls {
			var wire []brep.EdgeUse
			for k, v := range loop {
				if e := b.tick(1); e != nil {
					return empty, e
				}
				a, c := r.VertexMap[v], r.VertexMap[loop[(k+1)%len(loop)]]
				rev := a > c
				if rev {
					a, c = c, a
				}
				pair := [2]int{a, c}
				id, ok := edgeIDs[pair]
				if !ok {
					id = len(edges)
					edgeIDs[pair] = id
					edges = append(edges, brep.Edge{Start: a, End: c})
				}
				wire = append(wire, brep.EdgeUse{Edge: id, Reversed: rev})
			}
			if j == 0 {
				faces[i].Boundary = wire
			} else {
				faces[i].Holes = append(faces[i].Holes, wire)
			}
		}
	}
	out, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return empty, fmt.Errorf("%w: %v", ErrUnsupported, e)
	}
	if e = validate(ctx, out, o); e != nil {
		return empty, e
	}
	r.Solid = out
	r.RemovedEdges = len(es) - len(edges)
	r.RemovedVertices = len(vs) - len(vertices)
	return r, nil
}
