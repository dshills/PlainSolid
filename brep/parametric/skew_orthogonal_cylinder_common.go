package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SkewOrthogonalCylinderCommon retains both cylinder walls when the X and Z
// cylinder axes have different local Y coordinates. The end planes must lie
// beyond the entire intersection. This native chart currently requires the
// Y projection of one cylinder to be strictly contained in the other.
func SkewOrthogonalCylinderCommon(ctx context.Context, center geom.Vec3, radiusX, radiusZ, yOffset, xHalfLength, zHalfLength, tolerance float64) (SteinmetzCommon, error) {
	var empty SteinmetzCommon
	if ctx == nil || !center.IsFinite() || !finite(radiusX) || !finite(radiusZ) || !finite(yOffset) || !finite(xHalfLength) || !finite(zHalfLength) ||
		!finite(tolerance) || radiusX <= 0 || radiusZ <= 0 || xHalfLength <= 0 || zHalfLength <= 0 || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if yOffset == 0 {
		return PerpendicularCylinderCommonRadii(ctx, center, radiusX, radiusZ, xHalfLength, zHalfLength, tolerance)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	d := q(math.Abs(yOffset))
	sum := add(q(radiusX), q(radiusZ))
	if d.Cmp(sum) > 0 {
		return empty, nil
	}
	if d.Cmp(sum) == 0 {
		return empty, geom.ErrConvergence
	}
	small, large := min(radiusX, radiusZ), max(radiusX, radiusZ)
	if q(small).Cmp(pad) <= 0 || sub(sub(q(large), q(small)), d).Cmp(pad) <= 0 ||
		sub(q(xHalfLength), q(radiusZ)).Cmp(pad) <= 0 || sub(q(zHalfLength), q(radiusX)).Cmp(pad) <= 0 {
		return empty, geom.ErrConvergence
	}
	swap := radiusX < radiusZ
	offset := yOffset
	if !swap {
		offset = -yOffset
	}
	s := skewCylinderChart{center: center, small: small, large: large, shift: yOffset, largeOffset: offset, swap: swap}
	var out SteinmetzCommon
	var faces []Face
	for k := range 2 {
		s.k = k
		for _, sign := range []int{1, -1} {
			s.sign = sign
			if (k == 0) == swap {
				out.XAxisFaces = append(out.XAxisFaces, len(faces))
			} else {
				out.ZAxisFaces = append(out.ZAxisFaces, len(faces))
			}
			faces = append(faces, Face{Surface: s, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1},
				Reversed: (k == 0 && sign < 0 || k == 1 && sign > 0) != swap})
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

type skewCylinderChart struct {
	center                           geom.Vec3
	small, large, shift, largeOffset float64
	k, sign                          int
	swap                             bool
}

func (s skewCylinderChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s skewCylinderChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s skewCylinderChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < -1 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if math.Abs(u) == math.Pi/2 {
		cs = 0
	}
	y, dy := s.small*sn, s.small*cs
	x, dx := dy, -y
	w := y - s.largeOffset
	z := math.Sqrt((1-w/s.large)*(1+w/s.large)) * s.large
	if !finite(z) || z <= 0 {
		return p, du, dv, geom.ErrConvergence
	}
	dz := -(w / z) * dy
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
	} else {
		p.Y += s.shift
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

func (s skewCylinderChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	w := new(big.Rat).Add(q(s.small), q(math.Abs(s.largeOffset)))
	m := new(big.Rat).Sub(q(s.large), w)
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	// sqrt(L²−w²) >= (L²−w²)/L, with no rounded square root.
	m.Quo(mul(m, new(big.Rat).Add(q(s.large), w)), q(s.large))
	r := q(s.small)
	wd := new(big.Rat).Add(mul(mul(r, r), big.NewRat(1, 2)), mul(q(math.Abs(s.largeOffset)), r))
	first := new(big.Rat).Quo(wd, m)
	second := new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Add(mul(r, r), mul(w, r)), m),
		new(big.Rat).Quo(mul(wd, wd), mul(m, mul(m, m))))
	return upAdd(upMul(2, s.small), upperBoreRat(second)), max(s.small, upperBoreRat(first)), 0, nil
}
