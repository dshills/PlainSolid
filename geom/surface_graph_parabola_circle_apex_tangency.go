package geom

import (
	"context"
	"math/big"
)

// ParabolaCircleApexOwner records the original UV and positive free-coordinate
// tangent shared by both original branches on one roof. DependentPartial
// lifts the exact curvature gap from the chart into model coordinates.
type ParabolaCircleApexOwner struct {
	Parameters       [2]*big.Rat
	FreeTangent      [3]*big.Rat
	DependentPartial [3]*big.Rat
}

// SplineGraphParabolaCircleApexTangency describes exactly one interior
// tangency between a wall-to-wall parabola and a circle in the original
// identity XY chart. DiskRegion is 0 below the parabola or 1 above it.
// RegionSides records A.Z−B.Z below, above, and inside the disk. Below
// and above exclude the disk. The disk and opposite open region share
// only the apex in their closures. The enclosing open region has two
// distinct local sectors at the apex. All rationals are caller-owned.
type SplineGraphParabolaCircleApexTangency struct {
	FreeAxis                    int
	ParabolaControls            [3][2]*big.Rat
	ParabolaExits               [2][3]*big.Rat
	CircleCenter                [2]*big.Rat
	RadiusSquared               *big.Rat
	DiskRegion                  int
	RegionSides                 [3]int
	Parameters                  [2]*big.Rat
	Point                       [3]*big.Rat
	Owners                      [2]ParabolaCircleApexOwner
	SubstitutedSecondDerivative *big.Rat // second derivative of the circle law on the parabola
	ParabolaSecondDerivative    *big.Rat // dependent coordinate per squared free coordinate
	CircleSecondDerivative      *big.Rat // circle germ facing the parabola, same clock
	ExactOperations             int
}

// DiscoverSplineGraphParabolaCircleApexTangency proves the complete original
// numerator is K*(y-p(x))*Q, where Q is the circle polynomial. At the
// circle center station cx the original parabola must have its apex s.
// Writing t=x−cx, p=s+k*t² and the circle center ordinate as s+δ, the
// required δ²=r² gives Q(x,p(x))=t²*(1−2*k*δ+k²*t²). Strict positivity of
// 1−2*k*δ proves exactly a double root and excludes every other contact.
// The disk interior is entirely on the center's side. A closed disk point
// can meet the opposite open region's closure only at the original apex.
func DiscoverSplineGraphParabolaCircleApexTangency(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaCircleApexTangency, error) {
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
	second := new(big.Rat).Mul(two, p[2])
	slope := new(big.Rat).Add(p[1], new(big.Rat).Mul(second, center[0]))
	if err := b.check(slope); err != nil {
		return nil, err
	}
	if slope.Sign() != 0 {
		return nil, ErrConvergence
	}
	apex, err := polynomialValue(p, center[0], b)
	if err != nil {
		return nil, err
	}
	delta := new(big.Rat).Sub(center[1], apex)
	square := new(big.Rat).Mul(delta, delta)
	if err := b.check(square); err != nil {
		return nil, err
	}
	if square.Cmp(r2) != 0 {
		return nil, ErrConvergence
	}
	lower := new(big.Rat).Sub(one, new(big.Rat).Mul(second, delta))
	if err := b.check(lower); err != nil {
		return nil, err
	}
	if lower.Sign() <= 0 {
		return nil, ErrConvergence
	}
	out := &SplineGraphParabolaCircleApexTangency{
		FreeAxis: freeAxis, RadiusSquared: r2, RegionSides: [3]int{-sign, sign},
		SubstitutedSecondDerivative: new(big.Rat).Mul(two, lower),
		ParabolaSecondDerivative:    second, CircleSecondDerivative: new(big.Rat).Inv(delta),
	}
	if delta.Sign() > 0 {
		out.DiskRegion = 1
	}
	for _, value := range [3]*big.Rat{out.SubstitutedSecondDerivative, out.ParabolaSecondDerivative, out.CircleSecondDerivative} {
		if err := b.check(value); err != nil {
			return nil, err
		}
	}
	out.RegionSides[2] = -out.RegionSides[out.DiskRegion]
	out.CircleCenter[freeAxis], out.CircleCenter[1-freeAxis] = center[0], center[1]
	out.Parameters[freeAxis], out.Parameters[1-freeAxis] = new(big.Rat).Set(center[0]), apex

	ordinates := [3]*big.Rat{p[0], new(big.Rat).Add(p[0], new(big.Rat).Quo(p[1], two)),
		new(big.Rat).Add(p[0], new(big.Rat).Add(p[1], p[2]))}
	for i, q := range ordinates {
		if err := b.check(q); err != nil {
			return nil, err
		}
		if q.Sign() <= 0 || q.Cmp(one) >= 0 {
			return nil, ErrConvergence
		}
		out.ParabolaControls[i][freeAxis], out.ParabolaControls[i][1-freeAxis] = big.NewRat(int64(i), 2), q
	}
	patches := [2]*SurfacePatch{a, c}
	for i, end := range [2]int{0, 2} {
		jets, err := parabolaCircleOriginalJets(ctx, patches, out.ParabolaControls[end], b)
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
		out.Owners[i] = ParabolaCircleApexOwner{Parameters: [2]*big.Rat{new(big.Rat).Set(out.Parameters[0]), new(big.Rat).Set(out.Parameters[1])},
			FreeTangent: jet.Du, DependentPartial: jet.Dv}
		if freeAxis == 1 {
			out.Owners[i].FreeTangent, out.Owners[i].DependentPartial = jet.Dv, jet.Du
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}
