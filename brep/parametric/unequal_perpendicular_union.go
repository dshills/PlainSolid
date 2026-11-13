package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// UnequalPerpendicularUnion retains the original X/Z cylinder walls and four
// planar caps. VolumePiCoefficient is the exact sum-of-cylinder-volume factor;
// subtract the unequal-cylinder intersection volume to obtain union volume.
type UnequalPerpendicularUnion struct {
	Solid                                   Solid
	WallFaces, LowerCapFaces, UpperCapFaces [2][]int
	VolumePiCoefficient                     *big.Rat
}

// UnequalPerpendicularCylinderUnion constructs the centered union of a wider
// X cylinder and narrower Z cylinder. Caps must strictly clear the opposite
// radius by 8*tolerance. The shoulder sqrt(xRadius²−zRadius²) must be exactly
// representable; other radii fail explicitly. No generic Boolean is implied.
func UnequalPerpendicularCylinderUnion(ctx context.Context, center geom.Vec3, xRadius, zRadius, xHalf, zHalf, tolerance float64) (UnequalPerpendicularUnion, error) {
	empty := UnequalPerpendicularUnion{}
	if ctx == nil || !center.IsFinite() || !finite(xRadius) || !finite(zRadius) || !finite(xHalf) || !finite(zHalf) || !finite(tolerance) || tolerance <= 0 || zRadius <= 0 || xRadius <= zRadius {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if q(zRadius).Cmp(pad) <= 0 || q(xHalf).Cmp(new(big.Rat).Add(q(zRadius), pad)) <= 0 || q(zHalf).Cmp(new(big.Rat).Add(q(xRadius), pad)) <= 0 {
		return empty, geom.ErrDomain
	}
	squared := new(big.Rat).Sub(new(big.Rat).Mul(q(xRadius), q(xRadius)), new(big.Rat).Mul(q(zRadius), q(zRadius)))
	nr, dr := new(big.Int).Sqrt(squared.Num()), new(big.Int).Sqrt(squared.Denom())
	shoulder := new(big.Rat).SetFrac(nr, dr)
	if new(big.Rat).Mul(shoulder, shoulder).Cmp(squared) != 0 {
		return empty, geom.ErrUnrepresentable
	}
	width, e := geom.RepresentedExactScalar(shoulder)
	if e != nil {
		return empty, e
	}
	var out UnequalPerpendicularUnion
	var faces []Face
	for kind := 0; kind < 6; kind++ {
		for _, side := range []int{-1, 1} {
			for _, half := range []int{-1, 1} {
				ys := []int{1}
				if kind == 1 || kind == 3 {
					ys = []int{-1, 1}
				}
				for _, ySign := range ys {
					chart := unequalUnionChart{center, xRadius, zRadius, width, xHalf, zHalf, kind, side, half, ySign}
					domain := Interval{-math.Pi / 2, math.Pi / 2}
					if kind == 1 || kind == 3 {
						domain = Interval{0, math.Pi / 2}
					}
					p, du, dv, e := chart.evaluate(domain.at(.5), .5)
					if e != nil {
						return empty, e
					}
					var normal geom.Vec3
					switch kind {
					case 0, 1:
						normal = geom.Vec3{Y: p.Y - center.Y, Z: p.Z - center.Z}
					case 2, 3:
						normal = geom.Vec3{X: float64(side)}
					case 4:
						normal = geom.Vec3{X: p.X - center.X, Y: p.Y - center.Y}
					case 5:
						normal = geom.Vec3{Z: float64(side)}
					}
					sign := du.Cross(dv).Dot(normal)
					if !finite(sign) || sign == 0 {
						return empty, geom.ErrDomain
					}
					id := len(faces)
					faces = append(faces, Face{Surface: chart, U: domain, V: Interval{0, 1}, Reversed: sign < 0})
					owner := 0
					if kind >= 4 {
						owner = 1
					}
					if kind == 2 || kind == 3 || kind == 5 {
						if side < 0 {
							out.LowerCapFaces[owner] = append(out.LowerCapFaces[owner], id)
						} else {
							out.UpperCapFaces[owner] = append(out.UpperCapFaces[owner], id)
						}
					} else {
						out.WallFaces[owner] = append(out.WallFaces[owner], id)
					}
				}
			}
		}
	}
	out.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return empty, e
	}
	out.VolumePiCoefficient = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(q(xRadius), q(xRadius)), q(xHalf)), new(big.Rat).Mul(new(big.Rat).Mul(q(zRadius), q(zRadius)), q(zHalf))))
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

// Every chart is a subdomain of an original cylinder or cap. Central and outer
// bands meet at y=±r, with their common boundary linear in the same v parameter.
type unequalUnionChart struct {
	center                               geom.Vec3
	wide, narrow, shoulder, xHalf, zHalf float64
	kind, side, half, ySign              int
}

