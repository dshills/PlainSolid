package geom

import (
	"context"
	"math/big"
)

// CertifySurfaceAffineCovector proves normal dot model = scale*UV[parameterAxis]
// + offset on every original knot cell. Normal is an unnormalized rational
// covector; no Euclidean frame normalization or coefficient rounding occurs.
// The source patch is immutable. Input contraction and the coefficient identity
// proof share one work and coefficient-bit allowance.
func CertifySurfaceAffineCovector(ctx context.Context, p *SurfacePatch, normal [3]*big.Rat, parameterAxis int, scale, offset *big.Rat, o SurfaceAffineCoordinateOptions) (SurfaceAffineCoordinateProof, error) {
	if ctx == nil || p == nil {
		return SurfaceAffineCoordinateProof{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SurfaceAffineCoordinateProof{}, e
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return SurfaceAffineCoordinateProof{}, e
	}
	if len(p.net) > opts.MaxPoles {
		return SurfaceAffineCoordinateProof{}, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	nonzero := false
	for _, v := range normal {
		if v == nil {
			return SurfaceAffineCoordinateProof{}, ErrInvalid
		}
		if e = b.check(v); e != nil {
			return SurfaceAffineCoordinateProof{}, e
		}
		nonzero = nonzero || v.Sign() != 0
	}
	if !nonzero {
		return SurfaceAffineCoordinateProof{}, ErrInvalid
	}
	q := *p
	q.net = make([]exactHomogeneous, len(p.net))
	for i, h := range p.net {
		if e = ctx.Err(); e != nil {
			return SurfaceAffineCoordinateProof{}, e
		}
		sum := new(big.Rat)
		for j := 0; j < 3; j++ {
			if e = b.check(h[j]); e != nil {
				return SurfaceAffineCoordinateProof{}, e
			}
			term := new(big.Rat).Mul(normal[j], h[j])
			if e = b.check(term); e != nil {
				return SurfaceAffineCoordinateProof{}, e
			}
			sum.Add(sum, term)
			if e = b.check(sum); e != nil {
				return SurfaceAffineCoordinateProof{}, e
			}
		}
		q.net[i] = h
		q.net[i][0] = sum
	}
	if b.operations >= opts.MaxOperations {
		return SurfaceAffineCoordinateProof{}, ErrConvergence
	}
	o.SurfacePatchOptions = opts
	o.MaxOperations -= b.operations
	proof, e := CertifySurfaceAffineCoordinate(ctx, &q, 0, parameterAxis, scale, offset, o)
	if e != nil {
		return SurfaceAffineCoordinateProof{}, e
	}
	proof.ExactOperations += b.operations
	return proof, nil
}
