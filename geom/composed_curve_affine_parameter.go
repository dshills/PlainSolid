package geom

import (
	"context"
	"math/big"
)

// ComposedAffineParameterComparison concerns one supplied affine parameter map,
// not arbitrary same-image equality. Equivalent=false with Proven=true means
// that map fails polynomial identity on the matched nonempty domains; isolated
// common points may still exist. Reversed records a negative parameter scale.
type ComposedAffineParameterComparison struct {
	Proven, Equivalent, Reversed bool
	Operations                   int
}

// CompareComposedCurvesAffineParameter proves B(offset+scale*t)=A(t) on the
// entire exact domains. It first proves mapped endpoint equality, including
// reversal, then compares all homogeneous cross-polynomial coefficients.
// Different positive rational weight fields are allowed. Degree is bounded at
// 64 per homogeneous input coordinate; all work shares a cumulative budget.
// Domain mismatch returns ErrDomain. Zero scale is invalid. No sampled points,
// affine model coordinate, planar support, or algebraic image recovery is used.
func CompareComposedCurvesAffineParameter(ctx context.Context, a, b *ExactComposedCurveCell, offset, scale *big.Rat, o ExactCurvePlaneEventOptions) (out ComposedAffineParameterComparison, err error) {
	if ctx == nil || a == nil || b == nil || offset == nil || scale == nil || scale.Sign() == 0 || len(a.power[3]) == 0 || len(b.power[3]) == 0 {
		return out, ErrInvalid
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, q := range []*big.Rat{offset, scale} {
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	for _, cell := range []*ExactComposedCurveCell{a, b} {
		for _, p := range cell.power {
			if len(p) > 65 {
				return out, ErrConvergence
			}
			for _, q := range p {
				if e := budget.check(q); e != nil {
					return out, e
				}
			}
		}
	}
	out.Operations = budget.operations
	opts.MaxOperations -= budget.operations
	for side := 0; side < 2; side++ {
		if opts.MaxOperations < 1 || opts.MaxSubdivisions < 1 {
			return out, ErrConvergence
		}
		source, e := a.EndpointRoot(side)
		if e != nil {
			return out, e
		}
		mapped, n, e := source.AffineMap(ctx, offset, scale, opts)
		out.Operations += n
		opts.MaxOperations -= n
		if e != nil {
			return out, e
		}
		if opts.MaxOperations < 1 {
			return out, ErrConvergence
		}
		targetSide := side
		if scale.Sign() < 0 {
			targetSide = 1 - side
		}
		target, e := b.EndpointRoot(targetSide)
		if e != nil {
			return out, e
		}
		cmp, e := mapped.Compare(ctx, target, opts)
		out.Operations += cmp.Operations
		opts.MaxOperations -= cmp.Operations
		opts.MaxSubdivisions -= cmp.Subdivisions
		if e != nil {
			return out, e
		}
		if !cmp.Proven {
			return out, ErrConvergence
		}
		if cmp.Order != 0 {
			return out, ErrDomain
		}
	}
	if opts.MaxOperations < 1 {
		return out, ErrConvergence
	}
	budget = &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() { out.Operations += budget.operations }()
	var mapped [4]rationalPolynomial
	for axis := 0; axis < 4; axis++ {
		mapped[axis], e = composeProjectionPolynomial(b.power[axis], rationalPolynomial{offset, scale}, budget)
		if e != nil {
			return out, e
		}
	}
	equal := true
	for axis := 0; axis < 3; axis++ {
		left, e := polynomialProduct(a.power[axis], mapped[3], budget)
		if e != nil {
			return out, e
		}
		right, e := polynomialProduct(mapped[axis], a.power[3], budget)
		if e != nil {
			return out, e
		}
		left, right = trimPolynomial(left), trimPolynomial(right)
		if len(left) != len(right) {
			equal = false
			continue
		}
		for i := range left {
			if left[i].Cmp(right[i]) != 0 {
				equal = false
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Proven = true
	out.Equivalent = equal
	out.Reversed = scale.Sign() < 0
	return out, nil
}
