package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CombinePolynomialSplinePlaneClip supports one active cutter plane
// and single-span polynomial graph crossings of degree at most four. Exact
// Bernstein bounds prove one regular section per face. Native SplineUV controls
// must be losslessly representable; cap and original support curves are compared
// by exact whole-composition identities. A nonaffine crossing must be a graph
// over the cutter projection: XY when possible, then YZ, then ZX. Vertex events,
// mixed/nonlinear dependent terms, weighted patches and unresolved topology fail
// atomically. Other cutter planes strictly contain stock.
// Each face tries at most two axes; limits apply per bounded surface/edge proof
// and loop certificate, with at most64 total section pieces.
func CombinePolynomialSplinePlaneClip(ctx context.Context, stock, tool Solid, active int, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions) (SplineCornerPlaneClip, error) {
	return combinePolynomialSplinePlaneClip(ctx, stock, tool, active, op, o, nil, false)
}

// CombineRationalGraphSplinePlaneClip additionally accepts single-span
// polynomial supports with a cutter residual A(s)+B(s)t, where the free-axis
// degree of A is at most four and B is affine and never zero. The original
// surface is unchanged; the section is an exact rational Bézier trim in both
// original UV charts. The section endpoints must still be representable.
func CombineRationalGraphSplinePlaneClip(ctx context.Context, stock, tool Solid, active int, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions) (SplineCornerPlaneClip, error) {
	return combinePolynomialSplinePlaneClip(ctx, stock, tool, active, op, o, nil, true)
}

