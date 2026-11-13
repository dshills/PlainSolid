package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// PerpendicularCylinderDifferenceResult contains negative-X then positive-X
// regularized material components. Their interiors are disjoint but their
// closures TOUCH at the two reported pole points; this is not a disjoint-shell
// assembly certificate. Each component has its own closed manifold topology.
// Ideal volumes equal π*VolumePiCoefficients[i]+VolumeOffsets[i].
type PerpendicularCylinderDifferenceResult struct {
	Solids                              [2]Solid
	XAxisFaces, ToolFaces, EndCapFaces  [2][]int
	ContactPoints                       [2]geom.Vec3
	VolumePiCoefficients, VolumeOffsets [2]*big.Rat
}

// PerpendicularCylinderDifference subtracts the equal-radius Z-cylinder from
// the X-cylinder with finite ends lower/upper. Both X ends and the Z tool ends
// must extend beyond the radial extremum. Actual cylinder supports and inward
// elliptical cut seams are preserved. The two isolated closure contacts are
// intrinsic to the equal-radius configuration and are explicitly retained.
func PerpendicularCylinderDifference(ctx context.Context, center geom.Vec3, radius, lower, upper, zToolHalfLength, tolerance float64) (PerpendicularCylinderDifferenceResult, error) {
	fail := func(e error) (PerpendicularCylinderDifferenceResult, error) {
		return PerpendicularCylinderDifferenceResult{}, e
	}
	if !finite(lower) || !finite(upper) || lower >= 0 || upper <= 0 {
		return fail(geom.ErrInvalid)
	}
	if _, e := PerpendicularCylinderCommon(ctx, center, radius, math.Min(-lower, upper), zToolHalfLength, tolerance); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	result := PerpendicularCylinderDifferenceResult{}
	for i, sign := range []float64{-1, 1} {
		y, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(center.Y), new(big.Rat).Mul(q(sign), q(radius))))
		if e != nil {
			return fail(e)
		}
		result.ContactPoints[i] = geom.Vec3{X: center.X, Y: y, Z: center.Z}
	}
	u := Interval{-math.Pi / 2, math.Pi / 2}
	for i, end := range []float64{lower, upper} {
		side := -1
		if i == 1 {
			side = 1
		}
		capX, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(center.X), q(end)))
		if e != nil {
			return fail(e)
		}
		faces := []Face{{Surface: steinmetzChart{center, radius, 0, side}, U: u, V: Interval{-1, 1}, Reversed: side > 0}}
		for _, sign := range []int{1, -1} {
			faces = append(faces, Face{Surface: steinmetzExteriorCylinder{center, radius, end, side, sign}, U: u, V: Interval{0, 1}, Reversed: side*sign > 0})
		}
		faces = append(faces, Face{Surface: steinmetzDisk{geom.Vec3{X: capX, Y: center.Y, Z: center.Z}, radius}, U: u, V: Interval{-1, 1}, Reversed: side < 0})
		solid, e := joinPrimitivePatches(faces, tolerance)
		if e != nil {
			return fail(e)
		}
		result.Solids[i] = solid
		result.ToolFaces[i] = []int{0}
		result.XAxisFaces[i] = []int{1, 2}
		result.EndCapFaces[i] = []int{3}
		r2 := new(big.Rat).Mul(q(radius), q(radius))
		result.VolumePiCoefficients[i] = new(big.Rat).Mul(r2, q(math.Abs(end)))
		result.VolumeOffsets[i] = new(big.Rat).Mul(big.NewRat(-8, 3), new(big.Rat).Mul(r2, q(radius)))
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return result, nil
}

type steinmetzExteriorCylinder struct {
	center      geom.Vec3
	radius, end float64
	side, sign  int
}

func (s steinmetzExteriorCylinder) valid() bool {
	return s.center.IsFinite() && finite(s.radius) && finite(s.end) && s.radius > 0 && (s.side == 1 || s.side == -1) && (s.sign == 1 || s.sign == -1) && float64(s.side)*s.end > s.radius
}
func (s steinmetzExteriorCylinder) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s steinmetzExteriorCylinder) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s steinmetzExteriorCylinder) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !s.valid() || !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < 0 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	sin, cos := math.Sincos(u)
	if u == -math.Pi/2 || u == math.Pi/2 {
		cos = 0
	}
	a, da := s.radius*cos, -s.radius*sin
	k := float64(s.side)
	sign := float64(s.sign)
	p := s.center.Add(geom.Vec3{X: (1-v)*k*a + v*s.end, Y: s.radius * sin, Z: sign * a})
	du := geom.Vec3{X: (1 - v) * k * da, Y: a, Z: sign * da}
	dv := geom.Vec3{X: s.end - k*a}
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return p, du, dv, nil
}
func (s steinmetzExteriorCylinder) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	b := math.Nextafter(2*s.radius, math.Inf(1))
	if !finite(b) {
		return 0, 0, 0, geom.ErrInvalid
	}
	return b, b, 0, nil
}
