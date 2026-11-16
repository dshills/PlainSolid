package geom

import (
	"context"
	"math/big"
)

// ElevateDegree raises the exact cyclic degree by one without changing
// the physical curve. Each original knot multiplicity increases by one,
// preserving its original continuity requirement, including at the seam.
func (c *ExactPeriodicSplineCurve) ElevateDegree(ctx context.Context, o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
	if ctx == nil || c == nil || len(c.net) == 0 {
		return nil, ErrInvalid
	}
	o, b, err := periodicDegreeBudget(ctx, o, len(c.net))
	if err != nil {
		return nil, err
	}
	a := exactTensorSplineAxis{c.exactCyclicAxis, len(c.net)}
	plan, err := a.elevation(ctx, o.MaxPoles, b)
	if err != nil {
		return nil, err
	}
	net, err := plan.elevate(ctx, c.net, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineCurve{plan.axis, net}, nil
}

// ElevatePeriodicDegree raises one cyclic tensor direction by one. The
// exact full tensor, original knot stations and physical partials remain
// unchanged. Direction 0 is U and 1 is V.
func (s *ExactPeriodicSplineSurface) ElevatePeriodicDegree(ctx context.Context, direction int, o SurfacePatchOptions) (*ExactPeriodicSplineSurface, error) {
	if ctx == nil || s == nil || len(s.net) == 0 || direction < 0 || direction > 1 {
		return nil, ErrInvalid
	}
	a, other := s.axis[direction], s.axis[1-direction]
	if a.period == nil {
		return nil, ErrDomain
	}
	o, b, err := periodicDegreeBudget(ctx, o, len(s.net))
	if err != nil {
		return nil, err
	}
	plan, err := a.elevation(ctx, o.MaxPoles/other.count, b)
	if err != nil {
		return nil, err
	}
	axes := s.axis
	axes[direction] = exactTensorSplineAxis{plan.axis, len(plan.axis.knots) - 1}
	net := make([]exactHomogeneous, axes[0].count*axes[1].count)
	row := make([]exactHomogeneous, a.count)
	for i := range other.count {
		for j := range row {
			if direction == 0 {
				row[j] = s.net[j*other.count+i]
			} else {
				row[j] = s.net[i*a.count+j]
			}
		}
		newRow, err := plan.elevate(ctx, row, b)
		if err != nil {
			return nil, err
		}
		for j, h := range newRow {
			if direction == 0 {
				net[j*axes[1].count+i] = h
			} else {
				net[i*axes[1].count+j] = h
			}
		}
	}
	return &ExactPeriodicSplineSurface{axes, net}, nil
}

func periodicDegreeBudget(ctx context.Context, o SurfacePatchOptions, count int) (SurfacePatchOptions, *polynomialBudget, error) {
	if err := ctx.Err(); err != nil {
		return o, nil, err
	}
	o, err := patchOptions(o)
	if err != nil {
		return o, nil, err
	}
	if count > o.MaxPoles || count > o.MaxOperations/4 {
		return o, nil, ErrConvergence
	}
	return o, &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}, nil
}

type cyclicElevation struct {
	windows  []periodicTensorWindow
	removals []cyclicKnotRemoval
	axis     exactCyclicAxis
	degree   int
}

func (a exactTensorSplineAxis) elevation(ctx context.Context, maxPoles int, b *polynomialBudget) (cyclicElevation, error) {
	var e cyclicElevation
	if a.degree >= 64 {
		return e, ErrDomain
	}
	q := a.degree + 1
	windows, err := a.bezierWindows(ctx, ExactParameterInterval{Lower: a.knots[0], Upper: a.knots[a.count]}, maxPoles/q, b)
	if err != nil {
		return e, err
	}
	knots := make([]*big.Rat, 0, q*len(windows)+1)
	for i := range windows {
		w := &windows[i]
		station := w.owner.Domain.Lower
		for range q {
			knots = append(knots, station)
		}
		old := w.weights
		w.weights = make([][]*big.Rat, q+1)
		w.weights[0], w.weights[q] = old[0], old[q-1]
		for j := 1; j < q; j++ {
			alpha, beta := big.NewRat(int64(j), int64(q)), big.NewRat(int64(q-j), int64(q))
			row := make([]*big.Rat, q)
			for k := range row {
				v := new(big.Rat).Add(new(big.Rat).Mul(alpha, old[j-1][k]), new(big.Rat).Mul(beta, old[j][k]))
				if err := b.check(v); err != nil {
					return e, err
				}
				row[k] = v
			}
			w.weights[j] = row
		}
	}
	knots = append(knots, a.knots[a.count])
	axis, err := newExactCyclicAxis(ctx, q, knots, b)
	if err != nil {
		return e, err
	}
	// In the initial elevated Bézier basis every knot has degree
	// multiplicity. Prove each redundant copy can be removed so that
	// the final degree-minus-multiplicity equals the original one.
	var removals []cyclicKnotRemoval
	previous := -1
	for _, w := range windows {
		desired := w.owner.Span - previous + 1
		previous = w.owner.Span
		for j := desired; j < q; j++ {
			p, err := axis.removal(ctx, w.owner.Domain.Lower, b)
			if err != nil {
				return e, err
			}
			removals = append(removals, p)
			axis = p.axis
		}
	}
	return cyclicElevation{windows, removals, axis, q}, nil
}

func (e cyclicElevation) elevate(ctx context.Context, source []exactHomogeneous, b *polynomialBudget) ([]exactHomogeneous, error) {
	q := e.degree
	net := make([]exactHomogeneous, q*len(e.windows))
	active := make([][4]*big.Rat, q)
	for i, w := range e.windows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j := range active {
			active[j] = source[(w.owner.Span+j)%len(source)]
			if err := checkRationalSplinePole(active[j], b); err != nil {
				return nil, err
			}
		}
		for j, weights := range w.weights {
			h, err := exactWeightedHomogeneous(active, 1, weights, b)
			if err != nil {
				return nil, err
			}
			if h[3].Sign() <= 0 {
				return nil, ErrConvergence
			}
			index := ((i+1)*q - 1 + j) % len(net)
			if net[index][3] != nil {
				if err := checkKnotRemovalPole(net[index], h, b); err != nil {
					return nil, err
				}
			} else {
				net[index] = h
			}
		}
	}
	for _, r := range e.removals {
		var err error
		net, err = r.recover(ctx, net, b)
		if err != nil {
			return nil, err
		}
	}
	return net, nil
}
