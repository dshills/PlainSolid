package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ExactQuadraticModelLineHit carries one original trim event. Adjacent cells
// can own the same event at an exact support knot. Distinct trim parameters
// remain distinct even if their model images coincide.
type ExactQuadraticModelLineHit struct {
	geom.ComposedCurveLineHit
	Cells []int
}

// ExactQuadraticModelLineSection is exhaustive on success. Hits and
// coincident portions are in original normalized trim-parameter order.
type ExactQuadraticModelLineSection struct {
	Hits       []ExactQuadraticModelLineHit
	Coincident []ExactQuadraticAlgebraicPlaneSpan
}

// IntersectModelFiniteLine proves the contact set of the exact composed
// trim and origin+t*direction over the closed rational limits. The direction
// is used as represented, without normalization. Knot roots are compared
// algebraically; an overlapping display bracket does not prove equality.
func (q ExactQuadraticUV) IntersectModelFiniteLine(ctx context.Context, p *geom.SurfacePatch, origin, direction geom.Vec3,
	limits [2]*big.Rat, compose geom.SurfaceCurveAgreementOptions, o geom.ComposedCurveLineOptions) (ExactQuadraticModelLineSection, error) {
	var empty ExactQuadraticModelLineSection
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	c, e := q.ModelCurveCells(ctx, p, compose)
	if e != nil {
		return empty, e
	}
	return intersectModelCurveCellsFiniteLine(ctx, c, origin, direction, limits, o)
}

func intersectModelCurveCellsFiniteLine(ctx context.Context, c geom.SurfaceCurveCompositionCells, origin, direction geom.Vec3,
	limits [2]*big.Rat, o geom.ComposedCurveLineOptions) (ExactQuadraticModelLineSection, error) {
	var empty ExactQuadraticModelLineSection
	if !c.Complete || len(c.Cells) == 0 {
		return empty, geom.ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	compare := func(a, b *geom.ExactComposedPlaneEvent) (int, error) {
		x, y := a.Interval(), b.Interval()
		if x.Upper.Cmp(y.Lower) < 0 {
			return -1, nil
		}
		if y.Upper.Cmp(x.Lower) < 0 {
			return 1, nil
		}
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return 0, geom.ErrConvergence
		}
		r, e := a.CompareParameter(ctx, b, o.ExactCurvePlaneEventOptions)
		o.MaxOperations -= r.Operations
		o.MaxSubdivisions -= r.Subdivisions
		if e == nil && !r.Proven {
			e = geom.ErrConvergence
		}
		return r.Order, e
	}
	var out ExactQuadraticModelLineSection
	for i, cell := range c.Cells {
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		r, e := cell.IntersectFiniteLine(ctx, origin, direction, limits, o)
		if e != nil {
			return empty, e
		}
		if !r.Complete {
			return empty, geom.ErrConvergence
		}
		o.MaxOperations -= r.Operations
		o.MaxSubdivisions -= r.Subdivisions
		for _, part := range r.Portions {
			if n := len(out.Coincident); n > 0 {
				last := &out.Coincident[n-1]
				order, err := compare(last.End, part.Start)
				if err != nil {
					return empty, err
				}
				if order > 0 {
					return empty, geom.ErrConvergence
				}
				if order == 0 {
					last.End = part.End
					if last.Cells[len(last.Cells)-1] != i {
						last.Cells = append(last.Cells, i)
					}
					continue
				}
			}
			out.Coincident = append(out.Coincident, ExactQuadraticAlgebraicPlaneSpan{part.Start, part.End, []int{i}})
		}
		for _, hit := range r.Hits {
			if n := len(out.Hits); n > 0 {
				last := &out.Hits[n-1]
				order, err := compare(last.Event, hit.Event)
				if err != nil {
					return empty, err
				}
				if order > 0 {
					return empty, geom.ErrConvergence
				}
				if order == 0 {
					lo := maxRat(last.LineParameter.Lower, hit.LineParameter.Lower)
					hi := minRat(last.LineParameter.Upper, hit.LineParameter.Upper)
					if lo.Cmp(hi) > 0 || last.BoundaryMask != hit.BoundaryMask {
						return empty, geom.ErrConvergence
					}
					last.LineParameter = geom.ExactParameterInterval{Lower: lo, Upper: hi}
					last.Cells = append(last.Cells, i)
					continue
				}
			}
			out.Hits = append(out.Hits, ExactQuadraticModelLineHit{hit, []int{i}})
		}
	}
	keep, span := out.Hits[:0], 0
	for _, hit := range out.Hits {
		for span < len(out.Coincident) {
			order, err := compare(hit.Event, out.Coincident[span].End)
			if err != nil {
				return empty, err
			}
			if order <= 0 {
				break
			}
			span++
		}
		if span < len(out.Coincident) {
			order, err := compare(hit.Event, out.Coincident[span].Start)
			if err != nil {
				return empty, err
			}
			if order >= 0 {
				continue
			}
		}
		keep = append(keep, hit)
	}
	out.Hits = keep
	return out, nil
}
