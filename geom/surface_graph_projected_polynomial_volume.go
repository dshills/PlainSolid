package geom

import (
	"context"
	"math/big"
)

// ProjectedSplineGraphPolynomialVolumes describes each positive A-minus-B
// layer and its exact physical volume in reference-region order.
type ProjectedSplineGraphPolynomialVolumes struct {
	Projection      ProjectedSplineGraphPolynomialArrangement
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateProjectedSplineGraphPolynomialVolumes proves both original charts
// and integrates their positive height difference in an independent identity
// XY reference chart. Every union-knot cell must have a positive constant
// difference denominator and the same physical quadratic scale. The reference
// height does not enter the integral. Failed calls return only work consumed.
func IntegrateProjectedSplineGraphPolynomialVolumes(ctx context.Context, reference, a, b *SurfacePatch, axis int,
	o GraphPointContactOptions) (ProjectedSplineGraphPolynomialVolumes, error) {
	var volumes polynomialPositiveVolumes
	proof, e := discoverProjectedSplineGraphPolynomialArrangement(ctx, reference, a, b, axis, o,
		polynomialVolumeDiscovery(false, nil, &volumes))
	if e != nil {
		return ProjectedSplineGraphPolynomialVolumes{ExactOperations: proof.ExactOperations}, e
	}
	return ProjectedSplineGraphPolynomialVolumes{
		Projection: proof, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total,
		ExactOperations: proof.ExactOperations,
	}, nil
}
