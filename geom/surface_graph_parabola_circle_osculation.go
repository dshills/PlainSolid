package geom

import (
	"context"
	"math/big"
)

// ParabolaCircleOsculationOwner binds the original roof parameters and the
// lifted contact jets. FourthJetGap is the circle minus parabola fourth
// derivative in the original free-coordinate clock.
type ParabolaCircleOsculationOwner struct {
	ParabolaCircleApexOwner
	FourthJetGap [3]*big.Rat
}

// SplineGraphParabolaCircleOsculation proves a sole interior contact of
// multiplicity four. The disk and opposite open region have only that
// point in common in their closures. RegionSides records the sign of
// A.Z−B.Z below, above and inside the disk, excluding the disk from each
// open region. All coordinates and jets use the original owner charts.
type SplineGraphParabolaCircleOsculation struct {
	FreeAxis                    int
	ParabolaControls            [3][2]*big.Rat
	ParabolaExits               [2][3]*big.Rat
	CircleCenter                [2]*big.Rat
	RadiusSquared               *big.Rat
	DiskRegion                  int
	RegionSides                 [3]int
	Parameters                  [2]*big.Rat
	Point                       [3]*big.Rat
	Owners                      [2]ParabolaCircleOsculationOwner
	SharedSecondDerivative      *big.Rat
	CircleFourthDerivative      *big.Rat
	SubstitutedFourthDerivative *big.Rat
	ExactOperations             int
}

// DiscoverSplineGraphParabolaCircleOsculation proves the complete original
// numerator is K*(y-p(x))*Q(x,y), for the original circle law Q. At the
// circle center's free station x0 the parabola must have its stationary
// point s. In the local clock t=x−x0, p=s+k*t² and the circle center is
// (x0,s+δ), where δ²=r² and 2*k*δ=1. Then Q(x,p(x))=k²*t⁴ on the
// whole domain. The parabola and circle facing germ have identical first
// through third derivatives, and their fourth derivative gap is 12*k²/δ.
// Strict extrema prove the circle and wall-to-wall parabola stay in the
// stock between their original endpoints.
func DiscoverSplineGraphParabolaCircleOsculation(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaCircleOsculation, error) {
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
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	p, center, r2, sign, err := unitParabolaCircleFactors(ctx, a, c, freeAxis, b)
	if err != nil {
		return nil, err
	}
	one, two := big.NewRat(1, 1), big.NewRat(2, 1)
	for _, distance := range [4]*big.Rat{center[0], center[1], new(big.Rat).Sub(one, center[0]), new(big.Rat).Sub(one, center[1])} {
		square := new(big.Rat).Mul(distance, distance)
		if err := b.check(square); err != nil {
			return nil, err
		}
		if distance.Sign() <= 0 || square.Cmp(r2) <= 0 {
			return nil, ErrConvergence
		}
	}
	k := p[2]
	second := new(big.Rat).Mul(two, k)
	slope := new(big.Rat).Add(p[1], new(big.Rat).Mul(second, center[0]))
	s, err := polynomialValue(p, center[0], b)
	if err != nil {
		return nil, err
	}
	delta := new(big.Rat).Sub(center[1], s)
	verticalRadius := new(big.Rat).Mul(delta, delta)
	curvatureMatch := new(big.Rat).Mul(second, delta)
	exit := new(big.Rat).Add(p[0], new(big.Rat).Add(p[1], k))
	mid := new(big.Rat).Add(p[0], new(big.Rat).Quo(p[1], two))
	for _, q := range []*big.Rat{slope, verticalRadius, curvatureMatch, exit, mid} {
		if err := b.check(q); err != nil {
			return nil, err
		}
	}
	if slope.Sign() != 0 || verticalRadius.Cmp(r2) != 0 || curvatureMatch.Cmp(one) != 0 {
		return nil, ErrConvergence
	}
	// These are all true extrema and endpoints. The middle Bernstein
	// coefficient of an osculating parabola can lie outside the stock.
	for _, y := range [3]*big.Rat{p[0], s, exit} {
		if y.Sign() <= 0 || y.Cmp(one) >= 0 {
			return nil, ErrConvergence
		}
	}
	fourth := new(big.Rat).Mul(k, k)
	circleFourth := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(12, 1), fourth), delta)
	fourth.Mul(fourth, big.NewRat(24, 1))
	for _, q := range [3]*big.Rat{second, circleFourth, fourth} {
		if err := b.check(q); err != nil {
			return nil, err
		}
	}
	out := &SplineGraphParabolaCircleOsculation{
		FreeAxis: freeAxis, RadiusSquared: r2, RegionSides: [3]int{-sign, sign},
		SharedSecondDerivative: second, CircleFourthDerivative: circleFourth, SubstitutedFourthDerivative: fourth,
	}
	if delta.Sign() > 0 {
		out.DiskRegion = 1
	}
	out.RegionSides[2] = -out.RegionSides[out.DiskRegion]
	dependent := 1 - freeAxis
	out.CircleCenter[freeAxis], out.CircleCenter[dependent] = center[0], center[1]
	out.Parameters[freeAxis], out.Parameters[dependent] = new(big.Rat).Set(center[0]), s
	for i, y := range [3]*big.Rat{p[0], mid, exit} {
		out.ParabolaControls[i][freeAxis], out.ParabolaControls[i][dependent] = big.NewRat(int64(i), 2), y
	}
	patches := [2]*SurfacePatch{a, c}
	for i, control := range [2]int{0, 2} {
		jets, err := parabolaCircleOriginalJets(ctx, patches, out.ParabolaControls[control], b)
		if err != nil {
			return nil, err
		}
		out.ParabolaExits[i] = jets[0].Point
	}
	jets, err := parabolaCircleOriginalJets(ctx, patches, out.Parameters, b)
	if err != nil {
		return nil, err
	}
	out.Point = jets[0].Point
	for i := range out.Point {
		if jets[0].Du[i].Cmp(jets[1].Du[i]) != 0 || jets[0].Dv[i].Cmp(jets[1].Dv[i]) != 0 {
			return nil, ErrConvergence
		}
	}
	for i, jet := range jets {
		owner := ParabolaCircleOsculationOwner{ParabolaCircleApexOwner: ParabolaCircleApexOwner{
			Parameters:  [2]*big.Rat{new(big.Rat).Set(out.Parameters[0]), new(big.Rat).Set(out.Parameters[1])},
			FreeTangent: jet.Du, DependentPartial: jet.Dv,
		}}
		if freeAxis == 1 {
			owner.FreeTangent, owner.DependentPartial = jet.Dv, jet.Du
		}
		// Both traces have identical chart jets through order three. In
		// their first unequal lift only the owner's dependent partial survives.
		for j, q := range owner.DependentPartial {
			owner.FourthJetGap[j] = new(big.Rat).Mul(q, circleFourth)
			if err := b.check(owner.FourthJetGap[j]); err != nil {
				return nil, err
			}
		}
		out.Owners[i] = owner
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}
