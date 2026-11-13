package geom

import (
	"context"
	"math/big"
)

// AffineParameterCurve restricts one exact rational Bezier surface patch to the
// exact affine UV segment start+(end-start)*t, t in [0,1]. The resulting spatial
// curve retains homogeneous coefficients without float pole reconstruction.
// Both endpoints must lie in the patch rectangle. Multi-span patches require
// prior knot-aligned restriction and otherwise return ErrConvergence.
func (p *SurfacePatch) AffineParameterCurve(ctx context.Context, start, end [2]*big.Rat, o TubeAffineLineOptions) (*ExactSplineCurve, error) {
	if ctx == nil || p == nil || len(p.net) == 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return nil, ErrInvalid
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return nil, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	d := p.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for axis := 0; axis < 2; axis++ {
		for _, x := range []*big.Rat{start[axis], end[axis]} {
			if x == nil {
				return nil, ErrInvalid
			}
			if e := budget.check(x); e != nil {
				return nil, e
			}
			if x.Cmp(new(big.Rat).SetFloat64(ranges[axis][0])) < 0 || x.Cmp(new(big.Rat).SetFloat64(ranges[axis][1])) > 0 {
				return nil, ErrDomain
			}
		}
	}
	degree := p.degreeU + p.degreeV
	if degree+1 > o.MaxPoles/2 {
		return nil, ErrConvergence
	}
	var lower, upper [2]*big.Rat
	var reversed [2]bool
	for axis := 0; axis < 2; axis++ {
		index := axis
		origin := new(big.Rat).SetFloat64(ranges[index][0])
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(ranges[index][1]), origin)
		lo := new(big.Rat).Quo(new(big.Rat).Sub(start[index], origin), width)
		hi := new(big.Rat).Quo(new(big.Rat).Sub(end[index], origin), width)
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
			reversed[axis] = true
		}
		lower[axis], upper[axis] = lo, hi
	}
	net, e := restrictComparisonTensor(comparisonTensor{nu: p.countU, nv: p.countV, net: p.net}, lower[0], upper[0], lower[1], upper[1], budget)
	if e != nil {
		return nil, e
	}
	diagonal := make([]exactHomogeneous, degree+1)
	for i := range diagonal {
		for axis := 0; axis < 4; axis++ {
			diagonal[i][axis] = new(big.Rat)
		}
	}
	for i := 0; i < p.countU; i++ {
		for j := 0; j < p.countV; j++ {
			u, v := i, j
			if reversed[0] {
				u = p.countU - 1 - i
			}
			if reversed[1] {
				v = p.countV - 1 - j
			}
			h := net[u*p.countV+v]
			numerator := new(big.Int).Mul(new(big.Int).Binomial(int64(p.degreeU), int64(i)), new(big.Int).Binomial(int64(p.degreeV), int64(j)))
			factor := new(big.Rat).SetFrac(numerator, new(big.Int).Binomial(int64(degree), int64(i+j)))
			if e := budget.check(factor); e != nil {
				return nil, e
			}
			for axis := 0; axis < 4; axis++ {
				diagonal[i+j][axis].Add(diagonal[i+j][axis], new(big.Rat).Mul(factor, h[axis]))
				if e := budget.check(diagonal[i+j][axis]); e != nil {
					return nil, e
				}
			}
		}
	}
	return exactCurveFromHomogeneousBezier(diagonal, o, budget)
}
