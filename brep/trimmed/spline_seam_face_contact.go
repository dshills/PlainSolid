//go:build research

package trimmed

import (
	"context"
	"errors"
	"sort"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type SplineSeamFaceOptions struct {
	Composition  geom.SurfaceCurveAgreementOptions
	Intersection geom.CurveSurfaceIntersectionOptions
	Trim         trimvalidate.Options
	MaxTrimBoxes int
	// EnableIdentityGraph tries exact scalar reduction on a single Bézier
	// model edge when its whole XY projection lies in the target graph chart.
	EnableIdentityGraph bool
}

type SplineSeamFaceHit struct {
	geom.CurveSurfaceHit
	Cell int // original model composition cell, or -1 for a spline model edge
}

type SplineSeamFaceReport struct {
	SourceEdge, TargetFace     int
	SourceFaces                [2]int
	Hits, Excluded, Unresolved []SplineSeamFaceHit
	Cells                      []geom.CurveSurfaceIntersection
	Graph                      *SplineSeamGraphContacts
	ExactOperations            int // support isolation, including a graph attempt
	Complete                   bool
}

// IntersectSplineSeamFace isolates regular contacts of a certified shared
// spline edge with an independent trimmed spline face. It retains source
// owners, original model-edge parameter units and target UV boxes.
// Contacts near a target trim or the seam's algebraic cell boundaries remain
// unresolved unless their whole enclosures can be classified. It does not
// infer branch continuation or classify singular contacts.
func IntersectSplineSeamFace(ctx context.Context, a Solid, edge int, b Solid, face int,
	o SplineSeamFaceOptions) (SplineSeamFaceReport, error) {
	out := SplineSeamFaceReport{SourceEdge: edge, TargetFace: face, SourceFaces: [2]int{-1, -1}}
	if ctx == nil || edge < 0 || edge >= len(a.edges) || face < 0 || face >= len(b.faces) || o.MaxTrimBoxes < 0 || o.MaxTrimBoxes > 65536 {
		return out, geom.ErrInvalid
	}
	if o.MaxTrimBoxes == 0 {
		o.MaxTrimBoxes = 256
	}
	if o.Intersection.MaxCells == 0 {
		o.Intersection.MaxCells = 8192
	}
	if o.Intersection.MaxOperations == 0 {
		o.Intersection.MaxOperations = 8000000
	}
	if o.Intersection.MaxCells < 1 || o.Intersection.MaxOperations < 1 {
		return out, geom.ErrInvalid
	}
	f := b.faces[face]
	p, e := exactFaceSupportPatch(f, o.Composition.SurfacePatchOptions)
	if e != nil {
		return out, e
	}
	complete := true
	graphAttempt := func(run func(SplineSeamFaceOptions) (SplineSeamGraphContacts, error)) (bool, error) {
		options := o
		if options.Composition.MaxOperations == 0 {
			options.Composition.MaxOperations = 1000000
		}
		options.Composition.MaxOperations = min(options.Composition.MaxOperations, o.Intersection.MaxOperations)
		g, e := run(options)
		out.ExactOperations += g.ExactOperations
		o.Intersection.MaxOperations -= g.ExactOperations
		if e == nil {
			out.Graph = &g
			out.Complete = g.Complete
			if !g.Complete {
				return true, geom.ErrConvergence
			}
			return true, nil
		}
		if !isUnresolvedOpenSpline(e) && !errors.Is(e, geom.ErrDomain) {
			return true, e
		}
		if o.Intersection.MaxOperations < 1 {
			return true, geom.ErrConvergence
		}
		return false, nil
	}
	record := func(i int, r geom.CurveSurfaceIntersection, e error, m *splineEdgeParameterMap) error {
		out.Cells = append(out.Cells, r)
		out.ExactOperations += r.ExactOperations
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return e
		}
		complete = complete && r.Complete
		o.Intersection.MaxCells -= r.VisitedCells
		o.Intersection.MaxOperations -= r.ExactOperations
		for _, h := range r.Hits {
			if m != nil {
				lo, hi := m.bounds(ratInset(h.CurveParameter.Min), ratInset(h.CurveParameter.Max))
				l, err := geom.RationalOutwardRange(lo)
				if err != nil {
					return err
				}
				u, err := geom.RationalOutwardRange(hi)
				if err != nil {
					return err
				}
				h.CurveParameter = geom.CertifiedScalarRange{Min: l.Min, Max: u.Max}
				lo, hi = m.bounds(ratInset(h.CurveCell.Start), ratInset(h.CurveCell.End))
				l, err = geom.RationalOutwardRange(lo)
				if err != nil {
					return err
				}
				u, err = geom.RationalOutwardRange(hi)
				if err != nil {
					return err
				}
				h.CurveCell = geom.ParameterSpan{Start: l.Min, End: u.Max}
				if m.reverse {
					h.Direction = -h.Direction
				}
			}
			if f.Reversed {
				h.Direction = -h.Direction
			}
			loc, proved, e := classifySplineFaceRootBox(ctx, f, h.UV, o.Trim, o.MaxTrimBoxes)
			if e != nil {
				return e
			}
			x := SplineSeamFaceHit{h, i}
			if !proved || loc == trimvalidate.Boundary {
				out.Unresolved = append(out.Unresolved, x)
			} else if loc == trimvalidate.Outside {
				out.Excluded = append(out.Excluded, x)
			} else {
				out.Hits = append(out.Hits, x)
			}
		}
		return nil
	}
	if m, ok := a.edges[edge].Curve.(surfaceTrimCurve); ok && m.curve != nil {
		model, owners, e := algebraicSeamSource(ctx, a, edge, o.Composition)
		if e != nil {
			return out, e
		}
		out.SourceFaces = [2]int{owners[0].face, owners[1].face}
		q, e := model.curve.ModelCurveCells(ctx, model.patch, o.Composition)
		if e != nil {
			return out, e
		}
		if !q.Complete || len(q.Cells) == 0 {
			return out, geom.ErrConvergence
		}
		if o.EnableIdentityGraph {
			if stop, e := graphAttempt(func(options SplineSeamFaceOptions) (SplineSeamGraphContacts, error) {
				return exactComposedSplineSeamGraphContacts(ctx, q.Cells, p, f, options)
			}); stop {
				return out, e
			}
		}
		for i, c := range q.Cells {
			if o.Intersection.MaxCells < 1 || o.Intersection.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			r, e := c.IntersectSplineSurface(ctx, p, o.Intersection)
			if err := record(i, r, e, nil); err != nil {
				return out, err
			}
		}
	} else {
		m := a.edges[edge]
		c, rev, e := exactEdgeSplineWithOptions(m, o.Composition.SurfacePatchOptions)
		if e != nil {
			return out, e
		}
		if c == nil {
			return out, geom.ErrConvergence
		}
		n := 0
		limits := edgecheck.Options{MaxWork: o.Composition.MaxOperations, MaxBits: o.Composition.MaxCoefficientBits, MaxIntervals: o.Composition.MaxEvents}
		for i, f := range a.faces {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			for _, ring := range append([][]Use{f.Outer}, f.Holes...) {
				for _, u := range ring {
					if u.Edge != edge {
						continue
					}
					if n >= 2 || n == 1 && out.SourceFaces[0] == i {
						return out, geom.ErrConvergence
					}
					if !sameSplineTrim(m, f.Surface, u) {
						ok, e := compareSplineEdgeUse(ctx, m, f.Surface, u, limits)
						if e != nil {
							return out, e
						}
						if !ok {
							return out, geom.ErrConvergence
						}
					}
					out.SourceFaces[n] = i
					n++
				}
			}
		}
		if n != 2 {
			return out, geom.ErrConvergence
		}
		lo, hi := c.Domain()
		mapToModel := splineEdgeParameterMap{lo, hi, m.Range.Min, m.Range.Max, rev}
		if o.EnableIdentityGraph && lo == 0 && hi == 1 {
			if stop, e := graphAttempt(func(options SplineSeamFaceOptions) (SplineSeamGraphContacts, error) {
				return exactSplineSeamGraphContacts(ctx, c, p, f, mapToModel, options)
			}); stop {
				return out, e
			}
		}
		r, e := geom.IntersectExactSplineCurveSurface(ctx, c, p, o.Intersection)
		if err := record(-1, r, e, &mapToModel); err != nil {
			return out, err
		}
	}
	for _, hits := range [][]SplineSeamFaceHit{out.Hits, out.Excluded, out.Unresolved} {
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].CurveParameter.Min < hits[j].CurveParameter.Min })
	}
	out.Complete = complete && len(out.Unresolved) == 0
	if !out.Complete {
		return out, geom.ErrConvergence
	}
	return out, nil
}
