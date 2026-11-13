package geom

import (
	"context"
	"math"
	"math/big"
)

// EqualCylinderCommonWall maps one wall of the infinite-support common solid.
// U=phi∈[-pi/2,pi/2] is common-perpendicular latitude; V∈[-1,1] spans the
// other cylinder's section coordinate. Both U ends deliberately collapse to
// shared pole nodes. All edge formulas use a common ordered basis for sewing.
type EqualCylinderCommonWall struct {
	center, A, B, W Vec3
	radius          float64
	owner, side     int
	bounds          SphereCylinderLensFanBounds
}

// NewEqualCylinderCommonWalls returns four walls: owner0 sides−1,+1 then
// owner1 sides−1,+1. Axes intersect at center, with equal exact squared norm
// and nonzero cross product. The chart derives from exact section constraints
// |a|,|b|<=r*cos(phi); support identity is not inferred from sampled points.
// Numerical basis/evaluation rounding is independent of ideal Hessian bounds.
func NewEqualCylinderCommonWalls(ctx context.Context, center, u, v Vec3, radius float64, o SurfacePatchEvaluationOptions) (out [4]EqualCylinderCommonWall, err error) {
	if !center.IsFinite() {
		return out, ErrInvalid
	}
	proof, e := MeasureEqualCylinderCommon(ctx, u, v, radius, nil, nil, o)
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
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: proof.ExactOperations}
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	U, V, W := [3]*big.Rat{}, [3]*big.Rat{}, [3]*big.Rat{}
	N, C := new(big.Rat), new(big.Rat)
	for i := 0; i < 3; i++ {
		U[i] = r(coordinate(u, i))
		V[i] = r(coordinate(v, i))
		N.Add(N, new(big.Rat).Mul(U[i], U[i]))
		C.Add(C, new(big.Rat).Mul(U[i], V[i]))
	}
	D := new(big.Rat).Sub(new(big.Rat).Mul(N, N), new(big.Rat).Mul(C, C))
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		W[i] = new(big.Rat).Sub(new(big.Rat).Mul(U[j], V[k]), new(big.Rat).Mul(U[k], V[j]))
	}
	// |Q(t)|² <= 2N/(N-|C|), and |A|²=|B|²=N²/D.
	q2 := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(2, 1), N), new(big.Rat).Sub(N, new(big.Rat).Abs(C)))
	R2 := new(big.Rat).Mul(r(radius), r(radius))
	_, UU, e := rationalSqrtBounds(new(big.Rat).Mul(R2, q2), 128, b)
	if e != nil {
		return out, e
	}
	_, UV, e := rationalSqrtBounds(new(big.Rat).Quo(R2, proof.SinSquared), 128, b)
	if e != nil {
		return out, e
	}
	scale2 := new(big.Rat).Quo(N, D)
	scaleFloat, _ := scale2.Float64()
	wscale2 := new(big.Rat).Inv(D)
	wscaleFloat, _ := wscale2.Float64()
	s, ws := math.Sqrt(scaleFloat), math.Sqrt(wscaleFloat)
	if !finite(s) || !finite(ws) || s <= 0 || ws <= 0 {
		return out, ErrUnrepresentable
	}
	vector := func(a [3]*big.Rat, scale float64) (Vec3, error) {
		var c [3]float64
		for i := 0; i < 3; i++ {
			x, _ := a[i].Float64()
			c[i] = x * scale
			if !finite(c[i]) {
				return Vec3{}, ErrUnrepresentable
			}
		}
		return Vec3{X: c[0], Y: c[1], Z: c[2]}, nil
	}
	A, e := vector(U, s)
	if e != nil {
		return out, e
	}
	B, e := vector(V, s)
	if e != nil {
		return out, e
	}
	normal, e := vector(W, ws)
	if e != nil {
		return out, e
	}
	for owner := 0; owner < 2; owner++ {
		for j, side := range []int{-1, 1} {
			out[owner*2+j] = EqualCylinderCommonWall{center, A, B, normal, radius, owner, side, SphereCylinderLensFanBounds{new(big.Rat).Set(UU), new(big.Rat).Set(UV), new(big.Rat)}}
		}
	}
	return out, nil
}
func (p EqualCylinderCommonWall) Domain() ParameterRectangle {
	return ParameterRectangle{UMin: -math.Pi / 2, UMax: math.Pi / 2, VMin: -1, VMax: 1}
}

// Ownership returns cylinder owner0/1, signed perpendicular side, and whether
// reversing the parameter normal gives the outward material orientation.
func (p EqualCylinderCommonWall) Ownership() (owner, side int, reversed bool) {
	return p.owner, p.side, p.owner == 0 && p.side < 0 || p.owner == 1 && p.side > 0
}
func (p EqualCylinderCommonWall) HessianBounds() (SphereCylinderLensFanBounds, error) {
	if p.bounds.UU == nil {
		return SphereCylinderLensFanBounds{}, ErrInvalid
	}
	return SphereCylinderLensFanBounds{new(big.Rat).Set(p.bounds.UU), new(big.Rat).Set(p.bounds.UV), new(big.Rat)}, nil
}
func (p EqualCylinderCommonWall) jet(phi, t float64) (point, du, dv, duu, duv, dvv Vec3, err error) {
	if p.bounds.UU == nil || !finite(phi) || !finite(t) || phi < -math.Pi/2 || phi > math.Pi/2 || t < -1 || t > 1 {
		return point, du, dv, duu, duv, dvv, ErrDomain
	}
	sn, cs := math.Sincos(phi)
	if phi == math.Pi/2 {
		sn, cs = 1, 0
	} else if phi == -math.Pi/2 {
		sn, cs = -1, 0
	}
	cu, cv := t, float64(p.side)
	direction := p.A
	if p.owner == 1 {
		cu, cv = float64(p.side), t
		direction = p.B
	}
	Q := p.A.Scale(cu).Add(p.B.Scale(cv))
	point = p.center.Add(Q.Scale(p.radius * cs)).Add(p.W.Scale(p.radius * sn))
	du = Q.Scale(-p.radius * sn).Add(p.W.Scale(p.radius * cs))
	dv = direction.Scale(p.radius * cs)
	duu = Q.Scale(-p.radius * cs).Add(p.W.Scale(-p.radius * sn))
	duv = direction.Scale(-p.radius * sn)
	for _, q := range []Vec3{point, du, dv, duu, duv} {
		if !q.IsFinite() {
			return point, du, dv, duu, duv, dvv, ErrConvergence
		}
	}
	return
}
func (p EqualCylinderCommonWall) Eval(u, v float64) (Vec3, error) {
	x, _, _, _, _, _, e := p.jet(u, v)
	return x, e
}
func (p EqualCylinderCommonWall) Partials(u, v float64) (Vec3, Vec3, error) {
	_, a, b, _, _, _, e := p.jet(u, v)
	return a, b, e
}
func (p EqualCylinderCommonWall) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	_, _, _, a, b, c, e := p.jet(u, v)
	return a, b, c, e
}
