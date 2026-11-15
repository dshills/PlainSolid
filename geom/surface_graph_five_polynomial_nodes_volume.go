package geom

import (
	"context"
	"math/big"
	"slices"
)

// SplineGraphFivePolynomialNodeVolumes gives the exact positive
// A-minus-B layer volumes in original arrangement order. ExactOperations
// includes source reproof and rational integration.
type SplineGraphFivePolynomialNodeVolumes struct {
	Arrangement     SplineGraphPolynomialArrangement
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateSplineGraphFivePolynomialNodeVolumes re-proves five simple
// crossings on identity XY charts. The original height
// difference needs positive constant cell denominators and the same physical
// leading coefficient in each union cell. It integrates the two exterior
// layers or all six between-branch layers exactly.
func IntegrateSplineGraphFivePolynomialNodeVolumes(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphFivePolynomialNodeVolumes, error) {
	var arrangement SplineGraphPolynomialArrangement
	var volumes polynomialPositiveVolumes
	mode := polynomialGraphContactMode{
		allowQuintic: true, polynomialHeight: true, discoverArrangement: true,
		certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, b *polynomialBudget) error {
			if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, b); e != nil {
				return e
			}
			if !slices.Equal(arrangement.ContactOrder, []int{1, 1, 1, 1, 1}) {
				return ErrConvergence
			}
			var e error
			volumes, e = integratePolynomialPositiveLayers(ctx, arrangement.Power, arrangement.Regions, arrangement.ContactOrder, roots, domain, pair.heightScale, b)
			return e
		},
	}
	pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
	if e != nil {
		return SplineGraphFivePolynomialNodeVolumes{}, e
	}
	arrangement.Proven, arrangement.ExactOperations = true, pair.ExactOperations
	return SplineGraphFivePolynomialNodeVolumes{
		Arrangement: arrangement, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total, ExactOperations: pair.ExactOperations,
	}, nil
}
