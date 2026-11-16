package geom

import (
	"context"
	"math/big"
)

// RationalPeriodicCurveJet describes one exact evaluation on the original
// cyclic curve and identifies its physical knot span. Derivative is expressed
// in the original parameter units. Returned rationals are caller owned.
type RationalPeriodicCurveJet struct {
	Point, Derivative             [3]*big.Rat
	PointBounds, DerivativeBounds Box
	Domain                        ExactParameterInterval
	Cycle                         *big.Int
	Span, ExactOperations         int
}

// EvaluateRational evaluates the original curve at an exact physical
// parameter, even in a cycle that a float64 can no longer resolve. Knots and
// seams select the right piece. No rounded phase or fitted curve is used.
func (c *PeriodicBSpline) EvaluateRational(ctx context.Context, t *big.Rat, o SurfacePatchOptions) (result RationalPeriodicCurveJet, err error) {
	if ctx == nil || c == nil || c.curve == nil || t == nil {
		return result, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	o, err = patchOptions(o)
	if err != nil {
		return result, err
	}
	if c.degree+1 > o.MaxPoles {
		return result, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	if err := b.check(t); err != nil {
		return result, err
	}
	owner, local, err := periodicSpanAtPosition(len(c.poles), c.start, c.period, t, b)
	if err != nil {
		return result, err
	}
	for _, q := range []*big.Rat{owner.Domain.Lower, owner.Domain.Upper, local} {
		if err := b.check(q); err != nil {
			return result, err
		}
	}
	h, d, err := c.periodicHomogeneousJet(ctx, owner.Span, local, b)
	if err != nil {
		return result, err
	}
	width := new(big.Rat).Sub(owner.Domain.Upper, owner.Domain.Lower)
	return exactPeriodicCurveResult(ctx, h, d, width, owner, b)
}

func exactPeriodicCurveResult(ctx context.Context, h, d exactHomogeneous, width *big.Rat,
	owner ExactSurfaceKnotSpan, b *polynomialBudget) (result RationalPeriodicCurveJet, err error) {
	den := new(big.Rat).Mul(h[3], h[3])
	den.Mul(den, width)
	if err := b.check(den); err != nil {
		return result, err
	}
	var p, derivative [3]*big.Rat
	for i := range p {
		p[i] = new(big.Rat).Quo(h[i], h[3])
		x := new(big.Rat).Sub(new(big.Rat).Mul(d[i], h[3]), new(big.Rat).Mul(h[i], d[3]))
		derivative[i] = x.Quo(x, den)
		for _, q := range []*big.Rat{p[i], derivative[i]} {
			if err := b.check(q); err != nil {
				return result, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return RationalPeriodicCurveJet{Point: p, Derivative: derivative, Domain: owner.Domain, Cycle: owner.Cycle, Span: owner.Span,
		PointBounds: rationalSurfaceVectorBounds(p), DerivativeBounds: rationalSurfaceVectorBounds(derivative)}, nil
}

func periodicSpanAtPosition(count int, start, period float64, position *big.Rat, b *polynomialBudget) (ExactSurfaceKnotSpan, *big.Rat, error) {
	origin, step := exactUniformPeriodicAxis(count, start, period)
	station := new(big.Rat).Quo(new(big.Rat).Sub(position, origin), step)
	if err := b.check(station); err != nil {
		return ExactSurfaceKnotSpan{}, nil, err
	}
	index := new(big.Int).Div(station.Num(), station.Denom())
	cycle, offset := new(big.Int), new(big.Int)
	cycle.DivMod(index, big.NewInt(int64(count)), offset)
	global := new(big.Rat).SetInt(index)
	left := new(big.Rat).Add(origin, new(big.Rat).Mul(step, global))
	right := new(big.Rat).Add(left, step)
	return ExactSurfaceKnotSpan{Domain: ExactParameterInterval{Lower: left, Upper: right}, Cycle: cycle, Span: int(offset.Int64())},
		new(big.Rat).Sub(station, global), nil
}

// Differentiate the same uniform de Boor recurrence used to blend the
// active homogeneous poles. The two triangular arrays have one entry per
// active pole, regardless of the number of spans in the original curve.
func (c *PeriodicBSpline) periodicHomogeneousJet(ctx context.Context, span int, local *big.Rat,
	b *polynomialBudget) (value, derivative exactHomogeneous, err error) {
	p := c.degree
	controls := make([]exactHomogeneous, p+1)
	for i := range controls {
		pole := c.curve.poles[span+i]
		w := new(big.Rat).SetFloat64(c.curve.weights[span+i])
		controls[i][3] = w
		for axis := 0; axis < 3; axis++ {
			controls[i][axis] = new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(pole, axis)), w)
		}
	}
	return differentiatePeriodicNet(ctx, controls, func(j, r int) (a, slope *big.Rat) {
		slope = big.NewRat(1, int64(p-r+1))
		a = new(big.Rat).Add(local, big.NewRat(int64(p-j), 1))
		a.Mul(a, slope)
		return a, slope
	}, b)
}

func differentiatePeriodicNet(ctx context.Context, controls []exactHomogeneous, blend func(j, r int) (a, slope *big.Rat),
	b *polynomialBudget) (value, derivative exactHomogeneous, err error) {
	p := len(controls) - 1
	jets := make([]exactHomogeneous, len(controls))
	for i, h := range controls {
		for axis, q := range h {
			if err := b.check(q); err != nil {
				return value, derivative, err
			}
			jets[i][axis] = new(big.Rat)
		}
	}
	for r := 1; r <= p; r++ {
		if err := ctx.Err(); err != nil {
			return value, derivative, err
		}
		for j := p; j >= r; j-- {
			a, slope := blend(j, r)
			q := new(big.Rat).Sub(big.NewRat(1, 1), a)
			for _, alpha := range []*big.Rat{a, q, slope} {
				if err := b.check(alpha); err != nil {
					return value, derivative, err
				}
			}
			for axis := 0; axis < 4; axis++ {
				left, right := controls[j-1][axis], controls[j][axis]
				h := new(big.Rat).Add(new(big.Rat).Mul(q, left), new(big.Rat).Mul(a, right))
				d := new(big.Rat).Sub(right, left)
				d.Mul(d, slope).Add(d, new(big.Rat).Mul(q, jets[j-1][axis])).Add(d, new(big.Rat).Mul(a, jets[j][axis]))
				for _, x := range []*big.Rat{h, d} {
					if err := b.check(x); err != nil {
						return value, derivative, err
					}
				}
				controls[j][axis], jets[j][axis] = h, d
			}
		}
	}
	return controls[p], jets[p], nil
}
