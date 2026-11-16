package geom

import (
	"context"
	"math/big"
)

// RephaseAtKnot chooses t as the new fundamental seam. t must coincide
// with an existing knot in any original cycle. The exact physical curve
// is unchanged; knot span and cycle labels use the new origin.
func (c *ExactPeriodicSplineCurve) RephaseAtKnot(ctx context.Context, t *big.Rat, o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
	if ctx == nil || c == nil || len(c.net) == 0 || t == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := periodicRephaseBudget(o, len(c.net))
	if err != nil {
		return nil, err
	}
	axis, first, err := c.rephase(ctx, t, b)
	if err != nil {
		return nil, err
	}
	controls := make([][4]*big.Rat, len(c.net))
	for i := range controls {
		controls[i] = c.net[(first+i)%len(controls)]
	}
	net, err := ownedHomogeneousNet(ctx, controls, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineCurve{axis, net}, nil
}

// RephasePeriodicKnot changes the selected periodic seam to an existing
// physical knot. Direction 0 is U and 1 is V. The other basis, the exact
// surface and both physical partials remain unchanged.
func (s *ExactPeriodicSplineSurface) RephasePeriodicKnot(ctx context.Context, direction int, t *big.Rat,
	o SurfacePatchOptions) (*ExactPeriodicSplineSurface, error) {
	if ctx == nil || s == nil || len(s.net) == 0 || direction < 0 || direction > 1 || t == nil {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a := s.axis[direction]
	if a.period == nil {
		return nil, ErrDomain
	}
	b, err := periodicRephaseBudget(o, len(s.net))
	if err != nil {
		return nil, err
	}
	axis, first, err := a.rephase(ctx, t, b)
	if err != nil {
		return nil, err
	}
	axes := s.axis
	axes[direction] = exactTensorSplineAxis{axis, a.count}
	controls := make([][4]*big.Rat, len(s.net))
	for i := range controls {
		u, v := i/axes[1].count, i%axes[1].count
		if direction == 0 {
			u = (u + first) % axes[0].count
		} else {
			v = (v + first) % axes[1].count
		}
		controls[i] = s.net[u*axes[1].count+v]
	}
	net, err := ownedHomogeneousNet(ctx, controls, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineSurface{axes, net}, nil
}

func periodicRephaseBudget(o SurfacePatchOptions, count int) (*polynomialBudget, error) {
	o, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	if count > o.MaxPoles || count > o.MaxOperations/4 {
		return nil, ErrConvergence
	}
	return &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}, nil
}

func (a exactCyclicAxis) rephase(ctx context.Context, t *big.Rat, b *polynomialBudget) (exactCyclicAxis, int, error) {
	x, shift, _, s, err := a.locate(t, b)
	if err != nil {
		return exactCyclicAxis{}, 0, err
	}
	if x.Cmp(a.knots[s]) != 0 {
		return exactCyclicAxis{}, 0, ErrDomain
	}
	// Find the first copy of the new seam. The fundamental list holds
	// all equal starts and only the first knot at the closing endpoint.
	for s > 0 && a.knots[s-1].Cmp(x) == 0 {
		s--
	}
	knots := make([]*big.Rat, len(a.knots))
	for i := range knots {
		if err := ctx.Err(); err != nil {
			return exactCyclicAxis{}, 0, err
		}
		knots[i] = new(big.Rat).Add(a.knot(s+i), shift)
	}
	axis, err := newExactCyclicAxis(ctx, a.degree, knots, b)
	return axis, s, err
}
