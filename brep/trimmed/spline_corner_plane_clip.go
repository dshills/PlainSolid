package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineCornerPlaneClip retains original support identity and face/vertex
// ancestry after exact clipping by the sole active cutter halfspace.
type SplineCornerPlaneClip struct {
	Solid                                                  Solid
	StockFaces, ToolFaces                                  [][]int
	StockEdges                                             [][]int
	SectionEdges                                           []int
	OriginalVertices                                       []int // -1 when discarded; otherwise one shared result vertex
	KeptStockFaces, DiscardedStockFaces, ClippedStockFaces []int
	ActiveToolFace                                         int
}

// CombineSplineCornerPlaneClip supports Intersection and Difference only.
// It requires the complete simple corner section graph, embedded outward input
// rectangle shells, an affine convex cutter and strict containment of all stock
// by every inactive cutter plane. Crossing stock faces must be affine; untouched
// curved faces require exact full-domain halfspace certificates. All generated
// UV/model coordinates must be losslessly representable. Work limits apply to
// each of the bounded face/plane/edge reproofs; no sampled classification is used.
func CombineSplineCornerPlaneClip(ctx context.Context, graph *SplineCornerSectionGraph, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions) (SplineCornerPlaneClip, error) {
	var zero SplineCornerPlaneClip
	if ctx == nil || graph == nil || graph.ledger == nil || (op != parametric.Intersection && op != parametric.Difference) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	fan := graph.ledger.fan
	stock, tool := fan.anchors[0].solids[0], fan.anchors[0].solids[1]
	active := fan.isolated.faces[1]
	var patches [2][]splineEmbeddingFace
	for i, s := range [2]Solid{stock, tool} {
		r, e := s.validateSplineRectangleEmbedding(ctx, o, &patches[i])
		if e != nil {
			return zero, e
		}
		if !r.Complete {
			return zero, geom.ErrConvergence
		}
	}
	planePoints := func(s Solid, f splineEmbeddingFace, reversed bool) [3]geom.Vec3 {
		p := [3]geom.Vec3{s.vertices[f.vertex[0]], s.vertices[f.vertex[1]], s.vertices[f.vertex[3]]}
		if reversed {
			p[1], p[2] = p[2], p[1]
		}
		return p
	}
	var planes [][3]geom.Vec3
	for j, f := range patches[1] {
		if _, e := geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}); e != nil {
			return zero, e
		}
		planes = append(planes, planePoints(tool, f, tool.faces[j].Reversed))
	}
	for j, p := range planes {
		// A closed embedded shell supported by all its outward face halfspaces
		// is the boundary of their convex intersection.
		for k, f := range patches[1] {
			if k == j {
				continue
			}
			r, e := geom.CertifySurfaceSupportingPlane(ctx, f.patch, p[0], p[1], p[2], o.Plane)
			if e != nil {
				return zero, e
			}
			if !r.Complete || r.Coincident || r.HalfspaceSign >= 0 {
				return zero, geom.ErrConvergence
			}
		}
		if j == active {
			continue
		}
		for _, f := range patches[0] {
			r, e := geom.CertifySurfaceSupportingPlane(ctx, f.patch, p[0], p[1], p[2], o.Plane)
			if e != nil {
				return zero, e
			}
			if !r.Complete || r.Coincident || r.HalfspaceSign >= 0 || r.SideMask != 0 || r.CornerMask != 0 {
				return zero, geom.ErrConvergence
			}
		}
	}
	raw := planes[active]
	var base, normal [3]*big.Rat
	for i := 0; i < 3; i++ {
		base[i] = new(big.Rat).SetFloat64([]float64{raw[0].X, raw[0].Y, raw[0].Z}[i])
		j, k := (i+1)%3, (i+2)%3
		coords := func(v geom.Vec3, axis int) *big.Rat { return new(big.Rat).SetFloat64([]float64{v.X, v.Y, v.Z}[axis]) }
		normal[i] = new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(coords(raw[1], j), coords(raw[0], j)), new(big.Rat).Sub(coords(raw[2], k), coords(raw[0], k))), new(big.Rat).Mul(new(big.Rat).Sub(coords(raw[1], k), coords(raw[0], k)), new(big.Rat).Sub(coords(raw[2], j), coords(raw[0], j))))
	}
	signed := func(point [3]*big.Rat) *big.Rat {
		r := new(big.Rat)
		for i := range point {
			r.Add(r, new(big.Rat).Mul(normal[i], new(big.Rat).Sub(point[i], base[i])))
		}
		return r
	}
	sign := 1
	if op == parametric.Difference {
		sign = -1
	}
	crossed := map[int]bool{}
	for _, arc := range graph.arcs {
		crossed[arc.Faces[0]] = true
	}
	var parts []SewFace
	var owners [][2]int
	out := SplineCornerPlaneClip{StockFaces: make([][]int, len(stock.faces)), ToolFaces: make([][]int, len(tool.faces)), ActiveToolFace: active}
	add := func(part SewFace, owner, id int) {
		index := len(parts)
		parts = append(parts, part)
		owners = append(owners, [2]int{owner, id})
		if owner == 0 {
			out.StockFaces[id] = append(out.StockFaces[id], index)
		} else {
			out.ToolFaces[id] = append(out.ToolFaces[id], index)
		}
	}
	makeFace := func(face Face, patch *geom.SurfacePatch, uv [][2]*big.Rat) (SewFace, error) {
		var part SewFace
		part.Face = Face{Surface: face.Surface, Reversed: face.Reversed}
		for _, q := range uv {
			u, e := geom.RepresentedExactScalar(q[0])
			if e != nil {
				return part, e
			}
			v, e := geom.RepresentedExactScalar(q[1])
			if e != nil {
				return part, e
			}
			jet, e := patch.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
			if e != nil {
				return part, e
			}
			var p geom.Vec3
			for i, d := range []*float64{&p.X, &p.Y, &p.Z} {
				*d, e = geom.RepresentedExactScalar(jet.Point[i])
				if e != nil {
					return part, e
				}
			}
			part.Vertices = append(part.Vertices, p)
			part.Face.Outer = append(part.Face.Outer, Use{Edge: len(part.Face.Outer), UVStart: geom.Vec2{X: u, Y: v}})
		}
		for i := range part.Vertices {
			j := (i + 1) % len(part.Vertices)
			a, b := part.Face.Outer[i].UVStart, part.Face.Outer[j].UVStart
			part.Face.Outer[i].UVEnd = b
			part.Edges = append(part.Edges, parametric.Edge{Start: i, End: j, Curve: surfaceTrimCurve{face.Surface, a, b, nil}, Range: parametric.Interval{Max: 1}})
		}
		return part, nil
	}
	for id, f := range patches[0] {
		if !crossed[id] {
			r, e := geom.CertifySurfaceSupportingPlane(ctx, f.patch, raw[0], raw[1], raw[2], o.Plane)
			if e != nil {
				return zero, e
			}
			if !r.Complete || r.Coincident || r.HalfspaceSign == 0 {
				return zero, geom.ErrConvergence
			}
			if r.SideMask != 0 || r.CornerMask != 0 {
				if id != fan.isolated.faces[0] || r.SideMask != 0 {
					return zero, geom.ErrConvergence
				}
			}
			if sign*r.HalfspaceSign < 0 {
				add(stock.splineSewFace(stock.faces[id]), 0, id)
				out.KeptStockFaces = append(out.KeptStockFaces, id)
			} else {
				out.DiscardedStockFaces = append(out.DiscardedStockFaces, id)
			}
			continue
		}
		if _, e := geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}); e != nil {
			return zero, e
		}
		var uv [][2]*big.Rat
		var heights []*big.Rat
		for _, use := range stock.faces[id].Outer {
			q := [2]*big.Rat{new(big.Rat).SetFloat64(use.UVStart.X), new(big.Rat).SetFloat64(use.UVStart.Y)}
			jet, e := f.patch.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
			if e != nil {
				return zero, e
			}
			uv = append(uv, q)
			heights = append(heights, new(big.Rat).Mul(big.NewRat(int64(sign), 1), signed(jet.Point)))
		}
		var clipped [][2]*big.Rat
		appendPoint := func(q [2]*big.Rat) {
			if len(clipped) == 0 || clipped[len(clipped)-1][0].Cmp(q[0]) != 0 || clipped[len(clipped)-1][1].Cmp(q[1]) != 0 {
				clipped = append(clipped, q)
			}
		}
		for i, a := range uv {
			j := (i + 1) % len(uv)
			b := uv[j]
			ha, hb := heights[i], heights[j]
			if ha.Sign() <= 0 {
				appendPoint(a)
			}
			if ha.Sign()*hb.Sign() < 0 {
				alpha := new(big.Rat).Quo(ha, new(big.Rat).Sub(ha, hb))
				var q [2]*big.Rat
				for k := range q {
					q[k] = new(big.Rat).Add(a[k], new(big.Rat).Mul(alpha, new(big.Rat).Sub(b[k], a[k])))
				}
				appendPoint(q)
			}
		}
		if len(clipped) > 1 && clipped[0][0].Cmp(clipped[len(clipped)-1][0]) == 0 && clipped[0][1].Cmp(clipped[len(clipped)-1][1]) == 0 {
			clipped = clipped[:len(clipped)-1]
		}
		if len(clipped) < 3 || len(clipped) > 5 {
			return zero, geom.ErrConvergence
		}
		part, e := makeFace(stock.faces[id], f.patch, clipped)
		if e != nil {
			return zero, e
		}
		add(part, 0, id)
		out.ClippedStockFaces = append(out.ClippedStockFaces, id)
	}
	// Traverse the proved section cycle, then orient its UV wire CCW. Face
	// reversal selects the active cutter normal for common, its inverse for cut.
	var capUV [][2]*big.Rat
	current := 0
	for i := 0; i < len(graph.nodes); i++ {
		capUV = append(capUV, graph.nodes[current].uv)
		next := -1
		for _, arc := range graph.arcs {
			if arc.From == current {
				next = arc.To
			}
		}
		if next < 0 {
			return zero, geom.ErrConvergence
		}
		current = next
	}
	area := new(big.Rat)
	for i, a := range capUV {
		b := capUV[(i+1)%len(capUV)]
		area.Add(area, new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0])))
	}
	if area.Sign() == 0 {
		return zero, geom.ErrConvergence
	}
	if area.Sign() < 0 {
		for i, j := 0, len(capUV)-1; i < j; i, j = i+1, j-1 {
			capUV[i], capUV[j] = capUV[j], capUV[i]
		}
	}
	capFace := tool.faces[active]
	if op == parametric.Difference {
		capFace.Reversed = !capFace.Reversed
	}
	cap, e := makeFace(capFace, patches[1][active].patch, capUV)
	if e != nil {
		return zero, e
	}
	add(cap, 1, active)
	result, history, e := Sew(ctx, parts, SewOptions{Tolerance: max(stock.tolerance, tool.tolerance), Proof: o.EdgeAgreement})
	if e != nil {
		return zero, e
	}
	out.StockEdges = make([][]int, len(stock.edges))
	section := map[int]bool{}
	for partIndex, part := range parts {
		owner := owners[partIndex]
		if owner[0] != 0 {
			continue
		}
		for _, use := range part.Face.Outer {
			original := -1
			for _, old := range stock.faces[owner[1]].Outer {
				a, b, p, q := old.UVStart, old.UVEnd, use.UVStart, use.UVEnd
				vertical := a.X == b.X && p.X == a.X && q.X == a.X && min(p.Y, q.Y) >= min(a.Y, b.Y) && max(p.Y, q.Y) <= max(a.Y, b.Y)
				horizontal := a.Y == b.Y && p.Y == a.Y && q.Y == a.Y && min(p.X, q.X) >= min(a.X, b.X) && max(p.X, q.X) <= max(a.X, b.X)
				if vertical || horizontal {
					if original >= 0 {
						return zero, geom.ErrConvergence
					}
					original = old.Edge
				}
			}
			edge := history.Edges[partIndex][use.Edge]
			if original < 0 {
				section[edge] = true
				continue
			}
			exists := false
			for _, id := range out.StockEdges[original] {
				exists = exists || id == edge
			}
			if !exists {
				out.StockEdges[original] = append(out.StockEdges[original], edge)
			}
		}
	}
	for id := range result.edges {
		if section[id] {
			out.SectionEdges = append(out.SectionEdges, id)
		}
	}
	if len(out.SectionEdges) != 4 {
		return zero, geom.ErrConvergence
	}
	out.Solid = result
	out.OriginalVertices = make([]int, len(stock.vertices))
	for i, p := range stock.vertices {
		out.OriginalVertices[i] = -1
		for j, q := range result.vertices {
			if p == q {
				if out.OriginalVertices[i] >= 0 {
					return zero, geom.ErrConvergence
				}
				out.OriginalVertices[i] = j
			}
		}
	}
	if out.OriginalVertices[fan.vertex] < 0 {
		return zero, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
