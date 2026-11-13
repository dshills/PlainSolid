package geom

import (
	"context"
	"math/big"
)

// MonotoneLineOverlap retains one original cell's clipped closed portion, or
// an isolated point when Point is true. Endpoints carry original model/parameter
// provenance; LineStart/LineEnd are exact values in the supplied line units.
type MonotoneLineOverlap struct {
	Cell               int
	Start, End         *ExactComposedPlaneEvent
	LineStart, LineEnd *ExactAlgebraicRoot
	Point              bool
}
type MonotoneLineOverlapTable struct {
	Complete                 bool
	Domain                   [2]*ExactAlgebraicRoot
	CellCount, DirectionSign int
	Parts                    []MonotoneLineOverlap
}

// DiscoverMonotoneLineOverlaps accepts only supporting-line-coincident cells
// with exactly affine, strictly monotone line parameter in original curve units.
// All cells must have the same direction sign. Source-cell seams are retained,
// so adjacent Parts may share an exact endpoint; they are not duplicate edges.
// Empty clipping is complete; nonlinear speed, folds and noncoincidence reject.
// Limits bound cells, raw parts, and each exact proof stage independently.
func (c SurfaceCurveCompositionCells) DiscoverMonotoneLineOverlaps(ctx context.Context, origin, direction [3]*big.Rat, limits [2]*big.Rat, o ComposedLineContactsOptions) (MonotoneLineOverlapTable, error) {
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
		quotient, remainder, e := polynomialDivide(cell.power[pivot], cell.power[3], b)
		if e != nil {
			return zero, e
		}
		if len(trimPolynomial(remainder)) != 0 || len(trimPolynomial(quotient)) != 2 {
			return zero, ErrDomain
		}
		offset := new(big.Rat).Quo(new(big.Rat).Sub(quotient[0], origin[pivot]), direction[pivot])
		scale := new(big.Rat).Quo(quotient[1], direction[pivot])
		for _, v := range []*big.Rat{offset, scale} {
			if e := b.check(v); e != nil {
				return zero, e
			}
		}
		if scale.Sign() == 0 {
			return zero, ErrDomain
		}
		if out.DirectionSign == 0 {
			out.DirectionSign = scale.Sign()
		} else if out.DirectionSign != scale.Sign() {
			return zero, ErrDomain
		}
		appendPart := func(a, z *ExactComposedPlaneEvent, point bool) error {
			if len(out.Parts) >= o.MaxEvents {
				return ErrConvergence
			}
			lr, _, e := a.Root().AffineMap(ctx, offset, scale, opts)
			if e != nil {
				return e
			}
			rr, _, e := z.Root().AffineMap(ctx, offset, scale, opts)
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
func (c *SurfaceIsoCurve) DiscoverMonotoneLineOverlaps(ctx context.Context, origin, direction [3]*big.Rat, limits [2]*big.Rat, o ComposedLineContactsOptions) (MonotoneLineOverlapTable, error) {
	var zero MonotoneLineOverlapTable
	opts, e := graphRootOptions(o.LineOptions.ExactCurvePlaneEventOptions)
	if e != nil {
		return zero, e
	}
	cells, e := c.ModelContactCells(ctx, SurfacePatchOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}, o.MaxCells)
	if e != nil {
		return zero, e
	}
	return cells.DiscoverMonotoneLineOverlaps(ctx, origin, direction, limits, o)
}
