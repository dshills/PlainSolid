package geom

import (
	"context"
	"math/big"
)

// RationalSurfacePatchSecondJet is the exact position and first and second
// physical partials on the selected original knot cell. The rational values
// are owned by the result; the boxes enclose their floating representations.
type RationalSurfacePatchSecondJet struct {
	RationalSurfacePatchJet
	Duu, Duv, Dvv                   [3]*big.Rat
	DuuBounds, DuvBounds, DvvBounds Box
	KnotCell                        ParameterRectangle
}

// EvaluateSecondJetRational evaluates an exact second jet on a clamped
// tensor. It selects the original right-hand piece at interior knots and the
// left-hand piece at an upper domain endpoint. It does not assert smoothness
// across knot lines. The three second partials use original parameter units.
func (p *SurfacePatch) EvaluateSecondJetRational(ctx context.Context, u, v *big.Rat,
	o SurfacePatchEvaluationOptions) (r RationalSurfacePatchSecondJet, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || u == nil || v == nil {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	cell, err := p.rationalEvaluationCell(ctx, u, v, [2]int{}, b)
	if err != nil {
		return r, err
	}
	domain := cell.Domain()
	a := boundedRatArithmetic{b: b}
	blend := func(query *big.Rat, low, high float64) splineJetBlend {
		lo, hi := new(big.Rat).SetFloat64(low), new(big.Rat).SetFloat64(high)
		slope := a.quo(big.NewRat(1, 1), a.sub(hi, lo))
		t := a.mul(a.sub(query, lo), slope)
		return func(_, _ int) (*big.Rat, *big.Rat) { return t, slope }
	}
	ub, vb := blend(u, domain.UMin, domain.UMax), blend(v, domain.VMin, domain.VMax)
	if a.err != nil {
		return r, a.err
	}
	d, err := exactSplineSecondTensor(ctx, cell.countU, cell.countV, func(i, j int) exactHomogeneous {
		return cell.net[i*cell.countV+j]
	}, ub, vb, b)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.KnotCell = domain
	r.Point, r.Du, r.Dv = d[0][0], d[1][0], d[0][1]
	r.Duu, r.Duv, r.Dvv = d[2][0], d[1][1], d[0][2]
	r.PointBounds, r.DuBounds, r.DvBounds = rationalSurfaceVectorBounds(r.Point), rationalSurfaceVectorBounds(r.Du), rationalSurfaceVectorBounds(r.Dv)
	r.DuuBounds, r.DuvBounds, r.DvvBounds = rationalSurfaceVectorBounds(r.Duu), rationalSurfaceVectorBounds(r.Duv), rationalSurfaceVectorBounds(r.Dvv)
	return r, nil
}
