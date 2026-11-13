package geom

import (
	"context"
	"math"
	"math/big"
)

// SphereToolUpperPatch is a smooth annular upper-sphere chart. The sphere is
// centered at zero and tool axis at (Offset,0), both along Z. U is tool azimuth
// in radians, V∈[0,1] runs from the cylindrical rim to the equator. R>r+d and
// d>=0 are certified exactly; d need not be smaller than r. Numerical methods
// provide representatives; HessianBounds supplies independent exact evidence.
type SphereToolUpperPatch struct {
	R, r, d float64
	bounds  SphereToolUpperHessian
}

// SphereToolUpperHessian gives global Euclidean second-partial norm bounds
// per radian and unit V. The rational bounds include both XY and Z components.
// Standard bilinear patch deflection may use these without equator blow-up.
type SphereToolUpperHessian struct {
	UU, UV, VV      *big.Rat
	ExactOperations int
}

func NewSphereToolUpperPatch(ctx context.Context, R, r, d float64, o SurfacePatchEvaluationOptions) (out SphereToolUpperPatch, err error) {
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
	add := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Add(a, c) }
	sub := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Sub(a, c) }
	mul := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Mul(a, c) }
	div := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Quo(a, c) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	scale := func(a *big.Rat, n int64) *big.Rat { return mul(a, big.NewRat(n, 1)) }
	RR, rr, dd := rat(R), rat(r), rat(d)
	if sub(RR, add(rr, dd)).Sign() <= 0 {
		return out, ErrDomain
	}
	m2, h2 := sub(sq(RR), sq(dd)), sub(sq(RR), sq(add(dd, rr)))
	for _, q := range []*big.Rat{RR, rr, dd, m2, h2} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	m, _, e := rationalSqrtBounds(m2, 128, b)
	if e != nil {
		return out, e
	}
	h, _, e := rationalSqrtBounds(h2, 128, b)
	if e != nil {
		return out, e
	}
	if m.Sign() <= 0 || h.Sign() <= 0 {
		return out, ErrConvergence
	}
	K := sub(add(RR, dd), rr)
	E := add(RR, dd)
	S1 := div(sq(dd), scale(m, 2))
	S2 := add(div(sq(dd), m), div(sq(sq(dd)), scale(mul(m, sq(m)), 4)))
	E1, E2 := add(dd, S1), add(dd, S2)
	// F=2*k*s-k²*w², w=1-v. Its positive global lower bound h²
	// is the minimum tool-rim sphere height squared, valid for every w∈[0,1].
	Fu := scale(add(add(mul(E1, RR), mul(K, S1)), mul(K, E1)), 2)
	Fuu := add(add(scale(mul(E2, RR), 2), scale(mul(E1, S1), 4)), add(scale(mul(K, S2), 2), add(scale(sq(E1), 2), scale(mul(K, E2), 2))))
	Fv, Fvv, Fuv := scale(sq(K), 2), scale(sq(K), 2), scale(mul(K, E1), 4)
	twoH, fourH3 := scale(h, 2), scale(mul(h, sq(h)), 4)
	Gu, Gv := div(Fu, twoH), div(Fv, twoH)
	Guu := add(div(Fuu, twoH), div(sq(Fu), fourH3))
	Gvv := add(div(Fvv, twoH), div(sq(Fv), fourH3))
	Guv := add(div(Fuv, twoH), div(mul(Fu, Fv), fourH3))
	// z=w*g: z_uu=w*g_uu, z_uv=-g_u+w*g_uv,
	// z_vv=-2*g_v+w*g_vv. XY norm uses the radial unit basis.
	UU := add(add(add(E2, scale(E1, 2)), E), Guu)
	UV := add(add(scale(E1, 2), scale(K, 2)), add(Gu, Guv))
	VV := add(scale(K, 2), add(scale(Gv, 2), Gvv))
	for _, q := range []*big.Rat{K, S1, S2, E1, E2, Fu, Fuu, Fv, Fvv, Fuv, UU, UV, VV} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out = SphereToolUpperPatch{R: R, r: r, d: d, bounds: SphereToolUpperHessian{UU, UV, VV, b.operations}}
	return out, nil
}
func (p SphereToolUpperPatch) Parameters() (sphereRadius, toolRadius, offset float64) {
	return p.R, p.r, p.d
}
func (p SphereToolUpperPatch) Domain() ParameterRectangle {
	return ParameterRectangle{UMin: 0, UMax: 2 * math.Pi, VMin: 0, VMax: 1}
}
func (p SphereToolUpperPatch) HessianBounds() (SphereToolUpperHessian, error) {
	if p.bounds.UU == nil {
		return SphereToolUpperHessian{}, ErrInvalid
	}
	return SphereToolUpperHessian{new(big.Rat).Set(p.bounds.UU), new(big.Rat).Set(p.bounds.UV), new(big.Rat).Set(p.bounds.VV), p.bounds.ExactOperations}, nil
}
func (p SphereToolUpperPatch) jet(u, v float64) (point, du, dv, duu, duv, dvv Vec3, err error) {
	if p.R <= 0 || !finite(u) || !finite(v) || v < 0 || v > 1 {
		return point, du, dv, duu, duv, dvv, ErrDomain
	}
	sn, cs := math.Sincos(u)
	w := 1 - v
	S2 := p.R*p.R - p.d*p.d*sn*sn
	if S2 <= 0 || !finite(S2) {
		return point, du, dv, duu, duv, dvv, ErrConvergence
	}
	s := math.Sqrt(S2)
	ds := -p.d * p.d * sn * cs / s
	dds := -p.d*p.d*(cs*cs-sn*sn)/s - p.d*p.d*p.d*p.d*sn*sn*cs*cs/(s*s*s)
	e := -p.d*cs + s
	de := p.d*sn + ds
	dde := p.d*cs + dds
	k := e - p.r
	rho := e - k*w*w
	ru := de * (1 - w*w)
	rv := 2 * k * w
	ruu := dde * (1 - w*w)
	ruv := 2 * de * w
	rvv := -2 * k
	F := 2*k*s - k*k*w*w
	if F <= 0 || !finite(F) {
		return point, du, dv, duu, duv, dvv, ErrConvergence
	}
	Fu := 2*de*s + 2*k*ds - 2*k*de*w*w
	Fv := 2 * k * k * w
	Fuu := 2*dde*s + 4*de*ds + 2*k*dds - 2*(de*de+k*dde)*w*w
	Fuv := 4 * k * de * w
	Fvv := -2 * k * k
	g := math.Sqrt(F)
	gu, gv := Fu/(2*g), Fv/(2*g)
	guu := Fuu/(2*g) - Fu*Fu/(4*g*g*g)
	guv := Fuv/(2*g) - Fu*Fv/(4*g*g*g)
	gvv := Fvv/(2*g) - Fv*Fv/(4*g*g*g)
	point = Vec3{X: p.d + rho*cs, Y: rho * sn, Z: w * g}
	du = Vec3{X: ru*cs - rho*sn, Y: ru*sn + rho*cs, Z: w * gu}
	dv = Vec3{X: rv * cs, Y: rv * sn, Z: -g + w*gv}
	duu = Vec3{X: (ruu-rho)*cs - 2*ru*sn, Y: (ruu-rho)*sn + 2*ru*cs, Z: w * guu}
	duv = Vec3{X: ruv*cs - rv*sn, Y: ruv*sn + rv*cs, Z: -gu + w*guv}
	dvv = Vec3{X: rvv * cs, Y: rvv * sn, Z: -2*gv + w*gvv}
	for _, q := range []Vec3{point, du, dv, duu, duv, dvv} {
		if !q.IsFinite() {
			return point, du, dv, duu, duv, dvv, ErrConvergence
		}
	}
	return
}
func (p SphereToolUpperPatch) Eval(u, v float64) (Vec3, error) {
	x, _, _, _, _, _, e := p.jet(u, v)
	return x, e
}
func (p SphereToolUpperPatch) Partials(u, v float64) (Vec3, Vec3, error) {
	_, a, b, _, _, _, e := p.jet(u, v)
	return a, b, e
}
func (p SphereToolUpperPatch) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	_, _, _, a, b, c, e := p.jet(u, v)
	return a, b, c, e
}