func combinePolynomialSplinePlaneClip(ctx context.Context, stock, tool Solid, active int, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions, spans *polynomialClipSpanState, rationalGraph bool) (SplineCornerPlaneClip, error) {
	var zero SplineCornerPlaneClip
	if ctx == nil || active < 0 || active >= len(tool.faces) || (op != parametric.Intersection && op != parametric.Difference) {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
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
	plane, e := polynomialPlaneFromPoints(raw)
	if e != nil {
		return zero, e
	}
	keep := plane.orientation
	if op == parametric.Difference {
		keep = -keep
	}
	out := SplineCornerPlaneClip{StockFaces: make([][]int, len(stock.faces)), ToolFaces: make([][]int, len(tool.faces)), ActiveToolFace: active}
	var parts []SewFace
	var owners []int
	var arcs []PCurve
	add := func(p SewFace, id int) {
		out.StockFaces[id] = []int{len(parts)}
		parts = append(parts, p)
		owners = append(owners, id)
	}
	for id, f := range patches[0] {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		h, e := geom.CertifySurfaceSupportingPlane(ctx, f.patch, raw[0], raw[1], raw[2], o.Plane)
		if e == nil {
			if !h.Complete || h.Coincident || h.HalfspaceSign == 0 || h.SideMask != 0 || h.CornerMask != 0 {
				return zero, geom.ErrConvergence
			}
			sign := 1
			if op == parametric.Difference {
				sign = -1
			}
			if sign*h.HalfspaceSign < 0 {
				add(stock.splineSewFace(stock.faces[id]), id)
				out.KeptStockFaces = append(out.KeptStockFaces, id)
			} else {
				out.DiscardedStockFaces = append(out.DiscardedStockFaces, id)
			}
			continue
		}
		if !errors.Is(e, geom.ErrConvergence) {
			return zero, e
		}
		face := stock.faces[id]
		var uv [][2]*big.Rat
		var sections []PCurve
		if _, ae := geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}); ae == nil {
			points := slabFaceUV(face)
			var heights []*big.Rat
			var ends [][2]*big.Rat
			for _, q := range points {
				j, e := f.patch.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
				if e != nil {
					return zero, e
				}
				h := plane.dot(j.Point)
				h.Sub(h, plane.offset)
				if h.Sign() == 0 {
					return zero, geom.ErrConvergence
				}
				heights = append(heights, new(big.Rat).Mul(h, big.NewRat(int64(keep), 1)))
			}
			for i, a := range points {
				j := (i + 1) % len(points)
				b := points[j]
				ha, hb := heights[i], heights[j]
				if ha.Sign() < 0 {
					uv = append(uv, a)
				}
				if ha.Sign()*hb.Sign() < 0 {
					alpha := new(big.Rat).Quo(ha, new(big.Rat).Sub(ha, hb))
					var q [2]*big.Rat
					for k := range q {
						q[k] = new(big.Rat).Add(a[k], new(big.Rat).Mul(alpha, new(big.Rat).Sub(b[k], a[k])))
					}
					uv = append(uv, q)
					ends = append(ends, q)
				}
			}
			if len(ends) != 2 {
				return zero, geom.ErrConvergence
			}
			section, e := quadraticLine(ends[0], ends[1])
			if e != nil {
				return zero, e
			}
			sections = []PCurve{section}
		} else {
			axis, sign := 0, 0
			if spans == nil {
				cert, e := certifyPolynomialPlaneFaceSection(ctx, stock, id, plane, 0, o)
				if errors.Is(e, geom.ErrConvergence) {
					cert, e = certifyPolynomialPlaneFaceSection(ctx, stock, id, plane, 1, o)
				}
				if e != nil && rationalGraph && errors.Is(e, geom.ErrConvergence) {
					for a := range 2 {
						q, s, err := rationalGraphPlaneTrim(f.patch, plane, a)
						if err == nil {
							sections, axis, sign, e = []PCurve{q}, a, s, nil
							break
						}
						if !errors.Is(err, geom.ErrConvergence) {
							return zero, err
						}
					}
				}
				if e != nil {
					return zero, e
				}
				if cert != nil {
					sections = []PCurve{cert.trim}
					axis, sign = cert.freeAxis, cert.sign
				}
			} else {
				cert, pieces, e := certifyPiecewiseGraphPlaneFaceSection(ctx, stock, id, plane, 0, PolynomialPlaneSpanOptions{MaxSpans: spans.maxSpans, Faces: o}, rationalGraph)
				if errors.Is(e, geom.ErrConvergence) {
					cert, pieces, e = certifyPiecewiseGraphPlaneFaceSection(ctx, stock, id, plane, 1, PolynomialPlaneSpanOptions{MaxSpans: spans.maxSpans, Faces: o}, rationalGraph)
				}
				if e != nil {
					return zero, e
				}
				sections = pieces
				axis, sign = cert.axis, cert.sign
				spans.knots = append(spans.knots, cert.Knots()...)
			}
			var chain [][2]*big.Rat
			for i, q := range sections {
				ends, _, qe := polynomialPlaneEndpoints(q)
				if qe != nil {
					return zero, qe
				}
				if i == 0 {
					chain = append(chain, ends[0])
				}
				chain = append(chain, ends[1])
			}
			reverse := append([][2]*big.Rat(nil), chain...)
			for i, j := 0, len(reverse)-1; i < j; i, j = i+1, j-1 {
				reverse[i], reverse[j] = reverse[j], reverse[i]
			}
			d := f.patch.Domain()
			lo := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}
			hi := [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
			if axis == 1 {
				if keep*sign > 0 {
					uv = append([][2]*big.Rat{lo}, chain...)
					uv = append(uv, [2]*big.Rat{lo[0], hi[1]})
				} else {
					uv = append([][2]*big.Rat{{hi[0], lo[1]}, hi}, reverse...)
				}
			} else if keep*sign > 0 {
				uv = append([][2]*big.Rat{lo, {hi[0], lo[1]}}, reverse...)
			} else {
				uv = append(append([][2]*big.Rat(nil), chain...), hi, [2]*big.Rat{lo[0], hi[1]})
			}
		}
		if len(sections) > 64-len(arcs) {
			return zero, geom.ErrConvergence
		}
		part, e := slabSewFace(ctx, face, f.patch, uv, o)
		if e != nil {
			return zero, e
		}
		for _, section := range sections {
			_, display, qe := polynomialPlaneEndpoints(section)
			if qe != nil {
				return zero, qe
			}
			found := false
			for i, u := range part.Face.Outer {
				q := section
				if u.UVStart == display[1] && u.UVEnd == display[0] {
					q = reversePolynomialPlaneCurve(q)
				} else if u.UVStart != display[0] || u.UVEnd != display[1] {
					continue
				}
				if found {
					return zero, geom.ErrConvergence
				}
				found = true
				part.Face.Outer[i].Curve = q
				part.Edges[u.Edge].Curve = surfaceTrimCurve{face.Surface, u.UVStart, u.UVEnd, q}
			}
			if !found {
				return zero, geom.ErrConvergence
			}
			capq, e := polynomialPlaneCapCurve(ctx, f.patch, section, patches[1][active].patch, plane.axes, o)
			if e != nil {
				return zero, e
			}
			_, sp, qe := polynomialPlaneEndpoints(section)
			if qe != nil {
				return zero, qe
			}
			_, cp, qe := polynomialPlaneEndpoints(capq)
			if qe != nil {
				return zero, qe
			}
			paired, pe := comparePolynomialTrimUse(ctx, parametric.Edge{Start: 0, End: 1, Curve: surfaceTrimCurve{face.Surface, sp[0], sp[1], section}, Range: parametric.Interval{Max: 1}}, tool.faces[active].Surface, Use{UVStart: cp[0], UVEnd: cp[1], Curve: capq}, o.EdgeAgreement)
			if pe != nil {
				return zero, pe
			}
			if !paired {
				return zero, geom.ErrConvergence
			}
			arcs = append(arcs, capq)
		}
		if e = validatePolynomialPlanePart(ctx, part); e != nil {
			return zero, e
		}
		add(part, id)
		out.ClippedStockFaces = append(out.ClippedStockFaces, id)
	}
	if len(arcs) < 3 || len(arcs) > 64 {
		return zero, geom.ErrConvergence
	}
	loop, e := polynomialPlaneCycle(ctx, arcs)
	if e != nil {
		return zero, e
	}
	face := tool.faces[active]
	if op == parametric.Difference {
		face.Reversed = !face.Reversed
	}
	var uv [][2]*big.Rat
	for _, q := range loop {
		ends, _, qe := polynomialPlaneEndpoints(q)
		if qe != nil {
			return zero, qe
		}
		uv = append(uv, ends[0])
	}
	cap, e := slabSewFace(ctx, face, patches[1][active].patch, uv, o)
	if e != nil {
		return zero, e
	}
	for i, q := range loop {
		u := cap.Face.Outer[i]
		cap.Face.Outer[i].Curve = q
		cap.Edges[i].Curve = surfaceTrimCurve{face.Surface, u.UVStart, u.UVEnd, q}
	}
	if e = validatePolynomialPlanePart(ctx, cap); e != nil {
		return zero, e
	}
	capID := len(parts)
	out.ToolFaces[active] = []int{capID}
	parts = append(parts, cap)
	owners = append(owners, -1)
	result, history, e := Sew(ctx, parts, SewOptions{Tolerance: max(stock.tolerance, tool.tolerance), Proof: o.EdgeAgreement})
	if e != nil {
		return zero, e
	}
	out.StockEdges = make([][]int, len(stock.edges))
	sectionIDs := map[int]bool{}
	for _, edge := range history.Edges[capID] {
		sectionIDs[edge] = true
	}
	for i, part := range parts {
		if owners[i] < 0 {
			continue
		}
		for _, use := range part.Face.Outer {
			edge := history.Edges[i][use.Edge]
			if sectionIDs[edge] {
				continue
			}
			oldID := -1
			for _, old := range stock.faces[owners[i]].Outer {
				a, b, p, q := old.UVStart, old.UVEnd, use.UVStart, use.UVEnd
				vertical := a.X == b.X && p.X == a.X && q.X == a.X && min(p.Y, q.Y) >= min(a.Y, b.Y) && max(p.Y, q.Y) <= max(a.Y, b.Y)
				horizontal := a.Y == b.Y && p.Y == a.Y && q.Y == a.Y && min(p.X, q.X) >= min(a.X, b.X) && max(p.X, q.X) <= max(a.X, b.X)
				if vertical || horizontal {
					if oldID >= 0 {
						return zero, geom.ErrConvergence
					}
					oldID = old.Edge
				}
			}
			if oldID < 0 {
				return zero, geom.ErrConvergence
			}
			exists := false
			for _, v := range out.StockEdges[oldID] {
				exists = exists || v == edge
			}
			if !exists {
				out.StockEdges[oldID] = append(out.StockEdges[oldID], edge)
			}
		}
	}
	covered := map[int]bool{}
	for _, ids := range out.StockEdges {
		for _, id := range ids {
			if covered[id] {
				return zero, geom.ErrConvergence
			}
			covered[id] = true
		}
	}
	for id := range result.edges {
		if sectionIDs[id] {
			if covered[id] {
				return zero, geom.ErrConvergence
			}
			covered[id] = true
			out.SectionEdges = append(out.SectionEdges, id)
		}
	}
	if len(covered) != len(result.edges) {
		return zero, geom.ErrConvergence
	}
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
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.Solid = result
	return out, nil
}
