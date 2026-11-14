package geom

import (
	"context"
	"math/big"
)

// SurfacePlaneIdentityProof reports a complete original homogeneous identity.
type SurfacePlaneIdentityProof struct {
	Proven          bool
	ExactOperations int
}

// CertifySurfaceRationalPlaneIdentity proves normal·(S-origin)=0 on the entire
// immutable positive-weight tensor patch, including all original knot spans.
// Original homogeneous coefficients are compared exactly. Neither the covector
// nor the origin is normalized or converted to represented floating coordinates.
// Proven=false without error means the full coefficient identity does not hold.
func CertifySurfaceRationalPlaneIdentity(ctx context.Context, p *SurfacePatch, origin, normal [3]*big.Rat, o SurfacePatchOptions) (r SurfacePlaneIdentityProof, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return r, e
	}
	if len(p.net) > options.MaxPoles {
		return r, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	nonzero := false
	for i := 0; i < 3; i++ {
		if origin[i] == nil || normal[i] == nil {
			return r, ErrInvalid
		}
		if e = b.check(origin[i]); e != nil {
			return r, e
		}
		if e = b.check(normal[i]); e != nil {
			return r, e
		}
		nonzero = nonzero || normal[i].Sign() != 0
	}
	if !nonzero {
		return r, ErrDegenerate
	}
	for _, h := range p.net {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		value := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			term := new(big.Rat).Mul(origin[axis], h[3])
			if e = b.check(term); e != nil {
				return r, e
			}
			term.Sub(h[axis], term)
			if e = b.check(term); e != nil {
				return r, e
			}
			term.Mul(term, normal[axis])
			if e = b.check(term); e != nil {
				return r, e
			}
			value.Add(value, term)
			if e = b.check(value); e != nil {
				return r, e
			}
		}
		if value.Sign() != 0 {
			return r, nil
		}
	}
	r.Proven = true
	return r, nil
}
