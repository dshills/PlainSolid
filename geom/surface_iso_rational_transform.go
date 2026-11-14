package geom

import (
	"context"
	"math/big"
)

// TransformRational applies an exact rational affine matrix to the homogeneous
// control net. No direction normalization, pole export or binary64 conversion
// occurs. Original knots, parameter units and fixed-coordinate metadata remain
// unchanged; weights stay positive. Singular maps are allowed, as in Transform.
// Construction and subsequent evaluation use options. ExactOperations reports
// this transformation's cumulative arithmetic/bounds work. Failure is atomic.
func (c *SurfaceIsoCurve) TransformRational(ctx context.Context, m [3][4]*big.Rat, options SurfacePatchOptions) (*SurfaceIsoCurve, error) {
	if ctx == nil || c == nil || c.patch == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	source := c.patch
	if len(source.net) > o.MaxPoles {
		return nil, ErrConvergence
	}
	b := polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	var matrix [3][4]*big.Rat
	for i, row := range m {
		for j, q := range row {
			if q == nil {
				return nil, ErrInvalid
			}
			if e := b.check(q); e != nil {
				return nil, e
			}
			matrix[i][j] = new(big.Rat).Set(q)
		}
	}
	net := make([]exactHomogeneous, len(source.net))
	for i, h := range source.net {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for _, q := range h {
			if e := b.check(q); e != nil {
				return nil, e
			}
		}
		for axis := 0; axis < 3; axis++ {
			value := new(big.Rat)
			for j := 0; j < 4; j++ {
				product := new(big.Rat).Mul(matrix[axis][j], h[j])
				if e := b.check(product); e != nil {
					return nil, e
				}
				value.Add(value, product)
				if e := b.check(value); e != nil {
					return nil, e
				}
			}
			net[i][axis] = value
		}
		net[i][3] = new(big.Rat).Set(h[3])
	}
	p := &SurfacePatch{degreeU: source.degreeU, degreeV: source.degreeV, countU: source.countU, countV: source.countV, knotsU: append([]float64(nil), source.knotsU...), knotsV: append([]float64(nil), source.knotsV...), net: net, options: o}
	if e := p.computeBounds(&b); e != nil {
		return nil, e
	}
	p.operations = b.operations
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return &SurfaceIsoCurve{patch: p, fixedAxis: c.fixedAxis, fixedParameter: c.fixedParameter, operations: b.operations}, nil
}
