package geom

import (
	"context"
	"math/big"
)

// TorusCylinderNormals returns nonzero exact outward normal DIRECTIONS, not
// unit normals. Slots are torus/wall/lower cap/upper cap; absent slots are nil.
// At a seam every exposed original owner is retained, not averaged.
type TorusCylinderNormals struct {
	Owners          BoreBoundaryMask
	Directions      [4][3]*big.Rat
	ExactOperations int
}

// BoundaryNormalsRational proves original exposed boundary membership before
// returning analytic directions. It rejects interior/exterior points, including
// buried supports. Classification and gradients share one arithmetic budget.
func (s TorusCylinderMaterial) BoundaryNormalsRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (out TorusCylinderNormals, err error) {
	defer func() {
		if err != nil {
			out = TorusCylinderNormals{}
		}
	}()
	if ctx == nil || s.R == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, x := range p {
		if x == nil {
			return out, ErrInvalid
		}
	}
	limits, e := patchOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	loc, mask, e := s.classifyWithBudget(ctx, p, b)
	if e != nil {
		return out, e
	}
	if loc != BoreBoundary {
		return out, ErrDomain
	}
	a := sphereOverlapArithmetic{ctx: ctx, b: b}
	out.Owners = mask
	if mask&TorusCylinderTorus != 0 {
		R2 := a.mul(s.R, s.R)
		A := a.sub(R2, a.mul(s.r, s.r))
		for _, x := range p {
			A = a.add(A, a.mul(x, x))
		}
		radial := a.sub(A, a.mul(big.NewRat(2, 1), R2))
		out.Directions[0] = [3]*big.Rat{a.mul(p[0], radial), a.mul(p[1], radial), a.mul(p[2], A)}
	} // gradient/4, positive scaling
	if mask&TorusCylinderWall != 0 {
		sign := big.NewRat(1, 1)
		if s.op == TorusCylinderDifference {
			sign.Neg(sign)
		}
		out.Directions[1] = [3]*big.Rat{a.mul(sign, p[0]), a.mul(sign, p[1]), new(big.Rat)}
	}
	if mask&TorusCylinderLowerCap != 0 {
		out.Directions[2] = [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(-1, 1)}
	}
	if mask&TorusCylinderUpperCap != 0 {
		out.Directions[3] = [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
	}
	if a.err != nil {
		return out, a.err
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.ExactOperations = b.operations
	return out, nil
}
