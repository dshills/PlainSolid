package geom

import (
	"context"
	"math/big"
)

// BoxDualCircleBoundaryNormal is an outward unnormalized support normal
// Constant+t*Linear in original line units, with separate evidence per owner.
type BoxDualCircleBoundaryNormal struct {
	Source           BoxDualCircleSupport
	Constant, Linear [3]*big.Rat
	Bounds           [3]ExactParameterInterval
	DirectionSign    int
}

// BoxDualCircleFirstHit distinguishes isolated line events from a continuous
// boundary span. SpanEnd is nonnil only for the latter. Tangent refers to all
// active normals, not to polynomial multiplicity or material crossing.
type BoxDualCircleFirstHit struct {
	HasHit, Tangent    bool
	Parameter, SpanEnd *ExactAlgebraicRoot
	Bounds             ExactParameterInterval
	PointBounds        [3]ExactParameterInterval
	Support            BoxDualCircleSupport
	Normals            []BoxDualCircleBoundaryNormal
}

// FirstBoxDualCircleBoundaryHitRational returns the least boundary parameter in
// the closed supplied finite range. Root comparisons are exact; brackets only
// control numerical enclosures. Failure publishes no partial result. Work is
// bounded by one complete intersection pass, at most thirteen root comparisons,
// and nine support sign tests, each using the supplied arithmetic limits.
func FirstBoxDualCircleBoundaryHitRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleFirstHit, error) {
	return firstBoxDualCircleBoundaryHitRational(ctx, min, max, xs, radii, min[1], stopZ, origin, direction, minT, maxT, o)
}

func firstBoxDualCircleBoundaryHitRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, centerY, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleFirstHit, error) {
	var zero BoxDualCircleFirstHit
	line, e := intersectBoxDualCircleBoundaryRational(ctx, min, max, xs, radii, centerY, stopZ, origin, direction, minT, maxT, o)
	if e != nil {
		return zero, e
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return zero, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	var root, end *ExactAlgebraicRoot
	var mask BoxDualCircleSupport
	add := func(r, span *ExactAlgebraicRoot, m BoxDualCircleSupport) error {
		order := -1
		if root != nil {
			c, e := r.Compare(ctx, root, ro)
			if e != nil {
				return e
			}
			if !c.Proven {
				return ErrConvergence
			}
			order = c.Order
		}
		if order < 0 {
			root, end, mask = r, span, m
		} else if order == 0 {
			mask |= m
			if span != nil {
				end = span
			}
		}
		return nil
	}
	for _, h := range line.Hits {
		if e = add(h.Parameter, nil, h.Support); e != nil {
			return zero, e
		}
	}
	for _, iv := range line.Intervals {
		if e = add(iv.Start, iv.End, iv.Support|iv.StartSupport); e != nil {
			return zero, e
		}
	}
	if root == nil {
		return zero, nil
	}
	out := BoxDualCircleFirstHit{HasHit: true, Tangent: true, Parameter: root, SpanEnd: end, Bounds: root.Interval(), Support: mask}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	bounds := func(a, b *big.Rat) (ExactParameterInterval, error) {
		lo := new(big.Rat).Add(a, new(big.Rat).Mul(b, out.Bounds.Lower))
		hi := new(big.Rat).Add(a, new(big.Rat).Mul(b, out.Bounds.Upper))
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		for _, v := range []*big.Rat{lo, hi} {
			if e := budget.check(v); e != nil {
				return ExactParameterInterval{}, e
			}
		}
		return ExactParameterInterval{Lower: lo, Upper: hi}, nil
	}
	for j := 0; j < 3; j++ {
		out.PointBounds[j], e = bounds(origin[j], direction[j])
		if e != nil {
			return zero, e
		}
	}
	for _, source := range []BoxDualCircleSupport{BoxDualCircleStockX, BoxDualCircleStockY, BoxDualCircleStockZ, BoxDualCircleFirstWall, BoxDualCircleSecondWall, BoxDualCircleBlindFloor} {
		if mask&source == 0 {
			continue
		}
		n := BoxDualCircleBoundaryNormal{Source: source}
		for j := 0; j < 3; j++ {
			n.Constant[j] = new(big.Rat)
			n.Linear[j] = new(big.Rat)
		}
		switch source {
		case BoxDualCircleStockX, BoxDualCircleStockY, BoxDualCircleStockZ:
			axis := 0
			if source == BoxDualCircleStockY {
				axis = 1
			}
			if source == BoxDualCircleStockZ {
				axis = 2
			}
			sign, _, err := composedPolynomialRootSign(ctx, root.root, trimPolynomial(rationalPolynomial{new(big.Rat).Sub(origin[axis], min[axis]), direction[axis]}), budget, o.MaxSubdivisions)
			if err != nil {
				return zero, err
			}
			if sign == 0 {
				n.Constant[axis].SetInt64(-1)
			} else {
				n.Constant[axis].SetInt64(1)
			}
		case BoxDualCircleBlindFloor:
			n.Constant[2].SetInt64(-1)
		default:
			owner := 0
			if source == BoxDualCircleSecondWall {
				owner = 1
			}
			for j, c := range [2]*big.Rat{xs[owner], centerY} {
				n.Constant[j].Sub(c, origin[j])
				n.Linear[j].Neg(direction[j])
			}
		}
		dot := rationalPolynomial{new(big.Rat), new(big.Rat)}
		for j := 0; j < 3; j++ {
			n.Bounds[j], e = bounds(n.Constant[j], n.Linear[j])
			if e != nil {
				return zero, e
			}
			dot[0].Add(dot[0], new(big.Rat).Mul(n.Constant[j], direction[j]))
			dot[1].Add(dot[1], new(big.Rat).Mul(n.Linear[j], direction[j]))
		}
		n.DirectionSign, _, e = composedPolynomialRootSign(ctx, root.root, trimPolynomial(dot), budget, o.MaxSubdivisions)
		if e != nil {
			return zero, e
		}
		out.Tangent = out.Tangent && n.DirectionSign == 0
		out.Normals = append(out.Normals, n)
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
