package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
)

// SplinePlaneClipResult is shared with the corner-aware clip path.
type SplinePlaneClipResult = SplineCornerPlaneClip

// CombineSplinePlaneClip constructs Intersection or Difference with one active
// affine cutter plane and no original stock vertex on the section. It proves
// all stock faces either strictly to one side or crossed in a single exact
// affine segment; other curved crossings and all tangencies remain unresolved.
// The section must be a single simple oriented cycle of 3..64 segments, each
// endpoint strictly inside an original stock edge and inside the cutter face.
// Exact cutter convexity/inactive containment and closed source embedding are
// re-proved by the shared assembler. Limits apply per face/plane reproof.
func CombineSplinePlaneClip(ctx context.Context, stock, tool Solid, active int, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions) (SplinePlaneClipResult, error) {
	return combineSplinePlaneClipFans(ctx, stock, tool, active, nil, op, o)
}

func combineSplinePlaneClipFans(ctx context.Context, stock, tool Solid, active int, fans []*SplineCornerFan, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions) (SplinePlaneClipResult, error) {
	var zero SplineCornerPlaneClip
	if ctx == nil || active < 0 || active >= len(tool.faces) || (op != parametric.Intersection && op != parametric.Difference) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	limit := o.MaxFaces
	if limit == 0 {
		limit = 64
	}
	if limit < 1 || limit > 128 {
		return zero, geom.ErrInvalid
	}
	if len(stock.faces) > limit || len(tool.faces) > limit {
		return zero, geom.ErrConvergence
	}
	target, _, e := tool.splineEmbeddingFace(ctx, tool.faces[active], o)
	if e != nil {
		return zero, e
	}
	options := geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}
	targetChart, e := geom.NewAffineSurfaceChart(ctx, target.patch, options)
	if e != nil {
		return zero, e
	}
	plane := [3]geom.Vec3{tool.vertices[target.vertex[0]], tool.vertices[target.vertex[1]], tool.vertices[target.vertex[3]]}
	if tool.faces[active].Reversed {
		plane[1], plane[2] = plane[2], plane[1]
	}
	normal := func(p *geom.SurfacePatch, reversed bool) ([3]*big.Rat, error) {
		var r [3]*big.Rat
		d := p.Domain()
		j, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		for i := range r {
			k, l := (i+1)%3, (i+2)%3
			r[i] = new(big.Rat).Sub(new(big.Rat).Mul(j.Du[k], j.Dv[l]), new(big.Rat).Mul(j.Du[l], j.Dv[k]))
			if reversed {
				r[i].Neg(r[i])
			}
		}
		return r, nil
	}
	toolNormal, e := normal(target.patch, tool.faces[active].Reversed)
	if e != nil {
		return zero, e
	}
	proof := splinePlaneClipInput{stock: stock, tool: tool, active: active, pointFace: -1, corner: -1}
	keys := map[string]int{}
	pointFaces := map[int]bool{}
	branches := map[int]*SplineCornerAffineBranch{}
	vertices := map[int]bool{}
	for _, fan := range fans {
		if fan == nil || !fan.isolated.Matches(stock, tool, [2]int{fan.isolated.faces[0], active}) || vertices[fan.vertex] || pointFaces[fan.isolated.faces[0]] {
			return zero, geom.ErrInvalid
		}
		vertices[fan.vertex] = true
		pointFaces[fan.isolated.faces[0]] = true
		for _, anchor := range fan.anchors {
			if branches[anchor.pair[0]] != nil {
				return zero, geom.ErrConvergence
			}
			branch, e := CertifySplineCornerAffineBranch(ctx, anchor, o)
			if e != nil {
				return zero, e
			}
			branches[anchor.pair[0]] = branch
		}
	}
	proof.pointFaces, proof.corners = pointFaces, vertices
	for id, face := range stock.faces {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		source, _, e := stock.splineEmbeddingFace(ctx, face, o)
		if e != nil {
			return zero, e
		}
		halfspace, he := geom.CertifySurfaceSupportingPlane(ctx, source.patch, plane[0], plane[1], plane[2], o.Plane)
		if he == nil {
			if !halfspace.Complete || halfspace.Coincident || halfspace.HalfspaceSign == 0 || halfspace.SideMask != 0 || (halfspace.CornerMask != 0 && !pointFaces[id]) {
				return zero, geom.ErrConvergence
			}
			continue
		}
		if !errors.Is(he, geom.ErrConvergence) {
			return zero, he
		}
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		chart, e := geom.NewAffineSurfaceChart(ctx, source.patch, options)
		if e != nil {
			return zero, e
		}
		exact, e := geom.IntersectAffineSurfaceCharts(ctx, chart, targetChart, geom.AffineChartIntersectionOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
		if e != nil {
			return zero, e
		}
		if !exact.Complete || exact.Kind != geom.AffineChartsSegment || len(proof.arcs) >= 64 {
			return zero, geom.ErrConvergence
		}
		// Clipping against the finite cutter rectangle must not truncate a branch:
		// every endpoint must belong only to a stock side, never to the cutter wire.
		ends := exact.Endpoints
		sourceNormal, e := normal(source.patch, face.Reversed)
		if e != nil {
			return zero, e
		}
		direction := new(big.Rat)
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			cross := new(big.Rat).Sub(new(big.Rat).Mul(sourceNormal[j], toolNormal[k]), new(big.Rat).Mul(sourceNormal[k], toolNormal[j]))
			direction.Add(direction, new(big.Rat).Mul(new(big.Rat).Sub(ends[1].Point[i], ends[0].Point[i]), cross))
		}
		if direction.Sign() == 0 {
			return zero, geom.ErrConvergence
		}
		if direction.Sign() < 0 {
			ends[0], ends[1] = ends[1], ends[0]
		}
		arc := CornerSectionArc{Faces: [2]int{id, active}}
		for end, p := range ends {
			if p.BoundaryB != 0 || p.BoundaryA == 0 {
				return zero, geom.ErrConvergence
			}
			if p.BoundaryA&(p.BoundaryA-1) != 0 {
				branch := branches[id]
				if branch == nil {
					return zero, geom.ErrConvergence
				}
				bound := branch.endpoints[branch.corner]
				for i := range p.Point {
					if p.Point[i].Cmp(bound.Point[i]) != 0 {
						return zero, geom.ErrConvergence
					}
				}
				for i := range p.A {
					if p.A[i].Cmp(bound.A[i]) != 0 || p.B[i].Cmp(bound.B[i]) != 0 {
						return zero, geom.ErrConvergence
					}
				}
				key := "vertex:" + new(big.Int).SetInt64(int64(branch.anchor.vertex)).String()
				node, found := keys[key]
				if !found {
					node = len(proof.nodes)
					keys[key] = node
					proof.nodes = append(proof.nodes, cornerGraphNode{CornerSectionNode: CornerSectionNode{Point: p.Point, Vertex: branch.anchor.vertex, Edge: -1}, uv: p.B})
				} else {
					for i := range p.B {
						if proof.nodes[node].uv[i].Cmp(p.B[i]) != 0 {
							return zero, geom.ErrConvergence
						}
					}
				}
				if end == 0 {
					arc.From = node
				} else {
					arc.To = node
				}
				continue
			}
			side := geom.SurfaceEdge(0)
			mask := p.BoundaryA
			for mask != 1 {
				mask >>= 1
				side++
			}
			edgeID := source.edge[side]
			var use *Use
			for i := range face.Outer {
				if face.Outer[i].Edge == edgeID {
					if use != nil {
						return zero, geom.ErrConvergence
					}
					use = &face.Outer[i]
				}
			}
			if use == nil {
				return zero, geom.ErrConvergence
			}
			axis := 0
			start, finish := use.UVStart.X, use.UVEnd.X
			if start == finish {
				axis = 1
				start, finish = use.UVStart.Y, use.UVEnd.Y
			}
			if start == finish {
				return zero, geom.ErrConvergence
			}
			alpha := new(big.Rat).Quo(new(big.Rat).Sub(p.A[axis], new(big.Rat).SetFloat64(start)), new(big.Rat).Sub(new(big.Rat).SetFloat64(finish), new(big.Rat).SetFloat64(start)))
			if alpha.Sign() <= 0 || alpha.Cmp(big.NewRat(1, 1)) >= 0 {
				return zero, geom.ErrConvergence
			}
			if use.Reversed {
				alpha.Sub(big.NewRat(1, 1), alpha)
			}
			edge := stock.edges[edgeID]
			parameter := new(big.Rat).Add(new(big.Rat).SetFloat64(edge.Range.Min), new(big.Rat).Mul(alpha, new(big.Rat).Sub(new(big.Rat).SetFloat64(edge.Range.Max), new(big.Rat).SetFloat64(edge.Range.Min))))
			keyParts := []string{}
			for _, q := range p.Point {
				keyParts = append(keyParts, q.RatString())
			}
			key := strings.Join(keyParts, ",")
			node, found := keys[key]
			if found {
				old := proof.nodes[node]
				if old.Edge != edgeID || old.Parameter.Cmp(parameter) != 0 {
					return zero, geom.ErrConvergence
				}
				for i, q := range p.B {
					if old.uv[i].Cmp(q) != 0 {
						return zero, geom.ErrConvergence
					}
				}
			} else {
				node = len(proof.nodes)
				keys[key] = node
				proof.nodes = append(proof.nodes, cornerGraphNode{CornerSectionNode: CornerSectionNode{Point: p.Point, Vertex: -1, Edge: edgeID, Parameter: parameter}, uv: p.B})
			}
			if end == 0 {
				arc.From = node
			} else {
				arc.To = node
			}
		}
		if arc.From == arc.To {
			return zero, geom.ErrConvergence
		}
		proof.arcs = append(proof.arcs, arc)
	}
	n := len(proof.arcs)
	if n < 3 || len(proof.nodes) != n {
		return zero, geom.ErrConvergence
	}
	next, previous, owned := map[int]int{}, map[int]int{}, map[int]bool{}
	foundCorners := map[int]bool{}
	for _, node := range proof.nodes {
		if node.Vertex >= 0 {
			if foundCorners[node.Vertex] {
				return zero, geom.ErrConvergence
			}
			foundCorners[node.Vertex] = true
			continue
		}
		if owned[node.Edge] {
			return zero, geom.ErrConvergence
		}
		owned[node.Edge] = true
	}
	if len(foundCorners) != len(vertices) {
		return zero, geom.ErrConvergence
	}
	for i, arc := range proof.arcs {
		if _, ok := next[arc.From]; ok {
			return zero, geom.ErrConvergence
		}
		if _, ok := previous[arc.To]; ok {
			return zero, geom.ErrConvergence
		}
		next[arc.From], previous[arc.To] = i, i
	}
	if len(next) != n || len(previous) != n {
		return zero, geom.ErrConvergence
	}
	seen := map[int]bool{}
	current := 0
	for i := 0; i < n; i++ {
		if seen[current] {
			return zero, geom.ErrConvergence
		}
		seen[current] = true
		current = proof.arcs[next[current]].To
	}
	if current != 0 {
		return zero, geom.ErrConvergence
	}
	orient := func(a, b, c [2]*big.Rat) int {
		return new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0]))).Sign()
	}
	for i, a := range proof.arcs {
		for _, b := range proof.arcs[i+1:] {
			if a.From == b.From || a.From == b.To || a.To == b.From || a.To == b.To {
				continue
			}
			p, q, r, s := proof.nodes[a.From].uv, proof.nodes[a.To].uv, proof.nodes[b.From].uv, proof.nodes[b.To].uv
			x, y, z, w := orient(p, q, r), orient(p, q, s), orient(r, s, p), orient(r, s, q)
			if !(x != 0 && x == y || z != 0 && z == w) {
				return zero, geom.ErrConvergence
			}
		}
	}
	return combineSplinePlaneClip(ctx, proof, op, o)
}
