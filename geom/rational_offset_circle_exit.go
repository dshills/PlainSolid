package geom

import (
	"context"
	"math/big"
)

// BoundRationalOffsetCircleExit bounds A=-offset*cos(theta)+sqrt(aSquared-offset²*sin²(theta))
// for an exact signed rational offset, without rounding a chord-midpoint pole.
// Global value and absolute derivative bounds depend only on |offset|.
func BoundRationalOffsetCircleExit(ctx context.Context, aSquared, signedOffset *big.Rat, o SurfacePatchEvaluationOptions) (out OffsetCircleExitBounds, err error) {
	if ctx == nil || aSquared == nil || aSquared.Sign() <= 0 || signedOffset == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Proven = false
		}
	}()
	off := new(big.Rat).Abs(signedOffset)
	d2 := new(big.Rat).Mul(off, off)
	m2 := new(big.Rat).Sub(aSquared, d2)
	if e := b.check(signedOffset); e != nil {
		return out, e
	}
	if e := b.check(aSquared); e != nil {
		return out, e
	}
	if e := b.check(m2); e != nil {
		return out, e
	}
	if m2.Sign() <= 0 {
		return out, ErrDomain
	}
	al, ah, e := rationalSqrtBounds(aSquared, 128, b)
	if e != nil {
		return out, e
	}
	m, _, e := rationalSqrtBounds(m2, 128, b)
	if e != nil {
		return out, e
	}
	low, high := new(big.Rat).Sub(al, off), new(big.Rat).Add(ah, off)
	if low.Sign() <= 0 || m.Sign() <= 0 {
		return out, ErrConvergence
	}
	first := new(big.Rat).Add(off, new(big.Rat).Quo(d2, new(big.Rat).Mul(big.NewRat(2, 1), m)))
	second := new(big.Rat).Add(off, new(big.Rat).Add(new(big.Rat).Quo(d2, m), new(big.Rat).Quo(new(big.Rat).Mul(d2, d2), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(m, new(big.Rat).Mul(m, m))))))
	for _, q := range []*big.Rat{low, high, first, second} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	out.Value = [2]*big.Rat{low, high}
	out.FirstAbsolute = first
	out.SecondAbsolute = second
	out.Proven = true
	return out, nil
}

// BoundRationalPolarFan bounds P(theta,v)=v*A(theta)*(cos(theta),sin(theta))
// about the common interior pole. UU,UV,VV are Euclidean norms per radian/unit
// V; rigid translation of the pole has no effect. The boundary v=1 uses the
// exact signed-offset circular exit and v=0 is a deliberate polar collapse.
func BoundRationalPolarFan(ctx context.Context, aSquared, signedOffset *big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderLensFanBounds, err error) {
	q, e := BoundRationalOffsetCircleExit(ctx, aSquared, signedOffset, o)
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: q.ExactOperations}
	out.UU = new(big.Rat).Add(q.SecondAbsolute, new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), q.FirstAbsolute), q.Value[1]))
	out.UV = new(big.Rat).Add(q.FirstAbsolute, q.Value[1])
	out.VV = new(big.Rat)
	for _, v := range []*big.Rat{out.UU, out.UV, out.VV} {
		if e = b.check(v); e != nil {
			return SphereCylinderLensFanBounds{}, e
		}
	}
	return out, nil
}
