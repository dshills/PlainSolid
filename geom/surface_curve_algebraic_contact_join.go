package geom

import (
	"context"
	"math/big"
)

// ComposedPlaneJunction combines the retained sides of two incident cell germs.
// Before/After preserve each cell's order and velocity regularity separately.
// Crossing/Touching use only the nonzero retained residual signs. They neither
// prove differentiability nor identify tangents across a C0 support knot.
type ComposedPlaneJunction struct {
	Proven, Crossing, Touching bool
	Before, After              ComposedPlaneContact
	SideSigns                  [2]int
	Operations                 int
}

// ClassifyComposedPlaneJunction requires before at its cell's exact upper
// endpoint and after at its exact lower endpoint, in a common original
// parameter. It proves root equality and model equality by homogeneous cross
// products at that root; overlapping brackets or sampled points never suffice.
// C0 joins are accepted without inferring tangent continuity. Operations and
// sign-isolation subdivisions are bounded cumulatively. Failures return no
// Proven or partial classification (Operations may still report work).
func ClassifyComposedPlaneJunction(ctx context.Context, before, after *ExactComposedPlaneEvent, plane [4]*big.Rat, o ExactCurvePlaneEventOptions) (out ComposedPlaneJunction, err error) {
	if ctx == nil || before == nil || after == nil || before.cell == nil || after.cell == nil || before.root == nil || after.root == nil {
		return out, ErrInvalid
	}
	if before.endpointMask != 2 || after.endpointMask != 1 {
		return out, ErrDomain
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	o, err = graphRootOptions(o)
	if err != nil {
		return out, err
	}
	order, e := before.CompareParameter(ctx, after, o)
	out.Operations += order.Operations
	if e != nil {
		return out, e
	}
	if !order.Proven {
		return out, ErrConvergence
	}
	if order.Order != 0 {
		return out, ErrDomain
	}
	o.MaxOperations -= order.Operations
	o.MaxSubdivisions -= order.Subdivisions
	if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
		return out, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	steps := 0
	// Defer reports this stage even when exact equality cannot be established.
	initial := out.Operations
	defer func() {
		if out.Operations < initial+b.operations {
			out.Operations = initial + b.operations
		}
	}()
	for axis := 0; axis < 3; axis++ {
		a, e := polynomialProduct(before.cell.power[axis], after.cell.power[3], b)
		if e != nil {
			return out, e
		}
		c, e := polynomialProduct(after.cell.power[axis], before.cell.power[3], b)
		if e != nil {
			return out, e
		}
		p := make(rationalPolynomial, max(len(a), len(c)))
		for i := range p {
			p[i] = new(big.Rat)
			if i < len(a) {
				p[i].Set(a[i])
			}
			if i < len(c) {
				p[i].Sub(p[i], c[i])
			}
			if e = b.check(p[i]); e != nil {
				return out, e
			}
		}
		remaining := o.MaxSubdivisions - steps
		if remaining < 1 {
			return out, ErrConvergence
		}
		sign, n, e := composedPolynomialRootSign(ctx, before.root, trimPolynomial(p), b, remaining)
		steps += n
		if e != nil {
			return out, e
		}
		if sign != 0 {
			return out, ErrDomain
		}
	}
	out.Operations += b.operations
	o.MaxOperations -= b.operations
	o.MaxSubdivisions -= steps
	if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
		return out, ErrConvergence
	}
	left, e := before.ClassifyPlaneContact(ctx, plane, o)
	out.Operations += left.Operations
	if e != nil {
		return out, e
	}
	o.MaxOperations -= left.Operations
	o.MaxSubdivisions -= left.Subdivisions
	if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
		return out, ErrConvergence
	}
	right, e := after.ClassifyPlaneContact(ctx, plane, o)
	out.Operations += right.Operations
	if e != nil {
		return out, e
	}
	if !left.Proven || !right.Proven || left.SideSigns[0] == 0 || right.SideSigns[1] == 0 {
		return out, ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	out.Proven = true
	out.Before = left
	out.After = right
	out.SideSigns = [2]int{left.SideSigns[0], right.SideSigns[1]}
	out.Crossing = out.SideSigns[0] != out.SideSigns[1]
	out.Touching = !out.Crossing
	return out, nil
}
