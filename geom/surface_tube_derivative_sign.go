package geom

import (
	"context"
)

// SurfaceTubeDerivativeSigns describes derivatives with respect to increasing
// original free coordinate. Proven[i] distinguishes an exact zero Sign from
// an unresolved sign. Ranges are outward and valid only when HasRange[i].
// Signs alone need no numerical lower bound on the certified nonzero free minor.
type SurfaceTubeDerivativeSigns struct {
	FreeAxis         int
	Sign             [4]int
	Proven, HasRange [4]bool
	Ranges           [4]CertifiedScalarRange
	ExactOperations  int
}

// ParameterDerivativeSigns bounds canonical Jacobian cofactors on the original
// closed tube box. The implicit function theorem gives dq_i/dq_free=C_i/C_free.
// A zero cofactor interval proves a constant coordinate; a mixed interval stays
// unresolved. This is a local branch proof, not a closed-loop winding number.
func (t VerifiedSurfaceTube) ParameterDerivativeSigns(ctx context.Context, o SurfacePatchOptions) (r SurfaceTubeDerivativeSigns, err error) {
	if ctx == nil || t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	c, e := surfaceTubeCofactorIntervals(ctx, t.a, t.b, b)
	if e != nil {
		return r, e
	}
	r.FreeAxis = t.axis
	denominatorStrict := c[t.axis].lo.Sign() > 0 || c[t.axis].hi.Sign() < 0
	for i, q := range c {
		if q.lo.Sign() > 0 {
			r.Proven[i] = true
			r.Sign[i] = t.tangentSign
		} else if q.hi.Sign() < 0 {
			r.Proven[i] = true
			r.Sign[i] = -t.tangentSign
		} else if q.lo.Sign() == 0 && q.hi.Sign() == 0 {
			r.Proven[i] = true
		}
		if denominatorStrict {
			d, e := intervalDivide(q, c[t.axis], b)
			if e != nil {
				return r, e
			}
			r.HasRange[i] = true
			r.Ranges[i] = CertifiedScalarRange{Min: rationalFloatBound(d.lo, true), Max: rationalFloatBound(d.hi, false)}
		} else if r.Proven[i] && r.Sign[i] == 0 {
			r.HasRange[i] = true
			r.Ranges[i] = CertifiedScalarRange{}
		}
	}
	r.Sign[t.axis] = 1
	r.Proven[t.axis] = true
	r.HasRange[t.axis] = true
	r.Ranges[t.axis] = CertifiedScalarRange{Min: 1, Max: 1}
	return r, nil
}
