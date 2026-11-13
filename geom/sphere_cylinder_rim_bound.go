package geom

import (
	"context"
	"math"
	"math/big"
)

// SphereCylinderRimHeight certifies the upper rim measured along the signed
// coordinate axis from the sphere center's axial coordinate. θ uses radial
// coordinate j=(axis+1)%3 and signed k=(axis+2)%3. All public rational bounds
// are caller-owned; numerical evaluation uses private copied coefficients.
// Angular derivatives are per radian. The lower rim has opposite derivatives
// and identical absolute bounds.
type SphereCylinderRimHeight struct {
	Proven                                            bool
	HeightBounds                                      [2]*big.Rat
	FirstDerivativeAbsolute, SecondDerivativeAbsolute *big.Rat
	RimSecondDerivativeNormSquared                    *big.Rat
	ExactOperations                                   int
	a, b, c                                           *big.Rat
}

// BoundSphereCylinderRimHeight proves d+radius<sphereRadius exactly, for a
// coordinate-axis cylinder with arbitrary represented axis scale/sign. It
// bounds h=sqrt(A+Bcosθ+Csinθ) globally, with |h'|<=L/(2*hmin) and
// |h”|<=L/(2*hmin)+L²/(4*hmin³), L=sqrt(B²+C²). The full spatial rim has
// |p”|²<=radius²+|h”|². Thus its ideal chord deviation is bounded by
// sqrt(RimSecondDerivativeNormSquared)*deltaTheta²/8; numerical sample
// rounding must be budgeted independently. Near-zero margins may exceed the
// bounded square-root precision and return ErrConvergence.
func BoundSphereCylinderRimHeight(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, o SurfacePatchEvaluationOptions) (out SphereCylinderRimHeight, err error) {
	if ctx == nil || !center.IsFinite() || !origin.IsFinite() || !axis.IsFinite() || !finite(sphereRadius) || !finite(radius) || sphereRadius <= 0 || radius <= 0 {
		return out, ErrInvalid
	}
	ax := -1
	for i := 0; i < 3; i++ {
		if coordinate(axis, i) != 0 {
			if ax >= 0 {
				return out, ErrDomain
			}
			ax = i
		}
	}
	if ax < 0 {
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
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = budget.operations
		if err != nil {
			out.Proven = false
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	j, k := (ax+1)%3, (ax+2)%3
	dj := new(big.Rat).Sub(r(coordinate(center, j)), r(coordinate(origin, j)))
	dk := new(big.Rat).Sub(r(coordinate(center, k)), r(coordinate(origin, k)))
	rad := r(radius)
	twoR := new(big.Rat).Mul(big.NewRat(2, 1), rad)
	B, C := new(big.Rat).Mul(twoR, dj), new(big.Rat).Mul(twoR, dk)
	if coordinate(axis, ax) < 0 {
		C.Neg(C)
	}
	A := new(big.Rat).Sub(new(big.Rat).Sub(new(big.Rat).Mul(r(sphereRadius), r(sphereRadius)), new(big.Rat).Mul(rad, rad)), new(big.Rat).Add(new(big.Rat).Mul(dj, dj), new(big.Rat).Mul(dk, dk)))
	L2 := new(big.Rat).Add(new(big.Rat).Mul(B, B), new(big.Rat).Mul(C, C))
	margin := new(big.Rat).Sub(new(big.Rat).Mul(A, A), L2)
	for _, q := range []*big.Rat{A, B, C, L2, margin} {
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	if A.Sign() <= 0 || margin.Sign() <= 0 {
		return out, ErrDomain
	}
	_, L, e := rationalSqrtBounds(L2, 128, budget)
	if e != nil {
		return out, e
	}
	minimum := new(big.Rat).Sub(A, L)
	if minimum.Sign() <= 0 {
		return out, ErrConvergence
	}
	maximum := new(big.Rat).Add(A, L)
	hmin, _, e := rationalSqrtBounds(minimum, 128, budget)
	if e != nil {
		return out, e
	}
	_, hmax, e := rationalSqrtBounds(maximum, 128, budget)
	if e != nil {
		return out, e
	}
	if hmin.Sign() <= 0 {
		return out, ErrConvergence
	}
	first := new(big.Rat).Quo(L, new(big.Rat).Mul(big.NewRat(2, 1), hmin))
	cube := new(big.Rat).Mul(hmin, new(big.Rat).Mul(hmin, hmin))
	second := new(big.Rat).Add(first, new(big.Rat).Quo(L2, new(big.Rat).Mul(big.NewRat(4, 1), cube)))
	norm := new(big.Rat).Add(new(big.Rat).Mul(rad, rad), new(big.Rat).Mul(second, second))
	for _, q := range []*big.Rat{hmin, hmax, first, second, norm} {
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	out.HeightBounds = [2]*big.Rat{hmin, hmax}
	out.FirstDerivativeAbsolute = first
	out.SecondDerivativeAbsolute = second
	out.RimSecondDerivativeNormSquared = norm
	out.a = new(big.Rat).Set(A)
	out.b = new(big.Rat).Set(B)
	out.c = new(big.Rat).Set(C)
	out.Proven = true
	return out, nil
}

// Evaluate returns numerical h,h',h” at angle in radians. Exact enclosure
// evidence comes from BoundSphereCylinderRimHeight, not floating evaluation.
func (p SphereCylinderRimHeight) Evaluate(angle float64) (height, first, second float64, err error) {
	if p.a == nil || !finite(angle) {
		return 0, 0, 0, ErrInvalid
	}
	A, _ := p.a.Float64()
	B, _ := p.b.Float64()
	C, _ := p.c.Float64()
	s, c := math.Sincos(angle)
	D := A + B*c + C*s
	dp := -B*s + C*c
	dd := -B*c - C*s
	if D <= 0 || !finite(D) {
		return 0, 0, 0, ErrConvergence
	}
	height = math.Sqrt(D)
	first = dp / (2 * height)
	second = dd/(2*height) - dp*dp/(4*height*height*height)
	if !finite(first) || !finite(second) {
		return 0, 0, 0, ErrConvergence
	}
	return
}
