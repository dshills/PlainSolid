package geom

import (
	"context"
	"math/big"
)

// TubeGraphEventBinding is immutable evidence that an exact model event is the
// unique branch point at its algebraic free coordinate. Both supports must have
// invertible affine XY projections. It does not solve an arbitrary algebraic
// fixed section or assert that the source curve crosses the branch transversely.
type TubeGraphEventBinding struct {
	parameters       [4]*ExactAlgebraicRoot
	event            *ExactComposedCurveEvent
	boxes            [2]UVBox
	bounds           Box
	axis, operations int
}

func (b *TubeGraphEventBinding) Parameters() ([4]*ExactAlgebraicRoot, error) {
	if b == nil || b.event == nil {
		return [4]*ExactAlgebraicRoot{}, ErrInvalid
	}
	return b.parameters, nil
}
func (b *TubeGraphEventBinding) FreeRoot() (*ExactAlgebraicRoot, error) {
	if b == nil || b.event == nil {
		return nil, ErrInvalid
	}
	return b.parameters[b.axis], nil
}
func (b *TubeGraphEventBinding) ModelEvent() *ExactComposedCurveEvent {
	if b == nil {
		return nil
	}
	return b.event
}
func (b *TubeGraphEventBinding) Bounds() (UVBox, UVBox, Box, error) {
	if b == nil || b.event == nil {
		return UVBox{}, UVBox{}, Box{}, ErrInvalid
	}
	return b.boxes[0], b.boxes[1], b.bounds, nil
}
func (b *TubeGraphEventBinding) ExactOperations() int {
	if b == nil {
		return 0
	}
	return b.operations
}

// BindAffineGraphEvent rechecks model membership on each original local support
// by exact residual isolation. It accepts isolated or coincident source-curve
// membership, then uses strict dependent containment and the immutable tube's
// uniqueness proof to identify the branch point. Source image degree <=4 is the
// current CoordinateRoot limit. Returned evidence never trusts caller boxes.
func (t VerifiedSurfaceTube) BindAffineGraphEvent(ctx context.Context, event *ExactComposedCurveEvent, o TubeAlgebraicGraphEndpointOptions) (*TubeGraphEventBinding, error) {
	if ctx == nil || event == nil || event.cell == nil || event.root == nil || t.a == nil || t.b == nil || t.tangentSign == 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	o.SurfacePatchOptions = options
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxSubdivisions < 1 {
		return nil, ErrInvalid
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	if !finite(o.Tolerance) || o.Tolerance <= 0 {
		return nil, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	remaining := func() (int, error) {
		n := options.MaxOperations - budget.operations
		if n < 1 {
			return 0, ErrConvergence
		}
		return n, nil
	}
	result := &TubeGraphEventBinding{axis: t.axis, event: event}
	var boxes [4][2]float64
	boundary := false
	for owner, patch := range []*SurfacePatch{t.a, t.b} {
		child := o.SurfaceCurveAgreementOptions
		child.MaxOperations, e = remaining()
		if e != nil {
			return nil, e
		}
		contacts, e := IntersectExactComposedCurveAffineGraph(ctx, event.cell, patch, child)
		budget.operations += contacts.ExactOperations
		if e != nil {
			return nil, e
		}
		found := false
		for _, cell := range contacts.Cells {
			for _, hit := range cell.Intersection.Events {
				order, e := compareRootBudget(ctx, event.root, hit.root, budget, o.MaxSubdivisions)
				if e != nil {
					return nil, e
				}
				if order.Proven && order.Order == 0 {
					found = true
				}
			}
			if cell.Intersection.Coincident {
				inside := true
				for side, endpoint := range cell.Intersection.Boundary {
					order, e := compareRootBudget(ctx, event.root, endpoint.root, budget, o.MaxSubdivisions)
					if e != nil {
						return nil, e
					}
					if !order.Proven || side == 0 && order.Order < 0 || side == 1 && order.Order > 0 {
						inside = false
					}
				}
				found = found || inside
			}
		}
		if !found {
			return nil, ErrDomain
		}
		projected, e := transformComposedXY(event.cell, contacts.InverseXY, budget)
		if e != nil {
			return nil, e
		}
		d := patch.Domain()
		ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		for axis := 0; axis < 2; axis++ {
			n, e := remaining()
			if e != nil {
				return nil, e
			}
			root, ops, e := projected.CoordinateRoot(ctx, event.Root(), axis, ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
			budget.operations += ops
			if e != nil {
				return nil, e
			}
			index := 2*owner + axis
			result.parameters[index] = root
			for side := 0; side < 2; side++ {
				q := new(big.Rat).SetFloat64(ranges[axis][side])
				bound := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(q), big.NewRat(1, 1)}, bounds: [2]*big.Rat{q, q}}
				order, e := compareRootBudget(ctx, root.root, bound, budget, o.MaxSubdivisions)
				if e != nil {
					return nil, e
				}
				if !order.Proven {
					return nil, ErrConvergence
				}
				if side == 0 && order.Order < 0 || side == 1 && order.Order > 0 {
					return nil, ErrDomain
				}
				if order.Order == 0 && index != t.axis {
					boundary = true
				}
			}
			boxes[index] = [2]float64{max(ranges[axis][0], rationalFloatBound(root.root.bounds[0], true)), min(ranges[axis][1], rationalFloatBound(root.root.bounds[1], false))}
		}
	}
	if boundary {
		atAnchor, e := t.matchesAnchorRoots(ctx, result.parameters, budget, o.MaxSubdivisions)
		if e != nil {
			return nil, e
		}
		if !atAnchor {
			return nil, ErrConvergence
		}
	}
	n, e := remaining()
	if e != nil {
		return nil, e
	}
	enclosure, e := event.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: o.Tolerance, ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: options.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}})
	budget.operations += enclosure.ExactOperations
	if e != nil {
		return nil, e
	}
	for i := 0; i < 2; i++ {
		result.boxes[i] = UVBox{Min: Vec2{boxes[2*i][0], boxes[2*i+1][0]}, Max: Vec2{boxes[2*i][1], boxes[2*i+1][1]}}
	}
	result.bounds = enclosure.Bounds
	result.operations = budget.operations
	return result, nil
}
