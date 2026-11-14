package intersect

import (
	"context"
	"errors"
	"plainsolid/geom"
)

// Both tubes are created from the same ordered originals by the caller. Their
// immutable proofs and independently discovered exact endpoint coordinates are
// sufficient for paired-chart equality; floating enclosures only prefilter.
func algebraicGraphTubeJunction(ctx context.Context, left, right geom.VerifiedSurfaceTube, leftUpper, rightUpper bool, maxOperations, maxBits int) (connected bool, operations int, err error) {
	if maxOperations < 1 {
		return false, 0, geom.ErrConvergence
	}
	certificates := [2]geom.TubeAlgebraicGraphEndpointCertificate{}
	for i, tube := range []geom.VerifiedSurfaceTube{left, right} {
		remaining := maxOperations - operations
		if remaining < 1 {
			return false, operations, geom.ErrConvergence
		}
		upper := leftUpper
		if i == 1 {
			upper = rightUpper
		}
		q, e := tube.DiscoverAlgebraicGraphEndpoint(ctx, upper, geom.TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: maxBits}}})
		operations += q.ExactOperations
		if e != nil && (errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrDegenerate)) {
			remaining = maxOperations - operations
			if remaining < 1 {
				return false, operations, geom.ErrConvergence
			}
			_, domain, de := tube.ExactFreeDomain()
			if de != nil {
				return false, operations, de
			}
			free := domain.Lower
			if upper {
				free = domain.Upper
			}
			q, e = tube.EvaluateRationalPlane(ctx, free, geom.TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: maxBits}}})
			operations += q.ExactOperations
		}
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrDegenerate) {
				return false, operations, nil
			}
			return false, operations, e
		}
		if !q.Proven {
			return false, operations, nil
		}
		certificates[i] = q
	}
	for axis := 0; axis < 4; axis++ {
		remaining := maxOperations - operations
		if remaining < 1 {
			return false, operations, geom.ErrConvergence
		}
		q, e := certificates[0].Parameters[axis].Compare(ctx, certificates[1].Parameters[axis], geom.ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: maxBits})
		operations += q.Operations
		if e != nil {
			return false, operations, e
		}
		if !q.Proven || q.Order != 0 {
			return false, operations, nil
		}
	}
	return true, operations, nil
}
