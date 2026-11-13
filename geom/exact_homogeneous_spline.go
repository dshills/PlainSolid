package geom

import (
	"context"
	"math"
	"math/big"
)

// NewExactHomogeneousSpline copies a continuous clamped positive-weight spline
// without normalizing or rounding its homogeneous controls. Knots retain original
// parameter units. MaxPoles includes the duplicated auxiliary rows.
func NewExactHomogeneousSpline(ctx context.Context, degree int, controls [][4]*big.Rat, knots []float64, options SurfacePatchOptions) (*ExactSplineCurve, error) {
	o, e := patchOptions(options)
	if ctx == nil || e != nil || validateKnots(degree, len(controls), knots) != nil {
		return nil, ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if len(controls) > o.MaxPoles/2 {
		return nil, ErrConvergence
	}
	n := len(controls)
	for i := 0; i <= degree; i++ {
		if knots[i] != knots[degree] || knots[len(knots)-1-i] != knots[n] {
			return nil, ErrInvalid
		}
	}
	run := 0
	for i := degree + 1; i < n; i++ {
		if i == degree+1 || knots[i] != knots[i-1] {
			run = 1
		} else {
			run++
		}
		if run > degree {
			return nil, ErrInvalid
		}
	}
	k := append([]float64(nil), knots...)
	p := &SurfacePatch{degreeU: degree, degreeV: 1, countU: n, countV: 2,
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
	return &SurfaceIsoCurve{patch: p, fixedAxis: -1, fixedParameter: math.NaN(), operations: b.operations}, nil
}
