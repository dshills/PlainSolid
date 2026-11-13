package trimmed

import (
	"context"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// SplineSeamGraphHit retains the algebraic event in the exact curve
// descriptor's original parameter, and its outward enclosure mapped back to
// the authoritative model edge. Cells identify the target spline knot spans.
type SplineSeamGraphHit struct {
	Event         *geom.ExactComposedPlaneEvent
	EdgeParameter geom.ExactParameterInterval
	Model         geom.AlgebraicCurveEndpointBounds
	Cells         []int
	SourceCells   []int // original model composition cells; -1 for direct spline
	// Signs are before/after in original model-edge direction. Zero means
	// that side is unavailable; nonzero is the oriented face side.
	Signs [2]int
}

// SplineSeamGraphContacts records all scalar support events. Coincidence is
// retained as unresolved unless the entire contact is known to be isolated.
type SplineSeamGraphContacts struct {
	Hits, Excluded, Unresolved []SplineSeamGraphHit
	Support                    geom.IdentityGraphCurveIntersection
	SourceCellBySupport        []int
	Complete                   bool
	ExactOperations            int
}

func exactSplineSeamGraphContacts(ctx context.Context, c *geom.ExactSplineCurve, p *geom.SurfacePatch, f Face,
	m splineEdgeParameterMap, o SplineSeamFaceOptions) (out SplineSeamGraphContacts, err error) {
	return classifyExactSplineSeamGraphContacts(ctx, f, m, o, func(o geom.SurfaceCurveAgreementOptions) (geom.IdentityGraphCurveIntersection, []int, error) {
		r, e := geom.IntersectExactCurveIdentityGraph(ctx, c, p, o)
		ids := make([]int, len(r.Cells))
		for i := range ids {
			ids[i] = -1
		}
		return r, ids, e
	})
}

func exactComposedSplineSeamGraphContacts(ctx context.Context, cells []*geom.ExactComposedCurveCell, p *geom.SurfacePatch, f Face,
	o SplineSeamFaceOptions) (SplineSeamGraphContacts, error) {
	return classifyExactSplineSeamGraphContacts(ctx, f, splineEdgeParameterMap{0, 1, 0, 1, false}, o,
		func(o geom.SurfaceCurveAgreementOptions) (geom.IdentityGraphCurveIntersection, []int, error) {
			var out geom.IdentityGraphCurveIntersection
			var ids []int
			for i, c := range cells {
				child := o
				child.MaxOperations -= out.ExactOperations
				child.MaxSubdivisions -= out.Subdivisions
				if child.MaxOperations < 1 || child.MaxSubdivisions < 1 {
					return out, ids, geom.ErrConvergence
				}
				r, e := geom.IntersectExactComposedCurveIdentityGraph(ctx, c, p, child)
				out.ExactOperations += r.ExactOperations
				out.Subdivisions += r.Subdivisions
				if e != nil {
					return out, ids, e
				}
				if !r.Complete {
					return out, ids, geom.ErrConvergence
				}
				out.Cells = append(out.Cells, r.Cells...)
				for range r.Cells {
					ids = append(ids, i)
				}
			}
			out.Complete = len(cells) > 0
			return out, ids, nil
		})
}

func classifyExactSplineSeamGraphContacts(ctx context.Context, f Face, m splineEdgeParameterMap, o SplineSeamFaceOptions,
	intersect func(geom.SurfaceCurveAgreementOptions) (geom.IdentityGraphCurveIntersection, []int, error)) (out SplineSeamGraphContacts, err error) {
	if o.Composition.MaxOperations == 0 {
		o.Composition.MaxOperations = 1000000
	}
	if o.Composition.MaxSubdivisions == 0 {
		o.Composition.MaxSubdivisions = 8192
	}
	r, ids, e := intersect(o.Composition)
	out.Support, out.ExactOperations, out.SourceCellBySupport = r, r.ExactOperations, ids
	if e != nil {
		return out, e
	}
	if !r.Complete || len(ids) != len(r.Cells) {
		return out, geom.ErrConvergence
	}
	left := o.Composition.MaxSubdivisions - r.Subdivisions
	limits := func() (geom.ExactCurvePlaneEventOptions, error) {
		work := o.Composition.MaxOperations - out.ExactOperations
		if work < 1 || left < 1 {
			return geom.ExactCurvePlaneEventOptions{}, geom.ErrConvergence
		}
		return geom.ExactCurvePlaneEventOptions{MaxOperations: work, MaxSubdivisions: left, MaxCoefficientBits: o.Composition.MaxCoefficientBits}, nil
	}
	var hits []SplineSeamGraphHit
	coincident := false
	for i, cell := range r.Cells {
		if cell.Intersection.Coincident {
			coincident = true
			continue
		}
		for j, event := range cell.Intersection.Events {
			if j >= len(cell.EventSigns) {
				return out, geom.ErrConvergence
			}
			signs := cell.EventSigns[j]
			if f.Reversed {
				signs[0], signs[1] = -signs[0], -signs[1]
			}
			if n := len(hits); n > 0 {
				last := &hits[n-1]
				lim, e := limits()
				if e != nil {
					return out, e
				}
				order, e := last.Event.CompareParameter(ctx, event, lim)
				out.ExactOperations += order.Operations
				left -= order.Subdivisions
				if e != nil {
					return out, e
				}
				if !order.Proven || order.Order > 0 {
					return out, geom.ErrConvergence
				}
				if order.Order == 0 {
					last.Cells = append(last.Cells, i)
					if last.SourceCells[len(last.SourceCells)-1] != ids[i] {
						last.SourceCells = append(last.SourceCells, ids[i])
					}
					for k, s := range signs {
						if s == 0 {
							continue
						}
						if last.Signs[k] != 0 && last.Signs[k] != s {
							return out, geom.ErrConvergence
						}
						last.Signs[k] = s
					}
					continue
				}
			}
			lim, e := limits()
			if e != nil {
				return out, e
			}
			b, e := event.Enclose(ctx, geom.ComposedCurveEndpointOptions{Tolerance: 1e-10, ExactCurvePlaneEventOptions: lim})
			out.ExactOperations += b.ExactOperations
			left -= b.Subdivisions
			if e != nil {
				return out, e
			}
			if !b.Converged {
				return out, geom.ErrConvergence
			}
			a, z := m.bounds(b.Parameter.Lower, b.Parameter.Upper)
			hits = append(hits, SplineSeamGraphHit{Event: event, EdgeParameter: geom.ExactParameterInterval{Lower: a, Upper: z}, Model: b, Cells: []int{i}, SourceCells: []int{ids[i]}, Signs: signs})
		}
	}
	if m.reverse {
		for i := range hits {
			hits[i].Signs[0], hits[i].Signs[1] = hits[i].Signs[1], hits[i].Signs[0]
		}
		for i, j := 0, len(hits)-1; i < j; i, j = i+1, j-1 {
			hits[i], hits[j] = hits[j], hits[i]
		}
	}
	for _, h := range hits {
		box := geom.UVBox{Min: geom.Vec2{X: h.Model.Bounds.Min.X, Y: h.Model.Bounds.Min.Y}, Max: geom.Vec2{X: h.Model.Bounds.Max.X, Y: h.Model.Bounds.Max.Y}}
		loc, proved, e := classifySplineFaceRootBox(ctx, f, box, o.Trim, o.MaxTrimBoxes)
		if e != nil {
			return out, e
		}
		if !proved || loc == trimvalidate.Boundary {
			out.Unresolved = append(out.Unresolved, h)
		} else if loc == trimvalidate.Outside {
			out.Excluded = append(out.Excluded, h)
		} else {
			out.Hits = append(out.Hits, h)
		}
	}
	out.Complete = !coincident && len(out.Unresolved) == 0
	return out, nil
}
