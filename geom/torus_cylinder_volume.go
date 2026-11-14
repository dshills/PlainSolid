package geom

import (
	"context"
	"math/big"
)

// TorusCylinderVolumeOptions bounds a single shared arithmetic ledger and the
// dyadic quadrature. VolumeTolerance is an exact positive requested width.
// MaxCells defaults to4096 (maximum65536); SqrtBits defaults64, range16..256.
type TorusCylinderVolumeOptions struct {
	VolumeTolerance    *big.Rat
	MaxCells, SqrtBits int
	Arithmetic         SurfacePatchOptions
}

// TorusCylinderVolume gives outward rational volume bounds, not a floating
// quadrature estimate. Cells is the final dyadic partition; Evaluations includes
// all levels. Every reported measure uses the same pi/section enclosures.
type TorusCylinderVolume struct {
	Lower, Upper                        *big.Rat
	Common, Torus, Cylinder             [2]*big.Rat
	Cells, Evaluations, ExactOperations int
}

// BoundVolume encloses all three regularized operations in the constructor's
// transverse/clear-cap domain. Put a=c-R and h(x)=sqrt(r²-x²). Common volume is
// 4*pi*(R*integral[-r,a]h(x)dx - (r²-a²)^(3/2)/3). Concavity of h proves each
// trapezoid below the integral and each midpoint rectangle above, including the
// endpoint derivative singularity. Rational sqrt brackets and Machin alternating
// arctangent bounds enclose all arithmetic; no floating estimate proves a bound.
// Difference=Torus-Common; Union=Torus+Cylinder-Common. Limits/cancel fail atomically.
func (s TorusCylinderMaterial) BoundVolume(ctx context.Context, o TorusCylinderVolumeOptions) (out TorusCylinderVolume, err error) {
	defer func() {
		if err != nil {
			out = TorusCylinderVolume{}
		}
	}()
	if ctx == nil || s.R == nil || o.VolumeTolerance == nil || o.VolumeTolerance.Sign() <= 0 {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.SqrtBits == 0 {
		o.SqrtBits = 64
	}
	if o.MaxCells < 1 || o.MaxCells > 65536 || o.SqrtBits < 16 || o.SqrtBits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, q := range []*big.Rat{s.R, s.r, s.c, s.lower, s.upper, o.VolumeTolerance} {
		a.check(q)
	}
	if a.err != nil {
		return out, a.err
	}
	precision := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(o.SqrtBits+16)))
	u, e := helicalAtanSmall(ctx, big.NewRat(1, 5), precision, 4*o.SqrtBits+64, b)
	if e != nil {
		return out, e
	}
	v, e := helicalAtanSmall(ctx, big.NewRat(1, 239), precision, 4*o.SqrtBits+64, b)
	if e != nil {
		return out, e
	}
	pi := [2]*big.Rat{a.sub(a.mul(big.NewRat(16, 1), u[0]), a.mul(big.NewRat(4, 1), v[1])), a.sub(a.mul(big.NewRat(16, 1), u[1]), a.mul(big.NewRat(4, 1), v[0]))}
	r2 := a.mul(s.r, s.r)
	end := a.sub(s.c, s.R)
	start := a.sub(new(big.Rat), s.r)
	length := a.sub(end, start)
	h2 := a.sub(r2, a.mul(end, end))
	hLo, hHi, e := rationalSqrtBounds(h2, o.SqrtBits, b)
	if e != nil {
		return out, e
	}
	term := [2]*big.Rat{a.div(a.mul(h2, hLo), big.NewRat(3, 1)), a.div(a.mul(h2, hHi), big.NewRat(3, 1))}
	tc := a.mul(big.NewRat(2, 1), a.mul(s.R, r2))
	cc := a.mul(a.mul(s.c, s.c), a.sub(s.upper, s.lower))
	torus := [2]*big.Rat{a.mul(tc, a.mul(pi[0], pi[0])), a.mul(tc, a.mul(pi[1], pi[1]))}
	cylinder := [2]*big.Rat{a.mul(cc, pi[0]), a.mul(cc, pi[1])}
	if a.err != nil {
		return out, a.err
	}
	evaluations := 0
	height := func(x *big.Rat) ([2]*big.Rat, error) {
		q := a.sub(r2, a.mul(x, x))
		if a.err != nil {
			return [2]*big.Rat{}, a.err
		}
		lo, hi, e := rationalSqrtBounds(q, o.SqrtBits, b)
		evaluations++
		return [2]*big.Rat{lo, hi}, e
	}
	for n := 1; n <= o.MaxCells; n *= 2 {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		step := a.div(length, big.NewRat(int64(n), 1))
		lo, hi := new(big.Rat), new(big.Rat)
		left := start
		hl, e := height(left)
		if e != nil {
			return out, e
		}
		for i := 0; i < n; i++ {
			right := a.add(start, a.mul(step, big.NewRat(int64(i+1), 1)))
			mid := a.div(a.add(left, right), big.NewRat(2, 1))
			hr, e := height(right)
			if e != nil {
				return out, e
			}
			hm, e := height(mid)
			if e != nil {
				return out, e
			}
			lo = a.add(lo, a.div(a.mul(step, a.add(hl[0], hr[0])), big.NewRat(2, 1)))
			hi = a.add(hi, a.mul(step, hm[1]))
			left = right
			hl = hr
			if a.err != nil {
				return out, a.err
			}
		}
		integral := [2]*big.Rat{a.sub(a.mul(s.R, lo), term[1]), a.sub(a.mul(s.R, hi), term[0])}
		if integral[0].Sign() < 0 {
			integral[0] = new(big.Rat)
		}
		common := [2]*big.Rat{a.mul(big.NewRat(4, 1), a.mul(pi[0], integral[0])), a.mul(big.NewRat(4, 1), a.mul(pi[1], integral[1]))}
		lower, upper := common[0], common[1]
		switch s.op {
		case TorusCylinderDifference:
			lower = a.sub(torus[0], common[1])
			upper = a.sub(torus[1], common[0])
		case TorusCylinderUnion:
			lower = a.sub(a.add(torus[0], cylinder[0]), common[1])
			upper = a.sub(a.add(torus[1], cylinder[1]), common[0])
		}
		if lower.Sign() < 0 {
			lower = new(big.Rat)
		}
		width := a.sub(upper, lower)
		if a.err != nil {
			return out, a.err
		}
		if width.Cmp(o.VolumeTolerance) <= 0 {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			return TorusCylinderVolume{new(big.Rat).Set(lower), new(big.Rat).Set(upper), common, torus, cylinder, n, evaluations, b.operations}, nil
		}
	}
	return out, ErrConvergence
}
