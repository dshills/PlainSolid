package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"strings"
)

// CornerSectionNode retains exact source ownership. Vertex>=0 identifies the
// fan-bound existing vertex; otherwise Edge/Parameter identify an exact point
// in the interior of an original stock edge. All exposed rationals are owned.
type CornerSectionNode struct {
	Point        [3]*big.Rat
	Vertex, Edge int
	Parameter    *big.Rat
}
type CornerSectionArc struct {
	Faces    [2]int
	From, To int
}
type cornerGraphNode struct {
	CornerSectionNode
	uv [2]*big.Rat
}

// SplineCornerSectionGraph proves one simple, oriented, closed four-segment
// section cycle. The graph remains pending shell fragmentation and assembly.
type SplineCornerSectionGraph struct {
	ledger *SplineCornerSectionLedger
	nodes  []cornerGraphNode
	arcs   []CornerSectionArc
}

func (g *SplineCornerSectionGraph) PendingAssembly() bool { return g != nil && g.ledger != nil }
func (g *SplineCornerSectionGraph) Arcs() []CornerSectionArc {
	return append([]CornerSectionArc(nil), g.arcs...)
}
func (g *SplineCornerSectionGraph) Nodes() []CornerSectionNode {
	r := make([]CornerSectionNode, len(g.nodes))
	for i, n := range g.nodes {
		r[i] = n.CornerSectionNode
		for j, q := range n.Point {
			r[i].Point[j] = new(big.Rat).Set(q)
		}
		if n.Parameter != nil {
			r[i].Parameter = new(big.Rat).Set(n.Parameter)
		}
	}
	return r
}

