package geom

import (
	"context"
	"math/big"
)

// SplineGraphParabolaThreeCircles certifies a stock-spanning parabola and
// three pairwise disjoint original disks. Two circles cross the parabola
// twice. Their six signed regions retain the crossing proof's numbering.
// Region 6 is the third disk, removed from SeparatedRegion. Circles are in
// original free/dependent center order with physical UV coordinates.
type SplineGraphParabolaThreeCircles struct {
	Crossing          SplineGraphParabolaTwoCrossingCircles
	Circles           [3]GraphContactCircle
	CircleProjections [3][2]*big.Rat
	CrossingCircles   [2]int
	SeparatedCircle   int
	SeparatedRadius   *big.Rat
	SeparatedRegion   int
	SeparatedExtrema  [4]ParabolaSeparatedCircleVertex // physical +X, +Y, -X, -Y
	RegionSides       [7]int
	RegionWitnesses   [7][2]*big.Rat
	ExactOperations   int
}

// DiscoverSplineGraphParabolaThreeCircles recovers K*(y-p(x))*Q0*Q1*Q2 from
// the full original homogeneous difference on shared unit XY charts. The
// three circle factors have distinct rational centers and rational radii.
// Each stock-interior disk is classified by the complete quartic Qi(x,p(x)).
// The admitted arrangement has exactly two transverse crossing pairs and
// one separated disk. All source coefficients, branch owners and signs are
// proved before the result is returned.
func DiscoverSplineGraphParabolaThreeCircles(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaThreeCircles, error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || freeAxis < 0 || freeAxis > 1 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := alignedPolynomialProofOptions(o)
	if err != nil {
		return nil, err
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if err := validateUnitGraphCircleCharts(ctx, a, c, 8, b); err != nil {
		return nil, err
	}
	diff, err := orderingDifference(ctx, a, c, 1, b)
	if err != nil {
		return nil, err
	}
	if diff.Domain() != (ParameterRectangle{UMax: 1, VMax: 1}) || diff.countU != diff.degreeU+1 ||
		diff.countV != diff.degreeV+1 || diff.degreeU > 8 || diff.degreeV > 8 {
		return nil, ErrConvergence
	}
	law, err := graphMixedPowerColumns(diff, freeAxis, 8, 7, b)
	if err != nil {
		return nil, err
	}
	p, circleProduct, sign, err := divideGraphParabolaCircleProduct(law, 3, b)
	if err != nil {
		return nil, err
	}
	var visits int
	circles, err := factorGraphCircleProduct(ctx, circleProduct, 3, o.MaxCells, &visits, b)
	if err != nil {
		return nil, err
	}
	out := &SplineGraphParabolaThreeCircles{SeparatedCircle: -1}
	var crossing []GraphContactCircle
	for i, circle := range circles {
		for j := 0; j < i; j++ {
			if err := proveDisjointGraphCircles(circle, circles[j], b); err != nil {
				return nil, err
			}
		}
		radius, projection, err := graphCircleStockProjection(circle, b)
		if err != nil {
			return nil, err
		}
		out.Circles[i] = GraphContactCircle{Center: parabolaTwoCrossingCirclesUV(freeAxis, circle.Center[0], circle.Center[1]),
			RadiusSquared: circle.RadiusSquared}
		out.CircleProjections[i] = projection
		restriction, err := parabolaCircleRestriction(p, circle.Center, circle.RadiusSquared, b)
		if err != nil {
			return nil, err
		}
		sequence, err := polynomialSturm(restriction, b)
		if err != nil {
			return nil, err
		}
		roots, err := sturmCount(sequence, new(big.Rat), big.NewRat(1, 1), b)
		if err != nil {
			return nil, err
		}
		switch roots {
		case 0:
			if out.SeparatedCircle >= 0 {
				return nil, ErrConvergence
			}
			out.SeparatedCircle, out.SeparatedRadius = i, radius
			height, err := polynomialValue(p, circle.Center[0], b)
			if err != nil {
				return nil, err
			}
			order := circle.Center[1].Cmp(height)
			if order == 0 {
				return nil, ErrDegenerate
			}
			if order > 0 {
				out.SeparatedRegion = 1
			}
		case 2:
			if len(crossing) == len(out.CrossingCircles) {
				return nil, ErrConvergence
			}
			out.CrossingCircles[len(crossing)] = i
			crossing = append(crossing, circle)
		default:
			return nil, ErrDegenerate
		}
	}
	if len(crossing) != 2 || out.SeparatedCircle < 0 {
		return nil, ErrConvergence
	}
	patches := [2]*SurfacePatch{a, c}
	index := out.SeparatedCircle
	proof, err := proveTwoCrossingCirclePartition(ctx, patches, freeAxis, p, [2]GraphContactCircle(crossing), sign,
		out.CircleProjections[index][0], o.MaxCells, &visits, b)
	if err != nil {
		return nil, err
	}
	out.Crossing = *proof
	copy(out.RegionSides[:], proof.RegionSides[:])
	copy(out.RegionWitnesses[:], proof.RegionWitnesses[:])
	out.RegionSides[6] = -out.RegionSides[out.SeparatedRegion]
	center := out.Circles[index].Center
	out.RegionWitnesses[6] = parabolaTwoCrossingCirclesUV(freeAxis, center[freeAxis], center[1-freeAxis])
	out.SeparatedExtrema, err = originalSeparatedCircleExtrema(ctx, patches, center, out.SeparatedRadius, b)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}
