package geom

import (
	"context"
	"math"
	"math/big"
)

// DualCircleExitChart maps a common-pole annulus between concentric circles.
// Circle center is (0,0), pole is (PoleOffset,0), with exact rational offset.
// U is azimuth in radians and V interpolates INNER to OUTER radial exits.
// Pole strictly inside the inner circle and innerRadius²<outerRadius² prove
// both positive exits and positive strip width everywhere; there is no pole
// collapse in this chart. Tool-footprint clipping is a separate exact event
// problem and is not silently included here.
type DualCircleExitChart struct {
	inner, outer, pole     *big.Rat
	innerBound, outerBound OffsetCircleExitBounds
	bounds                 SphereCylinderLensFanBounds
	firstU, firstV         *big.Rat
	ai, ao, d              float64
}

func NewDualCircleExitChart(ctx context.Context, innerSquared, outerSquared, poleOffset *big.Rat, o SurfacePatchEvaluationOptions) (out DualCircleExitChart, err error) {
	if ctx == nil || innerSquared == nil || outerSquared == nil || poleOffset == nil || innerSquared.Sign() <= 0 || innerSquared.Cmp(outerSquared) >= 0 {
		return out, ErrInvalid
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
	a, e := BoundRationalOffsetCircleExit(ctx, innerSquared, poleOffset, o)
	if e != nil {
		return out, e
	}
	if a.ExactOperations >= ops {
		return out, ErrConvergence
	}
	oo := o
	oo.MaxOperations = ops - a.ExactOperations
	c, e := BoundRationalOffsetCircleExit(ctx, outerSquared, poleOffset, oo)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: a.ExactOperations + c.ExactOperations}
	max := func(x, y *big.Rat) *big.Rat {
		if x.Cmp(y) > 0 {
			return new(big.Rat).Set(x)
		}
		return new(big.Rat).Set(y)
	}
	A1, A2 := max(a.FirstAbsolute, c.FirstAbsolute), max(a.SecondAbsolute, c.SecondAbsolute)
	rho := c.Value[1]
	dv := new(big.Rat).Sub(c.Value[1], a.Value[0])
	duv := new(big.Rat).Add(a.FirstAbsolute, c.FirstAbsolute)
	UU := new(big.Rat).Add(A2, new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), A1), rho))
	UV := new(big.Rat).Add(duv, dv)
	firstU := new(big.Rat).Add(A1, rho)
	for _, q := range []*big.Rat{UU, UV, dv, firstU} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	ai, _ := innerSquared.Float64()
	ao, _ := outerSquared.Float64()
	d, _ := poleOffset.Float64()
	if !finite(ai) || !finite(ao) || !finite(d) || ai <= 0 {
		return out, ErrUnrepresentable
	}
	out = DualCircleExitChart{new(big.Rat).Set(innerSquared), new(big.Rat).Set(outerSquared), new(big.Rat).Set(poleOffset), a, c, SphereCylinderLensFanBounds{UU, UV, new(big.Rat)}, firstU, dv, ai, ao, d}
	return out, nil
}
func (p DualCircleExitChart) Parameters() (innerSquared, outerSquared, poleOffset *big.Rat, err error) {
	if p.inner == nil {
		return nil, nil, nil, ErrInvalid
	}
	return new(big.Rat).Set(p.inner), new(big.Rat).Set(p.outer), new(big.Rat).Set(p.pole), nil
}
func (p DualCircleExitChart) Domain() ParameterRectangle {
	return ParameterRectangle{UMin: 0, UMax: 2 * math.Pi, VMin: 0, VMax: 1}
}
func (p DualCircleExitChart) HessianBounds() (SphereCylinderLensFanBounds, error) {
	if p.inner == nil {
		return SphereCylinderLensFanBounds{}, ErrInvalid
	}
	return SphereCylinderLensFanBounds{new(big.Rat).Set(p.bounds.UU), new(big.Rat).Set(p.bounds.UV), new(big.Rat)}, nil
}