// BuildSplineCornerSectionGraph re-proves every nonempty branch pair by exact
// affine clipping; no caller links or sampled classifications are accepted.
// Work limits apply per bounded face/chart/clip reproof (at most four pairs).
// Initial scope is exactly four affine segments on one cutter face, one proved
// stock vertex fan and three distinct interior stock-edge events. Every original
// pair must remain explicitly classified. Nonadjacent UV segments must be disjoint.
func BuildSplineCornerSectionGraph(ctx context.Context, ledger *SplineCornerSectionLedger, o SplineRectangleEmbeddingOptions) (*SplineCornerSectionGraph, error) {
	if ctx == nil || ledger == nil || ledger.fan == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if !ledger.AllPairsClassified() {
		return nil, geom.ErrConvergence
	}
	fan := ledger.fan
	stock, tool := fan.anchors[0].solids[0], fan.anchors[0].solids[1]
	if len(ledger.pairs) != len(stock.faces)*len(tool.faces) {
		return nil, geom.ErrConvergence
	}
	g := &SplineCornerSectionGraph{ledger: ledger}
	keys := map[string]int{}
	pairs := map[[2]int]bool{}
	cornerProofs := map[[2]int]*SplineCornerAffineBranch{}
	for _, p := range ledger.affine {
		if p == nil || cornerProofs[p.FacePair()] != nil {
			return nil, geom.ErrConvergence
		}
		cornerProofs[p.FacePair()] = p
	}
	cornerCount, pointCount := 0, 0
	dot := func(a, b [3]*big.Rat) *big.Rat {
		r := new(big.Rat)
		for i := range a {
			r.Add(r, new(big.Rat).Mul(a[i], b[i]))
		}
		return r
	}
	cross := func(a, b [3]*big.Rat) (r [3]*big.Rat) {
		for i := range r {
			j, k := (i+1)%3, (i+2)%3
			r[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
		}
		return r
	}
	for index, record := range ledger.pairs {
		if record.Faces != ([2]int{index / len(tool.faces), index % len(tool.faces)}) || pairs[record.Faces] {
			return nil, geom.ErrConvergence
		}
		pairs[record.Faces] = true
		if record.Kind == SplinePairSeparated {
			continue
		}
		if record.Kind == SplinePairPointOnlyPendingFan {
			pointCount++
			if record.Faces != fan.isolated.faces {
				return nil, geom.ErrConvergence
			}
			continue
		}
		if record.Kind != SplinePairBranches && record.Kind != SplinePairAffineCornerBranchPendingAssembly || record.Branches != 1 || record.Faces[1] != fan.isolated.faces[1] || len(g.arcs) >= 4 {
			return nil, geom.ErrConvergence
		}
		if record.Kind == SplinePairAffineCornerBranchPendingAssembly && cornerProofs[record.Faces] == nil {
			return nil, geom.ErrConvergence
		}
		var faces [2]splineEmbeddingFace
		var charts [2]*geom.AffineSurfaceChart
		var normals [2][3]*big.Rat
		for i, s := range [2]Solid{stock, tool} {
			f, _, e := s.splineEmbeddingFace(ctx, s.faces[record.Faces[i]], o)
			if e != nil {
				return nil, e
			}
			faces[i] = f
			charts[i], e = geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
			if e != nil {
				return nil, e
			}
			d := f.patch.Domain()
			jet, e := f.patch.EvaluateRational(ctx, new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
			if e != nil {
				return nil, e
			}
			normals[i] = cross(jet.Du, jet.Dv)
			if s.faces[record.Faces[i]].Reversed {
				for _, v := range normals[i] {
					v.Neg(v)
				}
			}
		}
		exact, e := geom.IntersectAffineSurfaceCharts(ctx, charts[0], charts[1], geom.AffineChartIntersectionOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
		if e != nil {
			return nil, e
		}
		if !exact.Complete || exact.Kind != geom.AffineChartsSegment {
			return nil, geom.ErrConvergence
		}
		ends := exact.Endpoints
		var delta [3]*big.Rat
		for i := range delta {
			delta[i] = new(big.Rat).Sub(ends[1].Point[i], ends[0].Point[i])
		}
		direction := dot(delta, cross(normals[0], normals[1])).Sign()
		if direction == 0 {
			return nil, geom.ErrConvergence
		}
		if direction < 0 {
			ends[0], ends[1] = ends[1], ends[0]
		}
		arc := CornerSectionArc{Faces: record.Faces}
		for end, p := range ends {
			if p.BoundaryB != 0 || p.BoundaryA == 0 {
				return nil, geom.ErrConvergence
			}
			node := cornerGraphNode{CornerSectionNode: CornerSectionNode{Point: p.Point, Vertex: -1, Edge: -1}, uv: p.B}
			isCorner := p.BoundaryA&(p.BoundaryA-1) != 0
			if isCorner {
				proof := cornerProofs[record.Faces]
				if proof == nil {
					return nil, geom.ErrConvergence
				}
				origin := proof.anchor.Point()
				for i := range origin {
					if p.Point[i].Cmp(origin[i]) != 0 {
						return nil, geom.ErrConvergence
					}
				}
				node.Vertex = fan.vertex
			} else {
				side := geom.SurfaceEdge(0)
				mask := p.BoundaryA
				for mask != 1 {
					mask >>= 1
					side++
				}
				node.Edge = faces[0].edge[side]
				var use *Use
				for i := range stock.faces[record.Faces[0]].Outer {
					u := &stock.faces[record.Faces[0]].Outer[i]
					if u.Edge == node.Edge {
						if use != nil {
							return nil, geom.ErrConvergence
						}
						use = u
					}
				}
				if use == nil {
					return nil, geom.ErrConvergence
				}
				axis := 0
				start, finish := use.UVStart.X, use.UVEnd.X
				if start == finish {
					axis = 1
					start, finish = use.UVStart.Y, use.UVEnd.Y
				}
				if start == finish {
					return nil, geom.ErrConvergence
				}
				alpha := new(big.Rat).Quo(new(big.Rat).Sub(p.A[axis], new(big.Rat).SetFloat64(start)), new(big.Rat).Sub(new(big.Rat).SetFloat64(finish), new(big.Rat).SetFloat64(start)))
				if alpha.Sign() <= 0 || alpha.Cmp(big.NewRat(1, 1)) >= 0 {
					return nil, geom.ErrConvergence
				}
				if use.Reversed {
					alpha.Sub(big.NewRat(1, 1), alpha)
				}
				edge := stock.edges[node.Edge]
				node.Parameter = new(big.Rat).Add(new(big.Rat).SetFloat64(edge.Range.Min), new(big.Rat).Mul(alpha, new(big.Rat).Sub(new(big.Rat).SetFloat64(edge.Range.Max), new(big.Rat).SetFloat64(edge.Range.Min))))
			}
			parts := []string{}
			for _, q := range p.Point {
				parts = append(parts, q.RatString())
			}
			key := strings.Join(parts, ",")
			id, found := keys[key]
			if found {
				old := g.nodes[id]
				if old.Vertex != node.Vertex || old.Edge != node.Edge || (old.Parameter == nil) != (node.Parameter == nil) || old.Parameter != nil && old.Parameter.Cmp(node.Parameter) != 0 {
					return nil, geom.ErrConvergence
				}
				for i := range old.uv {
					if old.uv[i].Cmp(node.uv[i]) != 0 {
						return nil, geom.ErrConvergence
					}
				}
			} else {
				id = len(g.nodes)
				keys[key] = id
				g.nodes = append(g.nodes, node)
				if node.Vertex >= 0 {
					cornerCount++
				}
			}
			if end == 0 {
				arc.From = id
			} else {
				arc.To = id
			}
		}
		if arc.From == arc.To {
			return nil, geom.ErrConvergence
		}
		g.arcs = append(g.arcs, arc)
	}
	if len(g.arcs) != 4 || len(g.nodes) != 4 || cornerCount != 1 || pointCount != 1 {
		return nil, geom.ErrConvergence
	}
	ownedEdges := map[int]bool{}
	for _, n := range g.nodes {
		if n.Vertex < 0 {
			if ownedEdges[n.Edge] {
				return nil, geom.ErrConvergence
			}
			ownedEdges[n.Edge] = true
		}
	}
	if len(ownedEdges) != 3 {
		return nil, geom.ErrConvergence
	}
	next, previous := map[int]int{}, map[int]int{}
	for i, a := range g.arcs {
		if _, ok := next[a.From]; ok {
			return nil, geom.ErrConvergence
		}
		if _, ok := previous[a.To]; ok {
			return nil, geom.ErrConvergence
		}
		next[a.From], previous[a.To] = i, i
	}
	if len(next) != 4 || len(previous) != 4 {
		return nil, geom.ErrConvergence
	}
	seen := map[int]bool{}
	current := 0
	for i := 0; i < 4; i++ {
		if seen[current] {
			return nil, geom.ErrConvergence
		}
		seen[current] = true
		current = g.arcs[next[current]].To
	}
	if current != 0 {
		return nil, geom.ErrConvergence
	}
	orient := func(a, b, c [2]*big.Rat) int {
		return new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0]))).Sign()
	}
	// Strict opposite-edge separation is a sufficient simple-cycle certificate.
	for i, a := range g.arcs {
		for _, b := range g.arcs[i+1:] {
			if a.From == b.From || a.From == b.To || a.To == b.From || a.To == b.To {
				continue
			}
			p, q, r, s := g.nodes[a.From].uv, g.nodes[a.To].uv, g.nodes[b.From].uv, g.nodes[b.To].uv
			x, y, z, w := orient(p, q, r), orient(p, q, s), orient(r, s, p), orient(r, s, q)
			if !(x != 0 && x == y || z != 0 && z == w) {
				return nil, geom.ErrConvergence
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return g, nil
}
