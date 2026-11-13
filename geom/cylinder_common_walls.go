package geom

import (
	"context"
	"math"
	"math/big"
)

// CylinderCommonWall is one wall of two infinite intersecting cylinder solids.
// U is common-perpendicular latitude in [-pi/2,pi/2], V is a section coordinate
// in [-1,1]. Unequal radii give a shared line at each latitude endpoint on the
// smaller cylinder, and collapsed endpoints on the larger cylinder.
// Ideal geometric bounds do not include numerical evaluation rounding.
type CylinderCommonWall struct {
	center, A, B, W      Vec3
	ru, rv, small, delta float64
	owner, side          int
	bounds               SphereCylinderLensFanBounds
}

// NewCylinderCommonWalls accepts arbitrary nonparallel represented axes through
// center and positive radii. It returns owner0 sides -1,+1 then owner1 sides
// -1,+1. Exact squared norms/projections establish the support identities.
// Writing N=u²,M=v²,D=NM-(u.v)², the basis is A=u sqrt(M/D),
// B=v sqrt(N/D), W=(u cross v)/sqrt(D). At physical perpendicular height y,
// |a|<=sqrt(rv²-y²), |b|<=sqrt(ru²-y²) maps exactly to the common solid.
func NewCylinderCommonWalls(ctx context.Context, center, u, v Vec3, ru, rv float64, o SurfacePatchEvaluationOptions) (out [4]CylinderCommonWall, err error) {
	if !center.IsFinite() {
		return out, ErrInvalid
	}
	proof, e := ClassifyCylinderCommonCaps(ctx, u, v, ru, rv, nil, nil, o)
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
	budget := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: proof.ExactOperations}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	quo := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }
	U, V, W := [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}
	N, M, C := new(big.Rat), new(big.Rat), new(big.Rat)
	for i := 0; i < 3; i++ {
		U[i] = rat(coordinate(u, i))
		V[i] = rat(coordinate(v, i))
		N.Add(N, mul(U[i], U[i]))
		M.Add(M, mul(V[i], V[i]))
		C.Add(C, mul(U[i], V[i]))
	}
	D := new(big.Rat).Sub(mul(N, M), mul(C, C))
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		W[i] = new(big.Rat).Sub(mul(U[j], V[k]), mul(U[k], V[j]))
	}
	vector := func(a [3]*big.Rat, scale2 *big.Rat) (Vec3, error) {
		f, _ := scale2.Float64()
		s := math.Sqrt(f)
		if !finite(s) || s <= 0 {
			return Vec3{}, ErrUnrepresentable
		}
		var x [3]float64
		for i := 0; i < 3; i++ {
			f, _ = a[i].Float64()
			x[i] = f * s
			if !finite(x[i]) || (a[i].Sign() != 0 && x[i] == 0) {
				return Vec3{}, ErrUnrepresentable
			}
		}
		return Vec3{X: x[0], Y: x[1], Z: x[2]}, nil
	}
	A, e := vector(U, quo(M, D))
	if e != nil {
		return out, e
	}
	B, e := vector(V, quo(N, D))
	if e != nil {
		return out, e
	}
	normal, e := vector(W, new(big.Rat).Inv(D))
	if e != nil {
		return out, e
	}
	small, large := math.Min(ru, rv), math.Max(ru, rv)
	m := rat(small)
	m2 := mul(m, m)
	delta := new(big.Rat).Sub(mul(rat(large), rat(large)), m2)
	deltaFloat, _ := delta.Float64()
	if !finite(deltaFloat) || (delta.Sign() > 0 && deltaFloat <= 0) {
		return out, ErrUnrepresentable
	}
	_, invSin, e := rationalSqrtBounds(quo(mul(N, M), D), 128, budget)
	if e != nil {
		return out, e
	}
	first, second := new(big.Rat).Set(m), new(big.Rat).Set(m)
	if delta.Sign() > 0 {
		lower, _, e := rationalSqrtBounds(delta, 128, budget)
		if e != nil {
			return out, e
		}
		if lower.Sign() <= 0 {
			return out, ErrConvergence
		}
		first = quo(m2, mul(big.NewRat(2, 1), lower))
		second = add(quo(m2, lower), quo(mul(m2, m2), mul(big.NewRat(4, 1), mul(delta, lower))))
	}
	uu := add(mul(invSin, add(m, second)), m)
	for _, q := range []*big.Rat{N, M, C, D, delta, uu, first, second} {
		if e := budget.check(q); e != nil {
			return out, e
		}
	}
	for owner := 0; owner < 2; owner++ {
		varyingRadius := rv
		if owner == 1 {
			varyingRadius = ru
		}
		f := m
		if varyingRadius > small {
			f = first
		}
		uv := mul(invSin, f)
		if e := budget.check(uv); e != nil {
			return out, e
		}
		for j, side := range []int{-1, 1} {
			out[2*owner+j] = CylinderCommonWall{center, A, B, normal, ru, rv, small, deltaFloat, owner, side, SphereCylinderLensFanBounds{new(big.Rat).Set(uu), new(big.Rat).Set(uv), new(big.Rat)}}
		}
	}
	if e := ctx.Err(); e != nil {
		return [4]CylinderCommonWall{}, e
	}
	return out, nil
}
func (p CylinderCommonWall) Domain() ParameterRectangle {
	return ParameterRectangle{UMin: -math.Pi / 2, UMax: math.Pi / 2, VMin: -1, VMax: 1}
}

