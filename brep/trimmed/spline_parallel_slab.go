package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineParallelSlab retains original stock and two cutter support identities.
// SectionEdges are keyed by cutter; StockEdges contain only surviving portions
// of original edges. OriginalVertices uses -1 for discarded vertices.
// CapIntersectionEdges and CapIntersection are populated only by the transverse
// crossing-cap API, and are excluded from both cutter SectionEdges lists.
type SplineParallelSlab struct {
	Solid                  Solid
	StockFaces, StockEdges [][]int
	ToolFaces              [2][][]int
	SectionEdges           [2][]int
	OriginalVertices       []int
	CapIntersectionEdges   []int
	CapIntersection        *SplineCapIntersection
}

// CombineSplineParallelSlab intersects stock with two opposite parallel active
// cutter halfspaces. Each cutter independently satisfies CombineSplinePlaneClip:
// no vertex events, one simple section, affine crossing faces, strictly inactive
// other cutter planes. The two caps must each be strictly inside the other kept
// halfspace. Original curved faces survive only when neither cut crosses them.
// UV polygon intersection is exact; new UV/model vertices must be losslessly
// representable. The bounded source-face proofs and edge reproofs each use o.
// No sampled classification or sequential rectangle-shell assumption is used.
func CombineSplineParallelSlab(ctx context.Context, stock Solid, tools [2]Solid, active [2]int, o SplineRectangleEmbeddingOptions) (SplineParallelSlab, error) {
	return combineSplineSeparatedPlaneCuts(ctx, stock, tools, active, o, true)
}

func combineSplineSeparatedPlaneCuts(ctx context.Context, stock Solid, tools [2]Solid, active [2]int, o SplineRectangleEmbeddingOptions, parallel bool) (SplineParallelSlab, error) {
	return combineSplinePlaneCuts(ctx, stock, tools, active, o, parallel, false)
}

