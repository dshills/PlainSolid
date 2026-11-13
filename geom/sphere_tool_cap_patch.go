package geom

import (
	"context"
	"math"
	"math/big"
)

// SphereToolCapPatch parameterizes the upper sphere inside a contained offset
// cylindrical footprint: (d+r*v*cos(u),r*v*sin(u),sqrt(R²-x²-y²)). U is radians,
// V runs from the tool axis to its rim. The disk is smooth but V=0 is a polar
// coordinate collapse: Eval is independent of U and du vanishes there. On V>0
// the chart is regular, with inward du×dv. A boundary shell uses reversal for
// the sphere's exterior orientation.
type SphereToolCapPatch struct {
	R, r, d float64
	bounds  SphereToolCapHessian
}
type SphereToolCapHessian struct {
	UU, UV, VV      *big.Rat
	ExactOperations int
}

// NewSphereToolCapPatch proves d+r<R exactly and bounds Euclidean second
// partial norms globally, including the polar collapse. This supports Hessian
// deflection bounds without division by the vanishing polar Jacobian.
func NewSphereToolCapPatch(ctx context.Context, R, r, d float64, o SurfacePatchEvaluationOptions) (out SphereToolCapPatch, err error) {
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
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	rr, tool, off := rat(R), rat(r), rat(d)
	sum := new(big.Rat).Add(tool, off)
	if sum.Cmp(rr) >= 0 {
		return out, ErrDomain
	}
	h2 := new(big.Rat).Sub(new(big.Rat).Mul(rr, rr), new(big.Rat).Mul(sum, sum))
	if e := b.check(h2); e != nil {
		return out, e
	}
	h, _, e := rationalSqrtBounds(h2, 128, b)
	if e != nil {
		return out, e
	}
	if h.Sign() <= 0 {
		return out, ErrConvergence
	}
	mul := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Mul(a, c) }
	add := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Add(a, c) }
	twice := func(a *big.Rat) *big.Rat { return mul(big.NewRat(2, 1), a) }
	Fu := twice(mul(off, tool))
	Fv := twice(mul(tool, sum))
	Fvv := twice(mul(tool, tool))
	den1 := twice(h)
	den2 := mul(big.NewRat(4, 1), mul(h, mul(h, h)))
	second := func(x, a, c *big.Rat) *big.Rat {
		return add(new(big.Rat).Quo(x, den1), new(big.Rat).Quo(mul(a, c), den2))
	}
	UU := add(tool, second(Fu, Fu, Fu))
	UV := add(tool, second(Fu, Fu, Fv))
	VV := second(Fvv, Fv, Fv)
	for _, q := range []*big.Rat{rr, tool, off, Fu, Fv, Fvv, UU, UV, VV} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	return SphereToolCapPatch{R, r, d, SphereToolCapHessian{UU, UV, VV, b.operations}}, nil
}
func (p SphereToolCapPatch) Parameters() (sphereRadius, toolRadius, offset float64) {
	return p.R, p.r, p.d
}
func (p SphereToolCapPatch) Domain() ParameterRectangle {
	return ParameterRectangle{UMin: 0, UMax: 2 * math.Pi, VMin: 0, VMax: 1}
}
func (p SphereToolCapPatch) HessianBounds() (SphereToolCapHessian, error) {
	if p.bounds.UU == nil {
		return SphereToolCapHessian{}, ErrInvalid
	}
	return SphereToolCapHessian{new(big.Rat).Set(p.bounds.UU), new(big.Rat).Set(p.bounds.UV), new(big.Rat).Set(p.bounds.VV), p.bounds.ExactOperations}, nil
}
func (p SphereToolCapPatch) jet(u, v float64) (point, du, dv, duu, duv, dvv Vec3, err error) {
	if p.R <= 0 || !finite(u) || !finite(v) || v < 0 || v > 1 {
		return point, du, dv, duu, duv, dvv, ErrDomain
	}
	s, c := math.Sincos(u)
	rho := p.r * v
	x, y := p.d+rho*c, rho*s
	F := p.R*p.R - x*x - y*y
	if F <= 0 || !finite(F) {
		return point, du, dv, duu, duv, dvv, ErrConvergence
	}
	h := math.Sqrt(F)
	Fu := 2 * p.d * rho * s
	Fv := -2*p.r*rho - 2*p.d*p.r*c
	Fuu := 2 * p.d * rho * c
	Fuv := 2 * p.d * p.r * s
	Fvv := -2 * p.r * p.r
	hu, hv := Fu/(2*h), Fv/(2*h)
	huu := Fuu/(2*h) - Fu*Fu/(4*h*h*h)
	huv := Fuv/(2*h) - Fu*Fv/(4*h*h*h)
	hvv := Fvv/(2*h) - Fv*Fv/(4*h*h*h)
	point = Vec3{X: x, Y: y, Z: h}
	du = Vec3{X: -rho * s, Y: rho * c, Z: hu}
	dv = Vec3{X: p.r * c, Y: p.r * s, Z: hv}
	duu = Vec3{X: -rho * c, Y: -rho * s, Z: huu}
	duv = Vec3{X: -p.r * s, Y: p.r * c, Z: huv}
	dvv = Vec3{Z: hvv}
	for _, q := range []Vec3{point, du, dv, duu, duv, dvv} {
		if !q.IsFinite() {
			return point, du, dv, duu, duv, dvv, ErrConvergence
		}
	}
	return
}
func (p SphereToolCapPatch) Eval(u, v float64) (Vec3, error) {
	x, _, _, _, _, _, e := p.jet(u, v)
	return x, e
}
func (p SphereToolCapPatch) Partials(u, v float64) (Vec3, Vec3, error) {
	_, a, b, _, _, _, e := p.jet(u, v)
	return a, b, e
}
func (p SphereToolCapPatch) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	_, _, _, a, b, c, e := p.jet(u, v)
	return a, b, c, e
}
