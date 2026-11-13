package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// ExactFreeDomain returns the original chart axis, owned exact closed bounds,
// and whether this segment traverses them in decreasing order. It is separate
// from normalized display parameterization and from any trim-event parameter.
func (s TubeSegment) ExactFreeDomain() (axis int, bounds geom.ExactParameterInterval, reversed bool, err error) {
	if s.pa == nil {
		return 0, bounds, false, geom.ErrInvalid
	}
	axis, bounds, err = s.proof.ExactFreeDomain()
	return axis, bounds, s.reversed, err
}

func (s TubeSegment) ContainsFreeRoot(ctx context.Context, root *geom.ExactAlgebraicRoot, o geom.ExactCurvePlaneEventOptions) (geom.TubeFreeRootMembership, error) {
	if s.pa == nil {
		return geom.TubeFreeRootMembership{}, geom.ErrInvalid
	}
	return s.proof.ContainsFreeRoot(ctx, root, o)
}

// EncloseAtFree encloses the unique branch point at the represented original
// free coordinate. It avoids rounding a normalized display parameter into
// another free coordinate. The point and parameter bounds are outward.
func (s TubeSegment) EncloseAtFree(parameter float64, o geom.SurfaceTubeEvaluationOptions) (geom.SurfaceTubeEvaluation, error) {
	if s.pa == nil || !finite(parameter) || parameter < s.lo || parameter > s.hi {
		return geom.SurfaceTubeEvaluation{}, geom.ErrDomain
	}
	o.FreeAxis, o.Parameter = s.axis, parameter
	return s.proof.Evaluate(o)
}

// EvaluateRationalGraph accepts an original free-coordinate value, not a
// normalized floating sample. It returns exact chart roots and bounded model
// evidence for the unique branch point where the supported graph proof applies.
func (s TubeSegment) EvaluateRationalGraph(ctx context.Context, free *big.Rat, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	if s.pa == nil {
		return geom.TubeAlgebraicGraphPointCertificate{}, geom.ErrInvalid
	}
	return s.proof.EvaluateRationalGraph(ctx, free, o)
}
