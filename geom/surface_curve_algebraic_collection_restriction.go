package geom

import (
	"context"
	"math/big"
)

// RestrictAlgebraic restricts an ordered collection while preserving separate
// knot-cell slices. It does not trust the collection's mutable Complete flag:
// every original join is independently checked for exact parameter identity and
// model continuity. Complete covers the requested closed algebraic subdomain.
func (collection SurfaceCurveCompositionCells) RestrictAlgebraic(ctx context.Context, lower, upper *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (r SurfaceCurveCompositionCells, err error) {
	if ctx == nil || len(collection.Cells) == 0 || lower == nil || upper == nil || lower.root == nil || upper.root == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if len(collection.Cells) > 4096 {
		return r, ErrConvergence
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	compare := func(a, c *ExactGraphLineRoot) (int, error) {
		comparison, e := compareRootBudget(ctx, a, c, b, o.MaxSubdivisions-r.Subdivisions)
		r.Subdivisions += comparison.Subdivisions
		if e != nil {
			return 0, e
		}
		if !comparison.Proven {
			return 0, ErrConvergence
		}
		return comparison.Order, nil
	}
	order, e := compare(lower.root, upper.root)
	if e != nil {
		return r, e
	}
	if order != -1 {
		return r, ErrDomain
	}
	for i, cell := range collection.Cells {
		if cell == nil || cell.endpoints[0] == nil || cell.endpoints[1] == nil || len(cell.power[3]) == 0 {
			return r, ErrInvalid
		}
		if i == 0 {
			continue
		}
		previous := collection.Cells[i-1]
		order, e := compare(previous.endpoints[1], cell.endpoints[0])
		if e != nil {
			return r, e
		}
		if order != 0 {
			return r, ErrDomain
		}
		for axis := 0; axis < 3; axis++ {
			left, e := polynomialProduct(previous.power[axis], cell.power[3], b)
			if e != nil {
				return r, e
			}
			right, e := polynomialProduct(cell.power[axis], previous.power[3], b)
			if e != nil {
				return r, e
			}
			difference, e := subtractScaledPolynomial(left, right, big.NewRat(1, 1), b)
			if e != nil {
				return r, e
			}
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			sign, steps, e := composedPolynomialRootSign(ctx, previous.endpoints[1], difference, b, remaining)
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
			if sign != 0 {
				return r, ErrConvergence
			}
		}
	}
	first, last := collection.Cells[0], collection.Cells[len(collection.Cells)-1]
	order, e = compare(lower.root, first.endpoints[0])
	if e != nil {
		return r, e
	}
	if order < 0 {
		return r, ErrDomain
	}
	order, e = compare(upper.root, last.endpoints[1])
	if e != nil {
		return r, e
	}
	if order > 0 {
		return r, ErrDomain
	}
	for _, cell := range collection.Cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		endOrder, e := compare(cell.endpoints[1], lower.root)
		if e != nil {
			return r, e
		}
		startOrder, e := compare(cell.endpoints[0], upper.root)
		if e != nil {
			return r, e
		}
		if endOrder <= 0 || startOrder >= 0 {
			continue
		}
		start, end := lower, upper
		order, e := compare(cell.endpoints[0], lower.root)
		if e != nil {
			return r, e
		}
		if order > 0 {
			start = &ExactAlgebraicRoot{root: cell.endpoints[0]}
		}
		order, e = compare(cell.endpoints[1], upper.root)
		if e != nil {
			return r, e
		}
		if order < 0 {
			end = &ExactAlgebraicRoot{root: cell.endpoints[1]}
		}
		child := o
		child.MaxOperations -= b.operations
		child.MaxSubdivisions -= r.Subdivisions
		if child.MaxOperations < 1 || child.MaxSubdivisions < 1 {
			return r, ErrConvergence
		}
		restriction, e := cell.RestrictAlgebraic(ctx, start, end, child)
		b.operations += restriction.Operations
		r.Subdivisions += restriction.Subdivisions
		if e != nil {
			return r, e
		}
		if !restriction.Proven {
			return r, ErrConvergence
		}
		r.Cells = append(r.Cells, restriction.Cell)
	}
	if len(r.Cells) == 0 {
		return r, ErrConvergence
	}
	for side, root := range []*ExactGraphLineRoot{r.Cells[0].endpoints[0], r.Cells[len(r.Cells)-1].endpoints[1]} {
		target := lower.root
		if side == 1 {
			target = upper.root
		}
		order, e := compare(root, target)
		if e != nil {
			return r, e
		}
		if order != 0 {
			return r, ErrConvergence
		}
	}
	r.Events = append(r.Events, r.Cells[0].endpoints[0].Interval())
	for _, cell := range r.Cells {
		r.Events = append(r.Events, cell.endpoints[1].Interval())
	}
	r.Complete = true
	return r, nil
}
