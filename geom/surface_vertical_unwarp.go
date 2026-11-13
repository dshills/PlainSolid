package geom

import (
	"context"
	"math/big"
)

// RemoveVerticalQuadratic returns the exact rational image of p under
// (x,y,z) -> (x,y,z-k*(x-y)^2), preserving both original parameters.
// The operation uses homogeneous tensor Bernstein products and currently
// accepts single-span patches. Its inverse is globally one-to-one and has
// determinant one. Positive W² remains the new homogeneous denominator.
func (p *SurfacePatch) RemoveVerticalQuadratic(ctx context.Context, k float64, o SurfacePatchOptions) (*SurfacePatch, error) {
	if ctx == nil || p == nil || len(p.net) == 0 || !finite(k) {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return nil, ErrConvergence
	}
	o, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	n, m := 2*p.degreeU+1, 2*p.degreeV+1
	if n > o.MaxPoles/m {
		return nil, ErrConvergence
	}
	var net = make([]exactHomogeneous, n*m)
	for axis := 0; axis < 4; axis++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		r, e := tensorHomogeneousBernsteinProduct(p, axis, p, 3, b)
		if e != nil {
			return nil, e
		}
		for i, q := range r {
			net[i][axis] = q
		}
	}
	xy := *p
	xy.net = append([]exactHomogeneous(nil), p.net...)
	for i := range xy.net {
		x := new(big.Rat).Sub(xy.net[i][0], xy.net[i][1])
		if e := b.check(x); e != nil {
			return nil, e
		}
		xy.net[i][0] = x
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r, e := tensorHomogeneousBernsteinProduct(&xy, 0, &xy, 0, b)
	if e != nil {
		return nil, e
	}
	q := new(big.Rat).SetFloat64(k)
	for i, t := range r {
		net[i][2].Sub(net[i][2], t.Mul(t, q))
		if e := b.check(net[i][2]); e != nil {
			return nil, e
		}
	}
	d := p.Domain()
	knots := func(lo, hi float64, n int) []float64 {
		r := make([]float64, 2*n)
		for i := range n {
			r[i], r[i+n] = lo, hi
		}
		return r
	}
	out := &SurfacePatch{degreeU: n - 1, degreeV: m - 1, countU: n, countV: m,
		knotsU: knots(d.UMin, d.UMax, n), knotsV: knots(d.VMin, d.VMax, m), net: net, options: o}
	if e := out.computeBounds(b); e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.operations = b.operations
	return out, nil
}
