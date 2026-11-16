package geom

import (
	"context"
	"math/big"
)

// Reverse reflects the physical parameter about the midpoint of the
// fundamental interval. The result retains exact knots and controls.
// At knots its right-hand jet is the reversed original left-hand jet.
func (c *ExactPeriodicSplineCurve) Reverse(ctx context.Context, o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
	if ctx == nil || c == nil || len(c.net) == 0 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := periodicRephaseBudget(o, len(c.net))
	if err != nil {
		return nil, err
	}
	a, indices, err := c.reverse(ctx, b)
	if err != nil {
		return nil, err
	}
	h := make([][4]*big.Rat, len(c.net))
	for i, j := range indices {
		h[i] = c.net[j]
	}
	net, err := ownedHomogeneousNet(ctx, h, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineCurve{a, net}, nil
}

// ReversePeriodicDirection reflects only the selected cyclic physical
// parameter. Direction 0 is U and 1 is V; the other partial is unchanged.
func (s *ExactPeriodicSplineSurface) ReversePeriodicDirection(ctx context.Context, direction int, o SurfacePatchOptions) (*ExactPeriodicSplineSurface, error) {
	if ctx == nil || s == nil || len(s.net) == 0 || direction < 0 || direction > 1 {
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
	axis, indices, err := a.reverse(ctx, b)
	if err != nil {
		return nil, err
	}
	axes := s.axis
	axes[direction].exactCyclicAxis = axis
	controls := make([][4]*big.Rat, len(s.net))
	for i := range controls {
		u, v := i/axes[1].count, i%axes[1].count
		if direction == 0 {
			u = indices[u]
		} else {
			v = indices[v]
		}
		controls[i] = s.net[u*axes[1].count+v]
	}
	net, err := ownedHomogeneousNet(ctx, controls, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineSurface{axes, net}, nil
}

func (a exactCyclicAxis) reverse(ctx context.Context, b *polynomialBudget) (exactCyclicAxis, []int, error) {
	n := len(a.knots) - 1
	anchor := new(big.Rat).Add(a.knots[0], a.knots[n])
	if err := b.check(anchor); err != nil {
		return exactCyclicAxis{}, nil, err
	}
	m := 1
	for m < n && a.knots[m].Cmp(a.knots[0]) == 0 {
		m++
	}
	knots := make([]*big.Rat, n+1)
	for i := range knots {
		if err := ctx.Err(); err != nil {
			return exactCyclicAxis{}, nil, err
		}
		// Start at the final copy of the closing seam's orbit so
		// reversing places every repeated station at the start.
		knots[i] = new(big.Rat).Sub(anchor, a.knot(n+m-1-i))
	}
	axis, err := newExactCyclicAxis(ctx, a.degree, knots, b)
	if err != nil {
		return exactCyclicAxis{}, nil, err
	}
	indices := make([]int, n)
	for i := range indices {
		// The old control j has support [K_(j-p),K_(j+1)]. Its
		// reflection must match both endpoints of the new support.
		indices[i] = (n + m + a.degree - 2 - i) % n
	}
	return axis, indices, nil
}
