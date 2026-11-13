package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SplineGraphEllipseSection retains the exact UV conic and its original
// model-space compositions on both supports. Each arc's knot cells use
// the original normalized parameter and may have algebraic endpoints.
type SplineGraphEllipseSection struct {
	UV           geom.SplineGraphQuadraticEllipseArcs
	Model        [2][4]geom.SurfaceCurveCompositionCells
	displayCells [][]*geom.ExactComposedCurveCell
}

// SectionRationalEllipseSplineGraphs returns an exact closed curve in both
// model charts. It does not require representable spline controls or UV
// vertices because it builds no native topology. Equality is re-proved
// between both model compositions on each full arc.
func SectionRationalEllipseSplineGraphs(ctx context.Context, a, b *geom.SurfacePatch,
	hint geom.GraphQuadraticEllipseHint, point [2]*big.Rat, proof geom.GraphPointContactOptions,
	o geom.SurfaceCurveAgreementOptions) (SplineGraphEllipseSection, error) {
	var empty SplineGraphEllipseSection
	r, e := geom.BuildSplineGraphQuadraticEllipseArcs(ctx, a, b, hint, point, proof)
	if e != nil {
		return empty, e
	}
	if !r.Complete {
		return empty, geom.ErrConvergence
	}
	out := SplineGraphEllipseSection{UV: r}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	for j, arc := range r.Arcs {
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		x, e := geom.CompareSurfaceCurveCompositions(ctx, a, arc, b, arc, o)
		if e != nil {
			return empty, e
		}
		if !x.Complete || !x.Equal {
			return empty, geom.ErrConvergence
		}
		o.MaxOperations -= x.ExactOperations
		o.MaxSubdivisions -= x.Subdivisions
		for i, p := range [2]*geom.SurfacePatch{a, b} {
			if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
				return empty, geom.ErrConvergence
			}
			q, e := p.BuildSurfaceCurveCompositionCells(ctx, arc, o)
			if e != nil {
				return empty, e
			}
			if !q.Complete {
				return empty, geom.ErrConvergence
			}
			o.MaxOperations -= q.ExactOperations
			o.MaxSubdivisions -= q.Subdivisions
			out.Model[i][j] = q
			if i == 0 {
				out.displayCells = append(out.displayCells, append([]*geom.ExactComposedCurveCell(nil), q.Cells...))
			}
		}
	}
	return out, nil
}
