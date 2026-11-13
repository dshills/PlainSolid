package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SkewOrthogonalOverlapCommon constructs the native common of skew X/Z
// cylinders with strictly overlapping but nonnested Y projections. Each end
// of the shared Y range collapses a different cylinder radial coordinate.
// The original axial end planes must lie outside the entire common.
func SkewOrthogonalOverlapCommon(ctx context.Context, center geom.Vec3, radiusX, radiusZ, yOffset, xHalfLength, zHalfLength, tolerance float64) (SteinmetzCommon, error) {
	var empty SteinmetzCommon
	if ctx == nil || !center.IsFinite() || !finite(radiusX) || !finite(radiusZ) || !finite(yOffset) ||
		!finite(xHalfLength) || !finite(zHalfLength) || !finite(tolerance) || radiusX <= 0 || radiusZ <= 0 ||
		xHalfLength <= 0 || zHalfLength <= 0 || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	d := q(math.Abs(yOffset))
	rx, rz := q(radiusX), q(radiusZ)
	if sub(q(xHalfLength), rz).Cmp(pad) <= 0 || sub(q(zHalfLength), rx).Cmp(pad) <= 0 ||
		sub(d, new(big.Rat).Abs(sub(rx, rz))).Cmp(pad) <= 0 || sub(add(rx, rz), d).Cmp(pad) <= 0 {
		return empty, geom.ErrConvergence
	}
	// Reflect Y in the chart for a negative displacement. The positive
	// case has lower endpoint d−rZ and upper endpoint rX.
	length, e := geom.RepresentedExactScalar(sub(add(rx, rz), d))
	if e != nil {
		return empty, e
	}
	s := skewOverlapCylinderChart{center: center, radiusX: radiusX, radiusZ: radiusZ, offset: math.Abs(yOffset), length: length, reflected: yOffset < 0}
	var faces []Face
	var out SteinmetzCommon
	for k := range 2 {
		s.k = k
		for _, sign := range []int{1, -1} {
			s.sign = sign
			if k == 0 {
				out.ZAxisFaces = append(out.ZAxisFaces, len(faces))
			} else {
				out.XAxisFaces = append(out.XAxisFaces, len(faces))
			}
			faces = append(faces, Face{Surface: s, U: Interval{0, math.Pi / 2}, V: Interval{-1, 1},
				Reversed: (k == 0 && sign < 0 || k == 1 && sign > 0) != s.reflected})
		}
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

type skewOverlapCylinderChart struct {
	center                           geom.Vec3
	radiusX, radiusZ, offset, length float64
	k, sign                          int
	reflected                        bool
}

func (s skewOverlapCylinderChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s skewOverlapCylinderChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s skewOverlapCylinderChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < 0 || u > math.Pi/2 || v < -1 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if u == math.Pi/2 {
		sn, cs = 1, 0
	}
	L := s.length
	// y−(d−rZ)=L sin²(u), rX−y=L cos²(u).
	// Factoring each radicand removes both endpoint square roots.
	sx, sz := math.Sqrt(L)*math.Sqrt(2*s.radiusZ-L*sn*sn), math.Sqrt(L)*math.Sqrt(2*s.radiusX-L*cs*cs)
	if !finite(sx) || !finite(sz) || sx <= 0 || sz <= 0 {
		return p, du, dv, geom.ErrConvergence
	}
	y := s.radiusX - L*cs*cs
	dy := 2 * L * sn * cs
	x, z := sn*sx, cs*sz
	dx, dz := cs*sx-L*L*sn*sn*cs/sx, -sn*sz+L*L*cs*cs*sn/sz
	k := float64(s.sign)
	if s.k == 0 {
		p, du, dv = geom.Vec3{X: k * x, Y: y, Z: v * z}, geom.Vec3{X: k * dx, Y: dy, Z: v * dz}, geom.Vec3{Z: z}
	} else {
		p, du, dv = geom.Vec3{X: v * x, Y: y, Z: k * z}, geom.Vec3{X: v * dx, Y: dy, Z: k * dz}, geom.Vec3{X: x}
	}
	if s.reflected {
		p.Y, du.Y, dv.Y = -p.Y, -du.Y, -dv.Y
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrConvergence
	}
	return p, du, dv, nil
}

func (s skewOverlapCylinderChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	if u.Min < 0 || u.Max > math.Pi/2 || v.Min < -1 || v.Max > 1 {
		return 0, 0, 0, geom.ErrDomain
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	L := q(s.length)
	bound := func(r float64) (float64, float64) {
		// p=sqrt(L*(2r−L sin²u)); p≥L*min(1,(2r−L)/L).
		m := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), q(r)), L)
		if m.Cmp(L) > 0 {
			m.Set(L)
		}
		mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
		L2 := mul(L, L)
		p1 := new(big.Rat).Quo(L2, mul(big.NewRat(2, 1), m))
		p2 := new(big.Rat).Add(new(big.Rat).Quo(L2, m), new(big.Rat).Quo(mul(L2, L2), mul(big.NewRat(4, 1), mul(m, mul(m, m)))))
		// The other factor is a single sine or cosine.
		return upAdd(upMul(2, r), upperBoreRat(p1)), upAdd(upAdd(upMul(2, r), upMul(2, upperBoreRat(p1))), upperBoreRat(p2))
	}
	x1, x2 := bound(s.radiusZ)
	z1, z2 := bound(s.radiusX)
	return upAdd(upAdd(x2, z2), upMul(2, s.length)), max(x1, z1), 0, nil
}
