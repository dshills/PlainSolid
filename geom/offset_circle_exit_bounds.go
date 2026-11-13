package geom

import (
	"context"
	"math/big"
)

// OffsetCircleExitBounds encloses A=-d*cos(theta)+sqrt(a²-d²sin²(theta)),
// where a² remains exact rational. Derivatives are per radian.
type OffsetCircleExitBounds struct {
	Proven                        bool
	Value                         [2]*big.Rat
	FirstAbsolute, SecondAbsolute *big.Rat
	ExactOperations               int
}

func BoundOffsetCircleExit(ctx context.Context, aSquared *big.Rat, d float64, o SurfacePatchEvaluationOptions) (out OffsetCircleExitBounds, err error) {
	if ctx == nil || aSquared == nil || aSquared.Sign() <= 0 || !finite(d) || d < 0 {
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
	off := new(big.Rat).SetFloat64(d)
	d2 := new(big.Rat).Mul(off, off)
	m2 := new(big.Rat).Sub(aSquared, d2)
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

// SphereToolCapStripBounds applies only on angles where A(theta)<=toolRadius.
// If offset>0 that exact domain is cos(theta)>=CosineMinimum; an arbitrary
// caller-selected angular window must independently satisfy it. Empty records
// an empty angular domain. Offset zero has either all or no angles.
// P=(d+rho*cosθ,rho*sinθ,sqrt(R²-x²-y²)), rho=(1-v)A+v*r, v∈[0,1].
type SphereToolCapStripBounds struct {
	Proven, Empty   bool
	Exit            OffsetCircleExitBounds
	CosineMinimum   *big.Rat
	UU, UV, VV      *big.Rat
	ExactOperations int
}

func BoundSphereToolCapStrip(ctx context.Context, R, r, d float64, aSquared *big.Rat, o SurfacePatchEvaluationOptions) (out SphereToolCapStripBounds, err error) {
	if !finite(R) || !finite(r) || R <= 0 || r <= 0 {
		return out, ErrInvalid
	}
	exit, e := BoundOffsetCircleExit(ctx, aSquared, d, o)
	out.Exit = exit
	out.ExactOperations = exit.ExactOperations
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
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: exit.ExactOperations}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Proven = false
		}
	}()
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	mul := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Mul(a, c) }
	add := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Add(a, c) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	scale := func(a *big.Rat, n int64) *big.Rat { return mul(a, big.NewRat(n, 1)) }
	rr, tool, off := rat(R), rat(r), rat(d)
	sum := add(tool, off)
	if sum.Cmp(rr) >= 0 || aSquared.Cmp(sq(rr)) > 0 {
		return out, ErrDomain
	}
	if d == 0 {
		out.Empty = aSquared.Cmp(sq(tool)) > 0
	} else {
		out.CosineMinimum = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Sub(aSquared, sq(off)), sq(tool)), scale(mul(off, tool), 2))
		out.Empty = out.CosineMinimum.Cmp(big.NewRat(1, 1)) > 0
		if e = b.check(out.CosineMinimum); e != nil {
			return out, e
		}
	}
	h2 := new(big.Rat).Sub(sq(rr), sq(sum))
	h, _, e := rationalSqrtBounds(h2, 128, b)
	if e != nil {
		return out, e
	}
	if h.Sign() <= 0 {
		return out, ErrConvergence
	}
	A1, A2 := exit.FirstAbsolute, exit.SecondAbsolute
	Fu := add(scale(mul(sum, A1), 2), scale(mul(off, tool), 2))
	Fv := scale(mul(sum, tool), 2)
	Fuu := add(add(scale(sq(A1), 2), scale(mul(sum, A2), 2)), add(scale(mul(off, A1), 4), scale(mul(off, tool), 2)))
	Fuv := add(add(scale(mul(tool, A1), 2), scale(mul(sum, A1), 2)), scale(mul(off, tool), 2))
	Fvv := scale(sq(tool), 2)
	den1, den2 := scale(h, 2), scale(mul(h, sq(h)), 4)
	second := func(F, a, c *big.Rat) *big.Rat {
		return add(new(big.Rat).Quo(F, den1), new(big.Rat).Quo(mul(a, c), den2))
	}
	out.UU = add(add(add(A2, scale(A1, 2)), tool), second(Fuu, Fu, Fu))
	out.UV = add(add(A1, tool), second(Fuv, Fu, Fv))
	out.VV = second(Fvv, Fv, Fv)
	for _, q := range []*big.Rat{Fu, Fv, Fuu, Fuv, Fvv, out.UU, out.UV, out.VV} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	out.Proven = true
	return out, nil
}
