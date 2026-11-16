package geom

import (
	"context"
	"math/big"
)

// SplineGraphParabolaCircleFourCrossings proves the original stock-spanning
// parabola meets one strictly interior disk in two disjoint open intervals.
// CircleSide is -1 for four junctions on the lower semicircle, or +1 for
// the upper semicircle. Near denotes that same side of the parabola.
//
// Regions 0 and 1 are the near and far stock-spanning exterior regions.
// Region 2 is the bounded far-side pocket outside the disk. Regions 3 and 4
// are the two near-side disk caps in increasing free order. Region 5 is the
// connected far-side portion of the disk. RegionSides gives sign(A.Z-B.Z).
type SplineGraphParabolaCircleFourCrossings struct {
	FreeAxis         int
	CircleSide       int
	ParabolaControls [3][2]*big.Rat
	ParabolaExits    [2][3]*big.Rat
	Circle           GraphContactCircle
	CircleRadius     *big.Rat
	CircleProjection [2]*big.Rat
	Nodes            [4]ParabolaCircleCrossing
	CircleCCWRays    [4]int
	RegionSides      [6]int
	RegionWitnesses  [6][2]*big.Rat
	ExactOperations  int
}

// DiscoverSplineGraphParabolaCircleFourCrossings certifies the complete
// original difference K*(y-p(x))*Q(x,y) on shared unit XY charts. The
// original quartic Q(x,p(x)) has exactly four simple rational roots, all on
// the same open semicircle. The parabola is inside the disk only between
// the first two and last two roots. Those arcs cut off two disjoint disk
// caps. Between the middle roots the parabola lies outside that semicircle
// and encloses the bounded pocket with the intervening original circle arc.
// The opposite semicircle connects the far-side interior, and the remaining
// exterior regions continue to the stock walls. These six regions exhaust
// the signed complement. Both original owner jets are checked at every
// junction. All returned rationals are caller-owned.
func DiscoverSplineGraphParabolaCircleFourCrossings(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaCircleFourCrossings, error) {
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
	p, center, squared, sign, err := unitParabolaCircleFactors(ctx, a, c, freeAxis, b)
	if err != nil {
		return nil, err
	}
	radius, projection, err := graphCircleStockProjection(GraphContactCircle{Center: center, RadiusSquared: squared}, b)
	if err != nil {
		return nil, err
	}
	q, err := parabolaCircleRestriction(p, center, squared, b)
	if err != nil {
		return nil, err
	}
	var visits int
	roots, initial, err := discoverGraphPolynomialPairRoots(ctx, q, polynomialGraphContactMode{crossings: 4}, o.MaxCells, &visits, b)
	if err != nil {
		return nil, err
	}
	if initial != 1 || roots[0].Cmp(projection[0]) <= 0 || roots[3].Cmp(projection[1]) >= 0 {
		return nil, ErrConvergence
	}
	patches := [2]*SurfacePatch{a, c}
	out := &SplineGraphParabolaCircleFourCrossings{FreeAxis: freeAxis, CircleRadius: radius, CircleProjection: projection,
		Circle: GraphContactCircle{Center: parabolaTwoCrossingCirclesUV(freeAxis, center[0], center[1]), RadiusSquared: squared}}
	out.ParabolaControls, out.ParabolaExits, err = originalCrossingParabola(ctx, patches, freeAxis, p, b)
	if err != nil {
		return nil, err
	}
	if err := out.bindFourCircleJunctions(ctx, patches, p, center, [4]*big.Rat(roots), b); err != nil {
		return nil, err
	}
	near := out.CircleSide * sign
	out.RegionSides = [6]int{near, -near, -near, -near, -near, near}
	if err := out.bindFourCircleWitnesses(p, q, center, [4]*big.Rat(roots), b); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}

func (out *SplineGraphParabolaCircleFourCrossings) bindFourCircleJunctions(ctx context.Context, patches [2]*SurfacePatch,
	p rationalPolynomial, center [2]*big.Rat, roots [4]*big.Rat, b *polynomialBudget) error {
	for i, x := range roots {
		y, err := polynomialValue(p, x, b)
		if err != nil {
			return err
		}
		side := y.Cmp(center[1])
		if side == 0 || (i > 0 && out.CircleSide != side) {
			return ErrConvergence
		}
		out.CircleSide = side
		farExterior := 1
		if i == 1 || i == 2 {
			farExterior = 2
		}
		cap := 3 + i/2
		regions := [4]int{0, farExterior, cap, 5}
		if side > 0 {
			regions = [4]int{farExterior, 0, 5, cap}
		}
		slope := new(big.Rat).Add(p[1], new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(p[2], x)))
		if err := b.check(slope); err != nil {
			return err
		}
		out.Nodes[i], out.CircleCCWRays[i], err = originalParabolaCrossingCircleNode(ctx, patches, x, y, slope, center, out.FreeAxis, regions, b)
		if err != nil {
			return err
		}
	}
	return nil
}

func (out *SplineGraphParabolaCircleFourCrossings) bindFourCircleWitnesses(p, q rationalPolynomial,
	center [2]*big.Rat, roots [4]*big.Rat, b *polynomialBudget) error {
	witnesses, err := parabolaCrossingExteriorWitnesses(out.FreeAxis, p, [][2]*big.Rat{out.CircleProjection}, nil, b)
	if err != nil {
		return err
	}
	near, far := 0, 1
	if out.CircleSide > 0 {
		near, far = 1, 0
	}
	out.RegionWitnesses[0], out.RegionWitnesses[1] = witnesses[near], witnesses[far]
	// The original signed squared distance alternates over the three
	// intervals. Within the unit stock a step of |gap|/4 changes it by
	// less than |gap|, so the witnesses keep their required disk sides.
	stations := [4]int{1, 0, 2, 0}
	for i, interval := range stations {
		x := new(big.Rat).Quo(new(big.Rat).Add(roots[interval], roots[interval+1]), big.NewRat(2, 1))
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
		if (interval == 1 && (gap.Sign() <= 0 || y.Cmp(center[1]) != out.CircleSide)) || (interval != 1 && gap.Sign() >= 0) {
			return ErrConvergence
		}
		step := new(big.Rat).Quo(new(big.Rat).Abs(gap), big.NewRat(int64(4*out.CircleSide), 1))
		if i == 0 || i == 3 {
			step.Neg(step)
		}
		y.Add(y, step)
		if err := b.check(y); err != nil {
			return err
		}
		out.RegionWitnesses[2+i] = parabolaTwoCrossingCirclesUV(out.FreeAxis, x, y)
	}
	return nil
}
