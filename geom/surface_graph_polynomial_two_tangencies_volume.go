package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialTwoTangenciesVolumes gives the exact positive
// A-minus-B layer volumes in original arrangement order. ExactOperations
// includes source reproof and rational integration.
type SplineGraphPolynomialTwoTangenciesVolumes struct {
	Arrangement     SplineGraphPolynomialTwoTangencies
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphPolynomialTwoTangenciesVolumes re-proves both
// quadratic tangencies on identity XY charts. The original height
// difference needs positive constant cell denominators and the same physical
// leading coefficient in each union cell. It integrates the two exterior
// layers or all three between-branch layers exactly.
func IntegrateSplineGraphPolynomialTwoTangenciesVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialTwoTangenciesVolumes, error) {
	var arrangement SplineGraphPolynomialTwoTangencies
	var volumes polynomialPositiveVolumes
	mode := polynomialGraphContactMode{
		tangencies: 2, polynomialHeight: true,
		certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
			if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, b); e != nil {
				return e
			}
			var e error
			volumes, e = integratePolynomialPositiveLayers(ctx, arrangement.Power, arrangement.Regions[:], []int{2, 2}, roots, domain, pair.heightScale, b)
			return e
		},
	}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphPolynomialTwoTangenciesVolumes{}, e
	}
	arrangement.Proven, arrangement.ExactOperations = true, pair.ExactOperations
	return SplineGraphPolynomialTwoTangenciesVolumes{
		Arrangement: arrangement, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total, ExactOperations: pair.ExactOperations,
	}, nil
}
