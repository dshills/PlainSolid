package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SteinmetzCommon is the intersection of equal-radius cylinders along global
// X and Z, with both finite axial ranges strictly beyond [-radius,radius].
// The four retained support faces are genuine cylindrical surfaces; their
// shared seams are elliptical arcs, and two vertices are collapsed poles.
type SteinmetzCommon struct {
	Solid                  Solid
	XAxisFaces, ZAxisFaces []int
	IdealVolume            *big.Rat
}

// PerpendicularCylinderCommon constructs the translated equal-radius
// Steinmetz solid. Half-lengths only certify that neither end cap clips it.
// Native sewing/evaluation is floating analytic; IdealVolume is exact 16r³/3.
func PerpendicularCylinderCommon(ctx context.Context, center geom.Vec3, radius, xHalfLength, zHalfLength, tolerance float64) (SteinmetzCommon, error) {
	fail := func(e error) (SteinmetzCommon, error) { return SteinmetzCommon{}, e }
	if ctx == nil || !center.IsFinite() || !finite(radius) || !finite(xHalfLength) || !finite(zHalfLength) || !finite(tolerance) || radius <= 0 || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	limit := new(big.Rat).Add(q(radius), pad)
	if q(radius).Cmp(pad) <= 0 || q(xHalfLength).Cmp(limit) <= 0 || q(zHalfLength).Cmp(limit) <= 0 {
		return fail(geom.ErrDomain)
	}
	var faces []Face
	for axis := 0; axis < 2; axis++ {
		for _, sign := range []int{1, -1} {
			faces = append(faces, Face{Surface: steinmetzChart{center, radius, axis, sign}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1}, Reversed: axis == 0 && sign < 0 || axis == 1 && sign > 0})
		}
	}
	solid, e := joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	volume := new(big.Rat).Mul(big.NewRat(16, 3), new(big.Rat).Mul(new(big.Rat).Mul(q(radius), q(radius)), q(radius)))
	return SteinmetzCommon{Solid: solid, ZAxisFaces: []int{0, 1}, XAxisFaces: []int{2, 3}, IdealVolume: volume}, nil
}

// axis0 retains x²+y²=r²; axis1 retains y²+z²=r².
type steinmetzChart struct {
	center     geom.Vec3
	radius     float64
	axis, sign int
}

func (s steinmetzChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s steinmetzChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s steinmetzChart) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < -1 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	sin, cos := math.Sincos(u)
	if u == math.Pi/2 || u == -math.Pi/2 {
		cos = 0
	}
	r, k := s.radius, float64(s.sign)
	p := geom.Vec3{X: k * r * cos, Y: r * sin, Z: v * r * cos}
	du := geom.Vec3{X: -k * r * sin, Y: r * cos, Z: -v * r * sin}
	dv := geom.Vec3{Z: r * cos}
	if s.axis == 1 {
		p.X, p.Z = p.Z, p.X
		du.X, du.Z = du.Z, du.X
		dv.X, dv.Z = dv.Z, dv.X
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return p, du, dv, nil
}

// Conservative Euclidean second derivative norms on the complete rectangle.
func (s steinmetzChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	b := math.Nextafter(2*s.radius, math.Inf(1))
	if !finite(b) {
		return 0, 0, 0, geom.ErrInvalid
	}
	return b, b, 0, nil
}
