package geom

import (
	"context"
	"math/big"
)

// ModelContactCells extracts exact polynomial continuations in the original
// spline parameter, one per nonempty knot span. No tensor source or rounded
// control points are manufactured. Cell() remains zero (HasSurfaceCell=false).
// maxCells bounds span count; options bound the aggregate exact extraction.
func (c *SurfaceIsoCurve) ModelContactCells(ctx context.Context, options SurfacePatchOptions, maxCells int) (SurfaceCurveCompositionCells, error) {
	var zero SurfaceCurveCompositionCells
	if ctx == nil || c == nil || c.patch == nil {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if maxCells == 0 {
		maxCells = 64
	}
	if maxCells < 1 || maxCells > 4096 {
		return zero, ErrInvalid
	}
	o, e := patchOptions(options)
	if e != nil {
		return zero, e
	}
	if len(c.patch.net) > o.MaxPoles || c.patch.degreeU > 64 {
		return zero, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	out := SurfaceCurveCompositionCells{}
	stack := []*SurfacePatch{c.patch}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(out.Cells)+len(stack)+1 > maxCells {
			return zero, ErrConvergence
		}
		if knot, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
			left, right, e := p.split(knot, true, b)
			if e != nil {
				return zero, e
			}
			stack = append(stack, right, left)
			continue
		}
		if p.countU != p.degreeU+1 {
			return zero, ErrConvergence
		}
		d := p.Domain()
		lo, hi := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
		width := new(big.Rat).Sub(hi, lo)
		linear := rationalPolynomial{new(big.Rat).Quo(new(big.Rat).Neg(lo), width), new(big.Rat).Inv(width)}
		cell := &ExactComposedCurveCell{}
		for axis := 0; axis < 4; axis++ {
			values := make([]*big.Rat, p.countU)
			for i := range values {
				h := p.net[i*p.countV]
				if h[3].Sign() <= 0 {
					return zero, ErrInvalid
				}
				values[i] = h[axis]
			}
			power, e := bernsteinToPower(values, b)
			if e != nil {
				return zero, e
			}
			cell.power[axis], e = composeProjectionPolynomial(power, linear, b)
			if e != nil {
				return zero, e
			}
		}
		for i, v := range []*big.Rat{lo, hi} {
			if e := b.check(v); e != nil {
				return zero, e
			}
			cell.endpoints[i] = &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(v), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat).Set(v), new(big.Rat).Set(v)}}
		}
		out.Cells = append(out.Cells, cell)
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Complete = true
	out.ExactOperations = b.operations
	return out, nil
}

// DiscoverPlaneContacts provides an original-parameter finite contact table for
// a standalone exact model edge. Extraction has one EventOptions exact budget;
// subsequent collection proof uses its documented per-stage bounded budgets.
// Coincident spans and unproved seams fail atomically, not as finite contacts.
func (c *SurfaceIsoCurve) DiscoverPlaneContacts(ctx context.Context, plane [4]*big.Rat, o ComposedPlaneContactsOptions) (ComposedPlaneContactTable, error) {
	var zero ComposedPlaneContactTable
	opts, e := graphRootOptions(o.EventOptions)
	if e != nil {
		return zero, e
	}
	cells, e := c.ModelContactCells(ctx, SurfacePatchOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}, o.MaxCells)
	if e != nil {
		return zero, e
	}
	return cells.DiscoverPlaneContacts(ctx, plane, o)
}
