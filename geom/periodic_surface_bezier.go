package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactSurfaceKnotSpan retains a rational physical interval and its original
// owner. Cycle is nil on an ordinary spline axis. Span is the zero-based
// knot interval within a fundamental domain, counting repeated knot slots.
type ExactSurfaceKnotSpan struct {
	Domain ExactParameterInterval
	Cycle  *big.Int
	Span   int
}

// ExactPeriodicSurfaceBezierCell contains a tensor of original homogeneous
// controls after exact restriction. Its Bernstein coordinates increase from
// zero to one along each physical interval. Controls use u-major order.
type ExactPeriodicSurfaceBezierCell struct {
	U, V             ExactSurfaceKnotSpan
	DegreeU, DegreeV int
	Control          [][4]*big.Rat
}

// NormalizedPatch adapts the exact cell to the ordinary exact surface engine.
// The caller can recover physical parameters from the cell's knot intervals.
func (c ExactPeriodicSurfaceBezierCell) NormalizedPatch(ctx context.Context, o SurfacePatchOptions) (*SurfacePatch, error) {
	return NewExactHomogeneousBezierSurface(ctx, c.DegreeU, c.DegreeV, c.Control, o)
}

// PeriodicSurfaceBezierOptions bounds work and the total number of controls.
// MaxCells defaults to 256 and includes cells across every selected cycle.
type PeriodicSurfaceBezierOptions struct {
	SurfacePatchOptions
	MaxCells int
}

type periodicTensorWindow struct {
	owner   ExactSurfaceKnotSpan
	weights [][]*big.Rat
}

// ExactBezierCells extracts all knot cells intersecting a nonempty rational
// rectangle. Periodic axes use the exact lattice from the represented period
// and start. An ordinary axis uses its represented source knot vector. Every
// returned rational is independently owned. Failure discards the whole grid.
func (s *PeriodicBSplineSurface) ExactBezierCells(ctx context.Context, u, v ExactParameterInterval, options PeriodicSurfaceBezierOptions) (cells []ExactPeriodicSurfaceBezierCell, operations int, err error) {
	o, err := patchOptions(options.SurfacePatchOptions)
	if err != nil {
		return nil, 0, err
	}
	if ctx == nil || s == nil || s.surface == nil || options.MaxCells < 0 {
		return nil, 0, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return nil, 0, err
	}
	limit := options.MaxCells
	if limit == 0 {
		limit = 256
	}
	degreeU, degreeV := s.surface.degreeU, s.surface.degreeV
	limit = min(limit, o.MaxPoles/((degreeU+1)*(degreeV+1)))
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = budget.operations }()
	uWindows, err := s.exactTensorAxisWindows(ctx, 0, u, limit, budget)
	if err != nil {
		return nil, 0, err
	}
	vWindows, err := s.exactTensorAxisWindows(ctx, 1, v, limit/len(uWindows), budget)
	if err != nil {
		return nil, 0, err
	}
	cells = make([]ExactPeriodicSurfaceBezierCell, 0, len(uWindows)*len(vWindows))
	for _, uw := range uWindows {
		for _, vw := range vWindows {
			controls, err := s.exactTensorWindowControls(ctx, uw, vw, budget)
			if err != nil {
				return nil, 0, err
			}
			cells = append(cells, ExactPeriodicSurfaceBezierCell{
				U: cloneSurfaceKnotSpan(uw.owner), V: cloneSurfaceKnotSpan(vw.owner),
				DegreeU: degreeU, DegreeV: degreeV, Control: controls,
			})
		}
	}
	return cells, 0, nil
}

func cloneSurfaceKnotSpan(span ExactSurfaceKnotSpan) ExactSurfaceKnotSpan {
	span.Domain = ExactParameterInterval{Lower: new(big.Rat).Set(span.Domain.Lower), Upper: new(big.Rat).Set(span.Domain.Upper)}
	if span.Cycle != nil {
		span.Cycle = new(big.Int).Set(span.Cycle)
	}
	return span
}

