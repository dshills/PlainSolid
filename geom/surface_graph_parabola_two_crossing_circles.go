package geom

import (
	"context"
	"math/big"
	"slices"
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
		FreeAxis: freeAxis, RegionSides: [6]int{-sign, sign, sign, -sign, sign, -sign},
	}
	var err error
	out.ParabolaControls, out.ParabolaExits, err = originalCrossingParabola(ctx, patches, freeAxis, p, b)
	if err != nil {
		return nil, err
	}
	for i, circle := range circles {
		disk, err := proveParabolaCrossingDisk(ctx, patches, freeAxis, p, circle, i, maxCells, visits, b)
		if err != nil {
			return nil, err
		}
		out.Circles[i], out.CircleRadii[i], out.CircleProjections[i] = disk.circle, disk.radius, disk.projection
		out.Nodes[i], out.CircleCCWRays[i] = disk.nodes, disk.ccwRays
		copy(out.RegionWitnesses[2+2*i:], disk.witnesses[:])
	}
	if err := orderParabolaCrossingIntervals(freeAxis, out.Nodes[:], out.CircleOrder[:]); err != nil {
		return nil, err
	}
	witnesses, err := parabolaCrossingExteriorWitnesses(freeAxis, p, out.CircleProjections[:], otherProjectionLeft, b)
	if err != nil {
		return nil, err
	}
	copy(out.RegionWitnesses[:2], witnesses[:])
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}

func orderParabolaCrossingIntervals(axis int, nodes [][2]ParabolaCircleCrossing, order []int) error {
	for i := range nodes {
		order[i] = i
	}
	slices.SortFunc(order, func(i, j int) int {
		return nodes[i][0].Parameters[axis].Cmp(nodes[j][0].Parameters[axis])
	})
	for i := 1; i < len(order); i++ {
		earlier, later := nodes[order[i-1]], nodes[order[i]]
		if earlier[1].Parameters[axis].Cmp(later[0].Parameters[axis]) >= 0 {
			return ErrConvergence
		}
	}
	return nil
}

func parabolaCrossingExteriorWitnesses(axis int, p rationalPolynomial, projections [][2]*big.Rat,
	otherProjectionLeft *big.Rat, b *polynomialBudget) (witnesses [2][2]*big.Rat, err error) {
	x := projections[0][0]
	for _, projection := range projections[1:] {
		if projection[0].Cmp(x) < 0 {
			x = projection[0]
		}
	}
	if otherProjectionLeft != nil && otherProjectionLeft.Cmp(x) < 0 {
		x = otherProjectionLeft
	}
	x = new(big.Rat).Quo(x, big.NewRat(2, 1))
	if err := b.check(x); err != nil {
		return witnesses, err
	}
	y, err := polynomialValue(p, x, b)
	if err != nil {
		return witnesses, err
	}
	for i := range witnesses {
		mid := new(big.Rat).Quo(new(big.Rat).Add(y, big.NewRat(int64(i), 1)), big.NewRat(2, 1))
		if err := b.check(mid); err != nil {
			return witnesses, err
		}
		witnesses[i] = parabolaTwoCrossingCirclesUV(axis, x, mid)
	}
	return witnesses, nil
}

func parabolaTwoCrossingCirclesUV(axis int, x, y *big.Rat) (uv [2]*big.Rat) {
	uv[axis], uv[1-axis] = new(big.Rat).Set(x), new(big.Rat).Set(y)
	return uv
}

func originalCrossingParabola(ctx context.Context, patches [2]*SurfacePatch, axis int,
	p rationalPolynomial, b *polynomialBudget) (controls [3][2]*big.Rat, exits [2][3]*big.Rat, err error) {
	two := big.NewRat(2, 1)
	end := new(big.Rat).Add(p[0], new(big.Rat).Add(p[1], p[2]))
	station := new(big.Rat).Quo(new(big.Rat).Neg(p[1]), new(big.Rat).Mul(two, p[2]))
	if err := b.check(station); err != nil {
		return controls, exits, err
	}
	one := big.NewRat(1, 1)
	extrema := []*big.Rat{p[0], end}
	if station.Sign() > 0 && station.Cmp(one) < 0 {
		y, err := polynomialValue(p, station, b)
		if err != nil {
			return controls, exits, err
		}
		extrema = append(extrema, y)
	}
	for _, y := range extrema {
		if y.Sign() <= 0 || y.Cmp(one) >= 0 {
			return controls, exits, ErrConvergence
		}
	}
	for i, y := range [3]*big.Rat{p[0], new(big.Rat).Add(p[0], new(big.Rat).Quo(p[1], two)), end} {
		if err := b.check(y); err != nil {
			return controls, exits, err
		}
		controls[i] = parabolaTwoCrossingCirclesUV(axis, big.NewRat(int64(i), 2), y)
	}
	for i, end := range [2]int{0, 2} {
		jets, err := parabolaCircleOriginalJets(ctx, patches, controls[end], b)
		if err != nil {
			return controls, exits, err
		}
		exits[i] = jets[0].Point
	}
	return controls, exits, nil
}

