package intersect

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// CertifyAffineLine proves the exact degree-one model curve corresponding
// to explicitly proposed paired UV endpoints in this segment's traversal
// order. The free coordinate must span the whole segment. Curve evaluation
// uses normalized t in the same direction as TrimA and TrimB.
func (s TubeSegment) CertifyAffineLine(ctx context.Context, start, end Parameters, o geom.TubeAffineLineOptions) (geom.TubeAffineLineCertificate, error) {
	if s.pa == nil || !finite(start.A.X) || !finite(start.A.Y) || !finite(start.B.X) || !finite(start.B.Y) ||
		!finite(end.A.X) || !finite(end.A.Y) || !finite(end.B.X) || !finite(end.B.Y) {
		return geom.TubeAffineLineCertificate{}, geom.ErrInvalid
	}
	exact := func(p Parameters) [4]*big.Rat {
		return [4]*big.Rat{new(big.Rat).SetFloat64(p.A.X), new(big.Rat).SetFloat64(p.A.Y),
			new(big.Rat).SetFloat64(p.B.X), new(big.Rat).SetFloat64(p.B.Y)}
	}
	return s.CertifyAffineLineExact(ctx, exact(start), exact(end), o)
}

// CertifyAffineLineExact is CertifyAffineLine with exact rational UV values.
// Indices are [uA,vA,uB,vB]. Returned model coordinates are caller-owned.
func (s TubeSegment) CertifyAffineLineExact(ctx context.Context, start, end [4]*big.Rat, o geom.TubeAffineLineOptions) (geom.TubeAffineLineCertificate, error) {
	if ctx == nil || s.pa == nil || start[s.axis] == nil || end[s.axis] == nil {
		return geom.TubeAffineLineCertificate{}, geom.ErrInvalid
	}
	a, b := s.lo, s.hi
	if s.reversed {
		a, b = b, a
	}
	if start[s.axis].Cmp(new(big.Rat).SetFloat64(a)) != 0 || end[s.axis].Cmp(new(big.Rat).SetFloat64(b)) != 0 {
		return geom.TubeAffineLineCertificate{}, geom.ErrDomain
	}
	return s.proof.CertifyAffineParameterLine(ctx, start, end, o)
}
