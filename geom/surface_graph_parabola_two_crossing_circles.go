package geom

import (
	"context"
	"math/big"
)

// SplineGraphParabolaTwoCrossingCircles proves a complete parabola and two
// strictly disjoint circles on the original common unit XY chart. Each circle
// has two simple intersections in increasing free coordinate. Circles are
// indexed by free then dependent center. CircleOrder records the order in
// which their interiors meet the parabola. CircleProjections are the closed
// free-coordinate projections of the full disks, which may overlap.
// RegionSides gives A.Z−B.Z for the two exterior regions below and above the
// parabola, followed by the lower and upper caps of each circle in index order.
type SplineGraphParabolaTwoCrossingCircles struct {
	FreeAxis          int
	ParabolaControls  [3][2]*big.Rat
	ParabolaExits     [2][3]*big.Rat
	Circles           [2]GraphContactCircle
	CircleRadii       [2]*big.Rat
	CircleProjections [2][2]*big.Rat
	CircleOrder       [2]int
	Nodes             [2][2]ParabolaCircleCrossing
	CircleCCWRays     [2][2]int // index of the outgoing physical counterclockwise circle ray
	RegionSides       [6]int
	RegionWitnesses   [6][2]*big.Rat
	ExactOperations   int
}

// DiscoverSplineGraphParabolaTwoCrossingCircles recovers the complete original
// residual K*(y-p(x))*Q0*Q1. Each quartic Qi(x,p(x)) has exactly two rational
// simple roots on the stock. The original parabola is inside each disk only
// between its two roots, and those two intervals are strictly separated. Its
// embedded arc across each disk divides that disk into two caps. Replacing
// each open interior parabola segment by the corresponding circular boundary
// arc gives a simple strip boundary on that side. The bounded disk interiors
// and the stock walls are disjoint, so the strips and four caps exhaust the
// signed complement. All returned rationals are caller-owned.
func DiscoverSplineGraphParabolaTwoCrossingCircles(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaTwoCrossingCircles, error) {
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
	p, circles, sign, err := unitParabolaTwoCircleFactors(ctx, a, c, freeAxis, b)
	if err != nil {
		return nil, err
	}
	if err := proveDisjointGraphCircles(circles[0], circles[1], b); err != nil {
		return nil, err
	}
	var visits int
	return proveTwoCrossingCirclePartition(ctx, [2]*SurfacePatch{a, c}, freeAxis, p, circles, sign, nil, o.MaxCells, &visits, b)
}