// Ownership returns source cylinder, side, and outward-normal reversal flag.
func (p CylinderCommonWall) Ownership() (owner, side int, reversed bool) {
	return p.owner, p.side, p.owner == 0 && p.side < 0 || p.owner == 1 && p.side > 0
}
func (p CylinderCommonWall) HessianBounds() (SphereCylinderLensFanBounds, error) {
	if p.bounds.UU == nil {
		return SphereCylinderLensFanBounds{}, ErrInvalid
	}
	return SphereCylinderLensFanBounds{new(big.Rat).Set(p.bounds.UU), new(big.Rat).Set(p.bounds.UV), new(big.Rat)}, nil
}
func (p CylinderCommonWall) jet(phi, t float64) (point, du, dv, duu, duv, dvv Vec3, err error) {
	if p.bounds.UU == nil || !finite(phi) || !finite(t) || phi < -math.Pi/2 || phi > math.Pi/2 || t < -1 || t > 1 {
		return point, du, dv, duu, duv, dvv, ErrDomain
	}
	sn, cs := math.Sincos(phi)
	if phi == math.Pi/2 {
		sn, cs = 1, 0
	} else if phi == -math.Pi/2 {
		sn, cs = -1, 0
	}
	width := func(r float64) (w, d, dd float64) {
		if r == p.small {
			return p.small * cs, -p.small * sn, -p.small * cs
		}
		m2 := p.small * p.small
		w = math.Sqrt(p.delta + m2*cs*cs)
		d = -m2 * sn * cs / w
		dd = -m2*(cs*cs-sn*sn)/w - d*d/w
		return
	}
	a, da, dda := width(p.rv)
	b, db, ddb := width(p.ru)
	ca, cb := t, float64(p.side)
	direction, w, d := p.A, a, da
	if p.owner == 1 {
		ca, cb = float64(p.side), t
		direction, w, d = p.B, b, db
	}
	point = p.center.Add(p.A.Scale(ca * a).Add(p.B.Scale(cb * b))).Add(p.W.Scale(p.small * sn))
	du = p.A.Scale(ca * da).Add(p.B.Scale(cb * db)).Add(p.W.Scale(p.small * cs))
	dv = direction.Scale(w)
	duu = p.A.Scale(ca * dda).Add(p.B.Scale(cb * ddb)).Add(p.W.Scale(-p.small * sn))
	duv = direction.Scale(d)
	for _, q := range []Vec3{point, du, dv, duu, duv} {
		if !q.IsFinite() {
			return point, du, dv, duu, duv, dvv, ErrConvergence
		}
	}
	return
}
func (p CylinderCommonWall) Eval(u, v float64) (Vec3, error) {
	x, _, _, _, _, _, e := p.jet(u, v)
	return x, e
}
func (p CylinderCommonWall) Partials(u, v float64) (Vec3, Vec3, error) {
	_, a, b, _, _, _, e := p.jet(u, v)
	return a, b, e
}
func (p CylinderCommonWall) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	_, _, _, a, b, c, e := p.jet(u, v)
	return a, b, c, e
}
