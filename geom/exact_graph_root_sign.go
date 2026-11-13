package geom

import (
	"context"
	"math/big"
)

// CompareExactGraphLineRootRational proves root-versus-rational order, including
// equality at a cap or piece boundary. Order is -1,0,+1 for root <,=,> parameter.
// No float conversion, approximate comparison or tolerance equality is used.
func CompareExactGraphLineRootRational(ctx context.Context, root *ExactGraphLineRoot, parameter *big.Rat, o ExactCurvePlaneEventOptions) (ExactCurveEventOrder, error) {
	if parameter == nil {
		return ExactCurveEventOrder{}, ErrInvalid
	}
	cap := &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(parameter), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat).Set(parameter), new(big.Rat).Set(parameter)}}
	return CompareExactGraphLineRoots(ctx, root, cap, o)
}

// ExactGraphRootSign proves the sign of a selected graph piece minus its affine
// height at another graph's exact algebraic event. Side zero proves equality;
// Interval retains a closed enclosure of the source event in global parameter.
type ExactGraphRootSign struct {
	Proven                   bool
	Side                     int
	Interval                 ExactParameterInterval
	Operations, Subdivisions int
}

// ClassifyExactGraphRootHeight proves that the source event lies within the
// selected piece, then evaluates its exact height sign there. A polynomial GCD
// detects equality, including repeated roots. Otherwise interval Horner bounds
// on successively narrowed source-root brackets prove the sign without isolating
// any other roots of the target graph. Parameter identity does not imply model
// identity unless the caller independently proves the same XY path.
func ClassifyExactGraphRootHeight(ctx context.Context, root *ExactGraphLineRoot, piece SurfaceAffineCurvePiece, start, end *big.Rat, o ExactCurvePlaneEventOptions) (r ExactGraphRootSign, err error) {
	if ctx == nil || root == nil || len(root.polynomial) == 0 || root.bounds[0] == nil || piece.Curve == nil || piece.Curve.patch == nil || piece.Start == nil || piece.End == nil || start == nil || end == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if piece.Start.Sign() < 0 || piece.End.Cmp(big.NewRat(1, 1)) > 0 || piece.Start.Cmp(piece.End) >= 0 {
		return r, ErrInvalid
	}
	lo, hi := piece.Curve.Domain()
	if lo != 0 || hi != 1 || piece.Curve.patch.countU != piece.Curve.patch.degreeU+1 {
		return r, ErrConvergence
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = budget.operations }()
	for _, x := range []*big.Rat{piece.Start, piece.End, start, end, root.bounds[0], root.bounds[1]} {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	for _, x := range root.polynomial {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	bounds := [2]*big.Rat{new(big.Rat).Set(root.bounds[0]), new(big.Rat).Set(root.bounds[1])}
	if bounds[0].Cmp(piece.Start) < 0 {
		bounds[0].Set(piece.Start)
	}
	if bounds[1].Cmp(piece.End) > 0 {
		bounds[1].Set(piece.End)
	}
	if bounds[0].Cmp(bounds[1]) > 0 {
		return r, ErrDomain
	}
	count := func(poly rationalPolynomial) (int, error) {
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return 0, ErrConvergence
		}
		n := 0
		steps, e := isolatePolynomialRoots(poly, bounds[0], bounds[1], budget, remaining, func(_, _ *big.Rat, _ int) (bool, error) { n++; return true, ctx.Err() })
		r.Subdivisions += steps
		return n, e
	}
	n, e := count(root.polynomial)
	if e != nil {
		return r, e
	}
	if n != 1 {
		return r, ErrDomain
	}
	target, e := exactGraphHeightPolynomial(piece, start, end, budget)
	if e != nil {
		return r, e
	}
	finish := func(side int, lo, hi *big.Rat) {
		r.Proven = true
		r.Side = side
		r.Interval = ExactParameterInterval{Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi)}
	}
	if len(target) == 0 {
		finish(0, bounds[0], bounds[1])
		return r, nil
	}
	gcd, e := polynomialGCD(root.polynomial, target, budget)
	if e != nil {
		return r, e
	}
	n, e = count(gcd)
	if e != nil {
		return r, e
	}
	if n == 1 {
		finish(0, bounds[0], bounds[1])
		return r, nil
	}
	remaining := o.MaxSubdivisions - r.Subdivisions
	if remaining < 1 {
		return r, ErrConvergence
	}
	steps, e := isolatePolynomialRoots(root.polynomial, bounds[0], bounds[1], budget, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		value, e := polynomialInterval(target, exactInterval{lo: lo, hi: hi}, budget)
		if e != nil {
			return false, e
		}
		if value.lo.Sign() > 0 {
			finish(1, lo, hi)
			return true, nil
		}
		if value.hi.Sign() < 0 {
			finish(-1, lo, hi)
			return true, nil
		}
		return false, nil
	})
	r.Subdivisions += steps
	if e != nil {
		return r, e
	}
	if !r.Proven {
		return r, ErrConvergence
	}
	return r, nil
}
