package geom

import (
	"context"
	"math/big"
	"slices"
)

// AlignedSplineGraphPolynomialVolumes gives each positive graph layer's
// exact physical volume in the arrangement's region order. Reverse means
// the integrated height is B minus A.
type AlignedSplineGraphPolynomialVolumes struct {
	Alignment       AlignedSplineGraphPolynomialArrangement
	Reverse         bool
	Regions         []int
	Volumes         []*big.Rat
	Total           *big.Rat
	ExactOperations int
}

// IntegrateAlignedSplineGraphPolynomialVolumes proves both original charts
// and integrates their positive height difference over each retained region.
// A must be an identity model XY chart. B may be independently parameterized
// over the same projection. Every union-knot cell must have a constant
// positive difference denominator and the same physical quadratic scale.
// The alignment and root order remain in A's chart in both difference orders.
func IntegrateAlignedSplineGraphPolynomialVolumes(ctx context.Context, a, b *SurfacePatch, axis int, reverse bool, o GraphPointContactOptions) (AlignedSplineGraphPolynomialVolumes, error) {
	var volumes polynomialPositiveVolumes
	discover := func(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphPolynomialArrangement, error) {
		var arrangement SplineGraphPolynomialArrangement
		mode := polynomialGraphContactMode{
			allowQuintic: true, polynomialHeight: true, discoverArrangement: true,
			certify: func(ctx context.Context, a *SurfacePatch, pair SplineGraphTwoPolynomials, roots []*big.Rat, domain ParameterRectangle, budget *polynomialBudget) error {
				if e := arrangement.certifyArrangement(ctx, a, pair, roots, domain, budget); e != nil {
					return e
				}
				regions, scale := arrangement.Regions, pair.heightScale
				if reverse {
					regions = slices.Clone(regions)
					for i := range regions {
						regions[i].HeightSide = -regions[i].HeightSide
					}
					scale = new(big.Rat).Neg(scale)
					if e := budget.check(scale); e != nil {
						return e
					}
				}
				var e error
				volumes, e = integratePolynomialPositiveLayers(ctx, arrangement.Power, regions, arrangement.ContactOrder, roots, domain, scale, budget)
				return e
			},
		}
		pair, e := discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, mode)
		if e != nil {
			return SplineGraphPolynomialArrangement{ExactOperations: pair.ExactOperations}, e
		}
		arrangement.Proven, arrangement.ExactOperations = true, pair.ExactOperations
		return arrangement, nil
	}
	proof, e := discoverAlignedSplineGraphPolynomialArrangement(ctx, a, b, axis, o, discover)
	if e != nil {
		return AlignedSplineGraphPolynomialVolumes{ExactOperations: proof.ExactOperations}, e
	}
	return AlignedSplineGraphPolynomialVolumes{
		Alignment: proof, Reverse: reverse, Regions: volumes.regions, Volumes: volumes.volumes, Total: volumes.total,
		ExactOperations: proof.ExactOperations,
	}, nil
}
