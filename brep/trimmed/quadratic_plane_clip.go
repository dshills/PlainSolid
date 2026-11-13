package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CombineQuadraticSplinePlaneClip supports one horizontal active cutter plane.
// Crossing faces are affine or have the exact regular polynomial graph form
// certified by CertifyQuadraticPlaneFaceSection. Other cutter planes strictly
// contain stock. The complete cap is a certified simple quadratic loop, with
// exact shared model edges and original support/edge ancestry. No vertex events,
// sampled classifications, knot-crossing branches or weighted patches are accepted.
// Each nonaffine face tries free U then free V, at most two bounded reproofs.
// Work limits apply per surface/edge proof and trim loop validation; the single-
// span gate remains unchanged, including at exact internal source knots.
func CombineQuadraticSplinePlaneClip(ctx context.Context, stock, tool Solid, active int, op parametric.BooleanOp, o SplineRectangleEmbeddingOptions) (SplineCornerPlaneClip, error) {
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
	if raw[0].Z != raw[1].Z || raw[0].Z != raw[2].Z {
		return zero, geom.ErrConvergence
	}
	cross := slabUVSide([2]*big.Rat{ratInset(raw[0].X), ratInset(raw[0].Y)}, [2]*big.Rat{ratInset(raw[1].X), ratInset(raw[1].Y)}, [2]*big.Rat{ratInset(raw[2].X), ratInset(raw[2].Y)})
	keep := cross.Sign()
	if keep == 0 {
		return zero, geom.ErrConvergence
	}
	if op == parametric.Difference {
		keep = -keep
	}
	height := ratInset(raw[0].Z)
	out := SplineCornerPlaneClip{StockFaces: make([][]int, len(stock.faces)), ToolFaces: make([][]int, len(tool.faces)), ActiveToolFace: active}
	var parts []SewFace
	var owners []int
	var arcs []ExactQuadraticUV
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
		var section ExactQuadraticUV
		if _, ae := geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}); ae == nil {
			points := slabFaceUV(face)
			var heights []*big.Rat
			var ends [][2]*big.Rat
			for _, q := range points {
				j, e := f.patch.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
				if e != nil {
					return zero, e
				}
				h := new(big.Rat).Sub(j.Point[2], height)
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
			section, e = quadraticLine(ends[0], ends[1])
			if e != nil {
				return zero, e
			}
		} else {
			cert, e := CertifyQuadraticPlaneFaceSection(ctx, stock, id, raw[0].Z, o)
			if errors.Is(e, geom.ErrConvergence) {
				cert, e = CertifyQuadraticPlaneFaceSectionAxis(ctx, stock, id, raw[0].Z, 1, o)
			}
			if e != nil {
				return zero, e
			}
			section = cert.trim
			ends, _ := section.Endpoints()
			d := f.patch.Domain()
			lo := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}
			hi := [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
			if cert.freeAxis == 1 {
				if keep*cert.sign > 0 {
					uv = [][2]*big.Rat{lo, ends[0], ends[1], {lo[0], hi[1]}}
				} else {
					uv = [][2]*big.Rat{ends[0], {hi[0], lo[1]}, hi, ends[1]}
				}
			} else if keep*cert.sign > 0 {
				uv = [][2]*big.Rat{lo, {hi[0], lo[1]}, ends[1], ends[0]}
			} else {
				uv = [][2]*big.Rat{ends[0], ends[1], hi, {lo[0], hi[1]}}
			}
		}
		part, e := slabSewFace(ctx, face, f.patch, uv, o)
		if e != nil {
			return zero, e
		}
		_, display := section.Endpoints()
		found := false
		for i, u := range part.Face.Outer {
			q := section
			if u.UVStart == display[1] && u.UVEnd == display[0] {
				q = reversePlaneQuadratic(q)
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
		if e = validateQuadraticPlanePart(ctx, part); e != nil {
			return zero, e
		}
		capq, e := quadraticPlaneCapCurve(ctx, f.patch, section, patches[1][active].patch, o)
		if e != nil {
			return zero, e
		}
		_, sp := section.Endpoints()
		_, cp := capq.Endpoints()
		paired, pe := compareSplineEdgeUse(ctx, parametric.Edge{Start: 0, End: 1, Curve: surfaceTrimCurve{face.Surface, sp[0], sp[1], section}, Range: parametric.Interval{Max: 1}}, tool.faces[active].Surface, Use{UVStart: cp[0], UVEnd: cp[1], Curve: capq}, o.EdgeAgreement)
		if pe != nil {
			return zero, pe
		}
		if !paired {
			return zero, geom.ErrConvergence
		}
		arcs = append(arcs, capq)
		add(part, id)
		out.ClippedStockFaces = append(out.ClippedStockFaces, id)
	}
	if len(arcs) < 3 || len(arcs) > 64 {
		return zero, geom.ErrConvergence
	}
	loop, e := quadraticPlaneCycle(arcs)
	if e != nil {
		return zero, e
	}
	face := tool.faces[active]
	if op == parametric.Difference {
		face.Reversed = !face.Reversed
	}
	var uv [][2]*big.Rat
	for _, q := range loop {
		ends, _ := q.Endpoints()
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
	if e = validateQuadraticPlanePart(ctx, cap); e != nil {
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

func quadraticLine(a, b [2]*big.Rat) (ExactQuadraticUV, error) {
	var h [3][3]*big.Rat
	for i := range h {
		h[i][2] = big.NewRat(1, 1)
		for axis := 0; axis < 2; axis++ {
			h[i][axis] = new(big.Rat).Add(a[axis], new(big.Rat).Mul(big.NewRat(int64(i), 2), new(big.Rat).Sub(b[axis], a[axis])))
		}
	}
	return NewExactQuadraticUV(h)
}
func reversePlaneQuadratic(q ExactQuadraticUV) ExactQuadraticUV {
	h := q.HomogeneousControls()
	h[0], h[2] = h[2], h[0]
	r, _ := NewExactQuadraticUV(h)
	return r
}
func validateQuadraticPlanePart(ctx context.Context, p SewFace) error {
	var loop trimvalidate.Loop
	for _, u := range p.Face.Outer {
		s, ok := TrimValidationSegment(u)
		if !ok {
			return geom.ErrConvergence
		}
		loop = append(loop, s)
	}
	r, e := trimvalidate.ValidateLoops(ctx, loop, nil, trimvalidate.Options{})
	if e != nil {
		return e
	}
	if r.Status != trimvalidate.Certified {
		return geom.ErrConvergence
	}
	return nil
}
func quadraticPlaneCapCurve(ctx context.Context, source *geom.SurfacePatch, q ExactQuadraticUV, cap *geom.SurfacePatch, o SplineRectangleEmbeddingOptions) (ExactQuadraticUV, error) {
	var zero ExactQuadraticUV
	h := q.HomogeneousControls()
	var samples [3][3]*big.Rat
	for i, t := range []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)} {
		a := new(big.Rat).Sub(big.NewRat(1, 1), t)
		weights := [3]*big.Rat{new(big.Rat).Mul(a, a), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(a, t)), new(big.Rat).Mul(t, t)}
		var uv [2]*big.Rat
		for axis := range uv {
			uv[axis] = new(big.Rat)
			for j := range h {
				uv[axis].Add(uv[axis], new(big.Rat).Mul(weights[j], h[j][axis]))
			}
		}
		jet, e := source.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
		if e != nil {
			return zero, e
		}
		samples[i] = jet.Point
	}
	var model [3][3]*big.Rat
	model[0], model[2] = samples[0], samples[2]
	for axis := range model[1] {
		model[1][axis] = new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), samples[1][axis]), new(big.Rat).Quo(new(big.Rat).Add(samples[0][axis], samples[2][axis]), big.NewRat(2, 1)))
	}
	d := cap.Domain()
	jet, e := cap.EvaluateRational(ctx, ratInset(d.UMin), ratInset(d.VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	det := new(big.Rat).Sub(new(big.Rat).Mul(jet.Du[0], jet.Dv[1]), new(big.Rat).Mul(jet.Du[1], jet.Dv[0]))
	if det.Sign() == 0 {
		return zero, geom.ErrConvergence
	}
	var control [3][3]*big.Rat
	for i, p := range model {
		x, y := new(big.Rat).Sub(p[0], jet.Point[0]), new(big.Rat).Sub(p[1], jet.Point[1])
		u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(x, jet.Dv[1]), new(big.Rat).Mul(y, jet.Dv[0])), det)
		v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(jet.Du[0], y), new(big.Rat).Mul(jet.Du[1], x)), det)
		control[i] = [3]*big.Rat{u.Add(u, ratInset(d.UMin)), v.Add(v, ratInset(d.VMin)), big.NewRat(1, 1)}
	}
	return NewExactQuadraticUV(control)
}
func quadraticPlaneCycle(arcs []ExactQuadraticUV) ([]ExactQuadraticUV, error) {
	degree := map[geom.Vec2]int{}
	for _, q := range arcs {
		_, p := q.Endpoints()
		degree[p[0]]++
		degree[p[1]]++
	}
	for _, n := range degree {
		if n != 2 {
			return nil, geom.ErrConvergence
		}
	}
	loop := []ExactQuadraticUV{arcs[0]}
	used := map[int]bool{0: true}
	_, first := arcs[0].Endpoints()
	end := first[1]
	for len(loop) < len(arcs) {
		found := -1
		var next ExactQuadraticUV
		for i, q := range arcs {
			if used[i] {
				continue
			}
			_, p := q.Endpoints()
			if p[0] == end || p[1] == end {
				if found >= 0 {
					return nil, geom.ErrConvergence
				}
				found = i
				next = q
				if p[1] == end {
					next = reversePlaneQuadratic(q)
				}
			}
		}
		if found < 0 {
			return nil, geom.ErrConvergence
		}
		used[found] = true
		loop = append(loop, next)
		_, p := next.Endpoints()
		end = p[1]
	}
	if end != first[0] {
		return nil, geom.ErrConvergence
	}
	area := new(big.Rat)
	for _, q := range loop {
		h := q.HomogeneousControls()
		var power [3][2]*big.Rat
		for axis := 0; axis < 2; axis++ {
			power[0][axis] = h[0][axis]
			power[1][axis] = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(h[1][axis], h[0][axis]))
			power[2][axis] = new(big.Rat).Add(new(big.Rat).Sub(h[0][axis], new(big.Rat).Mul(big.NewRat(2, 1), h[1][axis])), h[2][axis])
		}
		for i := 0; i < 3; i++ {
			for j := 1; j < 3; j++ {
				v := new(big.Rat).Sub(new(big.Rat).Mul(power[i][0], power[j][1]), new(big.Rat).Mul(power[i][1], power[j][0]))
				area.Add(area, v.Mul(v, big.NewRat(int64(j), int64(i+j))))
			}
		}
	}
	if area.Sign() == 0 {
		return nil, geom.ErrConvergence
	}
	if area.Sign() < 0 {
		for i, j := 0, len(loop)-1; i <= j; i, j = i+1, j-1 {
			a, b := loop[i], loop[j]
			loop[i], loop[j] = reversePlaneQuadratic(b), reversePlaneQuadratic(a)
		}
	}
	return loop, nil
}
