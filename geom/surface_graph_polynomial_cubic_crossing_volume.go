package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialCubicCrossingVolumes gives the exact positive
// A-minus-B layer volumes in original arrangement order. ExactOperations
// includes source reproof and rational integration.
type SplineGraphPolynomialCubicCrossingVolumes struct {
	Arrangement     SplineGraphPolynomialCubicCrossing
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphPolynomialCubicCrossingVolumes re-proves the isolated
// order-three crossing on identity XY charts. The original height
// difference needs positive constant cell denominators and the same physical
// leading coefficient in each union cell. It integrates the two exterior
// layers or the two between-branch layers exactly.
func IntegrateSplineGraphPolynomialCubicCrossingVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialCubicCrossingVolumes, error) {
	var arrangement SplineGraphPolynomialCubicCrossing
	var volumes polynomialPositiveVolumes
	mode := polynomialGraphContactMode{
		cubicCrossings: 1, polynomialHeight: true,
		certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
			if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, b); e != nil {
				return e
			}
			var e error
			volumes, e = integratePolynomialPositiveLayers(ctx, arrangement.Power, arrangement.Regions[:], []int{3}, roots, domain, pair.heightScale, b)
			return e
		},
	}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialCubicCrossingVolumes{}, e
	}
	arrangement.Proven, arrangement.ExactOperations = true, pair.ExactOperations
	return SplineGraphPolynomialCubicCrossingVolumes{
		Arrangement: arrangement, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total, ExactOperations: pair.ExactOperations,
	}, nil
}