// SphereLiftBounds bounds the same planar strip lifted to positive
// z=sqrt(sphereRadius²-x²-y²). Outer radius² must be strictly smaller.
func (p DualCircleExitChart) SphereLiftBounds(ctx context.Context, sphereSquared *big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderLensFanBounds, err error) {
	if ctx == nil || p.inner == nil || sphereSquared == nil || sphereSquared.Cmp(p.outer) <= 0 {
		return out, ErrDomain
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
	h, _, e := rationalSqrtBounds(new(big.Rat).Sub(sphereSquared, p.outer), 128, b)
	if e != nil {
		return out, e
	}
	_, radius, e := rationalSqrtBounds(p.outer, 128, b)
	if e != nil {
		return out, e
	}
	if h.Sign() <= 0 {
		return out, ErrConvergence
	}
	gradient := new(big.Rat).Quo(radius, h)
	H := new(big.Rat).Add(new(big.Rat).Inv(h), new(big.Rat).Quo(p.outer, new(big.Rat).Mul(h, new(big.Rat).Mul(h, h))))
	bound := func(base, a, c *big.Rat) *big.Rat {
		return new(big.Rat).Add(base, new(big.Rat).Add(new(big.Rat).Mul(gradient, base), new(big.Rat).Mul(H, new(big.Rat).Mul(a, c))))
	}
	out.UU = bound(p.bounds.UU, p.firstU, p.firstU)
	out.UV = bound(p.bounds.UV, p.firstU, p.firstV)
	out.VV = bound(new(big.Rat), p.firstV, p.firstV)
	for _, q := range []*big.Rat{out.UU, out.UV, out.VV} {
		if e = b.check(q); e != nil {
			return SphereCylinderLensFanBounds{}, e
		}
	}
	return out, nil
}
func (p DualCircleExitChart) jet(u, v float64) (point, du, dv, duu, duv, dvv Vec3, err error) {
	if p.inner == nil || !finite(u) || !finite(v) || v < 0 || v > 1 {
		return point, du, dv, duu, duv, dvv, ErrDomain
	}
	s, c := math.Sincos(u)
	exit := func(a float64) (A, A1, A2 float64, ok bool) {
		D := a - p.d*p.d*s*s
		if D <= 0 {
			return
		}
		h := math.Sqrt(D)
		hp := -p.d * p.d * s * c / h
		hpp := -p.d*p.d*(c*c-s*s)/h - p.d*p.d*p.d*p.d*s*s*c*c/(h*h*h)
		return -p.d*c + h, p.d*s + hp, p.d*c + hpp, true
	}
	a, a1, a2, ok := exit(p.ai)
	z, z1, z2, ok2 := exit(p.ao)
	if !ok || !ok2 || a <= 0 || z <= a {
		return point, du, dv, duu, duv, dvv, ErrConvergence
	}
	rho := (1-v)*a + v*z
	ru := (1-v)*a1 + v*z1
	ruu := (1-v)*a2 + v*z2
	rv, ruv := z-a, z1-a1
	point = Vec3{X: p.d + rho*c, Y: rho * s}
	du = Vec3{X: ru*c - rho*s, Y: ru*s + rho*c}
	dv = Vec3{X: rv * c, Y: rv * s}
	duu = Vec3{X: (ruu-rho)*c - 2*ru*s, Y: (ruu-rho)*s + 2*ru*c}
	duv = Vec3{X: ruv*c - rv*s, Y: ruv*s + rv*c}
	for _, q := range []Vec3{point, du, dv, duu, duv} {
		if !q.IsFinite() {
			return point, du, dv, duu, duv, dvv, ErrConvergence
		}
	}
	return
}
func (p DualCircleExitChart) Eval(u, v float64) (Vec3, error) {
	x, _, _, _, _, _, e := p.jet(u, v)
	return x, e
}
func (p DualCircleExitChart) Partials(u, v float64) (Vec3, Vec3, error) {
	_, a, b, _, _, _, e := p.jet(u, v)
	return a, b, e
}
func (p DualCircleExitChart) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	_, _, _, a, b, c, e := p.jet(u, v)
	return a, b, c, e
}
