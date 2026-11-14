package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AffineArrangementPrismOptions bounds native materialization separately from
// the immutable arrangement proof. Every output coordinate must be losslessly
// representable and passes native planar face validation.
type AffineArrangementPrismOptions struct{ MaxComponents, MaxVertices, MaxOperations, MaxCoefficientBits int }

// AffineArrangementPrismWall retains all exact original side pieces even when
// consecutive collinear pieces become one native wall. Reversed means the native
// boundary orientation was reversed for an extrusion toward A's negative side.
type AffineArrangementPrismWall struct {
	Component, Face  int
	ArrangementEdges []int
	Sources          []intersect.AffineArrangementEdge
	Reversed         bool
}

// AffineArrangementPrism is a complete native extrusion of the regularized
// arrangement area. Lower-dimensional Common witnesses are not extruded into
// solids. Each component has base and top faces 0 and 1, then independently indexed walls.
type AffineArrangementPrism struct {
	source *intersect.AffineGeneralPolygonArrangement
	solids []brep.Solid
	cycles [][]int
	walls  []AffineArrangementPrismWall
	volume *big.Rat
	work   int
}

func (s *AffineArrangementPrism) Components() []brep.Solid {
	if s == nil {
		return nil
	}
	return append([]brep.Solid(nil), s.solids...)
}
func (s *AffineArrangementPrism) Arrangement() *intersect.AffineGeneralPolygonArrangement {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *AffineArrangementPrism) ComponentCycles() [][]int {
	if s == nil {
		return nil
	}
	out := make([][]int, len(s.cycles))
	for i, v := range s.cycles {
		out[i] = append([]int(nil), v...)
	}
	return out
}
func (s *AffineArrangementPrism) ExactVolume() *big.Rat {
	if s == nil || s.volume == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *AffineArrangementPrism) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}
func (s *AffineArrangementPrism) Walls() []AffineArrangementPrismWall {
	if s == nil {
		return nil
	}
	out := append([]AffineArrangementPrismWall(nil), s.walls...)
	for i, w := range out {
		out[i].ArrangementEdges = append([]int(nil), w.ArrangementEdges...)
		out[i].Sources = append([]intersect.AffineArrangementEdge(nil), w.Sources...)
		for j, u := range out[i].Sources {
			for k := 0; k < 2; k++ {
				out[i].Sources[j].Parameter[k] = new(big.Rat).Set(u.Parameter[k])
				for axis := 0; axis < 2; axis++ {
					out[i].Sources[j].UV[k][axis] = new(big.Rat).Set(u.UV[k][axis])
				}
			}
		}
	}
	return out
}

