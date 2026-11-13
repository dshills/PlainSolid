package geom

import (
	"context"
	"math/big"
)

// SphereCenteredBoreBounds are global outward rational bounds in radians for
// rho=d*cos(theta)+sqrt(r²-d²*sin²(theta)), h=sqrt(R²-rho²), and
// latitude=atan2(h,rho). Derivative fields bound absolute magnitudes.
// Assumptions 0<=d<r and d+r<R are proved with exact represented inputs.
type SphereCenteredBoreBounds struct {
	Proven                                                                        bool
	Rho, Height                                                                   [2]*big.Rat
	RhoFirst, RhoSecond, HeightFirst, HeightSecond, LatitudeFirst, LatitudeSecond *big.Rat
	ExactOperations                                                               int
}

// BoundSphereCenteredBore supplies Hessian ingredients for a sphere-centered
// angular chart of a strictly contained offset cylindrical bore. Root lower
// bounds have 128 fractional bits; an unresolved positive margin returns
// ErrConvergence. Numerical trigonometric rounding is not included.
func BoundSphereCenteredBore(ctx context.Context, R, r, d float64, o SurfacePatchEvaluationOptions) (out SphereCenteredBoreBounds, err error) {
	if ctx == nil || !finite(R) || !finite(r) || !finite(d) || R <= 0 || r <= 0 || d < 0 {
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
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	rr, tool, offset := rat(R), rat(r), rat(d)
	low, high := new(big.Rat).Sub(tool, offset), new(big.Rat).Add(tool, offset)
	if low.Sign() <= 0 || high.Cmp(rr) >= 0 {
		return out, ErrDomain
	}
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	cube := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, sq(x)) }
	d2 := sq(offset)
	m2 := new(big.Rat).Sub(sq(tool), d2)
	hmin2 := new(big.Rat).Sub(sq(rr), sq(high))
	hmax2 := new(big.Rat).Sub(sq(rr), sq(low))
	for _, q := range []*big.Rat{low, high, m2, hmin2, hmax2} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	m, _, e := rationalSqrtBounds(m2, 128, b)
	if e != nil {
		return out, e
	}
	hmin, _, e := rationalSqrtBounds(hmin2, 128, b)
	if e != nil {
		return out, e
	}
	_, hmax, e := rationalSqrtBounds(hmax2, 128, b)
	if e != nil {
		return out, e
	}
	if m.Sign() <= 0 || hmin.Sign() <= 0 {
		return out, ErrConvergence
	}
	// E=r²-d²sin²(theta): |E'|<=d², |E''|<=2d².
	B1 := new(big.Rat).Add(offset, new(big.Rat).Quo(d2, new(big.Rat).Mul(big.NewRat(2, 1), m)))
	B2 := new(big.Rat).Add(offset, new(big.Rat).Add(new(big.Rat).Quo(d2, m), new(big.Rat).Quo(sq(d2), new(big.Rat).Mul(big.NewRat(4, 1), cube(m)))))
	H1 := new(big.Rat).Quo(new(big.Rat).Mul(high, B1), hmin)
	H2 := new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Add(sq(B1), new(big.Rat).Mul(high, B2)), hmin), new(big.Rat).Quo(new(big.Rat).Mul(sq(high), sq(B1)), cube(hmin)))
	// L'=-rho'/h; L''=-rho''/h+rho'*h'/h².
	L1 := new(big.Rat).Quo(B1, hmin)
	L2 := new(big.Rat).Add(new(big.Rat).Quo(B2, hmin), new(big.Rat).Quo(new(big.Rat).Mul(B1, H1), sq(hmin)))
	for _, q := range []*big.Rat{hmin, hmax, B1, B2, H1, H2, L1, L2} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	out.Rho = [2]*big.Rat{low, high}
	out.Height = [2]*big.Rat{hmin, hmax}
	out.RhoFirst = B1
	out.RhoSecond = B2
	out.HeightFirst = H1
	out.HeightSecond = H2
	out.LatitudeFirst = L1
	out.LatitudeSecond = L2
	out.Proven = true
	return out, nil
}
