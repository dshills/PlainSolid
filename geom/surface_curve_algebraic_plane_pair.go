package geom

import (
	"context"
	"math/big"
)

// IntersectRawPlanePair isolates simultaneous contact with two independent raw
// planes, hence their exact spatial line. The coefficient order is Nx,Ny,Nz,D;
// no normal normalization or rounded line construction occurs. GCD root
// multiplicity is the minimum nonzero constraint order, not an asserted spatial
// intersection multiplicity. Coincident means the whole cell lies in both
// planes. Returned immutable events retain original-parameter provenance and
// strict membership in the algebraically bounded cell domain.
func (c *ExactComposedCurveCell) IntersectRawPlanePair(ctx context.Context, planes [2][4]*big.Rat, o ExactCurvePlaneEventOptions) (r ComposedCurvePlaneIntersection, err error) {
	if ctx == nil || c == nil || len(c.power[3]) == 0 || c.endpoints[0] == nil || c.endpoints[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	var constraints [2]rationalPolynomial
	for owner, plane := range planes {
		normal := false
		for axis, x := range plane {
			if x == nil {
				x = new(big.Rat)
			}
			planes[owner][axis] = new(big.Rat).Set(x)
			if e := b.check(x); e != nil {
				return r, e
			}
			normal = normal || axis < 3 && x.Sign() != 0
			for len(constraints[owner]) < len(c.power[axis]) {
				constraints[owner] = append(constraints[owner], new(big.Rat))
			}
			for i, value := range c.power[axis] {
				constraints[owner][i].Add(constraints[owner][i], new(big.Rat).Mul(x, value))
				if e := b.check(constraints[owner][i]); e != nil {
					return r, e
				}
			}
		}
		if !normal {
			return r, ErrInvalid
		}
		constraints[owner] = trimPolynomial(constraints[owner])
	}
	independent := false
	for axis := 0; axis < 3; axis++ {
		j, k := (axis+1)%3, (axis+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(planes[0][j], planes[1][k]), new(big.Rat).Mul(planes[0][k], planes[1][j]))
		if e := b.check(cross); e != nil {
			return r, e
		}
		independent = independent || cross.Sign() != 0
	}
	if !independent {
		return r, ErrDegenerate
	}
	var common rationalPolynomial
	if len(constraints[0]) == 0 {
		common = constraints[1]
	} else if len(constraints[1]) == 0 {
		common = constraints[0]
	} else {
		common, e = polynomialGCD(constraints[0], constraints[1], b)
		if e != nil {
			return r, e
		}
	}
	return c.intersectComposedPolynomial(ctx, common, o, b)
}
