package geom

import (
	"context"
	"math/big"
)

// BezierControlNet extracts an exact homogeneous net from a single-span curve.
// Controls are (w*x,w*y,w*z,w) in ascending parameter order; Start/End retain
// the original represented domain. Every rational is an independent copy.
// In particular, ellipse UV arcs expose all three positive rational weights
// without requiring float64 export or changing their projective scaling.
// Multiple knot spans return ErrConvergence; no implicit approximation occurs.
func (c *SurfaceIsoCurve) BezierControlNet(ctx context.Context, o SurfacePatchOptions) (r ExactBezierSpan, operations int, err error) {
	if ctx == nil || c == nil || c.patch == nil || len(c.patch.net) == 0 {
		return r, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, 0, e
	}
	o, e := patchOptions(o)
	if e != nil {
		return r, 0, e
	}
	p := c.patch
	if p.countU != p.degreeU+1 {
		return r, 0, ErrConvergence
	}
	if p.countU > o.MaxPoles/2 {
		return r, 0, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = budget.operations }()
	r.Start, r.End = c.Domain()
	r.Control = make([][4]*big.Rat, p.countU)
	for i := range r.Control {
		if e := ctx.Err(); e != nil {
			return ExactBezierSpan{}, 0, e
		}
		for j, x := range p.net[i*p.countV] {
			if e := budget.check(x); e != nil {
				return ExactBezierSpan{}, 0, e
			}
			r.Control[i][j] = new(big.Rat).Set(x)
		}
	}
	return r, budget.operations, nil
}
