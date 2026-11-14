package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// CertifyAffinePathExact proves the complete original-chart affine paths but
// permits a curved model image. Supplied order must match this segment.
func (s TubeSegment) CertifyAffinePathExact(ctx context.Context, start, end [4]*big.Rat, o geom.TubeAffineLineOptions) (geom.TubeAffinePathCertificate, error) {
	if ctx == nil || s.pa == nil || s.axis < 0 || s.axis > 3 || start[s.axis] == nil || end[s.axis] == nil {
		return geom.TubeAffinePathCertificate{}, geom.ErrInvalid
	}
	a, b := s.lo, s.hi
	if s.reversed {
		a, b = b, a
	}
	if start[s.axis].Cmp(new(big.Rat).SetFloat64(a)) != 0 || end[s.axis].Cmp(new(big.Rat).SetFloat64(b)) != 0 {
		return geom.TubeAffinePathCertificate{}, geom.ErrDomain
	}
	return s.proof.CertifyAffineParameterPath(ctx, start, end, o)
}

// CertifyAffinePath proposes numerical endpoint values, then proves their
// exact entire paired chart paths. Rounded proposals alone never suffice.
func (p TubeTrim) CertifyAffinePath(ctx context.Context, o geom.TubeAffineLineOptions) (geom.TubeAffinePathCertificate, error) {
	if ctx == nil {
		return geom.TubeAffinePathCertificate{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return geom.TubeAffinePathCertificate{}, e
	}
	a, e := p.segment.EvaluatePair(0)
	if e != nil {
		return geom.TubeAffinePathCertificate{}, e
	}
	b, e := p.segment.EvaluatePair(1)
	if e != nil {
		return geom.TubeAffinePathCertificate{}, e
	}
	exact := func(p Parameters) [4]*big.Rat {
		return [4]*big.Rat{new(big.Rat).SetFloat64(p.A.X), new(big.Rat).SetFloat64(p.A.Y), new(big.Rat).SetFloat64(p.B.X), new(big.Rat).SetFloat64(p.B.Y)}
	}
	return p.segment.CertifyAffinePathExact(ctx, exact(a), exact(b), o)
}
