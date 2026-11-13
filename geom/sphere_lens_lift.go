package geom

import (
	"context"
	"math"
	"math/big"
)

// SphereLensLift lifts a cap-material lens fan to the sphere on the selected
// signed-axis side. The shared planform chord halves remain shared curved
// seams after lifting. V=0 retains the deliberate fan collapse. It works even
// when neither circular center is inside the lens. Numeric evaluation is
// separate from exact fan provenance and ideal derivative bounds.
type SphereLensLift struct {
	fan      SphereCylinderLensFan
	center   Vec3
	R        float64
	ax, sign int
	bounds   SphereCylinderLensFanBounds
}

// NewSphereLensLifts requires a strict lens cap at positive height along the
// signed raw axis from the sphere center. Both lifted patches cover the sphere
// above exactly the planar lens. The nonzero cap height is an exact global
// graph denominator bound; an equatorial cap is explicitly unsupported.
func NewSphereLensLifts(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, capParameter *big.Rat, o SurfacePatchEvaluationOptions) (out [2]SphereLensLift, err error) {
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	source, e := ClassifySphereCylinderCapMaterial(ctx, center, sphereRadius, origin, axis, radius, capParameter, o)
	if e != nil {
		return out, e
	}
	if source.ExactOperations >= ops {
		return out, ErrConvergence
	}
	remaining := o
	remaining.MaxOperations = ops - source.ExactOperations
	fans, e := NewSphereCylinderLensFans(ctx, center, sphereRadius, origin, axis, radius, capParameter, remaining)
	if e != nil {
		return out, e
	}
	ax := fans[0].ax
	sign := fans[0].orientation
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	height := new(big.Rat).Add(new(big.Rat).Sub(r(coordinate(origin, ax)), r(coordinate(center, ax))), new(big.Rat).Mul(r(coordinate(axis, ax)), capParameter))
	height.Mul(height, big.NewRat(int64(sign), 1))
	if height.Sign() <= 0 {
		return out, ErrDomain
	}
	// Both source classifiers and the two radius square-root checks are charged.
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: 2*source.ExactOperations + 2}
	mul := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Mul(a, c) }
	add := func(a, c *big.Rat) *big.Rat { return new(big.Rat).Add(a, c) }
	scale := func(a *big.Rat, n int64) *big.Rat { return mul(a, big.NewRat(n, 1)) }
	R := r(sphereRadius)
	gradient := new(big.Rat).Quo(R, height)
	hessian := add(new(big.Rat).Inv(height), new(big.Rat).Quo(mul(R, R), mul(height, mul(height, height))))
	for i, fan := range fans {
		a := fan.radiusUpper
		u, v := scale(a, 8), scale(a, 2)
		uu, uv := scale(a, 64), scale(a, 8)
		UU := add(uu, add(mul(gradient, uu), mul(hessian, mul(u, u))))
		UV := add(uv, add(mul(gradient, uv), mul(hessian, mul(u, v))))
		VV := mul(hessian, mul(v, v))
		for _, q := range []*big.Rat{height, gradient, hessian, UU, UV, VV} {
			if e = b.check(q); e != nil {
				return [2]SphereLensLift{}, e
			}
		}
		out[i] = SphereLensLift{fan: fan, center: center, R: sphereRadius, ax: ax, sign: sign, bounds: SphereCylinderLensFanBounds{UU, UV, VV}}
	}
	return out, nil
}
func (p SphereLensLift) Domain() ParameterRectangle {
	return ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
}
func (p SphereLensLift) Planform() SphereCylinderLensFan { return p.fan }
func (p SphereLensLift) HessianBounds() (SphereCylinderLensFanBounds, error) {
	if p.bounds.UU == nil {
		return SphereCylinderLensFanBounds{}, ErrInvalid
	}
	return SphereCylinderLensFanBounds{new(big.Rat).Set(p.bounds.UU), new(big.Rat).Set(p.bounds.UV), new(big.Rat).Set(p.bounds.VV)}, nil
}
func (p SphereLensLift) jet(u, v float64) (point, du, dv, duu, duv, dvv Vec3, err error) {
	if p.bounds.UU == nil {
		return point, du, dv, duu, duv, dvv, ErrInvalid
	}
	point, du, dv, duu, duv, dvv, e := p.fan.jet(u, v)
	if e != nil {
		return point, du, dv, duu, duv, dvv, e
	}
	x := [3]float64{}
	radial := 0.0
	for i := 0; i < 3; i++ {
		if i != p.ax {
			x[i] = coordinate(point, i) - coordinate(p.center, i)
			radial += x[i] * x[i]
		}
	}
	D := p.R*p.R - radial
	if D <= 0 || !finite(D) {
		return point, du, dv, duu, duv, dvv, ErrConvergence
	}
	h := math.Sqrt(D)
	dotx := func(a Vec3) float64 { return x[0]*a.X + x[1]*a.Y + x[2]*a.Z }
	dot := func(a, b Vec3) float64 {
		sum := 0.0
		for i := 0; i < 3; i++ {
			if i != p.ax {
				sum += coordinate(a, i) * coordinate(b, i)
			}
		}
		return sum
	}
	pu, pv := dotx(du), dotx(dv)
	zu, zv := -pu/h, -pv/h
	zuu := -(dot(du, du)+dotx(duu))/h - pu*pu/(h*h*h)
	zuv := -(dot(du, dv)+dotx(duv))/h - pu*pv/(h*h*h)
	zvv := -(dot(dv, dv)+dotx(dvv))/h - pv*pv/(h*h*h)
	put := func(a *Vec3, z float64) {
		switch p.ax {
		case 0:
			a.X = z
		case 1:
			a.Y = z
		case 2:
			a.Z = z
		}
	}
	sgn := float64(p.sign)
	put(&point, coordinate(p.center, p.ax)+sgn*h)
	put(&du, sgn*zu)
	put(&dv, sgn*zv)
	put(&duu, sgn*zuu)
	put(&duv, sgn*zuv)
	put(&dvv, sgn*zvv)
	for _, q := range []Vec3{point, du, dv, duu, duv, dvv} {
		if !q.IsFinite() {
			return point, du, dv, duu, duv, dvv, ErrConvergence
		}
	}
	return
}
func (p SphereLensLift) Eval(u, v float64) (Vec3, error) {
	x, _, _, _, _, _, e := p.jet(u, v)
	return x, e
}
func (p SphereLensLift) Partials(u, v float64) (Vec3, Vec3, error) {
	_, a, b, _, _, _, e := p.jet(u, v)
	return a, b, e
}
func (p SphereLensLift) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	_, _, _, a, b, c, e := p.jet(u, v)
	return a, b, c, e
}
