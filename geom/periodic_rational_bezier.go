package geom

import (
	"context"
	"math/big"
)

// ExactBezierSpans extracts exact homogeneous pieces from the edited
// cyclic basis. A piece keeps its physical range and the actual cycle and
// knot span. Repeated knots do not produce empty pieces. A failed or
// cancelled extraction never returns a partial list.
func (c *ExactPeriodicSplineCurve) ExactBezierSpans(ctx context.Context, start, end *big.Rat,
	options ExactBezierSpanOptions) (spans []ExactPeriodicBezierSpan, operations int, err error) {
	if ctx == nil || c == nil || len(c.net) == 0 || options.MaxSpans < 0 {
		return nil, 0, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	o, err := patchOptions(options.SurfacePatchOptions)
	if err != nil {
		return nil, 0, err
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	a := exactTensorSplineAxis{c.exactCyclicAxis, len(c.net)}
	limit := defaultNonuniformWindowLimit(options.MaxSpans, o.MaxPoles, a.degree+1)
	windows, err := a.bezierWindows(ctx, ExactParameterInterval{Lower: start, Upper: end}, limit, b)
	if err != nil {
		return nil, 0, err
	}
	spans = make([]ExactPeriodicBezierSpan, 0, len(windows))
	active := make([][4]*big.Rat, a.degree+1)
	for _, w := range windows {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		for i := range active {
			active[i] = c.net[(w.owner.Span+i)%a.count]
			if err := checkRationalSplinePole(active[i], b); err != nil {
				return nil, 0, err
			}
		}
		p := ExactPeriodicBezierSpan{Start: w.owner.Domain.Lower, End: w.owner.Domain.Upper,
			Cycle: w.owner.Cycle, Span: w.owner.Span, Control: make([][4]*big.Rat, len(active))}
		for i := range p.Control {
			p.Control[i], err = exactWeightedHomogeneous(active, 1, w.weights[i], b)
			if err != nil {
				return nil, 0, err
			}
		}
		spans = append(spans, p)
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return spans, 0, nil
}

// ExactBezierCells extracts every nonempty clipped knot rectangle of the
// edited tensor. It preserves exact nonuniform stations, cycle IDs and
// one-sided ordinary discontinuities. Each cell is independently owned.
func (s *ExactPeriodicSplineSurface) ExactBezierCells(ctx context.Context, u, v ExactParameterInterval,
	options PeriodicSurfaceBezierOptions) (cells []ExactPeriodicSurfaceBezierCell, operations int, err error) {
	if ctx == nil || s == nil || len(s.net) == 0 || options.MaxCells < 0 {
		return nil, 0, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	o, err := patchOptions(options.SurfacePatchOptions)
	if err != nil {
		return nil, 0, err
	}
	a, c := s.axis[0], s.axis[1]
	nu, nv := a.degree+1, c.degree+1
	limit := defaultNonuniformWindowLimit(options.MaxCells, o.MaxPoles, nu*nv)
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	us, err := a.bezierWindows(ctx, u, limit, b)
	if err != nil {
		return nil, 0, err
	}
	vs, err := c.bezierWindows(ctx, v, limit/len(us), b)
	if err != nil {
		return nil, 0, err
	}
	cells = make([]ExactPeriodicSurfaceBezierCell, 0, len(us)*len(vs))
	active := make([][4]*big.Rat, nu*nv)
	for _, uw := range us {
		for _, vw := range vs {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			for i := range nu {
				row := ((uw.owner.Span + i) % a.count) * c.count
				for j := range nv {
					active[i*nv+j] = s.net[row+(vw.owner.Span+j)%c.count]
					if err := checkRationalSplinePole(active[i*nv+j], b); err != nil {
						return nil, 0, err
					}
				}
			}
			net, err := exactTensorBernsteinNet(ctx, active, uw, vw, b)
			if err != nil {
				return nil, 0, err
			}
			cells = append(cells, ExactPeriodicSurfaceBezierCell{U: cloneSurfaceKnotSpan(uw.owner), V: cloneSurfaceKnotSpan(vw.owner),
				DegreeU: a.degree, DegreeV: c.degree, Control: net})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return cells, 0, nil
}

func defaultNonuniformWindowLimit(limit, poles, polesPerWindow int) int {
	if limit == 0 {
		limit = 256
	}
	return min(limit, poles/polesPerWindow)
}

func checkRationalSplinePole(p [4]*big.Rat, b *polynomialBudget) error {
	for _, v := range p {
		if err := b.check(v); err != nil {
			return err
		}
	}
	return nil
}

func (a exactTensorSplineAxis) bezierWindows(ctx context.Context, window ExactParameterInterval, limit int, b *polynomialBudget) ([]periodicTensorWindow, error) {
	if window.Lower == nil || window.Upper == nil || window.Lower.Cmp(window.Upper) >= 0 {
		return nil, ErrInvalid
	}
	for _, t := range []*big.Rat{window.Lower, window.Upper} {
		if err := b.check(t); err != nil {
			return nil, err
		}
	}
	if a.period == nil && (window.Lower.Cmp(a.knots[a.degree]) < 0 || window.Upper.Cmp(a.knots[a.count]) > 0) {
		return nil, ErrDomain
	}
	var out []periodicTensorWindow
	for t := window.Lower; t.Cmp(window.Upper) < 0; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(out) >= limit {
			return nil, ErrConvergence
		}
		owner, local, knot, err := a.selectSpan(t, b)
		if err != nil {
			return nil, err
		}
		end := owner.Domain.Upper
		if end.Cmp(window.Upper) > 0 {
			end = window.Upper
		}
		localEnd := new(big.Rat).Add(local, new(big.Rat).Sub(end, t))
		if err := b.check(localEnd); err != nil {
			return nil, err
		}
		weights, err := rationalAxisBernsteinWeights(ctx, knot, a.degree, owner.Span, local, localEnd, b)
		if err != nil {
			return nil, err
		}
		owner.Domain = ExactParameterInterval{Lower: new(big.Rat).Set(t), Upper: new(big.Rat).Set(end)}
		out = append(out, periodicTensorWindow{owner: owner, weights: weights})
		t = end
	}
	return out, nil
}
