package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SplineCornerFan proves unique local continuation across a trihedral vertex:
// two affine face sectors cross the cutting plane, and the remaining graph
// sector has exactly the retained point contact. It does not authorize shell
// material selection or waive any global branch/embedding/coverage check.
type SplineCornerFan struct {
	anchors             [2]*SplineCornerAnchor
	isolated            *SplinePointContactLedger
	vertex              int
	edges, faces, signs [3]int
}

func (f *SplineCornerFan) Vertex() int           { return f.vertex }
func (f *SplineCornerFan) OrientedEdges() [3]int { return f.edges }
func (f *SplineCornerFan) OrientedFaces() [3]int { return f.faces }
func (f *SplineCornerFan) PlaneSigns() [3]int    { return f.signs }
func (f *SplineCornerFan) PendingAssembly() bool { return f != nil && f.isolated != nil }

// CertifySplineCornerFan accepts two independent stock-chart (operand zero)
// anchors and a point-only ledger on the same cutter face. It checks the entire
// original stock vertex link, exact affine supports of the two crossing faces
// and cutter, and exact signs along all three incident straight model edges.
// Zero signs, missing/repeated sectors, holes, or other vertex valences fail.
// Resource limits apply to each of the bounded three face-support proofs.
func CertifySplineCornerFan(ctx context.Context, anchors [2]*SplineCornerAnchor, isolated *SplinePointContactLedger, o SplineRectangleEmbeddingOptions) (*SplineCornerFan, error) {
	if ctx == nil || anchors[0] == nil || anchors[1] == nil || isolated == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	first, second := anchors[0], anchors[1]
	if first.chart != 0 || second.chart != 0 || first.vertex != second.vertex || first.pair[0] == second.pair[0] || first.pair[1] != second.pair[1] {
		return nil, geom.ErrConvergence
	}
	stock, tool := first.solids[0], first.solids[1]
	limit := o.MaxFaces
	if limit == 0 {
		limit = 64
	}
	if limit < 1 || limit > 128 {
		return nil, geom.ErrInvalid
	}
	for _, s := range []Solid{stock, tool} {
		if len(s.faces) > limit || len(s.edges) > 4*limit || len(s.vertices) > 4*limit {
			return nil, geom.ErrConvergence
		}
	}
	same := func(a, b Solid) bool {
		return len(a.faces) > 0 && len(b.faces) > 0 && len(a.edges) > 0 && len(b.edges) > 0 && &a.faces[0] == &b.faces[0] && &a.edges[0] == &b.edges[0]
	}
	if !same(stock, second.solids[0]) || !same(tool, second.solids[1]) || !isolated.Matches(stock, tool, isolated.faces) || isolated.faces[1] != first.pair[1] || isolated.faces[0] == first.pair[0] || isolated.faces[0] == second.pair[0] {
		return nil, geom.ErrConvergence
	}
	point := first.Point()
	for i, q := range isolated.Point() {
		if q.Cmp(point[i]) != 0 || second.Point()[i].Cmp(point[i]) != 0 {
			return nil, geom.ErrConvergence
		}
	}
	var plane *geom.SurfacePatch
	for i, entry := range []struct {
		s    Solid
		face int
	}{{stock, first.pair[0]}, {stock, second.pair[0]}, {tool, first.pair[1]}} {
		f, _, e := entry.s.splineEmbeddingFace(ctx, entry.s.faces[entry.face], o)
		if e != nil {
			return nil, e
		}
		if _, e = geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}); e != nil {
			return nil, e
		}
		if i == 2 {
			plane = f.patch
		}
	}
	d := plane.Domain()
	jet, e := plane.EvaluateRational(ctx, new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	var normal [3]*big.Rat
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		normal[i] = new(big.Rat).Sub(new(big.Rat).Mul(jet.Du[j], jet.Dv[k]), new(big.Rat).Mul(jet.Du[k], jet.Dv[j]))
	}
	signed := func(p geom.Vec3) int {
		sum := new(big.Rat)
		for i, v := range []float64{p.X, p.Y, p.Z} {
			sum.Add(sum, new(big.Rat).Mul(normal[i], new(big.Rat).Sub(new(big.Rat).SetFloat64(v), jet.Point[i])))
		}
		return sum.Sign()
	}
	vertex := first.vertex
	if signed(stock.vertices[vertex]) != 0 {
		return nil, geom.ErrConvergence
	}
	incident := map[int]int{}
	for id, edge := range stock.edges {
		if edge.Start == vertex || edge.End == vertex {
			if edge.Start == edge.End {
				return nil, geom.ErrConvergence
			}
			other := edge.Start
			if other == vertex {
				other = edge.End
			}
			sign := signed(stock.vertices[other])
			if sign == 0 {
				return nil, geom.ErrConvergence
			}
			incident[id] = sign
		}
	}
	if len(incident) != 3 {
		return nil, geom.ErrConvergence
	}
	type turn struct{ out, face int }
	next := map[int]turn{}
	allowed := map[int]bool{first.pair[0]: true, second.pair[0]: true, isolated.faces[0]: false}
	for id, face := range stock.faces {
		for _, hole := range face.Holes {
			for _, use := range hole {
				edge := stock.edges[use.Edge]
				if edge.Start == vertex || edge.End == vertex {
					return nil, geom.ErrConvergence
				}
			}
		}
		incoming, outgoing := -1, -1
		for _, use := range face.Outer {
			edge := stock.edges[use.Edge]
			start, end := edge.Start, edge.End
			if use.Reversed != face.Reversed {
				start, end = end, start
			}
			if start == vertex {
				if outgoing >= 0 {
					return nil, geom.ErrConvergence
				}
				outgoing = use.Edge
			}
			if end == vertex {
				if incoming >= 0 {
					return nil, geom.ErrConvergence
				}
				incoming = use.Edge
			}
		}
		if incoming < 0 && outgoing < 0 {
			continue
		}
		crosses, ok := allowed[id]
		if !ok || incoming < 0 || outgoing < 0 || len(face.Holes) != 0 {
			return nil, geom.ErrConvergence
		}
		if _, exists := next[incoming]; exists {
			return nil, geom.ErrConvergence
		}
		if (incident[incoming] != incident[outgoing]) != crosses {
			return nil, geom.ErrConvergence
		}
		next[incoming] = turn{outgoing, id}
	}
	if len(next) != 3 {
		return nil, geom.ErrConvergence
	}
	start := -1
	for id := range incident {
		if start < 0 || id < start {
			start = id
		}
	}
	result := &SplineCornerFan{anchors: anchors, isolated: isolated, vertex: vertex}
	current := start
	seen := map[int]bool{}
	for i := 0; i < 3; i++ {
		if seen[current] {
			return nil, geom.ErrConvergence
		}
		seen[current] = true
		t, ok := next[current]
		if !ok {
			return nil, geom.ErrConvergence
		}
		result.edges[i], result.faces[i], result.signs[i] = current, t.face, incident[current]
		current = t.out
	}
	if current != start {
		return nil, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return result, nil
}