func (s unequalUnionChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s unequalUnionChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s unequalUnionChart) evaluate(u, v float64) (p, du, dv geom.Vec3, e error) {
	outer := s.kind == 1 || s.kind == 3
	if !finite(u) || !finite(v) || v < 0 || v > 1 || u > math.Pi/2 || u < -math.Pi/2 || (outer && u < 0) {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if u == math.Pi/2 || u == -math.Pi/2 {
		cs = 0
	}
	side, half := float64(s.side), float64(s.half)
	y, dy := s.narrow*sn, s.narrow*cs
	root := math.Sqrt(s.shoulder*s.shoulder + s.narrow*s.narrow*cs*cs)
	dr := -y * dy / root
	if cs == 0 {
		root = s.shoulder
		dr = 0
	}
	x, dx := s.narrow*cs, -s.narrow*sn
	if outer {
		y = math.Sqrt(s.narrow*s.narrow + s.shoulder*s.shoulder*sn*sn)
		dy = s.shoulder * s.shoulder * sn * cs / y
		if u == math.Pi/2 {
			y = s.wide
			dy = 0
		}
		y *= float64(s.ySign)
		dy *= float64(s.ySign)
		root, dr = s.shoulder*cs, -s.shoulder*sn
	}
	switch s.kind {
	case 0:
		p = geom.Vec3{X: side * (x + (s.xHalf-x)*v), Y: y, Z: half * root}
		du = geom.Vec3{X: side * dx * (1 - v), Y: dy, Z: half * dr}
		dv = geom.Vec3{X: side * (s.xHalf - x)}
	case 1:
		p = geom.Vec3{X: side * s.xHalf * v, Y: y, Z: half * root}
		du = geom.Vec3{Y: dy, Z: half * dr}
		dv = geom.Vec3{X: side * s.xHalf}
	case 2, 3:
		p = geom.Vec3{X: side * s.xHalf, Y: y, Z: half * root * v}
		du = geom.Vec3{Y: dy, Z: half * dr * v}
		dv = geom.Vec3{Z: half * root}
	case 4:
		p = geom.Vec3{X: half * x, Y: y, Z: side * (root + (s.zHalf-root)*v)}
		du = geom.Vec3{X: half * dx, Y: dy, Z: side * dr * (1 - v)}
		dv = geom.Vec3{Z: side * (s.zHalf - root)}
	case 5:
		p = geom.Vec3{X: half * x * v, Y: y, Z: side * s.zHalf}
		du = geom.Vec3{X: half * dx * v, Y: dy}
		dv = geom.Vec3{X: half * x}
	default:
		return p, du, dv, geom.ErrInvalid
	}
	p = p.Add(s.center)
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return p, du, dv, geom.ErrDomain
	}
	return p, du, dv, nil
}

func (s unequalUnionChart) valid() bool {
	if !s.center.IsFinite() || !finite(s.wide) || !finite(s.narrow) || !finite(s.shoulder) || !finite(s.xHalf) || !finite(s.zHalf) || s.narrow <= 0 || s.wide <= s.narrow || s.shoulder <= 0 || s.xHalf <= s.narrow || s.zHalf <= s.wide || s.kind < 0 || s.kind > 5 || (s.side != 1 && s.side != -1) || (s.half != 1 && s.half != -1) || (s.ySign != 1 && s.ySign != -1) {
		return false
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	return new(big.Rat).Sub(new(big.Rat).Mul(q(s.wide), q(s.wide)), new(big.Rat).Mul(q(s.narrow), q(s.narrow))).Cmp(new(big.Rat).Mul(q(s.shoulder), q(s.shoulder))) == 0
}
func (s unequalUnionChart) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if ctx == nil || !s.valid() || u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 || ((s.kind == 1 || s.kind == 3) && u.Min < 0) {
		return 0, 0, 0, geom.ErrDomain
	}
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	// For sqrt(q²+r² cos²u), |f'|<=r²/q and
	// |f''|<=r²/q+r⁴/q³. The outer-band analogue swaps r,q.
	// Sum component absolute bounds dominates the Euclidean vector norm.
	r, q := new(big.Rat).SetFloat64(s.narrow), new(big.Rat).SetFloat64(s.shoulder)
	term := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Mul(a, a), b) }
	rp, qp := term(r, q), term(q, r)
	mixed := new(big.Rat).Add(new(big.Rat).Add(r, q), new(big.Rat).Add(rp, qp))
	second := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(r, q)), new(big.Rat).Add(rp, qp))
	second.Add(second, new(big.Rat).Quo(new(big.Rat).Mul(rp, rp), q))
	second.Add(second, new(big.Rat).Quo(new(big.Rat).Mul(qp, qp), r))
	upper := func(x *big.Rat) float64 {
		f, _ := x.Float64()
		if finite(f) && new(big.Rat).SetFloat64(f).Cmp(x) < 0 {
			f = math.Nextafter(f, math.Inf(1))
		}
		return f
	}
	a, b := upper(second), upper(mixed)
	if !finite(a) || !finite(b) {
		return 0, 0, 0, geom.ErrDomain
	}
	return a, b, 0, nil
}
