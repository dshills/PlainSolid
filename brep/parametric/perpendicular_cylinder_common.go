package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// PerpendicularCylinderCommonRadii constructs the native common of two
// centered perpendicular cylinders, with finite ends outside their common.
// RadiusX belongs to the X-axis cylinder; RadiusZ to the Z-axis cylinder.
// Unequal radii retain two disjoint closed seams between the supports.
func PerpendicularCylinderCommonRadii(ctx context.Context, center geom.Vec3, radiusX, radiusZ, xHalfLength, zHalfLength, tolerance float64) (SteinmetzCommon, error) {
	var empty SteinmetzCommon
	if ctx == nil || !center.IsFinite() || !finite(radiusX) || !finite(radiusZ) || !finite(xHalfLength) || !finite(zHalfLength) || !finite(tolerance) ||
		radiusX <= 0 || radiusZ <= 0 || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if radiusX == radiusZ {
		return PerpendicularCylinderCommon(ctx, center, radiusX, xHalfLength, zHalfLength, tolerance)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	small, large := min(radiusX, radiusZ), max(radiusX, radiusZ)
	if q(small).Cmp(pad) <= 0 || sub(q(xHalfLength), q(radiusZ)).Cmp(pad) <= 0 ||
		sub(q(zHalfLength), q(radiusX)).Cmp(pad) <= 0 || sub(q(large), q(small)).Cmp(pad) <= 0 {
		return empty, geom.ErrConvergence
	}
	var faces []Face
	var out SteinmetzCommon
	for k := range 2 {
		for _, sign := range []int{1, -1} {
			swap := radiusX < radiusZ
			f := Face{Surface: unequalCylinderCommonChart{center, small, large, k, sign, swap}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1},
				Reversed: (k == 0 && sign < 0 || k == 1 && sign > 0) != swap}
			if (k == 0) == swap {
				out.XAxisFaces = append(out.XAxisFaces, len(faces))
			} else {
				out.ZAxisFaces = append(out.ZAxisFaces, len(faces))
			}
			faces = append(faces, f)
		}
	}
	var e error
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

// When swap is false, k=0 is the Z support and k=1 is the X support.
// Swapping X and Z reverses the chart orientation.
type unequalCylinderCommonChart struct {
	center       geom.Vec3
	small, large float64
	k, sign      int
	swap         bool
}

func (s unequalCylinderCommonChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s unequalCylinderCommonChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s unequalCylinderCommonChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < -1 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if math.Abs(u) == math.Pi/2 {
		cs = 0
	}
	y, dy := s.small*sn, s.small*cs
	x, dx := dy, -y
	z := math.Sqrt((1-y/s.large)*(1+y/s.large)) * s.large
	if !finite(z) || z <= 0 {
		return p, du, dv, geom.ErrConvergence
	}
	dz := -(y / z) * dy
	k := float64(s.sign)
	if s.k == 0 {
		p, du, dv = geom.Vec3{X: k * x, Y: y, Z: v * z}, geom.Vec3{X: k * dx, Y: dy, Z: v * dz}, geom.Vec3{Z: z}
	} else {
		p, du, dv = geom.Vec3{X: v * x, Y: y, Z: k * z}, geom.Vec3{X: v * dx, Y: dy, Z: k * dz}, geom.Vec3{X: x}
	}
	if s.swap {
		p.X, p.Z = p.Z, p.X
		du.X, du.Z = du.Z, du.X
		dv.X, dv.Z = dv.Z, dv.X
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

func (s unequalCylinderCommonChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	// z >= large-small is a conservative exact lower bound. Applying
	// z'=-yy'/z and its derivative bounds the only noncircular term.
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	m := new(big.Rat).Sub(q(s.large), q(s.small))
	m2 := new(big.Rat).Mul(m, m)
	x2 := new(big.Rat).Mul(q(s.small), q(s.small))
	z1 := new(big.Rat).Quo(x2, m)
	z2 := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), z1), new(big.Rat).Quo(new(big.Rat).Mul(x2, x2), new(big.Rat).Mul(m2, m)))
	return upAdd(upMul(2, s.small), upperBoreRat(z2)), max(s.small, upperBoreRat(z1)), 0, nil
}
