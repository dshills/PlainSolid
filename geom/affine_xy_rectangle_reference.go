package geom

import (
	"context"
	"math/big"
)

// NewAffineXYRectangleReference proves the original affine XY projection and
// builds an identity XY chart over the same physical rectangle. The reference
// uses physical X,Y as its parameters and zero height. The original patch is
// untouched. Projections with non-axis-aligned sides return ErrDomain.
// Physical corners that cannot be represented exactly return ErrUnrepresentable.
func NewAffineXYRectangleReference(ctx context.Context, source *SurfacePatch, o SurfacePatchOptions) (*SurfacePatch, error) {
	options, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	if source == nil || len(source.net) == 0 || ctx == nil {
		return nil, ErrInvalid
	}
	if len(source.net) > options.MaxPoles {
		return nil, ErrConvergence
	}
	g, work, e := newAffineXYGraphWithWork(ctx, source, options)
	if e != nil {
		return nil, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: work}
	d := source.Domain()
	original := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	var physical [2][2]float64
	for ownerAxis, row := range g.inverse {
		if (row[0].Sign() == 0) == (row[1].Sign() == 0) {
			return nil, ErrDomain
		}
		axis := 0
		if row[0].Sign() == 0 {
			axis = 1
		}
		for side, endpoint := range original[ownerAxis] {
			p := new(big.Rat).Sub(new(big.Rat).SetFloat64(endpoint), row[2])
			p.Quo(p, row[axis])
			if e = budget.check(p); e != nil {
				return nil, e
			}
			physical[axis][side], e = RepresentedExactScalar(p)
			if e != nil {
				return nil, e
			}
		}
		if row[axis].Sign() < 0 {
			physical[axis][0], physical[axis][1] = physical[axis][1], physical[axis][0]
		}
	}
	x, y := physical[0], physical[1]
	plane, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{X: x[0], Y: y[0]}, {X: x[0], Y: y[1]}, {X: x[1], Y: y[0]}, {X: x[1], Y: y[1]}},
		[]float64{x[0], x[0], x[1], x[1]}, []float64{y[0], y[0], y[1], y[1]}, nil)
	if e != nil {
		return nil, e
	}
	referenceOptions := options
	referenceOptions.MaxOperations -= budget.operations
	if referenceOptions.MaxOperations < 1 {
		return nil, ErrConvergence
	}
	reference, e := NewSurfacePatch(plane, referenceOptions)
	if e != nil {
		return nil, e
	}
	reference.operations += budget.operations
	reference.options = options
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return reference, nil
}
