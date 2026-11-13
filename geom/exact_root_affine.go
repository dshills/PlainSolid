package geom

import (
	"context"
	"math/big"
)

// AffineMap returns the immutable algebraic number offset+scale*r exactly.
// Negative scales reverse the isolating interval. Zero scale returns a proved
// rational root. Inputs and returned coefficient storage are independent.
func (r *ExactAlgebraicRoot) AffineMap(ctx context.Context, offset, scale *big.Rat, o ExactCurvePlaneEventOptions) (out *ExactAlgebraicRoot, operations int, err error) {
	if ctx == nil || r == nil || r.root == nil || offset == nil || scale == nil {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	options, e := graphRootOptions(o)
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	for _, q := range []*big.Rat{offset, scale} {
		if e := b.check(q); e != nil {
			return nil, 0, e
		}
	}
	root, e := affineMapRoot(r.root, offset, scale, b)
	if e != nil {
		return nil, 0, e
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	return &ExactAlgebraicRoot{root: root}, b.operations, nil
}
func affineMapRoot(root *ExactGraphLineRoot, offset, scale *big.Rat, b *polynomialBudget) (*ExactGraphLineRoot, error) {
	if scale.Sign() == 0 {
		return &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(offset), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat).Set(offset), new(big.Rat).Set(offset)}}, nil
	}
	inverse := new(big.Rat).Inv(scale)
	origin := new(big.Rat).Neg(new(big.Rat).Mul(offset, inverse))
	mapped, e := mapComposedGraphRoot(root, origin, inverse, offset, scale, b)
	if e != nil {
		return nil, e
	}
	if scale.Sign() < 0 {
		mapped.bounds[0], mapped.bounds[1] = mapped.bounds[1], mapped.bounds[0]
	}
	return mapped, nil
}
