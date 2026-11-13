package geom

import (
	"context"
	"math/big"
)

// FirstBoundaryHitRationalUV finds the first exposed material boundary on
// uv=origin+t*direction, 0<=t<=maxT, without rounding any input or embedding.
// Exact parameters keep original UV-line units. Point and outward-normal
// enclosures are in original world XYZ; active normals remain separate.
// It uses one bounded rational clip, one bounded embedding, and the same bounded
// root comparisons and normal sign proofs as FirstBoundaryHit. Failure is atomic.
func (s SpherePairPlaneMaterial) FirstBoundaryHitRationalUV(ctx context.Context, origin, direction [2]*big.Rat, maxT *big.Rat, o ToroidLineOptions) (SpherePairFirstHit, error) {
	var zero SpherePairFirstHit
	if maxT == nil || maxT.Sign() < 0 {
		return zero, ErrInvalid
	}
	line, e := s.ClipRationalLine(ctx, origin, direction, new(big.Rat), maxT, o)
	if e != nil {
		return zero, e
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return zero, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	var org, dir [3]*big.Rat
	for j := 0; j < 3; j++ {
		org[j] = new(big.Rat).Set(s.origin[j])
		dir[j] = new(big.Rat)
		for k := 0; k < 2; k++ {
			a, d := new(big.Rat).Mul(s.basis[k][j], origin[k]), new(big.Rat).Mul(s.basis[k][j], direction[k])
			for _, v := range []*big.Rat{a, d} {
				if e := b.check(v); e != nil {
					return zero, e
				}
			}
			org[j].Add(org[j], a)
			dir[j].Add(dir[j], d)
		}
		for _, v := range []*big.Rat{org[j], dir[j]} {
			if e := b.check(v); e != nil {
				return zero, e
			}
		}
	}
	return s.source.firstBoundaryHitRational(ctx, line, org, dir, o)
}