func (s *PeriodicBSplineSurface) exactTensorAxisWindows(ctx context.Context, direction int, window ExactParameterInterval, limit int, b *polynomialBudget) ([]periodicTensorWindow, error) {
	if window.Lower == nil || window.Upper == nil {
		return nil, ErrInvalid
	}
	axis := s.axis[direction]
	degree, count, knots := s.tensorSplineAxis(direction)
	if axis.Period != 0 {
		pieces, err := uniformPeriodicSpanWindows(ctx, s.count[direction], axis.Start, axis.Period, window.Lower, window.Upper, limit, b)
		if err != nil {
			return nil, err
		}
		out := make([]periodicTensorWindow, 0, len(pieces))
		for _, piece := range pieces {
			start := big.NewRat(int64(piece.span), 1)
			lower := new(big.Rat).Add(start, piece.localStart)
			upper := new(big.Rat).Add(start, piece.localEnd)
			weights, err := exactAxisBernsteinWeights(ctx, knots, degree, piece.span+degree, lower, upper, b)
			if err != nil {
				return nil, err
			}
			out = append(out, periodicTensorWindow{owner: ExactSurfaceKnotSpan{
				Domain: ExactParameterInterval{Lower: piece.start, Upper: piece.end}, Cycle: piece.cycle, Span: piece.span,
			}, weights: weights})
		}
		return out, nil
	}
	if window.Lower.Cmp(window.Upper) >= 0 {
		return nil, ErrInvalid
	}
	for _, endpoint := range []*big.Rat{window.Lower, window.Upper} {
		if err := b.check(endpoint); err != nil {
			return nil, err
		}
	}
	knot := func(i int) *big.Rat { return new(big.Rat).SetFloat64(knots[i]) }
	if window.Lower.Cmp(knot(degree)) < 0 || window.Upper.Cmp(knot(count)) > 0 {
		return nil, ErrDomain
	}
	first := sort.Search(count-degree, func(i int) bool { return knot(degree+i+1).Cmp(window.Lower) > 0 }) + degree
	var out []periodicTensorWindow
	for span := first; span < count; span++ {
		lower, upper := knot(span), knot(span+1)
		if lower.Cmp(window.Upper) >= 0 {
			break
		}
		if lower.Cmp(upper) == 0 {
			continue
		}
		if len(out) >= limit {
			return nil, ErrConvergence
		}
		if lower.Cmp(window.Lower) < 0 {
			lower.Set(window.Lower)
		}
		if upper.Cmp(window.Upper) > 0 {
			upper.Set(window.Upper)
		}
		weights, err := exactAxisBernsteinWeights(ctx, knots, degree, span, lower, upper, b)
		if err != nil {
			return nil, err
		}
		out = append(out, periodicTensorWindow{owner: ExactSurfaceKnotSpan{
			Domain: ExactParameterInterval{Lower: lower, Upper: upper}, Span: span - degree,
		}, weights: weights})
	}
	return out, nil
}

func (s *PeriodicBSplineSurface) tensorSplineAxis(direction int) (degree, count int, knots []float64) {
	if direction == 0 {
		return s.surface.degreeU, s.surface.countU, s.surface.knotsU
	}
	return s.surface.degreeV, s.surface.countV, s.surface.knotsV
}

// Back-propagate each blossom evaluation through the de Boor blends. Each
// row gives the exact linear combination of the active controls for one
// output Bernstein pole. Using the clipped arguments applies restriction
// directly, avoiding a second tensor split.
func exactAxisBernsteinWeights(ctx context.Context, knots []float64, degree, span int, lower, upper *big.Rat, b *polynomialBudget) ([][]*big.Rat, error) {
	rows := make([][]*big.Rat, degree+1)
	one := big.NewRat(1, 1)
	for right := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row := make([]*big.Rat, degree+1)
		for i := range row {
			row[i] = new(big.Rat)
		}
		row[degree].Set(one)
		for level := degree; level >= 1; level-- {
			argument := lower
			if level > degree-right {
				argument = upper
			}
			for j := level; j <= degree; j++ {
				lo := new(big.Rat).SetFloat64(knots[span-degree+j])
				hi := new(big.Rat).SetFloat64(knots[span+j-level+1])
				alpha := new(big.Rat).Quo(new(big.Rat).Sub(argument, lo), new(big.Rat).Sub(hi, lo))
				left := new(big.Rat).Mul(new(big.Rat).Sub(one, alpha), row[j])
				row[j-1].Add(row[j-1], left)
				row[j].Mul(row[j], alpha)
				for _, coefficient := range []*big.Rat{alpha, row[j-1], row[j]} {
					if err := b.check(coefficient); err != nil {
						return nil, err
					}
				}
			}
		}
		rows[right] = row
	}
	return rows, nil
}

func (s *PeriodicBSplineSurface) exactTensorWindowControls(ctx context.Context, u, v periodicTensorWindow, b *polynomialBudget) ([][4]*big.Rat, error) {
	nu, nv := len(u.weights), len(v.weights)
	active := make([][4]*big.Rat, nu*nv)
	for i := 0; i < nu; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j := 0; j < nv; j++ {
			id := (u.owner.Span+i)*s.surface.countV + v.owner.Span + j
			p := s.surface.poles[id]
			w := new(big.Rat).SetFloat64(s.surface.weights[id])
			h := &active[i*nv+j]
			h[3] = w
			for axis := 0; axis < 3; axis++ {
				h[axis] = new(big.Rat).Mul(w, new(big.Rat).SetFloat64(coordinate(p, axis)))
			}
			for _, value := range h {
				if err := b.check(value); err != nil {
					return nil, err
				}
			}
		}
	}
	intermediate, controls := make([][4]*big.Rat, nu*nv), make([][4]*big.Rat, nu*nv)
	for i, row := range u.weights {
		for j := 0; j < nv; j++ {
			point, err := exactWeightedHomogeneous(active[j:], nv, row, b)
			if err != nil {
				return nil, err
			}
			intermediate[i*nv+j] = point
		}
	}
	for i := 0; i < nu; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j, row := range v.weights {
			point, err := exactWeightedHomogeneous(intermediate[i*nv:], 1, row, b)
			if err != nil {
				return nil, err
			}
			controls[i*nv+j] = point
		}
	}
	return controls, nil
}

func exactWeightedHomogeneous(controls [][4]*big.Rat, stride int, weights []*big.Rat, b *polynomialBudget) (out [4]*big.Rat, err error) {
	for axis := range out {
		out[axis] = new(big.Rat)
	}
	var term big.Rat
	for i, weight := range weights {
		if weight.Sign() == 0 {
			continue
		}
		for axis, value := range controls[i*stride] {
			out[axis].Add(out[axis], term.Mul(weight, value))
			if err := b.check(out[axis]); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
