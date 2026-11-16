package geom

import (
	"context"
	"math/big"
)

// ParabolaCircleCrossing records an original shared UV and point and the
// four original outgoing model rays in counterclockwise XY order, starting
// with increasing free parameter on the parabola. SectorRegions[i] is the
// open sector counterclockwise from Rays[i] to Rays[(i+1)%4].
type ParabolaCircleCrossing struct {
	Parameters    [2]*big.Rat
	Point         [3]*big.Rat
	Rays          [4][3]*big.Rat
	SectorRegions [4]int
}

// SplineGraphParabolaCircleCrossings proves two transverse intersections of a
// stock-spanning parabola and a strictly interior circle. Original roof charts
// both map UV identically to XY. Regions 0 and 1 are outside the disk below
// and above the parabola. Regions 2 and 3 are inside the disk below and above
// the parabola. Their height signs are A.Z−B.Z. All rationals are caller-owned.
type SplineGraphParabolaCircleCrossings struct {
	FreeAxis         int
	ParabolaControls [3][2]*big.Rat
	ParabolaExits    [2][3]*big.Rat
	CircleCenter     [2]*big.Rat
	RadiusSquared    *big.Rat
	CircleRadius     *big.Rat
	Nodes            [2]ParabolaCircleCrossing // increasing free coordinate
	RegionSides      [4]int
	RegionWitnesses  [4][2]*big.Rat
	ExactOperations  int
}

