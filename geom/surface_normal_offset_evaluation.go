package geom

import (
	"context"
	"math/big"
)

// SurfaceNormalOffsetEvaluationOptions bounds a normal offset point
// enclosure. SqrtBits defaults to 96 and controls the absolute error of the
// square root after scaling the cross product by its largest component.
// MaxOperations and MaxCoefficientBits use the exact patch defaults.
type SurfaceNormalOffsetEvaluationOptions struct {
	SqrtBits, MaxOperations, MaxCoefficientBits int
}

// RationalSurfaceOffsetPoint encloses one signed normal offset point.
// Lower and Upper are caller-owned exact rational bounds in physical
// coordinates. When the offset coordinates are rational, both can coincide.
// Bounds rounds the same enclosure outward to float64.
type RationalSurfaceOffsetPoint struct {
	Lower, Upper    [3]*big.Rat
	Bounds          Box
	ExactOperations int
}

// NormalOffsetRational encloses S + distance*(Su×Sv)/|Su×Sv| using the exact
// jet, including its one-sided choice at knots. Irrational unit normals
// are enclosed with directed rational bounds. The enclosure makes no
// regularity or intersection claim about the offset surface.
func (s RationalSurfacePatchJet) NormalOffsetRational(ctx context.Context, distance *big.Rat,
	o SurfaceNormalOffsetEvaluationOptions) (result RationalSurfaceOffsetPoint, err error) {
	if ctx == nil || distance == nil {
		return result, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if o.SqrtBits == 0 {
		o.SqrtBits = 96
	}
	if o.SqrtBits < 1 || o.SqrtBits > 1024 {
		return result, ErrInvalid
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return result, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	if err := b.check(distance); err != nil {
		return result, err
	}
	for _, v := range [3][3]*big.Rat{s.Point, s.Du, s.Dv} {
		for _, x := range v {
			if x == nil {
				return result, ErrInvalid
			}
			if err := b.check(x); err != nil {
				return result, err
			}
		}
	}
	a := boundedRatArithmetic{b: b}
	var n [3]*big.Rat
	scale := new(big.Rat)
	for i := range n {
		j, k := (i+1)%3, (i+2)%3
		n[i] = a.sub(a.mul(s.Du[j], s.Dv[k]), a.mul(s.Du[k], s.Dv[j]))
		if abs := new(big.Rat).Abs(n[i]); abs.Cmp(scale) > 0 {
			scale = abs
		}
	}
	if a.err != nil {
		return result, a.err
	}
	if scale.Sign() == 0 {
		return result, ErrDegenerate
	}
	// Scaling leaves the direction unchanged and makes the squared norm
	// lie in [1,3], preventing a spurious zero lower denominator.
	norm2 := new(big.Rat)
	for i := range n {
		n[i] = a.quo(n[i], scale)
		norm2 = a.add(norm2, a.mul(n[i], n[i]))
	}
	if a.err != nil {
		return result, a.err
	}
	lo, hi, err := rationalSqrtBounds(norm2, o.SqrtBits, b)
	if err != nil {
		return result, err
	}
	denominator, err := boundedExactInterval(lo, hi, b)
	if err != nil {
		return result, err
	}
	var lower, upper [3]*big.Rat
	for i := range n {
		num := a.mul(distance, n[i])
		if a.err != nil {
			return result, a.err
		}
		d, err := intervalDivide(exactInterval{num, num}, denominator, b)
		if err != nil {
			return result, err
		}
		lower[i], upper[i] = a.add(s.Point[i], d.lo), a.add(s.Point[i], d.hi)
	}
	if a.err != nil {
		return result, a.err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.Lower, result.Upper = lower, upper
	result.Bounds = Box{Min: rationalSurfaceVectorBounds(lower).Min, Max: rationalSurfaceVectorBounds(upper).Max}
	return result, nil
}
