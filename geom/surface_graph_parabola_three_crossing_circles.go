package geom

import (
	"context"
	"math/big"
)

// SplineGraphParabolaThreeCrossingCircles proves a complete original parabola
// and three strictly disjoint original circles on a shared unit XY chart.
// Circle indices order the free and dependent center coordinates, while
// CircleOrder gives the order of their interior intervals along the parabola.
// Each pair of nodes is in increasing free coordinate. RegionSides gives
// sign(A.Z-B.Z) for the exterior regions below and above the parabola and
// then for the lower and upper cap of each circle in circle index order.
type SplineGraphParabolaThreeCrossingCircles struct {
	FreeAxis          int
	ParabolaControls  [3][2]*big.Rat
	ParabolaExits     [2][3]*big.Rat
	Circles           [3]GraphContactCircle
	CircleRadii       [3]*big.Rat
	CircleProjections [3][2]*big.Rat
	CircleOrder       [3]int
	Nodes             [3][2]ParabolaCircleCrossing
	CircleCCWRays     [3][2]int // index of the outgoing physical counterclockwise circle ray
	RegionSides       [8]int
	RegionWitnesses   [8][2]*big.Rat
	ExactOperations   int
}

// DiscoverSplineGraphParabolaThreeCrossingCircles recovers the full original
// degree-eight graph difference K*(y-p(x))*Q0*Q1*Q2. Each stock-interior
// circle has exactly two simple rational intersections with the parabola.
// The open parabola arc between those nodes cuts the disk into two caps.
// Replacing the cut-in arcs with the respective outer circular arcs gives
// the two exterior regions. Disjoint disks and strictly ordered attachment
// intervals ensure that these regions and the six caps exhaust the signed
// complement. Both original source jets are checked at every junction.
// All returned rationals are caller-owned.
func DiscoverSplineGraphParabolaThreeCrossingCircles(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaThreeCrossingCircles, error) {
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
	var visits int
	p, circles, sign, err := unitParabolaThreeCircleFactors(ctx, a, c, freeAxis, o.MaxCells, &visits, b)
	if err != nil {
		return nil, err
	}
	patches := [2]*SurfacePatch{a, c}
	out := &SplineGraphParabolaThreeCrossingCircles{
		FreeAxis: freeAxis, RegionSides: [8]int{-sign, sign, sign, -sign, sign, -sign, sign, -sign},
	}
	out.ParabolaControls, out.ParabolaExits, err = originalCrossingParabola(ctx, patches, freeAxis, p, b)
	if err != nil {
		return nil, err
	}
	for i, circle := range circles {
		for j := 0; j < i; j++ {
			if err := proveDisjointGraphCircles(circle, circles[j], b); err != nil {
				return nil, err
			}
		}
		disk, err := proveParabolaCrossingDisk(ctx, patches, freeAxis, p, circle, i, o.MaxCells, &visits, b)
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
	witnesses, err := parabolaCrossingExteriorWitnesses(freeAxis, p, out.CircleProjections[:], nil, b)
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
