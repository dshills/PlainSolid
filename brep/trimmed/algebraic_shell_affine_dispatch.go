package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffineSplineShellOptions keeps whole-pair authorization and subsequent
// exact material construction independently bounded. No caller imprint is used.
type ConvexAffineSplineShellOptions struct {
	Authorization AffineShellPairedImprintOptions
	Material      AffineShellMaterialOptions
}

// CombineConvexAffineSplineShells authenticates both complete original shells,
// every face-pair section and paired edge occurrence, then constructs the exact
// convex material result. This bounded lane requires supported paired source-edge
// contacts; it is not a general disjoint/coplanar convex dispatcher. Unsupported
// or singular contacts fail without fallback.
func CombineConvexAffineSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o ConvexAffineSplineShellOptions) (*AffineShellMaterial, error) {
	var operation intersect.AffineRingOperation
	switch op {
	case parametric.Intersection:
		operation = intersect.AffineRingIntersection
	case parametric.Union:
		operation = intersect.AffineRingUnion
	case parametric.Difference:
		operation = intersect.AffineRingDifference
	default:
		return nil, geom.ErrInvalid
	}
	imprint, e := AuthorizeAffineSplineShellPairedImprint(ctx, a, b, o.Authorization)
	if e != nil {
		return nil, e
	}
	return imprint.CombineConvex(ctx, operation, o.Material)
}
func combineSplineShellsAffinePaired(ctx context.Context, a, b Solid, op parametric.BooleanOp, o ConvexAffineSplineShellOptions) (Solid, SplineShellCombineHistory, error) {
	r, e := CombineConvexAffineSplineShells(ctx, a, b, op, o)
	if e != nil {
		return Solid{}, SplineShellCombineHistory{}, e
	}
	h := SplineShellCombineHistory{Method: "exact-convex-affine-paired", FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)), EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges)), AffinePaired: r}
	appendUnique := func(ids []int, id int) []int {
		for _, old := range ids {
			if old == id {
				return ids
			}
		}
		return append(ids, id)
	}
	for fi, f := range r.FaceSources() {
		if e := ctx.Err(); e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		if f.Owner == 0 {
			h.FacesA[f.Face] = append(h.FacesA[f.Face], fi)
		} else {
			h.FacesB[f.Face] = append(h.FacesB[f.Face], fi)
		}
		proof := r.SourceFaceFragments(f.Owner, f.Face)
		for j, use := range proof.cycles[f.Fragment].Uses {
			edge := r.solid.faces[fi].Outer[j].Edge
			if use.OriginalEdge >= 0 {
				if f.Owner == 0 {
					h.EdgesA[use.OriginalEdge] = appendUnique(h.EdgesA[use.OriginalEdge], edge)
				} else {
					h.EdgesB[use.OriginalEdge] = appendUnique(h.EdgesB[use.OriginalEdge], edge)
				}
			}
			if use.SectionEdge >= 0 {
				h.SectionEdges = appendUnique(h.SectionEdges, edge)
			}
			if use.AuxiliaryEdge >= 0 {
				h.GeneratedEdges = appendUnique(h.GeneratedEdges, edge)
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, SplineShellCombineHistory{}, e
	}
	return r.solid, h, nil
}
