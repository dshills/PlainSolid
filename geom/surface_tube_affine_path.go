package geom

import (
	"context"
	"math/big"
)

// TubeAffinePathCertificate proves complete affine paths in both original UV
// charts and exact equality of their potentially curved rational model images.
// Curve follows normalized t in Start-to-End order; it is not a model chord.
// Tube uniqueness and full free-domain coverage identify the entire branch.
type TubeAffinePathCertificate struct {
	Proven               bool
	Start, End           [4]*big.Rat
	ModelStart, ModelEnd [3]*big.Rat
	Curve                *ExactSplineCurve
	ExactOperations      int
}

// CertifyAffineParameterPath retains the old line proof's endpoint/domain and
// exact full polynomial comparison, without assuming affine model coordinates.
// Distinct model endpoints and single original support cells remain required.
func (t VerifiedSurfaceTube) CertifyAffineParameterPath(ctx context.Context, start, end [4]*big.Rat, o TubeAffineLineOptions) (TubeAffinePathCertificate, error) {
	q, e := t.certifyAffineParameterPath(ctx, start, end, o, false)
	if e != nil {
		return TubeAffinePathCertificate{ExactOperations: q.ExactOperations}, e
	}
	return TubeAffinePathCertificate(q), nil
}
