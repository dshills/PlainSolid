package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CombineAxialWrappedCylinders regularizes Boolean operations between two
// canonical rational extrusions of equal or concentric homothetic profiles.
// Supports disjoint, touching, partially overlapping and nested height
// intervals, with stepped caps and enclosed voids. For general rational
// profiles the proof compares the parameterized curves and allows different
// knot ranges. Changed heights preserve the exact rational U support.
func CombineAxialWrappedCylinders(ctx context.Context, a, b Solid, op parametric.BooleanOp, o geom.SurfacePatchOptions) (Solid, SplineShellCombineHistory, error) {
	var h SplineShellCombineHistory
	if ctx == nil || op > parametric.Difference {
		return Solid{}, h, geom.ErrInvalid
	}
	wa, pa, e := a.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return Solid{}, h, e
	}
	wb, pb, e := b.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return Solid{}, h, e
	}
	if sameCylinderAxisAndHeight(pa, pb) && pa.Radius.Cmp(pb.Radius) != 0 {
		return combineConcentricWrappedCylinders(ctx, a, b, wa, wb, pa, pb, op, o)
	}
	if op == parametric.Difference && sameCylinderAxis(pa, pb) && pa.Radius.Cmp(pb.Radius) > 0 &&
		pa.LowerZ.Cmp(pb.LowerZ) <= 0 && pa.UpperZ.Cmp(pb.UpperZ) >= 0 {
		return subtractContainedWrappedCylinder(ctx, a, b, wa, wb, o)
	}
	if sameCylinderAxis(pa, pb) && pa.Radius.Cmp(pb.Radius) != 0 {
		return combineSteppedWrappedCylinders(ctx, a, b, [2]*geom.WrappedSurfacePatch{wa, wb},
			[2]geom.WrappedCylinderEmbedding{pa, pb}, op, o)
	}
	xy := geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 0, 0}}
	rim := func(w *geom.WrappedSurfacePatch) (*geom.ExactSplineCurve, error) {
		c, e := w.FundamentalPatch().IsoCurve(1, w.Domain().VMin, o)
		if e != nil {
			return nil, e
		}
		return c.Transform(xy, o)
	}
	ra, e := rim(wa)
	if e != nil {
		return Solid{}, h, e
	}
	rb, e := rim(wb)
	if e != nil {
		return Solid{}, h, e
	}
	p, e := geom.CompareExactCurvesContext(ctx, ra, rb, geom.ExactCurveComparisonOptions{
		NormalizeDomains: true, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if e != nil || !p.Complete || !p.Equal {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, h, e
	}
	type span struct{ lo, hi *big.Rat }
	sa, sb := span{pa.LowerZ, pa.UpperZ}, span{pb.LowerZ, pb.UpperZ}
	minimum := func(a, b *big.Rat) *big.Rat {
		if a.Cmp(b) <= 0 {
			return a
		}
		return b
	}
	maximum := func(a, b *big.Rat) *big.Rat {
		if a.Cmp(b) >= 0 {
			return a
		}
		return b
	}
	var spans []span
	switch op {
	case parametric.Union:
		if sa.hi.Cmp(sb.lo) < 0 {
			spans = []span{sa, sb}
		} else if sb.hi.Cmp(sa.lo) < 0 {
			spans = []span{sb, sa}
		} else {
			spans = []span{{minimum(sa.lo, sb.lo), maximum(sa.hi, sb.hi)}}
		}
	case parametric.Intersection:
		x := span{maximum(sa.lo, sb.lo), minimum(sa.hi, sb.hi)}
		if x.lo.Cmp(x.hi) < 0 {
			spans = []span{x}
		}
	case parametric.Difference:
		if sa.hi.Cmp(sb.lo) <= 0 || sb.hi.Cmp(sa.lo) <= 0 {
			spans = []span{sa}
		} else {
			if sa.lo.Cmp(sb.lo) < 0 {
				spans = append(spans, span{sa.lo, sb.lo})
			}
			if sa.hi.Cmp(sb.hi) > 0 {
				spans = append(spans, span{sb.hi, sa.hi})
			}
		}
	}
	h = SplineShellCombineHistory{Method: "axial-wrapped-cylinder",
		FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges))}
	if len(spans) == 0 {
		return Solid{}, h, nil
	}
	parts := make([]PlacedSolid, 0, len(spans))
	for _, r := range spans {
		s, e := geom.ReheightWrappedCylinder(ctx, wa, r.lo, r.hi, o)
		if e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		solid, _, e := NewWrappedCylinder(ctx, s, max(a.tolerance, b.tolerance), o)
		if e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		parts = append(parts, PlacedSolid{solid, geom.Identity()})
	}
	out := parts[0].Solid
	if len(parts) > 1 {
		out, e = AssembleShells(ctx, parts, 3*len(parts))
		if e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
	}
	// Exact ancestry is by positive-area/positive-length overlap on the
	// common lateral surface, cap plane or generator. Cap polarity matters
	// only to material selection, so subtraction can map a reversed B cap.
	for operand, input := range [2]Solid{a, b} {
		is := [2]span{sa, sb}[operand]
		ws := [2]*geom.WrappedSurfacePatch{wa, wb}[operand]
		faces, edges := [2][][]int{h.FacesA, h.FacesB}[operand], [2][][]int{h.EdgesA, h.EdgesB}[operand]
		for k, r := range spans {
			for i, f := range input.faces {
				if f.Surface == ws {
					if maximum(is.lo, r.lo).Cmp(minimum(is.hi, r.hi)) < 0 {
						faces[i] = append(faces[i], 3*k)
					}
					continue
				}
				z := ratInset(f.Surface.(geom.PlaneSurface).Frame().Origin().Z)
				if z.Cmp(r.lo) == 0 {
					faces[i] = append(faces[i], 3*k+1)
				}
				if z.Cmp(r.hi) == 0 {
					faces[i] = append(faces[i], 3*k+2)
				}
			}
			for i, edge := range input.edges {
				p, q := input.vertices[edge.Start], input.vertices[edge.End]
				if p.Z != q.Z {
					if maximum(is.lo, r.lo).Cmp(minimum(is.hi, r.hi)) < 0 {
						edges[i] = append(edges[i], 9*k+8)
					}
					continue
				}
				z := ratInset(p.Z)
				for side, end := range [2]*big.Rat{r.lo, r.hi} {
					if z.Cmp(end) != 0 {
						continue
					}
					for j := range 4 {
						outEdge := out.edges[9*k+4*side+j]
						x, y := out.vertices[outEdge.Start], out.vertices[outEdge.End]
						if x == p && y == q || x == q && y == p {
							edges[i] = append(edges[i], 9*k+4*side+j)
						}
					}
				}
			}
		}
	}
	return out, h, nil
}

