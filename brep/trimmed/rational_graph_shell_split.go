package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

type RationalGraphShellSplitOptions struct {
	Section RationalGraphShellSectionOptions
	Caps    RationalGraphSectionCapOptions
}

// RationalGraphShellHalf retains source face and cap face IDs in the resulting
// solid. SourceFaces and SourceEdges have one row per original element,
// nil if wholly removed. Edge indices cover the retained original intervals.
// CapFaces is ordered by the section's material regions; CapEdges retains
// one row per cap with the published model edge IDs in cap-local order.
type RationalGraphShellHalf struct {
	Solid       Solid
	SourceFaces [][]int
	SourceEdges [][]int
	CapFaces    []int
	CapEdges    [][]int
}

// RationalGraphShellSplit places the negative and positive material in
// Halves[0] and Halves[1], with respect to the original oriented plane.
type RationalGraphShellSplit struct {
	Halves  [2]RationalGraphShellHalf
	Section RationalGraphShellSection
}

// SplitRationalGraphSplineShell splits the complete certified transversal
// section into two exact closed boundaries, using the authored affine support
// for both caps. All source supports remain unchanged. Each intersected face
// must contain exactly one supported line or graph branch. By default model
// and UV endpoints must be exactly representable. The opt-in rational
// endpoint path supports quadratic section pieces and exact rational
// vertex identity independently of their display coordinates.
// Tangency, coincidence and unresolved geometry fail atomically. A plane
// strictly disjoint from the source returns the unchanged half and an empty half.
func (s Solid) SplitRationalGraphSplineShell(ctx context.Context, points [3]geom.Vec3, cap geom.Surface, domain geom.ParameterRectangle, o RationalGraphShellSplitOptions) (RationalGraphShellSplit, error) {
	var zero RationalGraphShellSplit
	if ctx == nil || cap == nil || !domain.Valid() {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	for _, p := range points {
		if !p.IsFinite() {
			return zero, geom.ErrInvalid
		}
	}
	plane, e := polynomialPlaneFromPoints(points)
	if e != nil {
		return zero, e
	}
	so := o.Section
	if so.MaxPieces == 0 {
		so.MaxPieces = 128
	}
	if so.MaxPieces < 1 || so.MaxPieces > 1024 || so.Spans.MaxSpans < 0 || so.Spans.MaxSpans > 128 {
		return zero, geom.ErrInvalid
	}
	if so.Regions == nil {
		so.Regions = &RationalGraphShellRegionOptions{}
	}
	var faces []splineEmbeddingFace
	if proof, e := s.validateSplineRectangleEmbedding(ctx, so.Spans.Faces, &faces); e != nil || !proof.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return zero, e
	}
	section, signs, e := s.discoverRationalGraphShellSection(ctx, points, plane, faces, so)
	if e != nil {
		return zero, e
	}
	out := RationalGraphShellSplit{Section: section}
	if len(section.Branches) == 0 {
		h := (signs[0] + 1) / 2
		out.Halves[h].Solid = s
		for i := range out.Halves {
			out.Halves[i].SourceFaces = make([][]int, len(faces))
			out.Halves[i].SourceEdges = make([][]int, len(s.edges))
		}
		for i, sign := range signs {
			if sign != signs[0] {
				return zero, geom.ErrConvergence
			}
			out.Halves[h].SourceFaces[i] = []int{i}
		}
		for i := range s.edges {
			out.Halves[h].SourceEdges[i] = []int{i}
		}
		return out, nil
	}
	caps, e := section.Regions.BuildCaps(ctx, cap, domain, o.Caps)
	if e != nil {
		return zero, e
	}
	var parts [2][]SewFace
	var owners [2][][]int
	crossing := make([]*RationalGraphShellBranch, len(faces))
	for i := range section.Branches {
		b := &section.Branches[i]
		crossing[b.Face] = b
	}
	for i := range out.Halves {
		out.Halves[i].SourceFaces = make([][]int, len(faces))
		out.Halves[i].SourceEdges = make([][]int, len(s.edges))
	}
	add := func(half, source int, p SewFace, edgeOwners []int) {
		out.Halves[half].SourceFaces[source] = []int{len(parts[half])}
		parts[half] = append(parts[half], p)
		owners[half] = append(owners[half], edgeOwners)
	}
	for id, f := range faces {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if signs[id] != 0 {
			src := s.faces[id]
			p, old := s.splineSewFaceMapped(src, nil)
			if o.Caps.RationalEndpoints {
				var e error
				p, e = rationalQuadraticSewFace(ctx, src, f.patch, slabFaceUV(src), so.Spans.Faces.Patches)
				if e != nil {
					return zero, e
				}
				old, e = rationalGraphFragmentEdgeSources(src, p)
				if e != nil {
					return zero, e
				}
			}
			add((signs[id]+1)/2, id, p, old)
			continue
		}
		for half := range 2 {
			b := crossing[id]
			if b == nil {
				return zero, geom.ErrConvergence
			}
			q, e := splitRationalGraphFaceFragment(ctx, s.faces[id], f.patch, *b, plane, 2*half-1, so.Spans.Faces, o.Caps.RationalEndpoints)
			if e != nil {
				return zero, fmt.Errorf("trimmed: plane split face %d: %w", id, e)
			}
			old, e := rationalGraphFragmentEdgeSources(s.faces[id], q)
			if e != nil {
				return zero, e
			}
			add(half, id, q, old)
		}
	}
	for h := range out.Halves {
		r := &out.Halves[h]
		for _, q := range caps {
			p := q.Boundary
			if h == 1 {
				p.Face.Reversed = !p.Face.Reversed
			}
			r.CapFaces = append(r.CapFaces, len(parts[h]))
			parts[h] = append(parts[h], p)
		}
		if o.Caps.RationalEndpoints {
			if e := proveRationalSplitVertexIdentity(ctx, parts[h]); e != nil {
				return zero, e
			}
		}
		solid, history, e := Sew(ctx, parts[h], SewOptions{Tolerance: s.tolerance, MaxFaces: len(parts[h]), Proof: so.Spans.Faces.EdgeAgreement})
		if e != nil {
			return zero, e
		}
		for _, id := range r.CapFaces {
			r.CapEdges = append(r.CapEdges, history.Edges[id])
		}
		edgeSources := make([]int, len(solid.edges))
		for i := range edgeSources {
			edgeSources[i] = -2
		}
		set := func(id, src int) bool {
			if edgeSources[id] == -2 {
				edgeSources[id] = src
			}
			return edgeSources[id] == src
		}
		for part, old := range owners[h] {
			for i, src := range old {
				if !set(history.Edges[part][i], src) {
					return zero, geom.ErrConvergence
				}
			}
		}
		for _, cap := range r.CapEdges {
			for _, id := range cap {
				if !set(id, -1) {
					return zero, geom.ErrConvergence
				}
			}
		}
		for id, src := range edgeSources {
			if src >= 0 {
				r.SourceEdges[src] = append(r.SourceEdges[src], id)
			} else if src != -1 {
				return zero, geom.ErrConvergence
			}
		}
		r.Solid = solid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

func rationalGraphFragmentEdgeSources(f Face, part SewFace) ([]int, error) {
	out := make([]int, len(part.Edges))
	for _, u := range part.Face.Outer {
		out[u.Edge] = -1
		p := [2][2]*big.Rat{{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}, {ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}}
		if x, ok := u.Curve.(ExactQuadraticUV); ok && x.rationalEndpoints {
			p, _ = x.Endpoints()
		}
		for _, v := range f.Outer {
			a, b := [2]float64{v.UVStart.X, v.UVStart.Y}, [2]float64{v.UVEnd.X, v.UVEnd.Y}
			on := func(fixed int) bool {
				other := 1 - fixed
				if a[fixed] != b[fixed] {
					return false
				}
				c := ratInset(a[fixed])
				lo, hi := ratInset(min(a[other], b[other])), ratInset(max(a[other], b[other]))
				for _, q := range p {
					if q[fixed].Cmp(c) != 0 || q[other].Cmp(lo) < 0 || q[other].Cmp(hi) > 0 {
						return false
					}
				}
				return true
			}
			if on(0) || on(1) {
				if out[u.Edge] >= 0 {
					return nil, geom.ErrConvergence
				}
				out[u.Edge] = v.Edge
			}
		}
		if out[u.Edge] < 0 && u.Curve == nil {
			return nil, geom.ErrConvergence
		}
	}
	return out, nil
}

func splitRationalGraphFaceFragment(ctx context.Context, face Face, patch *geom.SurfacePatch, b RationalGraphShellBranch, plane polynomialPlaneEquation, side int, o SplineRectangleEmbeddingOptions, rationalEndpoints bool) (SewFace, error) {
	var uv [][2]*big.Rat
	if b.freeAxis < 0 {
		corners := slabFaceUV(face)
		var heights [4]*big.Rat
		for i, p := range corners {
			q, e := patch.EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchEvaluationOptions{
				MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits,
			})
			if e != nil {
				return SewFace{}, e
			}
			heights[i] = new(big.Rat).Sub(plane.dot(q.Point), plane.offset)
		}
		for i, a := range corners {
			j := (i + 1) % 4
			b, p, q := corners[j], heights[i], heights[j]
			if p.Sign()*plane.orientation == side {
				uv = append(uv, a)
			}
			if p.Sign()*q.Sign() < 0 {
				t := new(big.Rat).Quo(p, new(big.Rat).Sub(p, q))
				var x [2]*big.Rat
				for k := range x {
					x[k] = new(big.Rat).Add(a[k], new(big.Rat).Mul(t, new(big.Rat).Sub(b[k], a[k])))
				}
				uv = append(uv, x)
			}
		}
	} else {
		chain := make([][2]*big.Rat, len(b.Pieces)+1)
		for i, piece := range b.Pieces {
			ends, _, e := graphSectionCapEndpoints(piece, rationalEndpoints)
			if e != nil {
				return SewFace{}, e
			}
			chain[i], chain[i+1] = ends[0], ends[1]
		}
		if b.reversed {
			reverseGraphUVChain(chain)
		}
		d := patch.Domain()
		lo, hi := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}, [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
		retainBelow := side != b.positive
		if b.freeAxis == 1 {
			if retainBelow {
				uv = append([][2]*big.Rat{lo}, chain...)
				uv = append(uv, [2]*big.Rat{lo[0], hi[1]})
			} else {
				reverseGraphUVChain(chain)
				uv = append([][2]*big.Rat{{hi[0], lo[1]}, hi}, chain...)
			}
		} else if retainBelow {
			reverseGraphUVChain(chain)
			uv = append([][2]*big.Rat{lo, {hi[0], lo[1]}}, chain...)
		} else {
			uv = append(chain, hi, [2]*big.Rat{lo[0], hi[1]})
		}
	}
	var part SewFace
	var e error
	if rationalEndpoints {
		part, e = rationalQuadraticSewFace(ctx, face, patch, uv, o.Patches)
	} else {
		part, e = slabSewFace(ctx, face, patch, uv, o)
	}
	if e != nil {
		return SewFace{}, e
	}
	for _, piece := range b.Pieces {
		if rationalEndpoints {
			src, ok := piece.(ExactQuadraticUV)
			if !ok {
				return SewFace{}, geom.ErrConvergence
			}
			q, e := NewRationalEndpointQuadraticUV(src.HomogeneousControls())
			if e != nil {
				return SewFace{}, e
			}
			piece = q
		}
		_, p, e := graphSectionCapEndpoints(piece, rationalEndpoints)
		if e != nil {
			return SewFace{}, e
		}
		match := 0
		for i := range part.Face.Outer {
			u := &part.Face.Outer[i]
			var q PCurve
			if u.UVStart == p[0] && u.UVEnd == p[1] {
				q = piece
			} else if u.UVStart == p[1] && u.UVEnd == p[0] {
				q = reversePolynomialPlaneCurve(piece)
			} else {
				continue
			}
			u.Curve = q
			part.Edges[u.Edge].Curve = surfaceTrimCurve{face.Surface, u.UVStart, u.UVEnd, q}
			match++
		}
		if match != 1 {
			return SewFace{}, geom.ErrConvergence
		}
	}
	if e = validatePolynomialPlanePart(ctx, part); e != nil {
		return SewFace{}, e
	}
	return part, nil
}

func reverseGraphUVChain(chain [][2]*big.Rat) {
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
}