func proveTwoCrossingCirclePartition(ctx context.Context, patches [2]*SurfacePatch, freeAxis int, p rationalPolynomial,
	circles [2]GraphContactCircle, sign int, otherProjectionLeft *big.Rat, maxCells int, visits *int,
	b *polynomialBudget) (*SplineGraphParabolaTwoCrossingCircles, error) {
	out := &SplineGraphParabolaTwoCrossingCircles{
		FreeAxis: freeAxis, CircleOrder: [2]int{0, 1}, RegionSides: [6]int{-sign, sign, sign, -sign, sign, -sign},
	}
	if err := out.bindOriginalParabola(ctx, patches, p, b); err != nil {
		return nil, err
	}
	for i, circle := range circles {
		if err := out.bindCrossingCircle(ctx, patches, p, circle, i, maxCells, visits, b); err != nil {
			return nil, err
		}
	}
	lo := out.Nodes[0][0].Parameters[freeAxis]
	if lo.Cmp(out.Nodes[1][0].Parameters[freeAxis]) > 0 {
		out.CircleOrder = [2]int{1, 0}
	}
	earlier, later := out.CircleOrder[0], out.CircleOrder[1]
	if out.Nodes[earlier][1].Parameters[freeAxis].Cmp(out.Nodes[later][0].Parameters[freeAxis]) >= 0 {
		return nil, ErrConvergence
	}
	x := out.CircleProjections[0][0]
	if end := out.CircleProjections[1][0]; end.Cmp(x) < 0 {
		x = end
	}
	if otherProjectionLeft != nil && otherProjectionLeft.Cmp(x) < 0 {
		x = otherProjectionLeft
	}
	x = new(big.Rat).Quo(x, big.NewRat(2, 1))
	if err := b.check(x); err != nil {
		return nil, err
	}
	y, err := polynomialValue(p, x, b)
	if err != nil {
		return nil, err
	}
	for i := range 2 {
		mid := new(big.Rat).Quo(new(big.Rat).Add(y, big.NewRat(int64(i), 1)), big.NewRat(2, 1))
		if err := b.check(mid); err != nil {
			return nil, err
		}
		out.RegionWitnesses[i] = parabolaTwoCrossingCirclesUV(freeAxis, x, mid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}

func parabolaTwoCrossingCirclesUV(axis int, x, y *big.Rat) (uv [2]*big.Rat) {
	uv[axis], uv[1-axis] = new(big.Rat).Set(x), new(big.Rat).Set(y)
	return uv
}

func (out *SplineGraphParabolaTwoCrossingCircles) bindOriginalParabola(ctx context.Context, patches [2]*SurfacePatch,
	p rationalPolynomial, b *polynomialBudget) error {
	two := big.NewRat(2, 1)
	end := new(big.Rat).Add(p[0], new(big.Rat).Add(p[1], p[2]))
	station := new(big.Rat).Quo(new(big.Rat).Neg(p[1]), new(big.Rat).Mul(two, p[2]))
	if err := b.check(station); err != nil {
		return err
	}
	one := big.NewRat(1, 1)
	extrema := []*big.Rat{p[0], end}
	if station.Sign() > 0 && station.Cmp(one) < 0 {
		y, err := polynomialValue(p, station, b)
		if err != nil {
			return err
		}
		extrema = append(extrema, y)
	}
	for _, y := range extrema {
		if y.Sign() <= 0 || y.Cmp(one) >= 0 {
			return ErrConvergence
		}
	}
	for i, y := range [3]*big.Rat{p[0], new(big.Rat).Add(p[0], new(big.Rat).Quo(p[1], two)), end} {
		if err := b.check(y); err != nil {
			return err
		}
		out.ParabolaControls[i] = parabolaTwoCrossingCirclesUV(out.FreeAxis, big.NewRat(int64(i), 2), y)
	}
	for i, end := range [2]int{0, 2} {
		jets, err := parabolaCircleOriginalJets(ctx, patches, out.ParabolaControls[end], b)
		if err != nil {
			return err
		}
		out.ParabolaExits[i] = jets[0].Point
	}
	return nil
}

func (out *SplineGraphParabolaTwoCrossingCircles) bindCrossingCircle(ctx context.Context, patches [2]*SurfacePatch,
	p rationalPolynomial, circle GraphContactCircle, index, maxCells int, visits *int, b *polynomialBudget) error {
	r, projection, err := graphCircleStockProjection(circle, b)
	if err != nil {
		return err
	}
	out.CircleRadii[index], out.CircleProjections[index] = r, projection
	out.Circles[index] = GraphContactCircle{
		Center:        parabolaTwoCrossingCirclesUV(out.FreeAxis, circle.Center[0], circle.Center[1]),
		RadiusSquared: circle.RadiusSquared,
	}
	q, err := parabolaCircleRestriction(p, circle.Center, circle.RadiusSquared, b)
	if err != nil {
		return err
	}
	roots, initial, err := discoverGraphPolynomialPairRoots(ctx, q, polynomialGraphContactMode{crossings: 2}, maxCells, visits, b)
	if err != nil {
		return err
	}
	if initial != 1 {
		return ErrConvergence
	}
	// Strict disk containment gives a positive initial sign. Exactly two
	// simple roots force a single full open interval inside the disk.
	for i, x := range roots {
		y, err := polynomialValue(p, x, b)
		if err != nil {
			return err
		}
		slope := new(big.Rat).Add(p[1], new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(p[2], x)))
		if err := b.check(slope); err != nil {
			return err
		}
		out.Nodes[index][i], out.CircleCCWRays[index][i], err =
			twoCrossingCirclesOriginalNode(ctx, patches, x, y, slope, circle.Center, out.FreeAxis, index, b)
		if err != nil {
			return err
		}
	}
	x := new(big.Rat).Quo(new(big.Rat).Add(roots[0], roots[1]), big.NewRat(2, 1))
	if err := b.check(x); err != nil {
		return err
	}
	y, err := polynomialValue(p, x, b)
	if err != nil {
		return err
	}
	gap, err := polynomialValue(q, x, b)
	if err != nil {
		return err
	}
	if gap.Sign() >= 0 {
		return ErrConvergence
	}
	// Inside the unit stock |y-cy|<1, and |gap|<1. Moving by
	// |gap|/4 changes the squared distance by less than |gap|.
	step := new(big.Rat).Quo(gap, big.NewRat(4, 1))
	for i := range 2 {
		ordinate := new(big.Rat).Add(y, step)
		if err := b.check(ordinate); err != nil {
			return err
		}
		out.RegionWitnesses[2+2*index+i] = parabolaTwoCrossingCirclesUV(out.FreeAxis, x, ordinate)
		step.Neg(step)
	}
	return nil
}

func twoCrossingCirclesOriginalNode(ctx context.Context, patches [2]*SurfacePatch, x, y, slope *big.Rat,
	center [2]*big.Rat, axis, circle int, b *polynomialBudget) (ParabolaCircleCrossing, int, error) {
	uv := parabolaTwoCrossingCirclesUV(axis, x, y)
	jets, err := parabolaCircleOriginalJets(ctx, patches, uv, b)
	if err != nil {
		return ParabolaCircleCrossing{}, 0, err
	}
	for dim := range 3 {
		if jets[0].Du[dim].Cmp(jets[1].Du[dim]) != 0 || jets[0].Dv[dim].Cmp(jets[1].Dv[dim]) != 0 {
			return ParabolaCircleCrossing{}, 0, ErrConvergence
		}
	}
	dx, dy := new(big.Rat).Sub(x, center[0]), new(big.Rat).Sub(y, center[1])
	determinant := new(big.Rat).Add(dx, new(big.Rat).Mul(slope, dy))
	if err := b.check(determinant); err != nil {
		return ParabolaCircleCrossing{}, 0, err
	}
	if determinant.Sign() == 0 {
		return ParabolaCircleCrossing{}, 0, ErrDegenerate
	}
	tx, ty := new(big.Rat).Neg(dy), new(big.Rat).Set(dx)
	circleRay := 1
	if determinant.Sign() < 0 {
		circleRay = 3
	}
	// The swap reverses both the physical circle orientation and the
	// orientation of the (parabola, circle) ray pair.
	if (determinant.Sign() < 0) != (axis == 1) {
		tx.Neg(tx)
		ty.Neg(ty)
	}
	rays := [4][2]*big.Rat{{big.NewRat(1, 1), slope}, {tx, ty}, {big.NewRat(-1, 1), new(big.Rat).Neg(slope)},
		{new(big.Rat).Neg(tx), new(big.Rat).Neg(ty)}}
	out := ParabolaCircleCrossing{Parameters: uv, Point: jets[0].Point}
	for i, ray := range rays {
		direction := parabolaTwoCrossingCirclesUV(axis, ray[0], ray[1])
		for dim := range 3 {
			value := new(big.Rat).Mul(jets[0].Du[dim], direction[0])
			value.Add(value, new(big.Rat).Mul(jets[0].Dv[dim], direction[1]))
			if err := b.check(value); err != nil {
				return ParabolaCircleCrossing{}, 0, err
			}
			out.Rays[i][dim] = value
		}
		next := rays[(i+1)%4]
		wx, wy := new(big.Rat).Add(ray[0], next[0]), new(big.Rat).Add(ray[1], next[1])
		dependentGap := new(big.Rat).Sub(wy, new(big.Rat).Mul(slope, wx))
		radialGap := new(big.Rat).Add(new(big.Rat).Mul(dx, wx), new(big.Rat).Mul(dy, wy))
		for _, gap := range [2]*big.Rat{dependentGap, radialGap} {
			if err := b.check(gap); err != nil {
				return ParabolaCircleCrossing{}, 0, err
			}
			if gap.Sign() == 0 {
				return ParabolaCircleCrossing{}, 0, ErrConvergence
			}
		}
		if radialGap.Sign() < 0 {
			out.SectorRegions[i] = 2 + 2*circle
		}
		if dependentGap.Sign() > 0 {
			out.SectorRegions[i]++
		}
	}
	return out, circleRay, nil
}
