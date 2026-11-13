package geom

import (
	"context"
	"math/big"
)

// ComposedMobiusParameterComparison proves one supplied parameter relation,
// not arbitrary same-image equivalence. OrientationSign is +1/-1 only on
// success; it is the exact traversal sign on the entire matched domain.
type ComposedMobiusParameterComparison struct {
	Proven, Equivalent          bool
	OrientationSign, Operations int
}

// CompareComposedCurvesMobiusParameter proves
// B((m[0]+m[1]*t)/(m[2]+m[3]*t))=A(t) on their entire exact domains. The linear
// denominator must have one strict sign throughout A's closed domain. Its sign
// is canonicalized positive before all image/coefficient work. Both endpoint
// identities are proved, respecting reversal, and homogeneous polynomial
// identities are checked after exact denominator clearing. CoordinateRoot's
// endpoint degree<=4 limit applies. Input homogeneous degrees are at most64.
// The operation budget is cumulative; the subdivision limit applies per root
// image/sign proof. Failures return no Proven/orientation claim.
func CompareComposedCurvesMobiusParameter(ctx context.Context, a, b *ExactComposedCurveCell, m [4]*big.Rat, o ExactCurvePlaneEventOptions) (out ComposedMobiusParameterComparison, err error) {
	if ctx == nil || a == nil || b == nil || a.endpoints[0] == nil || a.endpoints[1] == nil || b.endpoints[0] == nil || b.endpoints[1] == nil || len(a.power[3]) == 0 || len(b.power[3]) == 0 {
		return out, ErrInvalid
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, q := range m {
		if q == nil {
			return out, ErrInvalid
		}
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	degree := 0
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
	for _, p := range b.power {
		degree = max(degree, len(p)-1)
	}
	numerator := rationalPolynomial{new(big.Rat).Set(m[0]), new(big.Rat).Set(m[1])}
	denominator := rationalPolynomial{new(big.Rat).Set(m[2]), new(big.Rat).Set(m[3])}
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(m[1], m[2]), new(big.Rat).Mul(m[0], m[3]))
	if e := budget.check(determinant); e != nil {
		return out, e
	}
	if determinant.Sign() == 0 {
		return out, ErrDegenerate
	}
	signs := [2]int{}
	for i, root := range a.endpoints {
		var n int
		signs[i], n, e = composedPolynomialRootSign(ctx, root, trimPolynomial(denominator), budget, opts.MaxSubdivisions)
		opts.MaxSubdivisions -= n
		if e != nil {
			return out, e
		}
		if opts.MaxSubdivisions < 1 {
			return out, ErrConvergence
		}
	}
	if signs[0] == 0 || signs[0] != signs[1] {
		return out, ErrDomain
	}
	if signs[0] < 0 {
		for _, p := range []rationalPolynomial{numerator, denominator} {
			for _, q := range p {
				q.Neg(q)
			}
		}
	}
	out.Operations = budget.operations
	opts.MaxOperations -= budget.operations
	mapper := &ExactComposedCurveCell{endpoints: a.endpoints}
	mapper.power[0] = trimPolynomial(numerator)
	mapper.power[3] = trimPolynomial(denominator)
	for i := 0; i < 2; i++ {
		if opts.MaxOperations < 1 || opts.MaxSubdivisions < 1 {
			return out, ErrConvergence
		}
		source, _ := a.EndpointRoot(i)
		image, n, e := mapper.CoordinateRoot(ctx, source, 0, opts)
		out.Operations += n
		opts.MaxOperations -= n
		if e != nil {
			return out, e
		}
		if opts.MaxOperations < 1 || opts.MaxSubdivisions < 1 {
			return out, ErrConvergence
		}
		side := i
		if determinant.Sign() < 0 {
			side = 1 - i
		}
		target, _ := b.EndpointRoot(side)
		cmp, e := image.Compare(ctx, target, opts)
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
	if opts.MaxOperations < 1 || opts.MaxSubdivisions < 1 {
		return out, ErrConvergence
	}
	budget = &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	defer func() { out.Operations += budget.operations }()
	np, dp := make([]rationalPolynomial, degree+1), make([]rationalPolynomial, degree+1)
	np[0] = rationalPolynomial{big.NewRat(1, 1)}
	dp[0] = rationalPolynomial{big.NewRat(1, 1)}
	for i := 1; i <= degree; i++ {
		np[i], e = polynomialProduct(np[i-1], numerator, budget)
		if e != nil {
			return out, e
		}
		dp[i], e = polynomialProduct(dp[i-1], denominator, budget)
		if e != nil {
			return out, e
		}
	}
	var mapped [4]rationalPolynomial
	for axis, p := range b.power {
		mapped[axis] = make(rationalPolynomial, degree+1)
		for i := range mapped[axis] {
			mapped[axis][i] = new(big.Rat)
		}
		for j, q := range p {
			term, e := polynomialProduct(np[j], dp[degree-j], budget)
			if e != nil {
				return out, e
			}
			for k, v := range term {
				mapped[axis][k].Add(mapped[axis][k], new(big.Rat).Mul(q, v))
				if e := budget.check(mapped[axis][k]); e != nil {
					return out, e
				}
			}
		}
		mapped[axis] = trimPolynomial(mapped[axis])
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
	out.OrientationSign = determinant.Sign()
	return out, nil
}