// ExtrudeAffineArrangement materializes Common/Fuse/Cut of parallel straight
// extrusions sharing the original base plane and common offset. Original profile
// correspondence comes from the immutable arrangement, never caller flags. Exact
// nonzero plane/offset determinant proves the extrusion is embedded. All cycles
// are grouped by exact containment and all split-side provenance is retained.
// A final model vertex must fit binary64. Intermediate UV values can be rational.
func ExtrudeAffineArrangement(ctx context.Context, arr *intersect.AffineGeneralPolygonArrangement, offset geom.Vec3, o AffineArrangementPrismOptions) (*AffineArrangementPrism, error) {
	if ctx == nil || arr == nil || !arr.Complete() {
		return nil, geom.ErrInvalid
	}
	if o.MaxComponents == 0 {
		o.MaxComponents = 64
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxComponents < 1 || o.MaxComponents > 256 || o.MaxVertices < 6 || o.MaxVertices > 16384 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return nil, geom.ErrInvalid
	}
	work := 0
	var failure error
	check := func(q *big.Rat) {
		if failure != nil {
			return
		}
		if e := ctx.Err(); e != nil {
			failure = e
			return
		}
		work++
		if q == nil {
			failure = geom.ErrInvalid
		} else if work > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			failure = geom.ErrConvergence
		}
	}
	calc := func(op byte, x, y *big.Rat) *big.Rat {
		q := new(big.Rat)
		if failure != nil {
			return q
		}
		switch op {
		case '+':
			q.Add(x, y)
		case '-':
			q.Sub(x, y)
		case '*':
			q.Mul(x, y)
		case '/':
			if y.Sign() == 0 {
				failure = geom.ErrInvalid
				return q
			}
			q.Quo(x, y)
		}
		check(q)
		return q
	}
	rise := [3]*big.Rat{}
	for i, x := range []float64{offset.X, offset.Y, offset.Z} {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, geom.ErrInvalid
		}
		rise[i] = new(big.Rat).SetFloat64(x)
		check(rise[i])
	}
	if failure != nil {
		return nil, failure
	}
	source := arr.Area()
	vertices, edges, cycles := source.Vertices(), source.Edges(), source.Cycles()
	if len(vertices)*2 > o.MaxVertices {
		return nil, geom.ErrConvergence
	}
	for _, v := range vertices {
		for _, q := range v.A {
			check(q)
		}
		for _, q := range v.Point {
			check(q)
		}
	}
	if failure != nil {
		return nil, failure
	}
	cross := func(a, b, c [2]*big.Rat) *big.Rat {
		return calc('-', calc('*', calc('-', b[0], a[0]), calc('-', c[1], b[1])), calc('*', calc('-', b[1], a[1]), calc('-', c[0], b[0])))
	}
	type ring struct{ nodes, positions []int }
	rings := make([]ring, len(cycles))
	positive := []int{}
	negative := []int{}
	for ci, cycle := range cycles {
		check(cycle.SignedAreaA)
		if failure != nil {
			return nil, failure
		}
		if cycle.SignedAreaA.Sign() > 0 {
			positive = append(positive, ci)
		} else {
			negative = append(negative, ci)
		}
		for i, ei := range cycle.Edges {
			prev := vertices[edges[cycle.Edges[(i+len(cycle.Edges)-1)%len(cycle.Edges)]].Start].A
			v := vertices[edges[ei].Start].A
			next := vertices[edges[ei].End].A
			if cross(prev, v, next).Sign() != 0 {
				rings[ci].nodes = append(rings[ci].nodes, edges[ei].Start)
				rings[ci].positions = append(rings[ci].positions, i)
			}
		}
		if len(rings[ci].nodes) < 3 {
			return nil, geom.ErrInvalid
		}
	}
	if failure != nil {
		return nil, failure
	}
	if len(positive) > o.MaxComponents {
		return nil, geom.ErrConvergence
	}
	out := &AffineArrangementPrism{source: arr, volume: new(big.Rat)}
	for _, ci := range positive {
		out.cycles = append(out.cycles, []int{ci})
	}
	inside := func(q [2]*big.Rat, ci int) bool {
		wind := 0
		ids := rings[ci].nodes
		for i, id := range ids {
			v, w := vertices[id].A, vertices[ids[(i+1)%len(ids)]].A
			s := cross(v, w, q).Sign()
			if v[1].Cmp(q[1]) <= 0 && w[1].Cmp(q[1]) > 0 && s > 0 {
				wind++
			}
			if v[1].Cmp(q[1]) > 0 && w[1].Cmp(q[1]) <= 0 && s < 0 {
				wind--
			}
		}
		return wind != 0
	}
	for _, hole := range negative {
		owner := -1
		for i, ci := range positive {
			if inside(vertices[rings[hole].nodes[0]].A, ci) && (owner < 0 || cycles[ci].SignedAreaA.Cmp(cycles[positive[owner]].SignedAreaA) < 0) {
				owner = i
			}
		}
		if failure != nil {
			return nil, failure
		}
		if owner < 0 {
			return nil, geom.ErrInvalid
		}
		out.cycles[owner] = append(out.cycles[owner], hole)
	}
	uses := arr.EdgeSources()
	totalVertices := 0
	for component, group := range out.cycles {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		outer := rings[group[0]].nodes
		p0, p1, p2 := vertices[outer[0]], vertices[outer[1]], vertices[outer[2]]
		u, v := [3]*big.Rat{}, [3]*big.Rat{}
		for axis := 0; axis < 3; axis++ {
			u[axis] = calc('-', p1.Point[axis], p0.Point[axis])
			v[axis] = calc('-', p2.Point[axis], p0.Point[axis])
		}
		normal := [3]*big.Rat{}
		for axis := 0; axis < 3; axis++ {
			j, k := (axis+1)%3, (axis+2)%3
			normal[axis] = calc('-', calc('*', u[j], v[k]), calc('*', u[k], v[j]))
		}
		det := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			det = calc('+', det, calc('*', normal[axis], rise[axis]))
		}
		uvdet := cross(p0.A, p1.A, p2.A)
		factor := calc('/', det, uvdet)
		if failure != nil {
			return nil, failure
		}
		if factor.Sign() == 0 {
			return nil, geom.ErrDegenerate
		}
		reversed := factor.Sign() < 0
		absolute := new(big.Rat).Abs(factor)
		base := []geom.Vec3{}
		top := []geom.Vec3{}
		indices := [][]int{}
		exactFloat := func(q *big.Rat) (float64, error) {
			x, ok := q.Float64()
			if !ok || math.IsNaN(x) || math.IsInf(x, 0) {
				return 0, geom.ErrDomain
			}
			return x, nil
		}
		for _, ci := range group {
			ids := []int{}
			for _, id := range rings[ci].nodes {
				ids = append(ids, len(base))
				lo, hi := [3]float64{}, [3]float64{}
				for axis := 0; axis < 3; axis++ {
					var e error
					lo[axis], e = exactFloat(vertices[id].Point[axis])
					if e != nil {
						return nil, e
					}
					q := calc('+', vertices[id].Point[axis], rise[axis])
					if failure != nil {
						return nil, failure
					}
					hi[axis], e = exactFloat(q)
					if e != nil {
						return nil, e
					}
				}
				base = append(base, geom.Vec3{X: lo[0], Y: lo[1], Z: lo[2]})
				top = append(top, geom.Vec3{X: hi[0], Y: hi[1], Z: hi[2]})
			}
			indices = append(indices, ids)
			out.volume = calc('+', out.volume, calc('*', cycles[ci].SignedAreaA, absolute))
		}
		totalVertices += 2 * len(base)
		if totalVertices > o.MaxVertices {
			return nil, geom.ErrConvergence
		}
		count := len(base)
		polygons := [][][]int{make([][]int, len(indices)), make([][]int, len(indices))}
		for ri, ids := range indices {
			bottom, upper := append([]int(nil), ids...), make([]int, len(ids))
			reverseArrangementPrismIDs(bottom)
			for i, id := range ids {
				upper[i] = id + count
			}
			polygons[0][ri], polygons[1][ri] = bottom, upper
			ci := group[ri]
			for i, id := range ids {
				j := ids[(i+1)%len(ids)]
				face := len(polygons)
				polygons = append(polygons, [][]int{{id, j, j + count, id + count}})
				wall := AffineArrangementPrismWall{Component: component, Face: face, Reversed: reversed}
				begin, end := rings[ci].positions[i], rings[ci].positions[(i+1)%len(ids)]
				for k := begin; ; k = (k + 1) % len(cycles[ci].Edges) {
					if k == end && k != begin {
						break
					}
					ei := cycles[ci].Edges[k]
					wall.ArrangementEdges = append(wall.ArrangementEdges, ei)
					wall.Sources = append(wall.Sources, uses[ei]...)
					if (k+1)%len(cycles[ci].Edges) == end {
						break
					}
				}
				out.walls = append(out.walls, wall)
			}
		}
		if reversed {
			for _, face := range polygons {
				for _, ring := range face {
					reverseArrangementPrismIDs(ring)
				}
			}
		}
		solid, e := buildArrangementPrismFaces(append(base, top...), polygons)
		if e != nil {
			return nil, e
		}
		out.solids = append(out.solids, solid)
	}
	if failure != nil {
		return nil, failure
	}
	out.work = work
	return out, nil
}

func reverseArrangementPrismIDs(ids []int) {
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
}
func buildArrangementPrismFaces(vertices []geom.Vec3, polygons [][][]int) (brep.Solid, error) {
	edges := []brep.Edge{}
	faces := make([]brep.Face, len(polygons))
	lookup := map[[2]int]int{}
	for fi, rings := range polygons {
		for ri, ring := range rings {
			uses := []brep.EdgeUse{}
			for i, a := range ring {
				b := ring[(i+1)%len(ring)]
				lo, hi := a, b
				if lo > hi {
					lo, hi = hi, lo
				}
				key := [2]int{lo, hi}
				id, ok := lookup[key]
				if !ok {
					id = len(edges)
					lookup[key] = id
					edges = append(edges, brep.Edge{Start: lo, End: hi})
				}
				uses = append(uses, brep.EdgeUse{Edge: id, Reversed: a > b})
			}
			if ri == 0 {
				faces[fi].Boundary = uses
			} else {
				faces[fi].Holes = append(faces[fi].Holes, uses)
			}
		}
	}
	return brep.NewSolid(vertices, edges, faces)
}