func combineSplinePlaneCuts(ctx context.Context, stock Solid, tools [2]Solid, active [2]int, o SplineRectangleEmbeddingOptions, parallel, crossing bool) (SplineParallelSlab, error) {
	var zero SplineParallelSlab
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	var clips [2]SplinePlaneClipResult
	var capPatches [2]*geom.SurfacePatch
	var origins, normals [2][3]*big.Rat
	for k := 0; k < 2; k++ {
		r, e := CombineSplinePlaneClip(ctx, stock, tools[k], active[k], parametric.Intersection, o)
		if e != nil {
			return zero, e
		}
		clips[k] = r
		f, _, e := tools[k].splineEmbeddingFace(ctx, tools[k].faces[active[k]], o)
		if e != nil {
			return zero, e
		}
		capPatches[k] = f.patch
		d := f.patch.Domain()
		jet, e := f.patch.EvaluateRational(ctx, new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
		if e != nil {
			return zero, e
		}
		origins[k] = jet.Point
		for i := 0; i < 3; i++ {
			j, l := (i+1)%3, (i+2)%3
			normals[k][i] = new(big.Rat).Sub(new(big.Rat).Mul(jet.Du[j], jet.Dv[l]), new(big.Rat).Mul(jet.Du[l], jet.Dv[j]))
			if tools[k].faces[active[k]].Reversed {
				normals[k][i].Neg(normals[k][i])
			}
		}
	}
	if parallel {
		dot := new(big.Rat)
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			if new(big.Rat).Sub(new(big.Rat).Mul(normals[0][j], normals[1][k]), new(big.Rat).Mul(normals[0][k], normals[1][j])).Sign() != 0 {
				return zero, geom.ErrConvergence
			}
			dot.Add(dot, new(big.Rat).Mul(normals[0][i], normals[1][i]))
		}
		if dot.Sign() >= 0 {
			return zero, geom.ErrConvergence
		}
	}
	var crossedCaps [2]SewFace
	var capProof *SplineCapIntersection
	if crossing {
		var e error
		crossedCaps, capProof, e = clipSplineCrossingCaps(ctx, clips, active, capPatches, origins, normals, o)
		if e != nil {
			return zero, e
		}
	}
	if !crossing {
		// The cap is an affine image of this independently certified convex polygon.
		// A linear functional reaches its extrema at its vertices, so strict signs
		// prove exclusion over the entire cap fill, including its original wire.
		for k := 0; k < 2; k++ {
			if parallel {
				// Preserve the slab contract: a parallel plane is constant over
				// the whole cap, so its polygon need not be convex.
				h := new(big.Rat)
				for i := 0; i < 3; i++ {
					h.Add(h, new(big.Rat).Mul(normals[1-k][i], new(big.Rat).Sub(origins[k][i], origins[1-k][i])))
				}
				if h.Sign() >= 0 {
					return zero, geom.ErrConvergence
				}
				continue
			}
			cap := clips[k].Solid.faces[clips[k].ToolFaces[active[k]][0]]
			if !slabConvexCCW(slabFaceUV(cap)) {
				return zero, geom.ErrConvergence
			}
			for _, use := range cap.Outer {
				edge := clips[k].Solid.edges[use.Edge]
				id := edge.Start
				if use.Reversed {
					id = edge.End
				}
				p := clips[k].Solid.vertices[id]
				coords := [3]float64{p.X, p.Y, p.Z}
				h := new(big.Rat)
				for i := 0; i < 3; i++ {
					h.Add(h, new(big.Rat).Mul(normals[1-k][i], new(big.Rat).Sub(new(big.Rat).SetFloat64(coords[i]), origins[1-k][i])))
				}
				if h.Sign() >= 0 {
					return zero, geom.ErrConvergence
				}
			}
		}
	}
	out := SplineParallelSlab{StockFaces: make([][]int, len(stock.faces)), StockEdges: make([][]int, len(stock.edges))}
	var parts []SewFace
	owners := [][2]int{} // owner 0=stock, 1/2=cutter
	for id, face := range stock.faces {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if len(clips[0].StockFaces[id]) == 0 || len(clips[1].StockFaces[id]) == 0 {
			continue
		}
		fa := clips[0].Solid.faces[clips[0].StockFaces[id][0]]
		fb := clips[1].Solid.faces[clips[1].StockFaces[id][0]]
		f, _, e := stock.splineEmbeddingFace(ctx, face, o)
		if e != nil {
			return zero, e
		}
		// Unchanged faces may be nonlinear: keep their original support and wire.
		if slabSameUVWire(face, fa) && slabSameUVWire(face, fb) {
			out.StockFaces[id] = []int{len(parts)}
			parts = append(parts, stock.splineSewFace(face))
			owners = append(owners, [2]int{0, id})
			continue
		}
		if _, e = geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}); e != nil {
			return zero, e
		}
		pa, pb := slabFaceUV(fa), slabFaceUV(fb)
		if !slabConvexCCW(pa) || !slabConvexCCW(pb) {
			return zero, geom.ErrConvergence
		}
		uv := slabClipUV(pa, pb)
		if crossing && len(uv) == 0 {
			continue
		}
		if len(uv) < 3 || len(uv) > 12 || !slabConvexCCW(uv) {
			return zero, geom.ErrConvergence
		}
		part, e := slabSewFace(ctx, face, f.patch, uv, o)
		if e != nil {
			return zero, e
		}
		out.StockFaces[id] = []int{len(parts)}
		parts = append(parts, part)
		owners = append(owners, [2]int{0, id})
	}
	for k := 0; k < 2; k++ {
		out.ToolFaces[k] = make([][]int, len(tools[k].faces))
		out.ToolFaces[k][active[k]] = []int{len(parts)}
		cap := clips[k].Solid.faces[clips[k].ToolFaces[active[k]][0]]
		if crossing {
			parts = append(parts, crossedCaps[k])
		} else {
			parts = append(parts, clips[k].Solid.splineSewFace(cap))
		}
		owners = append(owners, [2]int{k + 1, active[k]})
	}
	result, history, e := Sew(ctx, parts, SewOptions{Tolerance: max(stock.tolerance, tools[0].tolerance, tools[1].tolerance), Proof: o.EdgeAgreement})
	if e != nil {
		return zero, e
	}
	generated := [2]map[int]bool{{}, {}}
	for i, part := range parts {
		owner := owners[i]
		for _, use := range part.Face.Outer {
			edge := history.Edges[i][use.Edge]
			if owner[0] > 0 {
				generated[owner[0]-1][edge] = true
				continue
			}
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
			if original >= 0 {
				found := false
				for _, v := range out.StockEdges[original] {
					found = found || v == edge
				}
				if !found {
					out.StockEdges[original] = append(out.StockEdges[original], edge)
				}
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
		if generated[0][id] && generated[1][id] {
			if !crossing || covered[id] {
				return zero, geom.ErrConvergence
			}
			covered[id] = true
			out.CapIntersectionEdges = append(out.CapIntersectionEdges, id)
			continue
		}
		for k := 0; k < 2; k++ {
			if generated[k][id] {
				if covered[id] {
					return zero, geom.ErrConvergence
				}
				covered[id] = true
				out.SectionEdges[k] = append(out.SectionEdges[k], id)
			}
		}
	}
	if crossing {
		if len(out.CapIntersectionEdges) != 1 {
			return zero, geom.ErrConvergence
		}
		capProof.Edge = out.CapIntersectionEdges[0]
		edge := result.edges[capProof.Edge]
		match := func(p [3]*big.Rat, v geom.Vec3) bool {
			return p[0].Cmp(new(big.Rat).SetFloat64(v.X)) == 0 && p[1].Cmp(new(big.Rat).SetFloat64(v.Y)) == 0 && p[2].Cmp(new(big.Rat).SetFloat64(v.Z)) == 0
		}
		if !match(capProof.Points[0], result.vertices[edge.Start]) {
			capProof.Points[0], capProof.Points[1] = capProof.Points[1], capProof.Points[0]
			for k := 0; k < 2; k++ {
				capProof.UV[k][0], capProof.UV[k][1] = capProof.UV[k][1], capProof.UV[k][0]
			}
		}
		if !match(capProof.Points[0], result.vertices[edge.Start]) || !match(capProof.Points[1], result.vertices[edge.End]) {
			return zero, geom.ErrConvergence
		}
		out.CapIntersection = capProof
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

func slabSameUVWire(a, b Face) bool {
	if len(a.Outer) != len(b.Outer) {
		return false
	}
	for i, u := range a.Outer {
		if u.UVStart != b.Outer[i].UVStart || u.UVEnd != b.Outer[i].UVEnd {
			return false
		}
	}
	return true
}
func slabFaceUV(f Face) [][2]*big.Rat {
	p := make([][2]*big.Rat, len(f.Outer))
	for i, u := range f.Outer {
		p[i] = [2]*big.Rat{new(big.Rat).SetFloat64(u.UVStart.X), new(big.Rat).SetFloat64(u.UVStart.Y)}
	}
	return p
}
func slabUVSide(a, b, p [2]*big.Rat) *big.Rat {
	return new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(p[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(p[0], a[0])))
}
func slabClipUV(p, q [][2]*big.Rat) [][2]*big.Rat {
	for j, a := range q {
		b := q[(j+1)%len(q)]
		var next [][2]*big.Rat
		add := func(v [2]*big.Rat) {
			if len(next) == 0 || next[len(next)-1][0].Cmp(v[0]) != 0 || next[len(next)-1][1].Cmp(v[1]) != 0 {
				next = append(next, v)
			}
		}
		for i, u := range p {
			v := p[(i+1)%len(p)]
			hu, hv := slabUVSide(a, b, u), slabUVSide(a, b, v)
			if hu.Sign() >= 0 {
				add(u)
			}
			if hu.Sign()*hv.Sign() < 0 {
				alpha := new(big.Rat).Quo(hu, new(big.Rat).Sub(hu, hv))
				var r [2]*big.Rat
				for k := range r {
					r[k] = new(big.Rat).Add(u[k], new(big.Rat).Mul(alpha, new(big.Rat).Sub(v[k], u[k])))
				}
				add(r)
			}
		}
		if len(next) > 1 && next[0][0].Cmp(next[len(next)-1][0]) == 0 && next[0][1].Cmp(next[len(next)-1][1]) == 0 {
			next = next[:len(next)-1]
		}
		p = next
	}
	return p
}
func slabSewFace(ctx context.Context, face Face, patch *geom.SurfacePatch, uv [][2]*big.Rat, o SplineRectangleEmbeddingOptions) (SewFace, error) {
	var part SewFace
	part.Face = Face{Surface: face.Surface, Reversed: face.Reversed}
	for _, q := range uv {
		u, e := geom.RepresentedExactScalar(q[0])
		if e != nil {
			return SewFace{}, e
		}
		v, e := geom.RepresentedExactScalar(q[1])
		if e != nil {
			return SewFace{}, e
		}
		jet, e := patch.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
		if e != nil {
			return SewFace{}, e
		}
		var p geom.Vec3
		for i, d := range []*float64{&p.X, &p.Y, &p.Z} {
			*d, e = geom.RepresentedExactScalar(jet.Point[i])
			if e != nil {
				return SewFace{}, e
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

// Independently check the halfplane-intersection premise, including winding,
// rather than inheriting it from a particular source wire authoring convention.
func slabConvexCCW(p [][2]*big.Rat) bool {
	if len(p) < 3 || len(p) > 12 {
		return false
	}
	for i, a := range p {
		for j := 0; j < i; j++ {
			if a[0].Cmp(p[j][0]) == 0 && a[1].Cmp(p[j][1]) == 0 {
				return false
			}
		}
		b := p[(i+1)%len(p)]
		positive := false
		for _, q := range p {
			s := slabUVSide(a, b, q).Sign()
			if s < 0 {
				return false
			}
			positive = positive || s > 0
		}
		if !positive {
			return false
		}
	}
	return true
}
