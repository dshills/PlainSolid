package geom

import (
	"context"
	"fmt"
	"math/big"
)

// VerifiedNormalOffsetSweepPlanarPrism describes the exact polygonal
// image of one original sweep. The corner indices select the lower or
// upper endpoint of the original U, V and displacement domains.
type VerifiedNormalOffsetSweepPlanarPrism struct {
	Corners [2][2][2][3]*big.Rat
	Volume  *big.Rat
}

// PlanarPrismsRational proves exact planar-prism images in original
// chart order under one shared arithmetic budget. Each reference chart
// must be a positive-weight bilinear Bézier patch with a strictly convex
// coplanar quadrilateral image and a rational directed unit normal.
// The returned corners and volumes are owned by the caller.
func (c *VerifiedNormalOffsetSweepOpenChain) PlanarPrismsRational(ctx context.Context,
	o SurfacePatchOptions) ([]VerifiedNormalOffsetSweepPlanarPrism, error) {
	if ctx == nil || c == nil || len(c.sweeps) < 2 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limits, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	prisms := make([]VerifiedNormalOffsetSweepPlanarPrism, len(c.sweeps))
	for owner, sweep := range c.sweeps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		prism, err := sweep.planarPrismRational(budget)
		if err != nil {
			return nil, fmt.Errorf("geom: original sweep owner %d planar prism: %w", owner, err)
		}
		prisms[owner] = prism
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return prisms, nil
}

func (s *VerifiedNormalOffsetSweep) planarPrismRational(b *polynomialBudget) (r VerifiedNormalOffsetSweepPlanarPrism, err error) {
	p := s.source
	if p.degreeU != 1 || p.degreeV != 1 || len(p.net) != 4 {
		return r, ErrDomain
	}
	a := boundedRatArithmetic{b: b}
	var corners [4][3]*big.Rat
	// Follow the positive UV boundary: (0,0), (1,0), (1,1), (0,1).
	for i, j := range [4]int{0, 2, 3, 1} {
		h := p.net[j]
		for _, x := range h {
			if err := b.check(x); err != nil {
				return r, err
			}
		}
		for axis := range corners[i] {
			corners[i][axis] = a.quo(h[axis], h[3])
		}
	}
	u := sweptPrismVectorDifference(&a, corners[1], corners[0])
	v := sweptPrismVectorDifference(&a, corners[3], corners[0])
	other := sweptPrismVectorDifference(&a, corners[2], corners[0])
	normal := sweptPrismCross(&a, u, v)
	normSquared := sweepLineDot(&a, normal, normal)
	planeResidual := sweepLineDot(&a, normal, other)
	if a.err != nil {
		return r, a.err
	}
	if normSquared.Sign() <= 0 || planeResidual.Sign() != 0 {
		return r, ErrDomain
	}
	// Every edge is an exact rational linear blend of its endpoints
	// with positive weights. Interior source points are strictly
	// inside the convex polygon by positive Bernstein coefficients.
	// Injectivity and local regularity make their image open there.
	// Compactness and the exact boundary image also make it closed
	// relative to the polygon interior, so the complete cap is covered.
	for i, p := range corners {
		edge := sweptPrismVectorDifference(&a, corners[(i+1)%4], p)
		for j := 2; j < 4; j++ {
			inside := sweptPrismVectorDifference(&a, corners[(i+j)%4], p)
			turn := sweepLineDot(&a, normal, sweptPrismCross(&a, edge, inside))
			if a.err != nil {
				return r, a.err
			}
			if turn.Sign() <= 0 {
				return r, ErrDomain
			}
		}
	}
	length, upper, err := rationalSqrtBounds(normSquared, 0, b)
	if err != nil {
		return r, err
	}
	if length.Cmp(upper) != 0 {
		return r, ErrDomain
	}
	unit := [3]*big.Rat{a.quo(normal[0], length), a.quo(normal[1], length), a.quo(normal[2], length)}
	displacement := [2]*big.Rat{s.lower, s.upper}
	for t, d := range displacement {
		if err := b.check(d); err != nil {
			return VerifiedNormalOffsetSweepPlanarPrism{}, err
		}
		for i, uv := range [4][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
			for axis := range unit {
				r.Corners[uv[0]][uv[1]][t][axis] = a.add(corners[i][axis], a.mul(d, unit[axis]))
			}
		}
	}
	twiceFirst := sweepLineDot(&a, unit, sweptPrismCross(&a, u, other))
	twiceSecond := sweepLineDot(&a, unit, sweptPrismCross(&a, other, v))
	area := a.quo(a.add(twiceFirst, twiceSecond), big.NewRat(2, 1))
	r.Volume = a.mul(area, a.sub(s.upper, s.lower))
	if a.err != nil {
		return VerifiedNormalOffsetSweepPlanarPrism{}, a.err
	}
	return r, nil
}

func sweptPrismVectorDifference(a *boundedRatArithmetic, x, y [3]*big.Rat) [3]*big.Rat {
	return [3]*big.Rat{a.sub(x[0], y[0]), a.sub(x[1], y[1]), a.sub(x[2], y[2])}
}

func sweptPrismCross(a *boundedRatArithmetic, x, y [3]*big.Rat) (cross [3]*big.Rat) {
	for i := range cross {
		j, k := (i+1)%3, (i+2)%3
		cross[i] = a.sub(a.mul(x[j], y[k]), a.mul(x[k], y[j]))
	}
	return cross
}
