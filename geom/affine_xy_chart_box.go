package geom

import (
	"context"
	"math/big"
)

// MapSourceBox maps a represented source-chart rectangle to original target UV.
// Bounds are outward; Exact is true only if all four mapped coordinates are
// represented exactly. The box may extend beyond the chart; no clipping occurs.
func (a *AffineXYChartAlignment) MapSourceBox(ctx context.Context, box ParameterRectangle, o SurfacePatchOptions) (bounds ParameterRectangle, exact bool, operations int, err error) {
	if ctx == nil || a == nil || a.target == nil || !box.Valid() {
		return bounds, false, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return bounds, false, 0, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return bounds, false, 0, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	ranges := [2][2]float64{{box.UMin, box.UMax}, {box.VMin, box.VMax}}
	var result [2][2]float64
	exact = true
	for i := 0; i < 2; i++ {
		axis := 0
		if a.mapping[i][1].Sign() != 0 {
			axis = 1
		}
		for side := 0; side < 2; side++ {
			input := side
			if a.mapping[i][axis].Sign() < 0 {
				input = 1 - side
			}
			q := new(big.Rat).Add(new(big.Rat).Mul(a.mapping[i][axis], new(big.Rat).SetFloat64(ranges[axis][input])), a.mapping[i][2])
			if e := b.check(q); e != nil {
				return bounds, false, 0, e
			}
			result[i][side] = rationalFloatBound(q, side == 0)
			if _, e := RepresentedExactScalar(q); e != nil {
				exact = false
			}
		}
	}
	bounds = ParameterRectangle{result[0][0], result[0][1], result[1][0], result[1][1]}
	if !bounds.Valid() {
		return bounds, false, 0, ErrConvergence
	}
	return bounds, exact, 0, nil
}
