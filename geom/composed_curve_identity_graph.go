package geom

import (
	"context"
	"math/big"
)

func intersectNormalizedComposedCurveIdentityGraph(ctx context.Context, cell *ExactComposedCurveCell, surface *SurfacePatch, o SurfaceCurveAgreementOptions) (r IdentityGraphCurveIntersection, err error) {
	if ctx == nil || cell == nil || surface == nil || cell.endpoints[0] == nil || cell.endpoints[1] == nil || len(cell.power[3]) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations += b.operations }()
	degree := 0
	for _, poly := range cell.power {
		degree = max(degree, len(poly)-1)
	}
	if degree < 1 || degree > 64 || 2*(degree+1) > options.MaxPoles {
		return r, ErrConvergence
	}
	net := make([]exactHomogeneous, degree+1)
	for i := 0; i <= degree; i++ {
		for axis := 0; axis < 4; axis++ {
			value := new(big.Rat)
			for j := 0; j <= i && j < len(cell.power[axis]); j++ {
				ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(degree), int64(j)))
				value.Add(value, new(big.Rat).Mul(cell.power[axis][j], ratio))
				if e := b.check(value); e != nil {
					return r, e
				}
			}
			net[i][axis] = value
		}
		if net[i][3].Sign() <= 0 {
			return r, ErrConvergence
		}
	}
	curve, e := exactCurveFromHomogeneousBezier(net, TubeAffineLineOptions{MaxPoles: options.MaxPoles, MaxOperations: options.MaxOperations, MaxCoefficientBits: options.MaxCoefficientBits}, b)
	if e != nil {
		return r, e
	}
	options.MaxOperations -= b.operations
	if options.MaxOperations < 1 {
		return r, ErrConvergence
	}
	o.SurfacePatchOptions = options
	return intersectExactCurveIdentityGraph(ctx, curve, surface, o, cell)
}
