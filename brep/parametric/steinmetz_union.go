package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// PerpendicularCylinderUnionResult retains both cylindrical walls and the
// four planar caps. The ideal volume is π*VolumePiCoefficient−OverlapVolume.
type PerpendicularCylinderUnionResult struct {
	Solid                                   Solid
	WallFaces, LowerCapFaces, UpperCapFaces [2][]int
	VolumePiCoefficient, OverlapVolume      *big.Rat
}

// PerpendicularCylinderUnion fuses centered equal-radius finite cylinders
// along the X and Z axes, with each cap beyond the other cylinder's radius.
func PerpendicularCylinderUnion(ctx context.Context, center geom.Vec3, radius, xHalfLength, zHalfLength, tolerance float64) (PerpendicularCylinderUnionResult, error) {
	var empty PerpendicularCylinderUnionResult
	if ctx == nil || !center.IsFinite() || !finite(radius) || !finite(xHalfLength) || !finite(zHalfLength) || !finite(tolerance) ||
		radius <= 0 || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	limit := new(big.Rat).Add(q(radius), pad)
	if q(radius).Cmp(pad) <= 0 || q(xHalfLength).Cmp(limit) <= 0 || q(zHalfLength).Cmp(limit) <= 0 {
		return empty, geom.ErrDomain
	}
	var out PerpendicularCylinderUnionResult
	var faces []Face
	for axis, L := range [2]float64{xHalfLength, zHalfLength} {
		for _, side := range []int{-1, 1} {
			for _, half := range []int{-1, 1} {
				for cap := range 2 {
					s := steinmetzUnionChart{center, radius, L, axis, side, half, cap == 1}
					id := len(faces)
					if cap == 0 {
						out.WallFaces[axis] = append(out.WallFaces[axis], id)
					} else if side < 0 {
						out.LowerCapFaces[axis] = append(out.LowerCapFaces[axis], id)
					} else {
						out.UpperCapFaces[axis] = append(out.UpperCapFaces[axis], id)
					}
					reverse := axis == 0 && side*half > 0 || axis == 1 && side*half < 0
					if s.cap {
						reverse = !reverse
					}
					faces = append(faces, Face{Surface: s, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{0, 1}, Reversed: reverse})
				}
			}
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
	r2 := new(big.Rat).Mul(q(radius), q(radius))
	out.VolumePiCoefficient = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(r2, new(big.Rat).Add(q(xHalfLength), q(zHalfLength))))
	out.OverlapVolume = new(big.Rat).Mul(big.NewRat(16, 3), new(big.Rat).Mul(r2, q(radius)))
	return out, nil
}

// axis0 is the X cylinder; axis1 swaps X and Z. A cap is swept by horizontal
// chords so its curved side has exactly the same parameter as the wall rim.
type steinmetzUnionChart struct {
	center             geom.Vec3
	radius, halfLength float64
	axis, side, half   int
	cap                bool
}

func (s steinmetzUnionChart) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s steinmetzUnionChart) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s steinmetzUnionChart) evaluate(u, v float64) (p, du, dv geom.Vec3, err error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < 0 || v > 1 {
		return p, du, dv, geom.ErrDomain
	}
	sn, cs := math.Sincos(u)
	if math.Abs(u) == math.Pi/2 {
		cs = 0
	}
	y, w := s.radius*sn, s.radius*cs
	dy, dw := w, -y
	side, half := float64(s.side), float64(s.half)
	if s.cap {
		p = geom.Vec3{X: side * s.halfLength, Y: y, Z: half * w * v}
		du = geom.Vec3{Y: dy, Z: half * dw * v}
		dv = geom.Vec3{Z: half * w}
	} else {
		p = geom.Vec3{X: side * ((1-v)*w + v*s.halfLength), Y: y, Z: half * w}
		du = geom.Vec3{X: side * (1 - v) * dw, Y: dy, Z: half * dw}
		dv = geom.Vec3{X: side * (s.halfLength - w)}
	}
	if s.axis == 1 {
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
