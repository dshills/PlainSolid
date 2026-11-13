package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ExactQuadraticAlgebraicPlaneContact identifies one distinct isolated
// plane event, possibly on a shared algebraic knot of two cells.
type ExactQuadraticAlgebraicPlaneContact struct {
	Event        *geom.ExactComposedPlaneEvent
	Cells        []int
	Multiplicity int // maximum local order, not a global crossing claim
}

// ExactQuadraticAlgebraicPlaneSpan retains both exact endpoint identities
// of a continuous coplanar span and its original knot cell ancestry.
type ExactQuadraticAlgebraicPlaneSpan struct {
	Start, End *geom.ExactComposedPlaneEvent
	Cells      []int
}

type ExactQuadraticAlgebraicPlaneSection struct {
	Hits       []ExactQuadraticAlgebraicPlaneContact
	Coincident []ExactQuadraticAlgebraicPlaneSpan
}

// IntersectModelRawPlane proves the exact model plane contact set of an
// exact quadratic UV trim. It retains algebraic knot parameters, merges
// repeated reports of the same isolated event, and joins coplanar cells.
func (q ExactQuadraticUV) IntersectModelRawPlane(ctx context.Context, p *geom.SurfacePatch, plane [4]*big.Rat,
	compose geom.SurfaceCurveAgreementOptions, o geom.ExactCurvePlaneEventOptions) (ExactQuadraticAlgebraicPlaneSection, error) {
	var empty ExactQuadraticAlgebraicPlaneSection
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	c, e := q.ModelCurveCells(ctx, p, compose)
	if e != nil {
		return empty, e
	}
	return intersectModelCurveCellsRawPlane(ctx, c, plane, o)
}

func intersectModelCurveCellsRawPlane(ctx context.Context, c geom.SurfaceCurveCompositionCells, plane [4]*big.Rat, o geom.ExactCurvePlaneEventOptions) (ExactQuadraticAlgebraicPlaneSection, error) {
	var empty ExactQuadraticAlgebraicPlaneSection
	if !c.Complete || len(c.Cells) == 0 {
		return empty, geom.ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	spend := func(work, subdivisions int) {
		o.MaxOperations -= work
		o.MaxSubdivisions -= subdivisions
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
		r, e := a.CompareParameter(ctx, b, o)
		spend(r.Operations, r.Subdivisions)
		if e != nil {
			return 0, e
		}
		if !r.Proven {
			return 0, geom.ErrConvergence
		}
		return r.Order, nil
	}
	var result ExactQuadraticAlgebraicPlaneSection
	for i, cell := range c.Cells {
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		r, e := cell.IntersectRawPlane(ctx, plane, o)
		if e != nil {
			return empty, e
		}
		spend(r.Operations, r.Subdivisions)
		if !r.Complete {
			return empty, geom.ErrConvergence
		}
		if r.Coincident {
			if n := len(result.Coincident); n > 0 && result.Coincident[n-1].Cells[len(result.Coincident[n-1].Cells)-1]+1 == i {
				old := &result.Coincident[n-1]
				ord, e := compare(old.End, r.Boundary[0])
				if e != nil || ord != 0 {
					if e == nil {
						e = geom.ErrConvergence
					}
					return empty, e
				}
				old.End = r.Boundary[1]
				old.Cells = append(old.Cells, i)
			} else {
				result.Coincident = append(result.Coincident, ExactQuadraticAlgebraicPlaneSpan{r.Boundary[0], r.Boundary[1], []int{i}})
			}
			continue
		}
		for _, event := range r.Events {
			if n := len(result.Hits); n > 0 {
				old := &result.Hits[n-1]
				ord, e := compare(old.Event, event)
				if e != nil {
					return empty, e
				}
				if ord == 0 {
					old.Multiplicity = max(old.Multiplicity, event.Multiplicity())
					old.Cells = append(old.Cells, i)
					continue
				}
				if ord != -1 {
					return empty, geom.ErrConvergence
				}
			}
			result.Hits = append(result.Hits, ExactQuadraticAlgebraicPlaneContact{event, []int{i}, event.Multiplicity()})
		}
	}
	if len(result.Coincident) != 0 {
		keep := result.Hits[:0]
		for _, h := range result.Hits {
			covered := false
			for _, span := range result.Coincident {
				a, e := compare(h.Event, span.Start)
				if e != nil {
					return empty, e
				}
				if a < 0 {
					break
				}
				b, e := compare(h.Event, span.End)
				if e != nil {
					return empty, e
				}
				if b <= 0 {
					covered = true
					break
				}
			}
			if !covered {
				keep = append(keep, h)
			}
		}
		result.Hits = keep
	}
	return result, nil
}
