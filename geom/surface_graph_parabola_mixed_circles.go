package geom

import (
	"context"
	"math/big"
)

// ParabolaSeparatedCircleVertex records an original quarter-arc endpoint
// and its positive counterclockwise tangent on each original roof.
type ParabolaSeparatedCircleVertex struct {
	Parameters [2]*big.Rat
	Point      [3]*big.Rat
	Tangents   [2][3]*big.Rat
}

// SplineGraphParabolaMixedCircles proves a wall-to-wall parabola and two
// disjoint circles on the original shared unit XY chart. Exactly one crosses
// the parabola transversely at its two free extrema. The other disk lies
// wholly within one exterior region. The crossing's four signed regions
// retain their original numbering. Region 4 is the separated disk, removed
// from the exterior region named by SeparatedRegion.
type SplineGraphParabolaMixedCircles struct {
	Crossing         SplineGraphParabolaCircleCrossings
	Circles          [2]GraphContactCircle // ordered by their free and dependent center coordinates
	CrossingCircle   int
	SeparatedRegion  int
	SeparatedExtrema [4]ParabolaSeparatedCircleVertex // positive X, positive Y, negative X, negative Y
	RegionSides      [5]int
	RegionWitnesses  [5][2]*big.Rat
	ExactOperations  int
}

// DiscoverSplineGraphParabolaMixedCircles recovers and authenticates the
// complete original product K*(y-p(x))*Q0*Q1 up to tensor degree eight.
// For the crossing circle Q=(x-cx)^2+(y-cy)^2-r^2, p-cy=k*T with
// T=(x-cx)^2-r^2, and Q(x,p(x))=T*(1+k^2*T). The positive quotient
// gives exactly two simple roots. For the other circle exact containment
// and separation, using quartic root isolation when needed, prove its disk
// stays on one strict parabola side and away from the crossing circle.
// All coordinates and jets bind the unmodified original charts.
func DiscoverSplineGraphParabolaMixedCircles(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaMixedCircles, error) {
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
	law, err := graphMixedPowerColumns(diff, freeAxis, 6, 5, b)
	if err != nil {
		return nil, err
	}
	p, circles, sign, err := factorParabolaTwoCircles(law, b)
	if err != nil {
		return nil, err
	}
	if err := proveDisjointGraphCircles(circles[0], circles[1], b); err != nil {
		return nil, err
	}
	cross := -1
	two := big.NewRat(2, 1)
	for i, circle := range circles {
		x, y := circle.Center[0], circle.Center[1]
		pAtCenter, err := polynomialValue(p, x, b)
		if err != nil {
			return nil, err
		}
		diameterGap := new(big.Rat).Add(pAtCenter, new(big.Rat).Mul(p[2], circle.RadiusSquared))
		diameterGap.Sub(diameterGap, y)
		slope := new(big.Rat).Add(p[1], new(big.Rat).Mul(two, new(big.Rat).Mul(p[2], x)))
		for _, q := range [2]*big.Rat{diameterGap, slope} {
			if err := b.check(q); err != nil {
				return nil, err
			}
		}
		if diameterGap.Sign() == 0 && slope.Sign() == 0 {
			if cross >= 0 {
				return nil, ErrConvergence
			}
			cross = i
		}
	}
	if cross < 0 {
		return nil, ErrConvergence
	}
	crossingCircle := circles[cross]
	crossing, err := proveParabolaCircleCrossingFactors(ctx, a, c, freeAxis, p, crossingCircle.Center, crossingCircle.RadiusSquared, sign, b)
	if err != nil {
		return nil, err
	}
	dependent := 1 - freeAxis
	minimum, maximum := crossing.ParabolaControls[0][dependent], crossing.ParabolaControls[0][dependent]
	for _, control := range crossing.ParabolaControls[1:] {
		if control[dependent].Cmp(minimum) < 0 {
			minimum = control[dependent]
		}
		if control[dependent].Cmp(maximum) > 0 {
			maximum = control[dependent]
		}
	}
	other := circles[1-cross]
	separatedRegion, err := separatedParabolaCircleRegion(p, minimum, maximum, other.Center, other.RadiusSquared, b)
	if err != nil {
		return nil, err
	}
	otherRadius, err := graphCircleSquareRoot(other.RadiusSquared, b)
	if err != nil {
		return nil, err
	}
	// The two exterior witnesses lie before both disk projections and
	// therefore retain their region identities in the perforated strip.
	x := new(big.Rat).Sub(other.Center[0], otherRadius)
	leftCross := crossing.Nodes[0].Parameters[freeAxis]
	if x.Cmp(leftCross) > 0 {
		x.Set(leftCross)
	}
	x.Quo(x, two)
	y, err := polynomialValue(p, x, b)
	if err != nil {
		return nil, err
	}
	for i, offset := range [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)} {
		midpoint := new(big.Rat).Quo(new(big.Rat).Add(y, offset), two)
		if err := b.check(midpoint); err != nil {
			return nil, err
		}
		crossing.RegionWitnesses[i][freeAxis], crossing.RegionWitnesses[i][dependent] = new(big.Rat).Set(x), midpoint
	}
	out := &SplineGraphParabolaMixedCircles{Crossing: *crossing, CrossingCircle: cross, SeparatedRegion: separatedRegion}
	copy(out.RegionSides[:4], crossing.RegionSides[:])
	copy(out.RegionWitnesses[:4], crossing.RegionWitnesses[:])
	for i, circle := range circles {
		out.Circles[i].Center[freeAxis], out.Circles[i].Center[dependent] = circle.Center[0], circle.Center[1]
		out.Circles[i].RadiusSquared = circle.RadiusSquared
	}
	center := out.Circles[1-cross].Center
	out.RegionSides[4] = -out.RegionSides[separatedRegion]
	out.RegionWitnesses[4] = center
	patches := [2]*SurfacePatch{a, c}
	for i, radial := range [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
		uv := [2]*big.Rat{}
		for axis, direction := range radial {
			uv[axis] = new(big.Rat).Add(center[axis], new(big.Rat).Mul(otherRadius, big.NewRat(direction, 1)))
			if err := b.check(uv[axis]); err != nil {
				return nil, err
			}
		}
		jets, err := parabolaCircleOriginalJets(ctx, patches, uv, b)
		if err != nil {
			return nil, err
		}
		v := ParabolaSeparatedCircleVertex{Parameters: uv, Point: jets[0].Point}
		for owner, jet := range jets {
			for dim, u := range jet.Du {
				d := new(big.Rat).Mul(u, big.NewRat(-radial[1], 1))
				d.Add(d, new(big.Rat).Mul(jet.Dv[dim], big.NewRat(radial[0], 1)))
				if err := b.check(d); err != nil {
					return nil, err
				}
				v.Tangents[owner][dim] = d
			}
		}
		out.SeparatedExtrema[i] = v
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}
