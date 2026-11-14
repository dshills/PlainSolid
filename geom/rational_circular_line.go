package geom

import (
	"context"
	"math/big"
)

func rationalLineVec3(v Vec3) [3]*big.Rat {
	return [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}

func validateRationalCircularLine(ctx context.Context, origin, direction []*big.Rat, lo, hi *big.Rat, o ToroidLineOptions) (SurfacePatchOptions, error) {
	if ctx == nil || lo == nil || hi == nil || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return o.Surface, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return o.Surface, e
	}
	options, e := patchOptions(o.Surface)
	if e != nil {
		return options, e
	}
	check := func(q *big.Rat) error {
		if q == nil {
			return ErrInvalid
		}
		if q.Num().BitLen() > options.MaxCoefficientBits || q.Denom().BitLen() > options.MaxCoefficientBits {
			return ErrConvergence
		}
		return nil
	}
	for _, coordinates := range [][]*big.Rat{origin, direction, {lo, hi}} {
		for _, q := range coordinates {
			if e := check(q); e != nil {
				return options, e
			}
		}
	}
	for _, q := range direction {
		if q.Sign() != 0 && lo.Cmp(hi) <= 0 {
			return options, nil
		}
	}
	return options, ErrInvalid
}
