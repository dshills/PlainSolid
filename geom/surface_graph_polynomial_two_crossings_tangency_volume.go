package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialTwoCrossingsTangencyVolumes gives the exact positive
// A-minus-B layer volumes in original arrangement order. ExactOperations
// includes source reproof and rational integration.
type SplineGraphPolynomialTwoCrossingsTangencyVolumes struct {
	Arrangement     SplineGraphPolynomialTwoCrossingsTangency
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphPolynomialTwoCrossingsTangencyVolumes re-proves both
// crossings and the tangency on identity XY charts. The original height
// difference needs positive constant cell denominators and the same physical
// leading coefficient in each union cell. It integrates the two exterior
// layers or all four between-branch layers exactly.
func IntegrateSplineGraphPolynomialTwoCrossingsTangencyVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialTwoCrossingsTangencyVolumes, error) {
	var arrangement SplineGraphPolynomialTwoCrossingsTangency
	var volumes polynomialPositiveVolumes
	mode := polynomialGraphContactMode{
		crossings: 2, tangencies: 1, polynomialHeight: true,
		certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
			if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, b); e != nil {
				return e
			}
			var e error
			volumes, e = integratePolynomialPositiveLayers(ctx, arrangement.Power, arrangement.Regions[:], arrangement.ContactOrder[:], roots, domain, pair.heightScale, b)
			return e
		},
	}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialTwoCrossingsTangencyVolumes{}, e
	}
	arrangement.Proven, arrangement.ExactOperations = true, pair.ExactOperations
	return SplineGraphPolynomialTwoCrossingsTangencyVolumes{
		Arrangement: arrangement, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total, ExactOperations: pair.ExactOperations,
	}, nil
}
