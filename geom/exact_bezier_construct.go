package geom

import (
	"context"
	"math/big"
)

// NewExactHomogeneousBezier constructs an exact positive-weight Bézier
// descriptor on [0,1]. Controls are (w*x,w*y,w*z,w) and are copied.
// No Euclidean pole or weight is converted to float64.
func NewExactHomogeneousBezier(ctx context.Context, controls [][4]*big.Rat, options SurfacePatchOptions) (*ExactSplineCurve, error) {
	o, e := patchOptions(options)
	if ctx == nil || e != nil || len(controls) < 2 || len(controls) > 65 {
		return nil, ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if len(controls) > o.MaxPoles/2 {
		return nil, ErrConvergence
	}
	n := len(controls)
	k := make([]float64, 2*n)
	for i := n; i < 2*n; i++ {
		k[i] = 1
	}
	p := &SurfacePatch{degreeU: n - 1, degreeV: 1, countU: n, countV: 2,
		knotsU: k, knotsV: []float64{0, 0, 1, 1}, net: make([]exactHomogeneous, 2*n), options: o}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for i, h := range controls {
		for j, q := range h {
			if q == nil || j == 3 && q.Sign() <= 0 {
				return nil, ErrInvalid
			}
			if e = b.check(q); e != nil {
				return nil, e
			}
			p.net[2*i][j] = new(big.Rat).Set(q)
		}
		p.net[2*i+1] = p.net[2*i]
	}
	if e = p.computeBounds(b); e != nil {
		return nil, e
	}
	p.operations = b.operations
	return &SurfaceIsoCurve{patch: p, fixedAxis: -1, operations: b.operations}, nil
}
