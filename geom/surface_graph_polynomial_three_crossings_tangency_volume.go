package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialThreeCrossingsTangencyVolumes gives the exact positive
// A-minus-B layer volumes in original arrangement order. ExactOperations
// includes source reproof and rational integration.
type SplineGraphPolynomialThreeCrossingsTangencyVolumes struct {
	Arrangement     SplineGraphPolynomialThreeCrossingsTangency
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphPolynomialThreeCrossingsTangencyVolumes re-proves all three
// crossings and the tangency on identity XY charts. The original height
// difference needs positive constant cell denominators and the same physical
// leading coefficient in each union cell. It integrates the two exterior
// layers or all five between-branch layers exactly.
func IntegrateSplineGraphPolynomialThreeCrossingsTangencyVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialThreeCrossingsTangencyVolumes, error) {
	var arrangement SplineGraphPolynomialThreeCrossingsTangency
	var volumes polynomialPositiveVolumes
	mode := polynomialGraphContactMode{
		crossings: 3, tangencies: 1, allowQuintic: true, polynomialHeight: true,
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
		return SplineGraphPolynomialThreeCrossingsTangencyVolumes{}, e
	}
	arrangement.Proven, arrangement.ExactOperations = true, pair.ExactOperations
	return SplineGraphPolynomialThreeCrossingsTangencyVolumes{
		Arrangement: arrangement, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total, ExactOperations: pair.ExactOperations,
	}, nil
}
