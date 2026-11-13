package geom

import (
	"context"
	"math"
	"math/big"
)

// SphereToolEquator reparameterizes a sphere's equator by rays from an offset
// tool center (d,0), with 0<=d<R. This does not require d<boreRadius. The map
// theta→sphere angle is an orientation-preserving degree-one diffeomorphism.
// Returned bounds are exact outward rationals; numerical methods retain their
// own private coefficients. Angles use radians and principal atan2 branches.
type SphereToolEquator struct {
	Proven                        bool
	RayRadius                     [2]*big.Rat
	SphereAngleFirst              [2]*big.Rat
	SphereAngleSecondAbsolute     *big.Rat
	ModelSecondDerivativeAbsolute *big.Rat
	ExactOperations               int
	R, d                          float64
}

// NewSphereToolEquator certifies a common seam parameter for a tool-centered
// upper sphere patch and a centered lower hemisphere. With s=sqrt(R²-d²sin²θ)
// and rho=-d cosθ+s, phi'=rho/s>0. The strict radial margin avoids a seam
// fold. ModelSecondDerivativeAbsolute bounds the spatial equator |p”|;
// ideal chord deflection is at most that bound*deltaTheta²/8.
func NewSphereToolEquator(ctx context.Context, R, d float64, o SurfacePatchEvaluationOptions) (out SphereToolEquator, err error) {
	if ctx == nil || !finite(R) || !finite(d) || R <= 0 || d < 0 {
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
	r, offset := new(big.Rat).SetFloat64(R), new(big.Rat).SetFloat64(d)
	lo, hi := new(big.Rat).Sub(r, offset), new(big.Rat).Add(r, offset)
	if lo.Sign() <= 0 {
		return out, ErrDomain
	}
	d2 := new(big.Rat).Mul(offset, offset)
	m2 := new(big.Rat).Sub(new(big.Rat).Mul(r, r), d2)
	if e := b.check(m2); e != nil {
		return out, e
	}
	m, _, e := rationalSqrtBounds(m2, 128, b)
	if e != nil {
		return out, e
	}
	if m.Sign() <= 0 {
		return out, ErrConvergence
	}
	firstLo := new(big.Rat).Quo(lo, r)
	firstHi := new(big.Rat).Quo(hi, m)
	rhoFirst := new(big.Rat).Add(offset, new(big.Rat).Quo(d2, new(big.Rat).Mul(big.NewRat(2, 1), m)))
	cube := new(big.Rat).Mul(m, new(big.Rat).Mul(m, m))
	second := new(big.Rat).Add(new(big.Rat).Quo(rhoFirst, m), new(big.Rat).Quo(new(big.Rat).Mul(hi, d2), new(big.Rat).Mul(big.NewRat(2, 1), cube)))
	modelSecond := new(big.Rat).Mul(r, new(big.Rat).Add(new(big.Rat).Mul(firstHi, firstHi), second))
	for _, q := range []*big.Rat{lo, hi, firstLo, firstHi, second, modelSecond} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	out.RayRadius = [2]*big.Rat{lo, hi}
	out.SphereAngleFirst = [2]*big.Rat{firstLo, firstHi}
	out.SphereAngleSecondAbsolute = second
	out.ModelSecondDerivativeAbsolute = modelSecond
	out.R = R
	out.d = d
	out.Proven = true
	return out, nil
}

// Evaluate returns the numerical equatorial point in sphere-centered XY,
// sphere azimuth phi, and phi's first and second derivatives in theta.
// Principal-angle wrapping is a display convention; derivatives refer to the
// continuous lift. For a lower hemisphere use (R*cos(latitude)*cos(phi),
// R*cos(latitude)*sin(phi),R*sin(latitude)), retaining theta on the seam.
func (c SphereToolEquator) Evaluate(theta float64) (point Vec2, phi, first, second float64, err error) {
	if c.R <= 0 || !finite(theta) {
		return point, 0, 0, 0, ErrInvalid
	}
	sn, cs := math.Sincos(theta)
	disc := c.R*c.R - c.d*c.d*sn*sn
	if disc <= 0 || !finite(disc) {
		return point, 0, 0, 0, ErrConvergence
	}
	s := math.Sqrt(disc)
	rho := -c.d*cs + s
	sp := -c.d * c.d * sn * cs / s
	rp := c.d*sn + sp
	point = Vec2{X: c.d + rho*cs, Y: rho * sn}
	phi = math.Atan2(point.Y, point.X)
	first = rho / s
	second = rp/s - rho*sp/(s*s)
	if !finite(point.X) || !finite(point.Y) || !finite(first) || !finite(second) || first <= 0 {
		return Vec2{}, 0, 0, 0, ErrConvergence
	}
	return
}

// Inverse returns the principal tool-ray angle at a sphere azimuth. It is a
// numerical inverse of the certified diffeomorphism, not an exact-angle claim.
func (c SphereToolEquator) Inverse(phi float64) (float64, error) {
	if c.R <= 0 || !finite(phi) {
		return 0, ErrInvalid
	}
	s, q := math.Sincos(phi)
	angle := math.Atan2(c.R*s, c.R*q-c.d)
	if !finite(angle) {
		return 0, ErrConvergence
	}
	return angle, nil
}
