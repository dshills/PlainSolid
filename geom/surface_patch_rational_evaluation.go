package geom

import (
	"context"
	"math/big"
)

// SurfacePatchEvaluationOptions bounds rational UV evaluation and knot-piece
// extraction. Zero selects 1M operations and 16384 coefficient bits.
type SurfacePatchEvaluationOptions struct{ MaxOperations, MaxCoefficientBits int }

// RationalSurfacePatchJet contains caller-owned exact coordinates and outward
// float boxes. Derivatives use original parameter units. At interior knots the
// right-hand piece is selected; at the final domain end the left piece is used.
// No cross-knot differentiability is asserted. Bounds may contain infinities
// when an exact derivative exceeds float64 range; exact values remain valid.
type RationalSurfacePatchJet struct {
	Point, Du, Dv                   [3]*big.Rat
	PointBounds, DuBounds, DvBounds Box
	ExactOperations                 int
}

// EvaluateRational evaluates at exact rational original UV. Neither knot
// selection nor the evaluation parameter is rounded to float64. Inputs are
// read-only; all returned rationals are independent of inputs and patch storage.
func (p *SurfacePatch) EvaluateRational(ctx context.Context, u, v *big.Rat, o SurfacePatchEvaluationOptions) (RationalSurfacePatchJet, error) {
	return p.evaluateRationalOnKnotSide(ctx, u, v, [2]int{}, o)
}

// A negative side selects the left cell at an exact internal knot equality.
// Callers must prove that the intended source curve enters that cell.
func (p *SurfacePatch) evaluateRationalOnKnotSide(ctx context.Context, u, v *big.Rat, side [2]int,
	o SurfacePatchEvaluationOptions) (r RationalSurfacePatchJet, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || u == nil || v == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	for _, x := range []*big.Rat{u, v} {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	d := p.Domain()
	if u.Cmp(new(big.Rat).SetFloat64(d.UMin)) < 0 || u.Cmp(new(big.Rat).SetFloat64(d.UMax)) > 0 || v.Cmp(new(big.Rat).SetFloat64(d.VMin)) < 0 || v.Cmp(new(big.Rat).SetFloat64(d.VMax)) > 0 {
		return r, ErrDomain
	}
	current := p
	for {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		t, axis, knot := patchSplitChoice(current)
		if !knot {
			break
		}
		left, right, e := current.split(t, axis, budget)
		if e != nil {
			return r, e
		}
		parameter, direction := u, side[0]
		if !axis {
			parameter, direction = v, side[1]
		}
		comparison := parameter.Cmp(new(big.Rat).SetFloat64(t))
		if comparison < 0 || (comparison == 0 && direction < 0) {
			current = left
		} else {
			current = right
		}
	}
	point, derivative, e := exactSurfaceJet(current, u, v, budget)
	if e != nil {
		return r, e
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	for axis := 0; axis < 3; axis++ {
		r.Point[axis] = new(big.Rat).Set(point[axis])
		r.Du[axis] = new(big.Rat).Set(derivative[axis][0])
		r.Dv[axis] = new(big.Rat).Set(derivative[axis][1])
	}
	r.PointBounds, r.DuBounds, r.DvBounds = rationalSurfaceVectorBounds(r.Point), rationalSurfaceVectorBounds(r.Du), rationalSurfaceVectorBounds(r.Dv)
	return r, nil
}

func rationalSurfaceVectorBounds(values [3]*big.Rat) Box {
	return Box{
		Min: Vec3{rationalFloatBound(values[0], true), rationalFloatBound(values[1], true), rationalFloatBound(values[2], true)},
		Max: Vec3{rationalFloatBound(values[0], false), rationalFloatBound(values[1], false), rationalFloatBound(values[2], false)},
	}
}
