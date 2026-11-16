package geom

import (
	"context"
	"math/big"
)

// RemoveKnot removes one copy of an existing knot throughout the cycle
// only if reinserting it exactly reproduces the original homogeneous net.
// It keeps one copy of the fundamental seam; rephase first to remove that
// station entirely. A nonremovable knot returns ErrUnrepresentable.
func (c *ExactPeriodicSplineCurve) RemoveKnot(ctx context.Context, t *big.Rat, o SurfacePatchOptions) (*ExactPeriodicSplineCurve, error) {
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
	r, err := c.removal(ctx, t, b)
	if err != nil {
		return nil, err
	}
	net, err := r.recover(ctx, c.net, b)
	if err != nil {
		return nil, err
	}
	return &ExactPeriodicSplineCurve{r.axis, net}, nil
}

// RemovePeriodicKnot removes one member of a periodic knot orbit from U
// (direction 0) or V (direction 1). Every row must satisfy the exact forward
// insertion equations; one nonremovable row rejects the operation.
func (s *ExactPeriodicSplineSurface) RemovePeriodicKnot(ctx context.Context, direction int, t *big.Rat,
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
	r, err := a.removal(ctx, t, b)
	if err != nil {
		return nil, err
	}
	axes := s.axis
	axes[direction] = exactTensorSplineAxis{r.axis, a.count - 1}
	net := make([]exactHomogeneous, axes[0].count*axes[1].count)
	row := make([]exactHomogeneous, a.count)
	for i := range s.axis[1-direction].count {
		for j := range row {
			if direction == 0 {
				row[j] = s.net[j*s.axis[1].count+i]
			} else {
				row[j] = s.net[i*s.axis[1].count+j]
			}
		}
		coarse, err := r.recover(ctx, row, b)
		if err != nil {
			return nil, err
		}
		for j, h := range coarse {
			if direction == 0 {
				net[j*axes[1].count+i] = h
			} else {
				net[i*axes[1].count+j] = h
			}
		}
	}
	return &ExactPeriodicSplineSurface{axes, net}, nil
}

type cyclicKnotRemoval struct {
	axis   exactCyclicAxis
	blends []cyclicKnotBlend
	first  int
}

func (a exactCyclicAxis) removal(ctx context.Context, t *big.Rat, b *polynomialBudget) (cyclicKnotRemoval, error) {
	var r cyclicKnotRemoval
	if len(a.knots)-2 <= a.degree {
		return r, ErrDomain
	}
	x, _, _, last, err := a.locate(t, b)
	if err != nil {
		return r, err
	}
	if last == 0 || x.Cmp(a.knots[last]) != 0 {
		return r, ErrDomain
	}
	knots := make([]*big.Rat, len(a.knots)-1)
	copy(knots, a.knots[:last])
	copy(knots[last:], a.knots[last+1:])
	coarse, err := newExactCyclicAxis(ctx, a.degree, knots, b)
	if err != nil {
		return r, err
	}
	_, _, _, span, err := coarse.locate(x, b)
	if err != nil {
		return r, err
	}
	_, blends, err := coarse.insertion(ctx, x, b)
	if err != nil {
		return r, err
	}
	return cyclicKnotRemoval{coarse, blends, (span + 1) % len(blends)}, nil
}

func (r cyclicKnotRemoval) recover(ctx context.Context, fine []exactHomogeneous, b *polynomialBudget) ([]exactHomogeneous, error) {
	coarse := make([]exactHomogeneous, len(fine)-1)
	set := func(i int, h exactHomogeneous) error {
		if h[3].Sign() <= 0 {
			return ErrUnrepresentable
		}
		if coarse[i][3] != nil {
			return checkKnotRemovalPole(coarse[i], h, b)
		}
		for axis, x := range h {
			if err := b.check(x); err != nil {
				return err
			}
			coarse[i][axis] = new(big.Rat).Set(x)
		}
		return nil
	}
	for j, q := range r.blends {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := checkRationalSplinePole(fine[j], b); err != nil {
			return nil, err
		}
		if q.alpha == nil || q.alpha.Sign() == 0 {
			if err := set(q.left, fine[j]); err != nil {
				return nil, err
			}
		} else if q.beta.Sign() == 0 {
			if err := set(q.right, fine[j]); err != nil {
				return nil, err
			}
		}
	}
	// Walk the original insertion band in chronological knot order,
	// starting just after its retained left neighbor, even across a seam.
	for offset := range min(r.axis.degree, len(r.blends)) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		j := (r.first + offset) % len(r.blends)
		q := r.blends[j]
		if q.alpha == nil || q.alpha.Sign() == 0 || q.beta.Sign() == 0 || coarse[q.right][3] != nil {
			continue
		}
		left := coarse[q.left]
		if left[3] == nil {
			return nil, ErrConvergence
		}
		var h exactHomogeneous
		for axis := range h {
			h[axis] = new(big.Rat).Sub(fine[j][axis], new(big.Rat).Mul(q.beta, left[axis]))
			h[axis].Quo(h[axis], q.alpha)
		}
		if err := set(q.right, h); err != nil {
			return nil, err
		}
	}
	// Underlying geometry is unchanged only if every original homogeneous
	// ordinate satisfies its equation, including the last blended control.
	for i, q := range r.blends {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if coarse[q.left][3] == nil || coarse[q.right][3] == nil {
			return nil, ErrConvergence
		}
		if err := checkKnotRemovalPole(q.apply(coarse[q.left], coarse[q.right]), fine[i], b); err != nil {
			return nil, err
		}
	}
	return coarse, nil
}

func checkKnotRemovalPole(a, c exactHomogeneous, b *polynomialBudget) error {
	for axis, x := range a {
		if err := b.check(x); err != nil {
			return err
		}
		if x.Cmp(c[axis]) != 0 {
			return ErrUnrepresentable
		}
	}
	return nil
}
