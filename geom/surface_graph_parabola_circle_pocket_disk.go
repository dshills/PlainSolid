package geom

import (
	"context"
	"math/big"
)

// SplineGraphParabolaCirclePocketDisk certifies an original four-crossing
// circle and a second strictly disjoint circle wholly inside its bounded
// pocket. Circles are in original free/dependent center order and their
// centers use physical UV. Crossing keeps its four original junctions
// and six region indices, with region 2's witness outside the new disk.
// Region 6 is the isolated disk, removed from region 2.
type SplineGraphParabolaCirclePocketDisk struct {
	Crossing         SplineGraphParabolaCircleFourCrossings
	Circles          [2]GraphContactCircle
	CrossingCircle   int
	PocketCircle     int
	PocketRadius     *big.Rat
	PocketProjection [2]*big.Rat
	PocketExtrema    [4]ParabolaSeparatedCircleVertex // physical +X, +Y, -X, -Y
	RegionSides      [7]int
	RegionWitnesses  [7][2]*big.Rat
	ExactOperations  int
}

// DiscoverSplineGraphParabolaCirclePocketDisk authenticates the full
// original K*(y-p(x))*Q0*Q1 on shared unit XY charts. One circle has the
// four simple same-semicircle junctions of the original pocket. The other
// has no parabola intersections and strict disk separation. Its center
// and entire projection lie inside the bounds of the bounded component,
// so its whole disk is contained in the open pocket. Removing its closed
// disk leaves a connected region with an inner circular boundary. Its
// strict interior reverses only that pocket sign. All returned rationals
// are caller-owned.
func DiscoverSplineGraphParabolaCirclePocketDisk(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaCirclePocketDisk, error) {
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
	crossing, inner := -1, -1
	var restrictions [2]rationalPolynomial
	for i, circle := range circles {
		q, err := parabolaCircleRestriction(p, circle.Center, circle.RadiusSquared, b)
		if err != nil {
			return nil, err
		}
		restrictions[i] = q
		sequence, err := polynomialSturm(q, b)
		if err != nil {
			return nil, err
		}
		count, err := sturmCount(sequence, new(big.Rat), big.NewRat(1, 1), b)
		if err != nil {
			return nil, err
		}
		switch count {
		case 0:
			inner = i
		case 4:
			crossing = i
		default:
			return nil, ErrConvergence
		}
	}
	if crossing < 0 || inner < 0 {
		return nil, ErrConvergence
	}
	patches := [2]*SurfacePatch{a, c}
	proof, err := proveParabolaCircleFourCrossingFactors(ctx, patches, freeAxis, p, circles[crossing], sign, o.MaxCells, b)
	if err != nil {
		return nil, err
	}
	radius, projection, err := graphCircleStockProjection(circles[inner], b)
	if err != nil {
		return nil, err
	}
	left, right := proof.Nodes[1].Parameters[freeAxis], proof.Nodes[2].Parameters[freeAxis]
	if projection[0].Cmp(left) <= 0 || projection[1].Cmp(right) >= 0 {
		return nil, ErrConvergence
	}
	center := circles[inner].Center
	largeY := circles[crossing].Center[1]
	height, err := polynomialValue(p, center[0], b)
	if err != nil {
		return nil, err
	}
	if center[1].Cmp(height) != -proof.CircleSide || center[1].Cmp(largeY) != proof.CircleSide {
		return nil, ErrConvergence
	}
	// At a free station before the nested disk, the pocket witness
	// retains its original roof order irrespective of the inner radius.
	x := new(big.Rat).Quo(new(big.Rat).Add(left, projection[0]), big.NewRat(2, 1))
	if err := b.check(x); err != nil {
		return nil, err
	}
	proof.RegionWitnesses[2], err = proof.circleWitnessAt(p, restrictions[crossing], x, largeY, false, -proof.CircleSide, b)
	if err != nil {
		return nil, err
	}
	out := &SplineGraphParabolaCirclePocketDisk{Crossing: *proof, CrossingCircle: crossing, PocketCircle: inner,
		PocketRadius: radius, PocketProjection: projection}
	for i, circle := range circles {
		out.Circles[i] = GraphContactCircle{Center: parabolaTwoCrossingCirclesUV(freeAxis, circle.Center[0], circle.Center[1]),
			RadiusSquared: circle.RadiusSquared}
	}
	copy(out.RegionSides[:], proof.RegionSides[:])
	copy(out.RegionWitnesses[:], proof.RegionWitnesses[:])
	out.RegionSides[6] = -out.RegionSides[2]
	out.RegionWitnesses[6] = parabolaTwoCrossingCirclesUV(freeAxis, center[0], center[1])
	out.PocketExtrema, err = originalSeparatedCircleExtrema(ctx, patches, out.Circles[inner].Center, radius, b)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}
