package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func discoverSplineShellVerticalQuadratic(ctx context.Context, a, b []splineEmbeddingFace, o geom.SurfacePatchOptions) (float64, error) {
	pairs := [2][]splineEmbeddingFace{a, b}
	opt := geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	difference := func(x [3]*big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Add(x[0], x[2]), new(big.Rat).Mul(x[1], big.NewRat(2, 1)))
	}
	for _, faces := range pairs {
		for _, f := range faces {
			if e := ctx.Err(); e != nil {
				return 0, e
			}
			p := f.patch
			d := p.Domain()
			for axis := range 2 {
				lo, hi := d.UMin, d.UMax
				if axis == 1 {
					lo, hi = d.VMin, d.VMax
				}
				l, h := ratInset(lo), ratInset(hi)
				m := new(big.Rat).Quo(new(big.Rat).Add(l, h), big.NewRat(2, 1))
				var z, delta [3]*big.Rat
				for j, v := range []*big.Rat{l, m, h} {
					u, w := v, ratInset(d.VMin)
					if axis == 1 {
						u, w = ratInset(d.UMin), v
					}
					x, e := p.EvaluateRational(ctx, u, w, opt)
					if e != nil {
						return 0, e
					}
					z[j] = x.Point[2]
					delta[j] = new(big.Rat).Sub(x.Point[0], x.Point[1])
				}
				if difference(delta).Sign() != 0 || delta[0].Cmp(delta[2]) == 0 {
					continue
				}
				for j, x := range delta {
					delta[j] = new(big.Rat).Mul(x, x)
				}
				n := difference(z)
				if n.Sign() == 0 {
					// For this nonconstant projected axis, any valid
					// common quadratic warp would have coefficient zero.
					return 0, geom.ErrConvergence
				}
				k, e := geom.RepresentedExactScalar(n.Quo(n, difference(delta)))
				if e != nil {
					return 0, e
				}
				for _, group := range pairs {
					for _, x := range group {
						q, e := x.patch.RemoveVerticalQuadratic(ctx, k, o)
						if e != nil {
							return 0, e
						}
						if _, e = geom.NewAffineSurfaceChart(ctx, q, geom.AffineSurfaceChartOptions{
							MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
						}); e != nil {
							if e == geom.ErrDegenerate {
								e = geom.ErrConvergence
							}
							return 0, e
						}
					}
				}
				return k, nil
			}
		}
	}
	return 0, geom.ErrConvergence
}
