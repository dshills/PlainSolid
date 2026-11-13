package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineShellTransverseOptions controls an independently certified closed
// intersection between two particular spline faces. An empty TubeCells
// report requests automatic tube discovery. Otherwise only the supplied
// tube boxes and free axes are used as hints and the proofs are recomputed.
type SplineShellTransverseOptions struct {
	Shells    SplineShellBooleanOptions
	Trace     intersect.SplinePairTraceOptions
	TubeCells intersect.SplineSeedReport
	Sew       SewOptions
}

// SplineShellTransverseHistory reports each retained input face fragment
// and the shared intersection edges in canonical tube order.
type SplineShellTransverseHistory struct {
	FacesA, FacesB [][]int
	SectionEdges   []int
	Trace          intersect.SplinePairTrace
}

// CombineTransverseSplineShells performs a Boolean when two embedded spline
// shells cross on closed transverse sections strictly inside the specified
// face of each input. The complete section set is independently certified. Every
// other face pair must be disjoint. Split face fragments are classified with
// certified rays, then exact original supports and paired trims are sewn.
// Open sections, mapped selected supports and contacts on unrelated faces
// remain outside this operation's domain. Multiple contours require exact
// winding classification of their arrangement in each chart.
func CombineTransverseSplineShells(ctx context.Context, a, b Solid, faceA, faceB int, op parametric.BooleanOp, o SplineShellTransverseOptions) (Solid, SplineShellTransverseHistory, error) {
	var h SplineShellTransverseHistory
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	if ctx == nil || faceA < 0 || faceA >= len(a.faces) || faceB < 0 || faceB >= len(b.faces) ||
		op > parametric.Difference || o.Shells.MaxRays < 1 || o.Shells.MaxRays > 12 {
		return Solid{}, h, geom.ErrInvalid
	}
	var ap, bp []splineEmbeddingFace
	if _, e := a.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &ap); e != nil {
		return Solid{}, h, e
	}
	if _, e := b.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &bp); e != nil {
		return Solid{}, h, e
	}
	for i, x := range ap {
		for j, y := range bp {
			if e := ctx.Err(); e != nil {
				return Solid{}, h, e
			}
			if i == faceA && j == faceB {
				continue
			}
			if e := proveSplinePatchPairDisjoint(ctx, x.patch, y.patch, o.Shells.Separation); e != nil {
				return Solid{}, h, fmt.Errorf("trimmed: extra possible spline contact on A:%d and B:%d: %w", i, j, e)
			}
		}
	}
	x, e := intersectSplineFace(a.faces[faceA], ap[faceA].patch.Domain())
	if e != nil {
		return Solid{}, h, e
	}
	y, e := intersectSplineFace(b.faces[faceB], bp[faceB].patch.Domain())
	if e != nil {
		return Solid{}, h, e
	}
	if len(o.TubeCells.Attempts) == 0 {
		h.Trace, e = intersect.TraceSplinePair(ctx, x, y, o.Trace)
	} else {
		h.Trace, e = intersect.CertifySplinePairBranches(ctx, x, y, o.TubeCells, o.Trace)
	}
	if e != nil {
		return Solid{}, h, e
	}
	if !h.Trace.Complete || len(h.Trace.Curves) == 0 || len(h.Trace.Curves) > 16 || len(h.Trace.Curves) != len(h.Trace.BranchGraph.Branches) {
		return Solid{}, h, fmt.Errorf("trimmed: expected complete closed transverse sections: %w", geom.ErrConvergence)
	}
	for _, branch := range h.Trace.BranchGraph.Branches {
		if !branch.Closed {
			return Solid{}, h, fmt.Errorf("trimmed: open transverse section: %w", geom.ErrConvergence)
		}
	}
	return assembleTransverseContours(ctx, a, b, ap, bp, faceA, faceB, op, h, o)
}

func sewTransverseFragments(ctx context.Context, parts []SewFace, section map[[2]int]int, count int, tol float64,
	h SplineShellTransverseHistory, o SewOptions) (Solid, SplineShellTransverseHistory, error) {
	s, _, ids, e := sewSplineSectionFragments(ctx, parts, section, count, tol, o)
	h.SectionEdges = ids
	return s, h, e
}

func sewSplineSectionFragments(ctx context.Context, parts []SewFace, section map[[2]int]int, count int, tol float64,
	o SewOptions) (Solid, SewHistory, []int, error) {
	if o.Tolerance == 0 {
		o.Tolerance = tol
	}
	s, sewing, e := Sew(ctx, parts, o)
	if e != nil {
		return Solid{}, SewHistory{}, nil, e
	}
	sectionEdges := make([]int, count)
	counts := make([]int, count)
	for p, i := range section {
		id := sewing.Edges[p[0]][p[1]]
		if counts[i] != 0 && sectionEdges[i] != id {
			return Solid{}, SewHistory{}, nil, geom.ErrConvergence
		}
		counts[i]++
		sectionEdges[i] = id
	}
	for _, n := range counts {
		if n != 2 {
			return Solid{}, SewHistory{}, nil, geom.ErrConvergence
		}
	}
	return s, sewing, sectionEdges, nil
}
