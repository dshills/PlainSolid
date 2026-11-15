package geom

import (
	"context"
	"math/big"
)

// ExactPeriodicBezierSpan describes one restricted piece of the original
// cyclic control net. Each piece's Bézier parameter runs from zero at Start
// to one at End, even if a requested endpoint clips the original span.
// Cycle counts periods from the original start, and Span is the zero-based
// knot span within that period. All values are owned by the result.
type ExactPeriodicBezierSpan struct {
	Start, End *big.Rat
	Cycle      *big.Int
	Span       int
	Control    [][4]*big.Rat
}

type periodicSpanWindow struct {
	start, end, localStart, localEnd *big.Rat
	cycle                            *big.Int
	span                             int
}

// ExactBezierSpans extracts the exact homogeneous pieces of a uniform
// periodic curve over a nonempty rational parameter interval. The physical
// knot lattice uses the represented start and period as exact coefficients.
// The result can cross seams and includes nonbinary knot positions. Unlike
// the float64 evaluator, the exact interval uses start+period without
// rounding the seam. MaxSpans and MaxPoles bound the number of pieces and
// total returned controls. On failure no partial list is returned.
func (c *PeriodicBSpline) ExactBezierSpans(ctx context.Context, start, end *big.Rat, options ExactBezierSpanOptions) (spans []ExactPeriodicBezierSpan, operations int, err error) {
	o, err := patchOptions(options.SurfacePatchOptions)
	if err != nil {
		return nil, 0, err
	}
	if ctx == nil || c == nil || c.curve == nil || start == nil || end == nil || options.MaxSpans < 0 {
		return nil, 0, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return nil, 0, err
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = budget.operations }()
	limit := options.MaxSpans
	if limit == 0 {
		limit = 256
	}
	windows, err := uniformPeriodicSpanWindows(ctx, len(c.poles), c.start, c.period, start, end, min(limit, o.MaxPoles/(c.degree+1)), budget)
	if err != nil {
		return nil, 0, err
	}
	spans = make([]ExactPeriodicBezierSpan, 0, len(windows))
	for _, window := range windows {
		controls, err := c.periodicBezierControls(ctx, window.span, budget)
		if err != nil {
			return nil, 0, err
		}
		controls, err = restrictExactBezier(controls, window.localStart, window.localEnd, budget)
		if err != nil {
			return nil, 0, err
		}
		piece := ExactPeriodicBezierSpan{Start: window.start, End: window.end, Cycle: window.cycle, Span: window.span, Control: make([][4]*big.Rat, len(controls))}
		for j, control := range controls {
			for axis, v := range control {
				piece.Control[j][axis] = new(big.Rat).Set(v)
			}
		}
		spans = append(spans, piece)
	}
	return spans, 0, nil
}

func uniformPeriodicSpanWindows(ctx context.Context, poleCount int, start, period float64, lower, upper *big.Rat, limit int, budget *polynomialBudget) ([]periodicSpanWindow, error) {
	for _, value := range []*big.Rat{lower, upper} {
		if err := budget.check(value); err != nil {
			return nil, err
		}
	}
	if lower.Cmp(upper) >= 0 {
		return nil, ErrInvalid
	}
	n := int64(poleCount)
	origin, width := exactUniformPeriodicAxis(poleCount, start, period)
	lo := new(big.Rat).Quo(new(big.Rat).Sub(lower, origin), width)
	hi := new(big.Rat).Quo(new(big.Rat).Sub(upper, origin), width)
	for _, value := range []*big.Rat{origin, width, lo, hi} {
		if err := budget.check(value); err != nil {
			return nil, err
		}
	}
	first := new(big.Int).Div(lo.Num(), lo.Denom())
	stop := new(big.Int).Neg(new(big.Int).Div(new(big.Int).Neg(hi.Num()), hi.Denom()))
	length := new(big.Int).Sub(stop, first)
	if length.Cmp(big.NewInt(int64(limit))) > 0 {
		return nil, ErrConvergence
	}
	windows := make([]periodicSpanWindow, 0, int(length.Int64()))
	count, one := big.NewInt(n), big.NewInt(1)
	for i := new(big.Int).Set(first); i.Cmp(stop) < 0; i.Add(i, one) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cycle, offset := new(big.Int), new(big.Int)
		cycle.DivMod(i, count, offset)
		index := int(offset.Int64())
		left := new(big.Rat).Add(origin, new(big.Rat).Mul(width, new(big.Rat).SetInt(i)))
		right := new(big.Rat).Add(left, width)
		a, b := left, right
		if lower.Cmp(a) > 0 {
			a = new(big.Rat).Set(lower)
		}
		if upper.Cmp(b) < 0 {
			b = new(big.Rat).Set(upper)
		}
		localStart := new(big.Rat).Quo(new(big.Rat).Sub(a, left), width)
		localEnd := new(big.Rat).Quo(new(big.Rat).Sub(b, left), width)
		for _, value := range []*big.Rat{left, right, localStart, localEnd} {
			if err := budget.check(value); err != nil {
				return nil, err
			}
		}
		windows = append(windows, periodicSpanWindow{start: a, end: b, localStart: localStart, localEnd: localEnd, cycle: cycle, span: index})
	}
	return windows, nil
}

func exactUniformPeriodicAxis(count int, start, period float64) (origin, step *big.Rat) {
	return new(big.Rat).SetFloat64(start), new(big.Rat).Quo(new(big.Rat).SetFloat64(period), big.NewRat(int64(count), 1))
}

func (c *PeriodicBSpline) periodicBezierControls(ctx context.Context, span int, budget *polynomialBudget) ([]exactHomogeneous, error) {
	p := c.degree
	active := make([]exactHomogeneous, p+1)
	for j := range active {
		pole := c.curve.poles[span+j]
		weight := new(big.Rat).SetFloat64(c.curve.weights[span+j])
		active[j][3] = weight
		for axis := 0; axis < 3; axis++ {
			active[j][axis] = new(big.Rat).Mul(weight, new(big.Rat).SetFloat64(coordinate(pole, axis)))
		}
		for _, value := range active[j] {
			if err := budget.check(value); err != nil {
				return nil, err
			}
		}
	}
	controls := make([]exactHomogeneous, p+1)
	work := make([]exactHomogeneous, p+1)
	for right := range controls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		copy(work, active)
		// Control right is the blossom with p-right left arguments and
		// right right arguments. The source is an integer knot lattice,
		// so the blend fractions are independent of the span's index.
		for r := 1; r <= p; r++ {
			arg := 0
			if r > p-right {
				arg = 1
			}
			for j := p; j >= r; j-- {
				alpha := big.NewRat(int64(p-j+arg), int64(p-r+1))
				beta := new(big.Rat).Sub(big.NewRat(1, 1), alpha)
				for axis := 0; axis < 4; axis++ {
					value := new(big.Rat).Mul(beta, work[j-1][axis])
					value.Add(value, new(big.Rat).Mul(alpha, work[j][axis]))
					if err := budget.check(value); err != nil {
						return nil, err
					}
					work[j][axis] = value
				}
			}
		}
		controls[right] = work[p]
	}
	return controls, nil
}
