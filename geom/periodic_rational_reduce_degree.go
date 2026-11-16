package geom

import (
	"context"
	"math/big"
)

// ReduceDegreeExact lowers the cyclic degree by one only if all original
// homogeneous pieces have an exact positive-weight lower-degree net. It
// keeps every distinct original station and removes at most one knot copy
// per station. A curve outside this lower-degree space is unrepresentable.
func (c *ExactPeriodicSplineCurve) ReduceDegreeExact(ctx context.Context, o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
	if ctx == nil || c == nil || len(c.net) == 0 {
		return nil, ErrInvalid
	}
	o, b, err := periodicDegreeBudget(ctx, o, len(c.net))
	if err != nil {
		return nil, err
	}
	plan, err := (exactTensorSplineAxis{c.exactCyclicAxis, len(c.net)}).degreeReduction(ctx, o.MaxPoles, b)
	if err != nil {
		return nil, err
	}
	net, err := plan.reduce(ctx, c.net, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineCurve{plan.axis, net}, nil
}

// ReducePeriodicDegreeExact lowers U (direction 0) or V (direction 1)
// when every original homogeneous row admits the exact reduction.
func (s *ExactPeriodicSplineSurface) ReducePeriodicDegreeExact(ctx context.Context, direction int, o SurfacePatchOptions) (*ExactPeriodicSplineSurface, error) {
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
	plan, err := a.degreeReduction(ctx, o.MaxPoles/other.count, b)
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
		reduced, err := plan.reduce(ctx, row, b)
		if err != nil {
			return nil, err
		}
		for j, h := range reduced {
			if direction == 0 {
				net[j*axes[1].count+i] = h
			} else {
				net[i*axes[1].count+j] = h
			}
		}
	}
	return &ExactPeriodicSplineSurface{axes, net}, nil
}

type cyclicDegreeReduction struct {
	windows  []periodicTensorWindow
	removals []cyclicKnotRemoval
	axis     exactCyclicAxis
}

func (a exactTensorSplineAxis) degreeReduction(ctx context.Context, maxPoles int, b *polynomialBudget) (cyclicDegreeReduction, error) {
	var r cyclicDegreeReduction
	if a.degree == 1 {
		return r, ErrDomain
	}
	q := a.degree - 1
	w, err := a.bezierWindows(ctx, ExactParameterInterval{Lower: a.knots[0], Upper: a.knots[a.count]}, maxPoles/q, b)
	if err != nil {
		return r, err
	}
	axis, removals, err := a.changedDegreeBasis(ctx, w, q, b)
	if err != nil {
		return r, err
	}
	return cyclicDegreeReduction{w, removals, axis}, nil
}

func (r cyclicDegreeReduction) reduce(ctx context.Context, source []exactHomogeneous, b *polynomialBudget) ([]exactHomogeneous, error) {
	q := r.axis.degree
	p := q + 1
	net := make([]exactHomogeneous, q*len(r.windows))
	active := make([][4]*big.Rat, p+1)
	for i, w := range r.windows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j := range active {
			active[j] = source[(w.owner.Span+j)%len(source)]
			if err := checkRationalSplinePole(active[j], b); err != nil {
				return nil, err
			}
		}
		var previous exactHomogeneous
		for j, weights := range w.weights {
			fine, err := exactWeightedHomogeneous(active, 1, weights, b)
			if err != nil {
				return nil, err
			}
			if j == p {
				// The final endpoint is an independent constraint on
				// the lower-degree net recovered from the other poles.
				if err := checkKnotRemovalPole(previous, fine, b); err != nil {
					return nil, err
				}
				break
			}
			// Invert the homogeneous degree-elevation equation:
			// P_j = (p Q_j − j P_{j−1}) / (p−j).
			if j > 0 {
				alpha, beta := big.NewRat(int64(p), int64(p-j)), big.NewRat(int64(j), int64(p-j))
				for d := range fine {
					fine[d].Sub(fine[d].Mul(fine[d], alpha), new(big.Rat).Mul(beta, previous[d]))
					if err := b.check(fine[d]); err != nil {
						return nil, err
					}
				}
			}
			if err := bindCyclicBezierPole(net, q, i, j, fine, b); err != nil {
				return nil, err
			}
			previous = fine
		}
	}
	return reduceCyclicBezierBasis(ctx, net, r.removals, b)
}
