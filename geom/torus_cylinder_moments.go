package geom

import (
	"context"
	"math/big"
)

// TorusCylinderMomentOptions requests separate dimensional widths. All four
// tolerances must be positive. MaxCells defaults4096(max65536); PrecisionBits
// defaults64(16..256). One arithmetic ledger covers every refinement level.
type TorusCylinderMomentOptions struct {
	VolumeTolerance, SecondMomentTolerance, CentroidTolerance, InertiaTolerance *big.Rat
	MaxCells, PrecisionBits                                                     int
	Arithmetic                                                                  SurfacePatchOptions
}

// TorusCylinderMoments uses original canonical physical coordinates and unit
// density. FirstX/Y, centroidX/Y and mixed raw moments are exactly zero.
// SecondXY means both integral x²dV and integral y²dV, NOT integral x*y dV.
// Inertias are diagonal; all off-diagonal entries are exactly zero. These are
// outward rational enclosures, including the centroidal parallel-axis quotient.
type TorusCylinderMoments struct {
	Volume, FirstZ, SecondXY, SecondZ, CentroidZ [2]*big.Rat
	InertiaOrigin, InertiaCentroid               [3][2]*big.Rat
	Cells, ExactOperations                       int
}

// BoundMoments integrates the original retained meridian without a mesh.
// Concavity of h=sqrt(r²-(rho-R)²) gives a chord lower and midpoint tangent
// upper. Each is integrated exactly against the nonnegative weights rho,
// rho³ and rho*(r²-(rho-R)²). Thus V=4pi*I1, Qxx=Qyy=2pi*I3,
// Qzz=4pi*Iz/3. Finite cylinder moments retain unequal signed cap heights.
// All operations, roots and pi enclosures are bounded; failures are atomic.
func (s TorusCylinderMaterial) BoundMoments(ctx context.Context, o TorusCylinderMomentOptions) (out TorusCylinderMoments, err error) {
	defer func() {
		if err != nil {
			out = TorusCylinderMoments{}
		}
	}()
	if ctx == nil || s.R == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, t := range []*big.Rat{o.VolumeTolerance, o.SecondMomentTolerance, o.CentroidTolerance, o.InertiaTolerance} {
		if t == nil || t.Sign() <= 0 {
			return out, ErrInvalid
		}
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 64
	}
	if o.MaxCells < 1 || o.MaxCells > 65536 || o.PrecisionBits < 16 || o.PrecisionBits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, q := range []*big.Rat{s.R, s.r, s.c, s.lower, s.upper, o.VolumeTolerance, o.SecondMomentTolerance, o.CentroidTolerance, o.InertiaTolerance} {
		a.check(q)
	}
	if a.err != nil {
		return out, a.err
	}
	tol := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(o.PrecisionBits+16)))
	terms := 4*o.PrecisionBits + 64
	u, e := helicalAtanSmall(ctx, big.NewRat(1, 5), tol, terms, b)
	if e != nil {
		return out, e
	}
	v, e := helicalAtanSmall(ctx, big.NewRat(1, 239), tol, terms, b)
	if e != nil {
		return out, e
	}
	pi := [2]*big.Rat{a.sub(a.mul(big.NewRat(16, 1), u[0]), a.mul(big.NewRat(4, 1), v[1])), a.sub(a.mul(big.NewRat(16, 1), u[1]), a.mul(big.NewRat(4, 1), v[0]))}
	type interval = [2]*big.Rat
	add := func(x, y interval) interval { return interval{a.add(x[0], y[0]), a.add(x[1], y[1])} }
	sub := func(x, y interval) interval { return interval{a.sub(x[0], y[1]), a.sub(x[1], y[0])} }
	scale := func(x interval, c *big.Rat) interval {
		if c.Sign() < 0 {
			return interval{a.mul(x[1], c), a.mul(x[0], c)}
		}
		return interval{a.mul(x[0], c), a.mul(x[1], c)}
	}
	mulPos := func(x, y interval) interval { return interval{a.mul(x[0], y[0]), a.mul(x[1], y[1])} }
	divPos := func(x, y interval) interval {
		var z interval
		for _, xx := range x {
			for _, yy := range y {
				q := a.div(xx, yy)
				if z[0] == nil || q.Cmp(z[0]) < 0 {
					z[0] = q
				}
				if z[1] == nil || q.Cmp(z[1]) > 0 {
					z[1] = q
				}
			}
		}
		return z
	}
	clamp := func(x interval) interval {
		if x[0].Sign() < 0 {
			x[0] = new(big.Rat)
		}
		return x
	}
	// Round each tangent upper bound outward to a common dyadic lattice.
	// This prevents coprime sqrt-denominator growth across cells; the rounding
	// contribution remains in the final requested-width check.
	lattice := new(big.Int).Lsh(big.NewInt(1), uint(o.PrecisionBits))
	upperDyadic := func(x *big.Rat) *big.Rat {
		scaled := a.mul(x, new(big.Rat).SetInt(lattice))
		q, rem := new(big.Int), new(big.Int)
		q.QuoRem(scaled.Num(), scaled.Denom(), rem)
		if rem.Sign() > 0 {
			q.Add(q, big.NewInt(1))
		}
		v := new(big.Rat).SetFrac(q, new(big.Int).Set(lattice))
		a.check(v)
		return v
	}
	r2 := a.mul(s.r, s.r)
	R2 := a.mul(s.R, s.R)
	c2 := a.mul(s.c, s.c)
	height := a.sub(s.upper, s.lower)
	tv := scale(mulPos(pi, pi), a.mul(big.NewRat(2, 1), a.mul(s.R, r2)))
	tx := scale(tv, a.add(a.div(R2, big.NewRat(2, 1)), a.mul(big.NewRat(3, 8), r2)))
	tz := scale(tv, a.div(r2, big.NewRat(4, 1)))
	cv := scale(pi, a.mul(c2, height))
	cx := scale(cv, a.div(c2, big.NewRat(4, 1)))
	cz := scale(cv, a.div(a.add(a.add(a.mul(s.upper, s.upper), a.mul(s.upper, s.lower)), a.mul(s.lower, s.lower)), big.NewRat(3, 1)))
	first := scale(cv, a.div(a.add(s.upper, s.lower), big.NewRat(2, 1)))
	start := a.sub(s.R, s.r)
	length := a.sub(s.c, start)
	zero := func() interval { return interval{new(big.Rat), new(big.Rat)} }
	sqrt := func(rho *big.Rat) (interval, error) {
		q := a.sub(r2, a.mul(a.sub(rho, s.R), a.sub(rho, s.R)))
		if a.err != nil {
			return interval{}, a.err
		}
		l, h, e := rationalSqrtBounds(q, o.PrecisionBits, b)
		return interval{l, h}, e
	}
	for n := 1; n <= o.MaxCells; n *= 2 {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		step := a.div(length, big.NewRat(int64(n), 1))
		sums := [3]interval{zero(), zero(), zero()}
		left := start
		hl, e := sqrt(left)
		if e != nil {
			return out, e
		}
		for cell := 0; cell < n; cell++ {
			right := a.add(start, a.mul(step, big.NewRat(int64(cell+1), 1)))
			mid := a.div(a.add(left, right), big.NewRat(2, 1))
			hr, e := sqrt(right)
			if e != nil {
				return out, e
			}
			hm, e := sqrt(mid)
			if e != nil {
				return out, e
			}
			if hm[0].Sign() <= 0 {
				return out, ErrConvergence
			}
			// P[k] = integral rho^k d(rho), evaluated exactly, k=1..4.
			var P [5]*big.Rat
			lp, rp := new(big.Rat).Set(left), new(big.Rat).Set(right)
			for k := 1; k <= 4; k++ {
				lp = a.mul(lp, left)
				rp = a.mul(rp, right)
				P[k] = a.div(a.sub(rp, lp), big.NewRat(int64(k+1), 1))
			}
			weighted := func(shift int) *big.Rat {
				return a.add(a.add(a.mul(a.sub(r2, R2), P[1+shift]), a.mul(a.mul(big.NewRat(2, 1), s.R), P[2+shift])), a.sub(new(big.Rat), P[3+shift]))
			}
			I := [3]*big.Rat{P[1], P[3], weighted(0)}
			J := [3]*big.Rat{P[2], P[4], weighted(1)}
			for k := 0; k < 3; k++ {
				wl := a.div(a.sub(a.mul(right, I[k]), J[k]), step)
				wr := a.div(a.sub(J[k], a.mul(left, I[k])), step)
				if wl.Sign() < 0 || wr.Sign() < 0 || I[k].Sign() < 0 {
					return out, ErrConvergence
				}
				lower := a.add(a.mul(wl, hl[0]), a.mul(wr, hr[0]))
				K := a.mul(a.sub(s.R, mid), a.sub(J[k], a.mul(mid, I[k])))
				den := hm[0]
				if K.Sign() < 0 {
					den = hm[1]
				}
				upper := a.add(a.mul(hm[1], I[k]), a.div(K, den))
				sums[k] = add(sums[k], interval{lower, upperDyadic(upper)})
			}
			left = right
			hl = hr
			if a.err != nil {
				return out, a.err
			}
		}
		commonV := scale(mulPos(pi, sums[0]), big.NewRat(4, 1))
		commonX := scale(mulPos(pi, sums[1]), big.NewRat(2, 1))
		commonZ := scale(mulPos(pi, sums[2]), big.NewRat(4, 3))
		out.Volume, out.SecondXY, out.SecondZ, out.FirstZ = commonV, commonX, commonZ, zero()
		if s.op == TorusCylinderDifference {
			out.Volume = clamp(sub(tv, commonV))
			out.SecondXY = clamp(sub(tx, commonX))
			out.SecondZ = clamp(sub(tz, commonZ))
		}
		if s.op == TorusCylinderUnion {
			out.Volume = clamp(sub(add(tv, cv), commonV))
			out.SecondXY = clamp(sub(add(tx, cx), commonX))
			out.SecondZ = clamp(sub(add(tz, cz), commonZ))
			out.FirstZ = first
		}
		if a.err != nil {
			return out, a.err
		}
		if out.Volume[0].Sign() <= 0 {
			continue
		}
		out.CentroidZ = divPos(out.FirstZ, out.Volume)
		shift := zero()
		if s.op == TorusCylinderUnion {
			square := interval{a.mul(first[0], first[0]), a.mul(first[1], first[1])}
			if square[0].Cmp(square[1]) > 0 {
				square[0], square[1] = square[1], square[0]
			}
			shift = divPos(square, out.Volume)
		}
		out.InertiaOrigin[0] = add(out.SecondXY, out.SecondZ)
		out.InertiaOrigin[1] = add(out.SecondXY, out.SecondZ)
		out.InertiaOrigin[2] = scale(out.SecondXY, big.NewRat(2, 1))
		out.InertiaCentroid[0] = clamp(sub(out.InertiaOrigin[0], shift))
		out.InertiaCentroid[1] = clamp(sub(out.InertiaOrigin[1], shift))
		out.InertiaCentroid[2] = scale(out.SecondXY, big.NewRat(2, 1))
		within := func(x interval, t *big.Rat) bool { return a.sub(x[1], x[0]).Cmp(t) <= 0 }
		ok := within(out.Volume, o.VolumeTolerance) && within(out.SecondXY, o.SecondMomentTolerance) && within(out.SecondZ, o.SecondMomentTolerance) && within(out.CentroidZ, o.CentroidTolerance)
		for _, x := range out.InertiaOrigin {
			ok = within(x, o.InertiaTolerance) && ok
		}
		for _, x := range out.InertiaCentroid {
			ok = within(x, o.InertiaTolerance) && ok
		}
		if a.err != nil {
			return out, a.err
		}
		if ok {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			out.Cells = n
			out.ExactOperations = b.operations
			return out, nil
		}
	}
	return out, ErrConvergence
}