// DiscoverSplineGraphParabolaCircleCrossings proves the complete original
// homogeneous difference is K*(y-p(x))*((x-cx)²+(y-cy)²-r²). This bounded
// profile requires p(cx-r)=p(cx+r)=cy, r rational, and the quadratic coefficient
// k of p satisfying k²*r²<1. Thus on the parabola the circle law factors as
// t*(1+k²*t), t=(x-cx)²-r². The second factor is strictly positive throughout
// the stock and the first has exactly two simple roots. The disk is wholly
// inside the stock and the parabola crosses it from one free extremum to the
// other. Its four complementary regions are connected: inside, each circular
// segment is a single interval between the parabola and one semicircle over
// every interior disk station; outside, each strip connects over or under the
// corresponding semicircle and continues to both stock exits.
func DiscoverSplineGraphParabolaCircleCrossings(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaCircleCrossings, error) {
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
	unit := ParameterRectangle{UMax: 1, VMax: 1}
	for _, patch := range [2]*SurfacePatch{a, c} {
		if patch.Domain() != unit || patch.countU != patch.degreeU+1 || patch.countV != patch.degreeV+1 ||
			patch.degreeU > 6 || patch.degreeV > 6 {
			return nil, ErrConvergence
		}
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return nil, ErrConvergence
		}
		graph, work, err := newAffineXYGraphWithWork(ctx, patch, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += work
		if err != nil {
			return nil, err
		}
		for row, diagonal := range [2][3]int64{{1, 0, 0}, {0, 1, 0}} {
			for col, want := range diagonal {
				if graph.inverse[row][col].Cmp(big.NewRat(want, 1)) != 0 {
					return nil, ErrConvergence
				}
			}
		}
	}
	diff, err := orderingDifference(ctx, a, c, 1, b)
	if err != nil {
		return nil, err
	}
	if diff.Domain() != unit || diff.countU != diff.degreeU+1 || diff.countV != diff.degreeV+1 ||
		diff.degreeU > 6 || diff.degreeV > 6 {
		return nil, ErrConvergence
	}
	columns, err := parabolaCirclePowerColumns(diff, freeAxis, b)
	if err != nil {
		return nil, err
	}
	p, center, radiusSquared, sign, err := factorParabolaCircle(columns, b)
	if err != nil {
		return nil, err
	}
	n, d := new(big.Int).Sqrt(radiusSquared.Num()), new(big.Int).Sqrt(radiusSquared.Denom())
	if new(big.Int).Mul(n, n).Cmp(radiusSquared.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(radiusSquared.Denom()) != 0 {
		return nil, ErrUnrepresentable
	}
	radius := new(big.Rat).SetFrac(n, d)
	out := &SplineGraphParabolaCircleCrossings{FreeAxis: freeAxis, RadiusSquared: radiusSquared, CircleRadius: radius,
		RegionSides: [4]int{-sign, sign, sign, -sign}}
	uv := func(x, y *big.Rat) (mapped [2]*big.Rat) {
		mapped[freeAxis], mapped[1-freeAxis] = new(big.Rat).Set(x), new(big.Rat).Set(y)
		return
	}
	out.CircleCenter = uv(center[0], center[1])
	zero, one, two := new(big.Rat), big.NewRat(1, 1), big.NewRat(2, 1)
	for _, coordinate := range center {
		lo, hi := new(big.Rat).Sub(coordinate, radius), new(big.Rat).Add(coordinate, radius)
		if err := b.check(lo); err != nil {
			return nil, err
		}
		if err := b.check(hi); err != nil {
			return nil, err
		}
		if lo.Sign() <= 0 || hi.Cmp(one) >= 0 {
			return nil, ErrConvergence
		}
	}
	ordinates := [3]*big.Rat{p[0], new(big.Rat).Add(p[0], new(big.Rat).Quo(p[1], two)),
		new(big.Rat).Add(p[0], new(big.Rat).Add(p[1], p[2]))}
	for i, q := range ordinates {
		if err := b.check(q); err != nil {
			return nil, err
		}
		if q.Sign() <= 0 || q.Cmp(one) >= 0 {
			return nil, ErrConvergence
		}
		out.ParabolaControls[i] = uv(big.NewRat(int64(i), 2), q)
	}
	quotientLower := new(big.Rat).Mul(p[2], p[2])
	quotientLower.Mul(quotientLower, radiusSquared)
	quotientLower.Sub(one, quotientLower)
	if err := b.check(quotientLower); err != nil {
		return nil, err
	}
	if quotientLower.Sign() <= 0 {
		return nil, ErrConvergence
	}
	for i, x := range [2]*big.Rat{new(big.Rat).Sub(center[0], radius), new(big.Rat).Add(center[0], radius)} {
		y, err := polynomialValue(p, x, b)
		if err != nil {
			return nil, err
		}
		if y.Cmp(center[1]) != 0 {
			return nil, ErrConvergence
		}
		// The two distinct roots of p-cy prove p-cy=k*t. The circle's
		// outgoing jets at both free extrema are vertical and the parabola
		// has unit free derivative, so the two curve tangents are transverse in XY.
		slope := new(big.Rat).Mul(p[2], x)
		slope.Mul(slope, two).Add(slope, p[1])
		if err := b.check(slope); err != nil {
			return nil, err
		}
		node, err := parabolaCircleOriginalCrossing(ctx, [2]*SurfacePatch{a, c}, uv(x, y), uv(one, slope), uv(zero, one), i, freeAxis, b)
		if err != nil {
			return nil, err
		}
		out.Nodes[i] = node
	}
	for i, end := range [2]int{0, 2} {
		jets, err := parabolaCircleOriginalJets(ctx, [2]*SurfacePatch{a, c}, out.ParabolaControls[end], b)
		if err != nil {
			return nil, err
		}
		out.ParabolaExits[i] = jets[0].Point
	}
	outsideX := new(big.Rat).Quo(out.Nodes[0].Parameters[freeAxis], two)
	if err := b.check(outsideX); err != nil {
		return nil, err
	}
	outsideY, err := polynomialValue(p, outsideX, b)
	if err != nil {
		return nil, err
	}
	insideY, err := polynomialValue(p, center[0], b)
	if err != nil {
		return nil, err
	}
	for i, interval := range [4][3]*big.Rat{{outsideX, zero, outsideY}, {outsideX, outsideY, one},
		{center[0], new(big.Rat).Sub(center[1], radius), insideY}, {center[0], insideY, new(big.Rat).Add(center[1], radius)}} {
		if interval[1].Cmp(interval[2]) >= 0 {
			return nil, ErrConvergence
		}
		mid := new(big.Rat).Add(interval[1], interval[2])
		mid.Quo(mid, two)
		if err := b.check(mid); err != nil {
			return nil, err
		}
		out.RegionWitnesses[i] = uv(interval[0], mid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}

func parabolaCircleOriginalJets(ctx context.Context, patches [2]*SurfacePatch, uv [2]*big.Rat,
	b *polynomialBudget) (jets [2]RationalSurfacePatchJet, err error) {
	for owner, patch := range patches {
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return jets, ErrConvergence
		}
		jets[owner], err = patch.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += jets[owner].ExactOperations
		if err != nil {
			return jets, err
		}
	}
	for i, x := range jets[0].Point {
		if x.Cmp(jets[1].Point[i]) != 0 {
			return jets, ErrConvergence
		}
	}
	return jets, nil
}

func parabolaCircleOriginalCrossing(ctx context.Context, patches [2]*SurfacePatch, uv, forward, up [2]*big.Rat,
	node, axis int, b *polynomialBudget) (ParabolaCircleCrossing, error) {
	jets, err := parabolaCircleOriginalJets(ctx, patches, uv, b)
	if err != nil {
		return ParabolaCircleCrossing{}, err
	}
	// Both original roofs have the same first jet at the product zero.
	// Verify it before choosing the model lift of either ray pair.
	for i := range jets[0].Point {
		if jets[0].Du[i].Cmp(jets[1].Du[i]) != 0 || jets[0].Dv[i].Cmp(jets[1].Dv[i]) != 0 {
			return ParabolaCircleCrossing{}, ErrConvergence
		}
	}
	out := ParabolaCircleCrossing{Parameters: uv, Point: jets[0].Point,
		SectorRegions: [2][4]int{{3, 1, 0, 2}, {1, 3, 2, 0}}[node]}
	for j, direction := range [2][2]*big.Rat{forward, up} {
		for i := range out.Point {
			q := new(big.Rat).Mul(jets[0].Du[i], direction[0])
			q.Add(q, new(big.Rat).Mul(jets[0].Dv[i], direction[1]))
			if err := b.check(q); err != nil {
				return ParabolaCircleCrossing{}, err
			}
			out.Rays[j][i], out.Rays[j+2][i] = q, new(big.Rat).Neg(q)
		}
	}
	if axis == 1 {
		out.Rays[1], out.Rays[3] = out.Rays[3], out.Rays[1]
		r := out.SectorRegions
		out.SectorRegions = [4]int{r[3], r[2], r[1], r[0]}
	}
	return out, nil
}
