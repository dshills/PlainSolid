package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialQuarticTangencyVolumes gives the exact positive
// A-minus-B layer volumes in original arrangement order. ExactOperations
// includes source reproof and rational integration.
type SplineGraphPolynomialQuarticTangencyVolumes struct {
	Arrangement     SplineGraphPolynomialQuarticTangency
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphPolynomialQuarticTangencyVolumes re-proves the isolated
// order-four tangency on identity XY charts. The original height
// difference needs positive constant cell denominators and the same physical
// leading coefficient in each union cell. It integrates the two exterior
// layers or the two between-branch layers exactly.
func IntegrateSplineGraphPolynomialQuarticTangencyVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialQuarticTangencyVolumes, error) {
	var arrangement SplineGraphPolynomialQuarticTangency
	var volumes polynomialPositiveVolumes
	mode := polynomialGraphContactMode{
		quarticTangencies: 1, polynomialHeight: true,
		certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
			if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, b); e != nil {
				return e
			}
			var e error
			volumes, e = integratePolynomialPositiveLayers(ctx, arrangement.Power, arrangement.Regions[:], []int{4}, roots, domain, pair.heightScale, b)
			return e
		},
	}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialQuarticTangencyVolumes{}, e
	}
	arrangement.Proven, arrangement.ExactOperations = true, pair.ExactOperations
	return SplineGraphPolynomialQuarticTangencyVolumes{
		Arrangement: arrangement, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total, ExactOperations: pair.ExactOperations,
	}, nil
}
