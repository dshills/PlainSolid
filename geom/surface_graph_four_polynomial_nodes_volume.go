package geom

import (
	"context"
	"math/big"
)

// SplineGraphFourPolynomialNodeVolumes gives the exact positive volumes of
// the original A-minus-B graph layers in arrangement region order.
// ExactOperations includes source reproof and rational integration.
type SplineGraphFourPolynomialNodeVolumes struct {
	Arrangement     SplineGraphFourPolynomialNodes
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphFourPolynomialNodeVolumes re-proves all four original
// crossings on identity XY charts. It requires a positive constant
// denominator in each height-difference cell and the same physical leading
// coefficient throughout. It exactly integrates the two exterior layers or
// the five between-branch layers. A failure returns an empty report.
func IntegrateSplineGraphFourPolynomialNodeVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphFourPolynomialNodeVolumes, error) {
	var arrangement SplineGraphFourPolynomialNodes
	var volumes polynomialPositiveVolumes
	mode := polynomialGraphContactMode{
		crossings: 4, polynomialHeight: true,
		certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
			if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, b); e != nil {
				return e
			}
			var e error
			volumes, e = integratePolynomialPositiveLayers(ctx, arrangement.Power, arrangement.Regions[:], []int{1, 1, 1, 1}, roots, domain, pair.heightScale, b)
			return e
		},
	}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphFourPolynomialNodeVolumes{}, e
	}
	arrangement.Proven = true
	arrangement.ExactOperations = pair.ExactOperations
	return SplineGraphFourPolynomialNodeVolumes{
		Arrangement: arrangement, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total, ExactOperations: pair.ExactOperations,
	}, nil
}