func combineConcentricWrappedCylinders(ctx context.Context, a, b Solid, wa, wb *geom.WrappedSurfacePatch,
	pa, pb geom.WrappedCylinderEmbedding, op parametric.BooleanOp, o geom.SurfacePatchOptions) (Solid, SplineShellCombineHistory, error) {
	var h SplineShellCombineHistory
	same, e := sameNormalizedCylinderProfile(ctx, wa, wb, pa, pb, o)
	if e != nil || !same {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, h, e
	}
	h = SplineShellCombineHistory{Method: "concentric-wrapped-cylinder",
		FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges))}
	biggerA := pa.Radius.Cmp(pb.Radius) > 0
	if op == parametric.Difference {
		if !biggerA {
			return Solid{}, h, nil
		}
		return subtractContainedWrappedCylinder(ctx, a, b, wa, wb, o)
	}
	keepA := (op == parametric.Union) == biggerA
	faces, edges := h.FacesB, h.EdgesB
	if keepA {
		faces, edges = h.FacesA, h.EdgesA
	}
	for i := range faces {
		faces[i] = []int{i}
	}
	for i := range edges {
		edges[i] = []int{i}
	}
	if keepA {
		return a, h, nil
	}
	return b, h, nil
}

func subtractContainedWrappedCylinder(ctx context.Context, a, b Solid, wa, wb *geom.WrappedSurfacePatch,
	o geom.SurfacePatchOptions) (Solid, SplineShellCombineHistory, error) {
	var h SplineShellCombineHistory
	s, mapped, e := NewWrappedCylinderBore(ctx, wa, wb, max(a.tolerance, b.tolerance), o)
	if e != nil {
		return Solid{}, h, e
	}
	h = SplineShellCombineHistory{Method: "concentric-wrapped-cylinder",
		FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges))}
	for oi, input := range [2]Solid{a, b} {
		w := [2]*geom.WrappedSurfacePatch{wa, wb}[oi]
		row := [2][3]int{mapped.OuterFaces, mapped.InnerFaces}[oi]
		faces, edges := [2][][]int{h.FacesA, h.FacesB}[oi], [2][][]int{h.EdgesA, h.EdgesB}[oi]
		for i, f := range input.faces {
			switch {
			case f.Surface == w:
				faces[i] = []int{row[0]}
			case f.Reversed:
				faces[i] = []int{row[1]}
			default:
				faces[i] = []int{row[2]}
			}
		}
		for i, edge := range input.edges {
			x, y := input.vertices[edge.Start], input.vertices[edge.End]
			for j := 9 * oi; j < 9*(oi+1); j++ {
				out := s.edges[j]
				p, q := s.vertices[out.Start], s.vertices[out.End]
				if x == p && y == q || x == q && y == p {
					edges[i] = []int{j}
					break
				}
			}
		}
	}
	return s, h, nil
}
