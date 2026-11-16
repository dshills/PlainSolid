package geom

import (
	"context"
	"math/big"
)

type sweepLineRulingPolynomials struct {
	normalFoot, direction, squared, weight rationalPolynomial
	weightHull                             exactInterval
	along, rate                            *big.Rat
}

func (w *sweepLineWork) intersectRulingAlgebraic(ctx context.Context, axis int, ruling [3]*big.Rat) (*NormalOffsetSweepLineSpan, error) {
	p, err := w.rulingPolynomials(ctx, axis, ruling)
	if err != nil {
		return nil, err
	}
	if len(p.normalFoot) == 0 {
		// A coincident family is outside the finite root proof.
		return nil, ErrConvergence
	}
	options, err := w.algebraicOptions()
	if err != nil {
		return nil, err
	}
	roots, err := IsolateExactPolynomialRoots(ctx, p.normalFoot, new(big.Rat), big.NewRat(1, 1), options)
	w.budget.operations += roots.Operations
	if err != nil {
		return nil, err
	}
	for _, root := range roots.Roots {
		tSign, err := w.rulingRootSign(ctx, root.Root, p.direction)
		if err != nil {
			return nil, err
		}
		if axis == 1 {
			tSign = -tSign
		}
		var exactT *big.Rat
		if tSign == 0 {
			exactT = new(big.Rat)
		}
		lower, err := w.compareRulingShift(ctx, root.Root, p, tSign, w.original[2].lo)
		if err != nil {
			return nil, err
		}
		if lower < 0 {
			continue
		}
		upper, err := w.compareRulingShift(ctx, root.Root, p, tSign, w.original[2].hi)
		if err != nil {
			return nil, err
		}
		if upper > 0 {
			continue
		}
		if lower == 0 {
			exactT = w.original[2].lo
		}
		if upper == 0 {
			exactT = w.original[2].hi
		}
		// The perpendicular ruling and sealed sweep injectivity
		// preclude a second admissible transverse root.
		return w.acceptRulingRoot(ctx, axis, ruling, root.Root, p, tSign, exactT)
	}
	return nil, nil
}

func (w *sweepLineWork) rulingPolynomials(ctx context.Context, axis int, ruling [3]*big.Rat) (result sweepLineRulingPolynomials, err error) {
	source := w.sweep.source
	count, step := source.countV, 1
	if axis == 1 {
		count, step = source.countU, source.countV
	}
	// The exact normal-foot polynomial of a rational profile of degree
	// m has degree at most 3m-1. Qualify the whole supported source.
	if 3*(count-1)-1 > ExactPolynomialMaxDegree {
		return result, ErrConvergence
	}
	b := w.budget
	a := boundedRatArithmetic{b: b}
	var delta [3]*big.Rat
	first := source.net[0]
	for i := range delta {
		delta[i] = a.sub(w.origin[i], a.quo(first[i], first[3]))
	}
	norm2 := sweepLineDot(&a, ruling, ruling)
	if a.err != nil {
		return result, a.err
	}
	result.along = a.quo(sweepLineDot(&a, delta, ruling), norm2)
	result.rate = a.quo(sweepLineDot(&a, w.direction, ruling), norm2)
	var projected [3]*big.Rat
	for i := range projected {
		projected[i] = a.sub(w.origin[i], a.mul(result.along, ruling[i]))
	}
	if a.err != nil {
		return result, a.err
	}
	var homogeneous [4]rationalPolynomial
	for i := range homogeneous {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		controls := make([]*big.Rat, count)
		for j := range controls {
			controls[j] = source.net[j*step][i]
		}
		homogeneous[i], err = bernsteinToPower(controls, b)
		if err != nil {
			return result, err
		}
		if i == 3 {
			result.weightHull = coefficientRange(controls)
		}
	}
	result.weight = homogeneous[3]
	dw := polynomialDerivative(result.weight)
	var residual, tangent [3]rationalPolynomial
	for i, h := range homogeneous[:3] {
		qw, err := polynomialProduct(result.weight, rationalPolynomial{projected[i]}, b)
		if err != nil {
			return result, err
		}
		residual[i], err = graphPolynomialSum(qw, h, -1, b)
		if err != nil {
			return result, err
		}
		tangent[i], err = offsetPolynomialDet(polynomialDerivative(h), h, dw, result.weight, b)
		if err != nil {
			return result, err
		}
	}
	var normal [3]rationalPolynomial
	for i := range normal {
		j, k := (i+1)%3, (i+2)%3
		normal[i], err = offsetPolynomialDet(rationalPolynomial{ruling[j]}, rationalPolynomial{ruling[k]}, tangent[j], tangent[k], b)
		if err != nil {
			return result, err
		}
	}
	if result.normalFoot, err = sweepLinePolynomialDot(residual, tangent, b); err != nil {
		return result, err
	}
	if result.direction, err = sweepLinePolynomialDot(residual, normal, b); err != nil {
		return result, err
	}
	result.squared, err = sweepLinePolynomialDot(residual, residual, b)
	return result, err
}

func sweepLinePolynomialDot(a, c [3]rationalPolynomial, b *polynomialBudget) (out rationalPolynomial, err error) {
	for i := range a {
		p, err := polynomialProduct(a[i], c[i], b)
		if err != nil {
			return nil, err
		}
		out, err = graphPolynomialSum(out, p, 1, b)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (w *sweepLineWork) compareRulingShift(ctx context.Context, root *ExactAlgebraicRoot, p sweepLineRulingPolynomials,
	tSign int, bound *big.Rat) (int, error) {
	if tSign != bound.Sign() {
		if tSign < bound.Sign() {
			return -1, nil
		}
		return 1, nil
	}
	if tSign == 0 {
		return 0, nil
	}
	a := boundedRatArithmetic{b: w.budget}
	square := a.mul(bound, bound)
	if a.err != nil {
		return 0, a.err
	}
	w2, err := polynomialProduct(p.weight, p.weight, w.budget)
	if err != nil {
		return 0, err
	}
	q, err := polynomialProduct(w2, rationalPolynomial{square}, w.budget)
	if err != nil {
		return 0, err
	}
	d, err := graphPolynomialSum(p.squared, q, -1, w.budget)
	if err != nil {
		return 0, err
	}
	sign, err := w.rulingRootSign(ctx, root, d)
	return sign * tSign, err
}

func (w *sweepLineWork) rulingRootSign(ctx context.Context, root *ExactAlgebraicRoot, p rationalPolynomial) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	sign, _, err := composedPolynomialRootSign(ctx, root.root, p, w.budget, 8192)
	return sign, err
}

func (w *sweepLineWork) algebraicOptions() (ExactCurvePlaneEventOptions, error) {
	remaining := w.budget.maxOperations - w.budget.operations
	if remaining <= 0 {
		return ExactCurvePlaneEventOptions{}, ErrConvergence
	}
	return ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: w.budget.maxBits}, nil
}
