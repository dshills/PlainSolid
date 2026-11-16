package geom

import (
	"context"
	"math/big"
)

// ParabolaCircleObliqueOwner records the original parameter and model
// tangent for an increase of one unit in the free coordinate. The
// dependent partial lifts the second-order gap between the curves.
type ParabolaCircleObliqueOwner struct {
	Parameters       [2]*big.Rat
	Tangent          [3]*big.Rat
	DependentPartial [3]*big.Rat
}

// SplineGraphParabolaCircleObliqueTangency proves exactly one interior contact
// with a nonhorizontal shared tangent in free/dependent coordinates. Regions
// 0 and 1 lie below and above the parabola outside the disk, and region 2
// is inside the disk. RegionSides gives the sign of A.Z−B.Z in each region.
// The disk closure touches the opposite region only at the contact. All
// rational coordinates and derivatives are in the original owner charts.
type SplineGraphParabolaCircleObliqueTangency struct {
	FreeAxis                    int
	ParabolaControls            [3][2]*big.Rat
	ParabolaExits               [2][3]*big.Rat
	CircleCenter                [2]*big.Rat
	RadiusSquared               *big.Rat
	DiskRegion                  int
	RegionSides                 [3]int
	Parameters                  [2]*big.Rat
	Point                       [3]*big.Rat
	Owners                      [2]ParabolaCircleObliqueOwner
	Slope                       *big.Rat
	NormalOrdinate              *big.Rat // signed dependent displacement from contact to circle center
	SubstitutedSecondDerivative *big.Rat
	ParabolaSecondDerivative    *big.Rat
	CircleSecondDerivative      *big.Rat
	ExactOperations             int
}

// DiscoverSplineGraphParabolaCircleObliqueTangency factors the original
// homogeneous height numerator as K*(y-p(x))*Q(x,y). It discovers the double
// root from the linear GCD of Q(x,p(x)) and its derivative. At that station,
// write p(x)=s+m*t+k*t², with m nonzero, and the original circle center as
// (x0−m*δ,s+δ). This proves Q(x,p(x))=t²*(1−2*k*δ+(m+k*t)²). The strict
// lower bound 1−2*k*δ>0 excludes all further roots. Strict interior stock
// bounds and the exact normal determine disk containment and original sides.
func DiscoverSplineGraphParabolaCircleObliqueTangency(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaCircleObliqueTangency, error) {
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
	offset := clonePolynomial(p)
	offset[0].Sub(offset[0], center[1])
	squared, err := polynomialProduct(offset, offset, b)
	if err != nil {
		return nil, err
	}
	horizontal := rationalPolynomial{
		new(big.Rat).Sub(new(big.Rat).Mul(center[0], center[0]), r2),
		new(big.Rat).Mul(big.NewRat(-2, 1), center[0]), one,
	}
	quartic, err := graphPolynomialSum(squared, horizontal, 1, b)
	if err != nil {
		return nil, err
	}
	gcd, err := polynomialGCD(quartic, polynomialDerivative(quartic), b)
	if err != nil {
		return nil, err
	}
	if len(gcd) != 2 {
		return nil, ErrConvergence
	}
	x := new(big.Rat).Quo(new(big.Rat).Neg(gcd[0]), gcd[1])
	if err := b.check(x); err != nil {
		return nil, err
	}
	if x.Sign() <= 0 || x.Cmp(one) >= 0 {
		return nil, ErrConvergence
	}
	y, err := polynomialValue(p, x, b)
	if err != nil {
		return nil, err
	}
	second := new(big.Rat).Mul(two, p[2])
	m := new(big.Rat).Add(p[1], new(big.Rat).Mul(second, x))
	delta := new(big.Rat).Sub(center[1], y)
	dx := new(big.Rat).Sub(x, center[0])
	normalX := new(big.Rat).Mul(m, delta)
	lengthFactor := new(big.Rat).Add(one, new(big.Rat).Mul(m, m))
	radiusCheck := new(big.Rat).Mul(lengthFactor, new(big.Rat).Mul(delta, delta))
	lower := new(big.Rat).Sub(one, new(big.Rat).Mul(second, delta))
	for _, v := range []*big.Rat{m, delta, dx, normalX, radiusCheck, lower} {
		if err := b.check(v); err != nil {
			return nil, err
		}
	}
	if m.Sign() == 0 || delta.Sign() == 0 || dx.Cmp(normalX) != 0 || radiusCheck.Cmp(r2) != 0 || lower.Sign() <= 0 {
		return nil, ErrConvergence
	}
	out := &SplineGraphParabolaCircleObliqueTangency{
		FreeAxis: freeAxis, RadiusSquared: r2, Slope: m, NormalOrdinate: delta, RegionSides: [3]int{-sign, sign},
		SubstitutedSecondDerivative: new(big.Rat).Mul(two, new(big.Rat).Sub(lengthFactor, new(big.Rat).Mul(second, delta))),
		ParabolaSecondDerivative:    second, CircleSecondDerivative: new(big.Rat).Quo(lengthFactor, delta),
	}
	for _, v := range []*big.Rat{out.SubstitutedSecondDerivative, out.ParabolaSecondDerivative, out.CircleSecondDerivative} {
		if err := b.check(v); err != nil {
			return nil, err
		}
	}
	if delta.Sign() > 0 {
		out.DiskRegion = 1
	}
	out.RegionSides[2] = -out.RegionSides[out.DiskRegion]
	out.Parameters[freeAxis], out.Parameters[1-freeAxis] = x, y
	out.CircleCenter[freeAxis], out.CircleCenter[1-freeAxis] = center[0], center[1]
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
	for i, j := range [2]int{0, 2} {
		jets, err := parabolaCircleOriginalJets(ctx, patches, out.ParabolaControls[j], b)
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
	for i, value := range jets[0].Du {
		if value.Cmp(jets[1].Du[i]) != 0 || jets[0].Dv[i].Cmp(jets[1].Dv[i]) != 0 {
			return nil, ErrConvergence
		}
	}
	for i, jet := range jets {
		dependent, free := jet.Dv, jet.Du
		if freeAxis == 1 {
			dependent, free = jet.Du, jet.Dv
		}
		owner := ParabolaCircleObliqueOwner{Parameters: [2]*big.Rat{new(big.Rat).Set(out.Parameters[0]), new(big.Rat).Set(out.Parameters[1])},
			DependentPartial: dependent}
		for j, value := range free {
			v := new(big.Rat).Add(value, new(big.Rat).Mul(m, dependent[j]))
			if err := b.check(v); err != nil {
				return nil, err
			}
			owner.Tangent[j] = v
		}
		out.Owners[i] = owner
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}
