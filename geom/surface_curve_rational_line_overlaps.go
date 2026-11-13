package geom

import (
	"context"
	"math/big"
)

// DiscoverRationalMonotoneLineOverlaps accepts supporting-line-coincident
// cells whose rational coordinate derivative has a strictly signed exact
// Bernstein hull over the entire enclosing parameter interval. The positive
// denominator was proved by each immutable cell. The hull is sufficient, not
// necessary: an inconclusive sign returns ErrConvergence, never a sample-based
// monotonicity claim. All cells must have consistent direction. Algebraic image
// degree limits of CoordinateRoot apply to clipped endpoints. Source-cell
// seams and zero-width cap occurrences are retained. Limits are per stage.
func (c SurfaceCurveCompositionCells) DiscoverRationalMonotoneLineOverlaps(ctx context.Context, origin, direction [3]*big.Rat, limits [2]*big.Rat, o ComposedLineContactsOptions) (MonotoneLineOverlapTable, error) {
	var zero MonotoneLineOverlapTable
	if ctx == nil || !c.Complete || len(c.Cells) == 0 {
		return zero, ErrInvalid
	}
	if o.MaxCells == 0 {
		o.MaxCells = 64
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 || o.MaxEvents < 1 || o.MaxEvents > 65536 {
		return zero, ErrInvalid
	}
	if len(c.Cells) > o.MaxCells {
		return zero, ErrConvergence
	}
	opts, e := graphRootOptions(o.LineOptions.ExactCurvePlaneEventOptions)
	if e != nil {
		return zero, e
	}
	if e = c.validateContactSeams(ctx, opts); e != nil {
		return zero, e
	}
	pivot := -1
	for j := 0; j < 3; j++ {
		if origin[j] == nil || direction[j] == nil {
			return zero, ErrInvalid
		}
		if direction[j].Sign() != 0 {
			pivot = j
		}
	}
	if pivot < 0 {
		return zero, ErrDegenerate
	}
	out := MonotoneLineOverlapTable{}
	for i, cell := range c.Cells {
		hit, e := cell.IntersectRationalFiniteLine(ctx, origin, direction, limits, o.LineOptions)
		if e != nil {
			return zero, e
		}
		if !hit.Complete || !hit.Coincident {
			return zero, ErrDomain
		}
		b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
		left, e := polynomialProduct(polynomialDerivative(cell.power[pivot]), cell.power[3], b)
		if e != nil {
			return zero, e
		}
		right, e := polynomialProduct(cell.power[pivot], polynomialDerivative(cell.power[3]), b)
		if e != nil {
			return zero, e
		}
		derivative := make(rationalPolynomial, max(len(left), len(right)))
		for k := range derivative {
			derivative[k] = new(big.Rat)
			if k < len(left) {
				derivative[k].Set(left[k])
			}
			if k < len(right) {
				derivative[k].Sub(derivative[k], right[k])
			}
			if e := b.check(derivative[k]); e != nil {
				return zero, e
			}
		}
		enclosure, e := polynomialBernsteinInterval(trimPolynomial(derivative), exactInterval{lo: cell.endpoints[0].bounds[0], hi: cell.endpoints[1].bounds[1]}, b)
		if e != nil {
			return zero, e
		}
		sign := 0
		if enclosure.lo.Sign() > 0 {
			sign = 1
		} else if enclosure.hi.Sign() < 0 {
			sign = -1
		} else {
			return zero, ErrConvergence
		}
		sign *= direction[pivot].Sign()
		if out.DirectionSign == 0 {
			out.DirectionSign = sign
		} else if out.DirectionSign != sign {
			return zero, ErrDomain
		}
		offset := new(big.Rat).Quo(new(big.Rat).Neg(origin[pivot]), direction[pivot])
		scale := new(big.Rat).Inv(direction[pivot])
		image := func(event *ExactComposedPlaneEvent) (*ExactAlgebraicRoot, error) {
			coordinate, _, e := event.CoordinateRoot(ctx, pivot, opts)
			if e != nil {
				return nil, e
			}
			mapped, _, e := coordinate.AffineMap(ctx, offset, scale, opts)
			return mapped, e
		}
		appendPart := func(a, z *ExactComposedPlaneEvent, point bool) error {
			if len(out.Parts) >= o.MaxEvents {
				return ErrConvergence
			}
			lr, e := image(a)
			if e != nil {
				return e
			}
			rr, e := image(z)
			if e != nil {
				return e
			}
			out.Parts = append(out.Parts, MonotoneLineOverlap{Cell: i, Start: a, End: z, LineStart: lr, LineEnd: rr, Point: point})
			return nil
		}
		for _, p := range hit.Portions {
			if e := appendPart(p.Start, p.End, false); e != nil {
				return zero, e
			}
		}
		for _, p := range hit.Hits {
			if e := appendPart(p.Event, p.Event, true); e != nil {
				return zero, e
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Domain[0], _ = c.Cells[0].EndpointRoot(0)
	out.Domain[1], _ = c.Cells[len(c.Cells)-1].EndpointRoot(1)
	out.CellCount = len(c.Cells)
	out.Complete = true
	return out, nil
}
func (c *SurfaceIsoCurve) DiscoverRationalMonotoneLineOverlaps(ctx context.Context, origin, direction [3]*big.Rat, limits [2]*big.Rat, o ComposedLineContactsOptions) (MonotoneLineOverlapTable, error) {
	var zero MonotoneLineOverlapTable
	opts, e := graphRootOptions(o.LineOptions.ExactCurvePlaneEventOptions)
	if e != nil {
		return zero, e
	}
	cells, e := c.ModelContactCells(ctx, SurfacePatchOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}, o.MaxCells)
	if e != nil {
		return zero, e
	}
	return cells.DiscoverRationalMonotoneLineOverlaps(ctx, origin, direction, limits, o)
}
