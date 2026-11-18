//go:build research

package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineHalfspaceCut identifies one active, original affine cutter face.
// The tool's remaining faces must independently contain the complete stock.
type SplineHalfspaceCut struct {
	Tool       Solid
	ActiveFace int
}

// SplineMultiPlaneCuts retains one original owner for every face and the
// surviving stock edge intervals, stock/cap seams, and cap/cap seams.
// CapIntersections contain independently corroborated original chart UVs.
type SplineMultiPlaneCuts struct {
	Solid                  Solid
	StockFaces, StockEdges [][]int
	ToolFaces              [][][]int
	SectionEdges           [][]int
	CapIntersections       []SplineMultiCapIntersection
	OriginalVertices       []int
}

type SplineMultiCapIntersection struct {
	Cuts [2]int
	SplineCapIntersection
}

// CombineSplineMultiPlaneCuts intersects stock with three through eight
// independently certified active halfspaces. Crossing stock supports must
// be affine. Untouched curved faces are retained with their entire original
// wire. Every active cap must have positive area after all cuts; coincident,
// inactive, or unrepresentable output rejects atomically. Caps and stock
// faces are clipped simultaneously in their independent original charts.
func CombineSplineMultiPlaneCuts(ctx context.Context, stock Solid, cuts []SplineHalfspaceCut,
	o SplineRectangleEmbeddingOptions) (out SplineMultiPlaneCuts, err error) {
	defer func() {
		if err != nil {
			out = SplineMultiPlaneCuts{}
		}
	}()
	if ctx == nil || len(cuts) < 3 || len(cuts) > 8 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	type planeProof struct {
		clip           SplinePlaneClipResult
		patch          *geom.SurfacePatch
		origin, normal [3]*big.Rat
		cap            Face
	}
	planes := make([]planeProof, len(cuts))
	options := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits}
	for i, c := range cuts {
		r, e := CombineSplinePlaneClip(ctx, stock, c.Tool, c.ActiveFace, parametric.Intersection, o)
		if e != nil {
			return out, fmt.Errorf("stock clip by cutter %d: %w", i, e)
		}
		f, _, e := c.Tool.splineEmbeddingFace(ctx, c.Tool.faces[c.ActiveFace], o)
		if e != nil {
			return out, e
		}
		p := &planes[i]
		p.clip, p.patch = r, f.patch
		p.cap = r.Solid.faces[r.ToolFaces[c.ActiveFace][0]]
		d := p.patch.Domain()
		j, e := p.patch.EvaluateRational(ctx, ratInset(d.UMin), ratInset(d.VMin), options)
		if e != nil {
			return out, e
		}
		p.origin = j.Point
		for a := range p.normal {
			u, v := (a+1)%3, (a+2)%3
			p.normal[a] = new(big.Rat).Sub(new(big.Rat).Mul(j.Du[u], j.Dv[v]), new(big.Rat).Mul(j.Du[v], j.Dv[u]))
			if p.cap.Reversed {
				p.normal[a].Neg(p.normal[a])
			}
		}
	}
	bits := o.Patches.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	var arithmetic int
	limit := o.Patches.MaxOperations
	if limit == 0 {
		limit = 16000000
	}
	check := func(v *big.Rat) error {
		arithmetic++
		if arithmetic > limit || v.Num().BitLen() > bits || v.Denom().BitLen() > bits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	value := func(p *planeProof, xyz [3]*big.Rat) (*big.Rat, error) {
		v := new(big.Rat)
		for i := range xyz {
			v.Add(v, new(big.Rat).Mul(p.normal[i], new(big.Rat).Sub(xyz[i], p.origin[i])))
			if e := check(v); e != nil {
				return nil, e
			}
		}
		return v, nil
	}
	clip := func(poly [][2]*big.Rat, height func([2]*big.Rat) (*big.Rat, error)) ([][2]*big.Rat, error) {
		if len(poly) == 0 {
			return nil, nil
		}
		signs := make([]*big.Rat, len(poly))
		for i, v := range poly {
			h, e := height(v)
			if e != nil {
				return nil, e
			}
			signs[i] = h
		}
		var next [][2]*big.Rat
		for i, a := range poly {
			j := (i + 1) % len(poly)
			b := poly[j]
			u, v := signs[i], signs[j]
			if u.Sign() <= 0 {
				next = append(next, a)
			}
			if u.Sign()*v.Sign() >= 0 {
				continue
			}
			t := new(big.Rat).Quo(u, new(big.Rat).Sub(u, v))
			if e := check(t); e != nil {
				return nil, e
			}
			var x [2]*big.Rat
			for k := range x {
				x[k] = new(big.Rat).Add(a[k], new(big.Rat).Mul(t, new(big.Rat).Sub(b[k], a[k])))
				if e := check(x[k]); e != nil {
					return nil, e
				}
			}
			next = append(next, x)
		}
		return next, nil
	}
	out.StockFaces, out.StockEdges = make([][]int, len(stock.faces)), make([][]int, len(stock.edges))
	out.ToolFaces, out.SectionEdges = make([][][]int, len(cuts)), make([][]int, len(cuts))
	var parts []SewFace
	var owners [][2]int // kind -1 = original stock; otherwise cutter index
	for i, original := range stock.faces {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		var caps []Face
		for _, p := range planes {
			if len(p.clip.StockFaces[i]) == 0 {
				break
			}
			caps = append(caps, p.clip.Solid.faces[p.clip.StockFaces[i][0]])
		}
		if len(caps) != len(planes) {
			continue
		}
		unchanged := true
		for _, f := range caps {
			unchanged = unchanged && slabSameUVWire(original, f)
		}
		if unchanged {
			out.StockFaces[i] = []int{len(parts)}
			parts = append(parts, stock.splineSewFace(original))
			owners = append(owners, [2]int{-1, i})
			continue
		}
		f, _, e := stock.splineEmbeddingFace(ctx, original, o)
		if e != nil {
			return out, e
		}
		if _, e = geom.NewAffineSurfaceChart(ctx, f.patch, geom.AffineSurfaceChartOptions{MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits}); e != nil {
			return out, e
		}
		polygon := slabFaceUV(original)
		if !slabConvexCCW(polygon) {
			return out, geom.ErrConvergence
		}
		for k := range planes {
			polygon, e = clip(polygon, func(uv [2]*big.Rat) (*big.Rat, error) {
				j, e := f.patch.EvaluateRational(ctx, uv[0], uv[1], options)
				if e != nil {
					return nil, e
				}
				return value(&planes[k], j.Point)
			})
			if e != nil {
				return out, e
			}
		}
		if len(polygon) < 3 {
			continue
		}
		if !slabConvexCCW(polygon) {
			return out, geom.ErrConvergence
		}
		part, e := slabSewFace(ctx, original, f.patch, polygon, o)
		if e != nil {
			return out, e
		}
		out.StockFaces[i] = []int{len(parts)}
		parts = append(parts, part)
		owners = append(owners, [2]int{-1, i})
	}
	for i, p := range planes {
		polygon := slabFaceUV(p.cap)
		if !slabConvexCCW(polygon) {
			return out, geom.ErrConvergence
		}
		for k := range planes {
			if k == i {
				continue
			}
			// Entire initial cap lying on another active plane has two
			// ambiguous surface owners; reject rather than choose one.
			allZero := len(polygon) > 0
			var e error
			polygon, e = clip(polygon, func(uv [2]*big.Rat) (*big.Rat, error) {
				j, e := p.patch.EvaluateRational(ctx, uv[0], uv[1], options)
				if e != nil {
					return nil, e
				}
				h, e := value(&planes[k], j.Point)
				if e == nil {
					allZero = allZero && h.Sign() == 0
				}
				return h, e
			})
			if e != nil {
				return out, e
			}
			if allZero {
				return out, geom.ErrConvergence
			}
		}
		if !slabConvexCCW(polygon) {
			return out, geom.ErrConvergence
		}
		part, e := slabSewFace(ctx, p.cap, p.patch, polygon, o)
		if e != nil {
			return out, e
		}
		out.ToolFaces[i] = make([][]int, len(cuts[i].Tool.faces))
		out.ToolFaces[i][cuts[i].ActiveFace] = []int{len(parts)}
		parts = append(parts, part)
		owners = append(owners, [2]int{i, cuts[i].ActiveFace})
	}
	tolerance := stock.tolerance
	for _, c := range cuts {
		tolerance = max(tolerance, c.Tool.tolerance)
	}
	result, h, e := Sew(ctx, parts, SewOptions{Tolerance: tolerance, Proof: o.EdgeAgreement})
	if e != nil {
		return out, e
	}
	// Sew's general entry point accepts a tolerance. Here both stored
	// endpoints must be exactly identical before any ancestry is published.
	for i, part := range parts {
		for j, v := range part.Vertices {
			if result.vertices[h.Vertices[i][j]] != v {
				return out, geom.ErrConvergence
			}
		}
	}
	type edgeOwner struct {
		source         [2]int
		use            Use
		part, original int
	}
	edgeOwners := make([][]edgeOwner, len(result.edges))
	for i, part := range parts {
		owner := owners[i]
		for _, u := range part.Face.Outer {
			old := -1
			if owner[0] < 0 {
				for _, v := range stock.faces[owner[1]].Outer {
					a, b := v.UVStart, v.UVEnd
					p, q := u.UVStart, u.UVEnd
					x := a.X == b.X && p.X == a.X && q.X == a.X && min(p.Y, q.Y) >= min(a.Y, b.Y) && max(p.Y, q.Y) <= max(a.Y, b.Y)
					y := a.Y == b.Y && p.Y == a.Y && q.Y == a.Y && min(p.X, q.X) >= min(a.X, b.X) && max(p.X, q.X) <= max(a.X, b.X)
					if x || y {
						if old >= 0 {
							return out, geom.ErrConvergence
						}
						old = v.Edge
					}
				}
			}
			edge := h.Edges[i][u.Edge]
			edgeOwners[edge] = append(edgeOwners[edge], edgeOwner{owner, u, i, old})
		}
	}
	for edge, uses := range edgeOwners {
		if len(uses) != 2 {
			return out, geom.ErrConvergence
		}
		if uses[0].source[0] > uses[1].source[0] {
			uses[0], uses[1] = uses[1], uses[0]
		}
		a, b := uses[0], uses[1]
		if a.source[0] < 0 {
			if b.source[0] < 0 {
				if a.original < 0 || a.original != b.original {
					return out, geom.ErrConvergence
				}
				out.StockEdges[a.original] = append(out.StockEdges[a.original], edge)
			} else {
				if a.original >= 0 {
					return out, geom.ErrConvergence
				}
				out.SectionEdges[b.source[0]] = append(out.SectionEdges[b.source[0]], edge)
			}
			continue
		}
		if a.source[0] == b.source[0] {
			return out, geom.ErrConvergence
		}
		entry := SplineMultiCapIntersection{Cuts: [2]int{a.source[0], b.source[0]}, SplineCapIntersection: SplineCapIntersection{Edge: edge, Faces: [2]int{a.source[1], b.source[1]}}}
		for j, u := range uses {
			p := parts[u.part]
			from, to := p.Edges[u.use.Edge].Start, p.Edges[u.use.Edge].End
			uv := [2]geom.Vec2{u.use.UVStart, u.use.UVEnd}
			if u.use.Reversed {
				from, to = to, from
			}
			if h.Vertices[u.part][from] != result.edges[edge].Start {
				from, to = to, from
				uv[0], uv[1] = uv[1], uv[0]
			}
			if h.Vertices[u.part][from] != result.edges[edge].Start || h.Vertices[u.part][to] != result.edges[edge].End {
				return out, geom.ErrConvergence
			}
			for k, v := range uv {
				entry.UV[j][k] = [2]*big.Rat{ratInset(v.X), ratInset(v.Y)}
				point, e := planes[u.source[0]].patch.EvaluateRational(ctx, entry.UV[j][k][0], entry.UV[j][k][1], options)
				if e != nil {
					return out, e
				}
				vertex := [2]int{result.edges[edge].Start, result.edges[edge].End}[k]
				w := result.vertices[vertex]
				for m, coordinate := range [3]float64{w.X, w.Y, w.Z} {
					if point.Point[m].Cmp(ratInset(coordinate)) != 0 {
						return out, geom.ErrConvergence
					}
				}
				if j == 0 {
					entry.Points[k] = point.Point
				}
			}
		}
		out.CapIntersections = append(out.CapIntersections, entry)
	}
	sort.Slice(out.CapIntersections, func(i, j int) bool {
		a, b := out.CapIntersections[i], out.CapIntersections[j]
		if a.Cuts != b.Cuts {
			if a.Cuts[0] != b.Cuts[0] {
				return a.Cuts[0] < b.Cuts[0]
			}
			return a.Cuts[1] < b.Cuts[1]
		}
		return a.Edge < b.Edge
	})
	out.OriginalVertices = make([]int, len(stock.vertices))
	for i, v := range stock.vertices {
		out.OriginalVertices[i] = -1
		for j, w := range result.vertices {
			if v == w {
				if out.OriginalVertices[i] >= 0 {
					return out, geom.ErrConvergence
				}
				out.OriginalVertices[i] = j
			}
		}
	}
	out.Solid = result
	return out, ctx.Err()
}
