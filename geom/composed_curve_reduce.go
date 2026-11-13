package geom

import "context"

// ReduceHomogeneous removes exact polynomial factors common to all four
// homogeneous coordinates. The model, parameter and true algebraic domain
// are unchanged. The original positive denominator proves that the factor
// cannot vanish on that domain; the reduced denominator is oriented positive.
func (c *ExactComposedCurveCell) ReduceHomogeneous(ctx context.Context, o ExactCurvePlaneEventOptions) (out *ExactComposedCurveCell, operations int, err error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 {
		return nil, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, 0, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return nil, 0, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	common := c.power[3]
	for i := 0; i < 3 && len(common) > 1; i++ {
		if e := ctx.Err(); e != nil {
			return nil, 0, e
		}
		if len(c.power[i]) == 0 {
			continue
		}
		common, e = polynomialGCD(common, c.power[i], b)
		if e != nil {
			return nil, 0, e
		}
	}
	if len(common) <= 1 {
		return c, 0, nil
	}
	copy := *c
	for i, p := range c.power {
		if len(p) == 0 {
			continue
		}
		q, r, e := polynomialDivide(p, common, b)
		if e != nil {
			return nil, 0, e
		}
		if len(r) != 0 {
			return nil, 0, ErrConvergence
		}
		copy.power[i] = q
	}
	sign, _, e := composedPolynomialRootSign(ctx, c.endpoints[0], copy.power[3], b, o.MaxSubdivisions)
	if e != nil {
		return nil, 0, e
	}
	if sign == 0 {
		return nil, 0, ErrConvergence
	}
	if sign < 0 {
		for _, p := range copy.power {
			for _, q := range p {
				q.Neg(q)
				if e := b.check(q); e != nil {
					return nil, 0, e
				}
			}
		}
	}
	return &copy, 0, nil
}
