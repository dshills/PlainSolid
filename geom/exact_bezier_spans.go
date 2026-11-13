package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactBezierSpan contains independent homogeneous (w*x,w*y,w*z,w)
// coefficients in ascending parameter order. Coefficients are owned by the
// result; modifying them does not affect the original curve.
type ExactBezierSpan struct {
	Start, End float64
	Control    [][4]*big.Rat
}

// ExactBezierSpanOptions bounds extraction across the whole requested range.
// MaxSpans defaults to 256. SurfacePatchOptions also bound temporary knot
// refinement, with two auxiliary rows per actual curve pole.
type ExactBezierSpanOptions struct {
	SurfacePatchOptions
	MaxSpans int
}

// ExactBezierSpans extracts each nonempty span of a continuous clamped
// positive-weight curve over [start,end], clipping its first and last spans
// when necessary. Homogeneous knot insertion preserves the original geometry
// even when the resulting poles cannot be represented as float64. Nothing is
// returned unless all spans have been extracted.
func (c *BSpline) ExactBezierSpans(ctx context.Context, start, end float64, options ExactBezierSpanOptions) ([]ExactBezierSpan, int, error) {
	o, e := patchOptions(options.SurfacePatchOptions)
	if e != nil {
		return nil, 0, e
	}
	if ctx == nil || c == nil || len(c.poles) == 0 || !finite(start) || !finite(end) || start >= end || options.MaxSpans < 0 {
		return nil, 0, ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return nil, 0, e
	}
	a, b := c.Domain()
	if start < a || end > b {
		return nil, 0, ErrDomain
	}
	if len(c.poles) > o.MaxPoles/2 {
		return nil, 0, ErrConvergence
	}
	limit := options.MaxSpans
	if limit == 0 {
		limit = 256
	}
	var cuts []float64
	for i := sort.Search(len(c.knots), func(i int) bool { return c.knots[i] > start }); i < len(c.knots) && c.knots[i] < end; i++ {
		k := c.knots[i]
		if len(cuts) == 0 || k != cuts[len(cuts)-1] {
			if len(cuts) == limit-1 {
				return nil, 0, ErrConvergence
			}
			cuts = append(cuts, k)
		}
	}
	exact, e := NewExactSplineCurve(c, o)
	if e != nil {
		return nil, 0, e
	}
	p := exact.patch
	budget := &polynomialBudget{operations: exact.operations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if start > a {
		_, p, e = p.split(start, true, budget)
		if e != nil {
			return nil, budget.operations, e
		}
	}
	if end < b {
		p, _, e = p.split(end, true, budget)
		if e != nil {
			return nil, budget.operations, e
		}
	}
	spans := make([]ExactBezierSpan, 0, len(cuts)+1)
	var visit func(*SurfacePatch, []float64) error
	visit = func(p *SurfacePatch, cuts []float64) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if len(cuts) != 0 {
			i := len(cuts) / 2
			left, right, e := p.split(cuts[i], true, budget)
			if e != nil {
				return e
			}
			if e = visit(left, cuts[:i]); e != nil {
				return e
			}
			return visit(right, cuts[i+1:])
		}
		if p.countU != c.degree+1 {
			return ErrInvalid
		}
		d := p.Domain()
		s := ExactBezierSpan{Start: d.UMin, End: d.UMax, Control: make([][4]*big.Rat, p.countU)}
		for i := range s.Control {
			for j, v := range p.net[i*p.countV] {
				if e := budget.check(v); e != nil {
					return e
				}
				s.Control[i][j] = new(big.Rat).Set(v)
			}
		}
		spans = append(spans, s)
		return nil
	}
	if e = visit(p, cuts); e != nil {
		return nil, budget.operations, e
	}
	return spans, budget.operations, nil
}