type parabolaCrossingDisk struct {
	circle     GraphContactCircle
	radius     *big.Rat
	projection [2]*big.Rat
	nodes      [2]ParabolaCircleCrossing
	ccwRays    [2]int
	witnesses  [2][2]*big.Rat
}

func proveParabolaCrossingDisk(ctx context.Context, patches [2]*SurfacePatch, axis int, p rationalPolynomial,
	circle GraphContactCircle, index, maxCells int, visits *int, b *polynomialBudget) (parabolaCrossingDisk, error) {
	var out parabolaCrossingDisk
	r, projection, err := graphCircleStockProjection(circle, b)
	if err != nil {
		return parabolaCrossingDisk{}, err
	}
	out.radius, out.projection = r, projection
	out.circle = GraphContactCircle{
		Center:        parabolaTwoCrossingCirclesUV(axis, circle.Center[0], circle.Center[1]),
		RadiusSquared: circle.RadiusSquared,
	}
	q, err := parabolaCircleRestriction(p, circle.Center, circle.RadiusSquared, b)
	if err != nil {
		return parabolaCrossingDisk{}, err
	}
	roots, initial, err := discoverGraphPolynomialPairRoots(ctx, q, polynomialGraphContactMode{crossings: 2}, maxCells, visits, b)
	if err != nil {
		return parabolaCrossingDisk{}, err
	}
	if initial != 1 {
		return parabolaCrossingDisk{}, ErrConvergence
	}
	// Strict disk containment gives a positive initial sign. Exactly two
	// simple roots force a single full open interval inside the disk.
	for i, x := range roots {
		y, err := polynomialValue(p, x, b)
		if err != nil {
			return parabolaCrossingDisk{}, err
		}
		slope := new(big.Rat).Add(p[1], new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(p[2], x)))
		if err := b.check(slope); err != nil {
			return parabolaCrossingDisk{}, err
		}
		regions := [4]int{0, 1, 2 + 2*index, 3 + 2*index}
		out.nodes[i], out.ccwRays[i], err =
			originalParabolaCrossingCircleNode(ctx, patches, x, y, slope, circle.Center, axis, regions, b)
		if err != nil {
			return parabolaCrossingDisk{}, err
		}
	}
	x := new(big.Rat).Quo(new(big.Rat).Add(roots[0], roots[1]), big.NewRat(2, 1))
	if err := b.check(x); err != nil {
		return parabolaCrossingDisk{}, err
	}
	y, err := polynomialValue(p, x, b)
	if err != nil {
		return parabolaCrossingDisk{}, err
	}
	gap, err := polynomialValue(q, x, b)
	if err != nil {
		return parabolaCrossingDisk{}, err
	}
	if gap.Sign() >= 0 {
		return parabolaCrossingDisk{}, ErrConvergence
	}
	// Inside the unit stock |y-cy|<1, and |gap|<1. Moving by
	// |gap|/4 changes the squared distance by less than |gap|.
	step := new(big.Rat).Quo(gap, big.NewRat(4, 1))
	for i := range 2 {
		ordinate := new(big.Rat).Add(y, step)
		if err := b.check(ordinate); err != nil {
			return parabolaCrossingDisk{}, err
		}
		out.witnesses[i] = parabolaTwoCrossingCirclesUV(axis, x, ordinate)
		step.Neg(step)
	}
	return out, nil
}

func originalParabolaCrossingCircleNode(ctx context.Context, patches [2]*SurfacePatch, x, y, slope *big.Rat,
	center [2]*big.Rat, axis int, regions [4]int, b *polynomialBudget) (ParabolaCircleCrossing, int, error) {
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
		region := 0
		if radialGap.Sign() < 0 {
			region = 2
		}
		if dependentGap.Sign() > 0 {
			region++
		}
		out.SectorRegions[i] = regions[region]
	}
	return out, circleRay, nil
}
