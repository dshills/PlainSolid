package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// SteinmetzEquatorHalf retains the cylinder common on one side of X=center.X.
// PositiveX selects the positive side; its cap points toward negative X.
type SteinmetzEquatorHalf struct {
	Solid                  Solid
	XAxisFaces, ZAxisFaces []int
	CapFace                int
	PositiveX              bool
	IdealVolume            *big.Rat
}

// PerpendicularCylinderEquatorHalf models one cylinder with an exact equator
// end cap and the opposite cap beyond the common, while the other cylinder's
// caps lie beyond both radial extrema. This is specifically the represented
// c=0 local cap, not a rounded approximation to a nonzero cap intersection.
func PerpendicularCylinderEquatorHalf(ctx context.Context, center geom.Vec3, radius, xOtherHalfLength, zHalfLength, tolerance float64, positiveX bool) (SteinmetzEquatorHalf, error) {
	fail := func(e error) (SteinmetzEquatorHalf, error) { return SteinmetzEquatorHalf{}, e }
	base, e := PerpendicularCylinderCommon(ctx, center, radius, xOtherHalfLength, zHalfLength, tolerance)
	if e != nil {
		return fail(e)
	}
	sign := -1
	v := Interval{-1, 0}
	if positiveX {
		sign = 1
		v = Interval{0, 1}
	}
	faces := []Face{{Surface: steinmetzChart{center, radius, 0, sign}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1}, Reversed: sign < 0}}
	for _, radialSign := range []int{1, -1} {
		faces = append(faces, Face{Surface: steinmetzChart{center, radius, 1, radialSign}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: v, Reversed: radialSign > 0})
	}
	faces = append(faces, Face{Surface: steinmetzDisk{center, radius}, U: Interval{-math.Pi / 2, math.Pi / 2}, V: Interval{-1, 1}, Reversed: positiveX})
	solid, e := joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	return SteinmetzEquatorHalf{Solid: solid, ZAxisFaces: []int{0}, XAxisFaces: []int{1, 2}, CapFace: 3, PositiveX: positiveX, IdealVolume: new(big.Rat).Quo(base.IdealVolume, big.NewRat(2, 1))}, nil
}

// steinmetzDisk is a planar YZ disk using the identical angle and pole model
// as the cylindrical charts. It has two shared semicircles and collapsed poles.
type steinmetzDisk struct {
	center geom.Vec3
	radius float64
}

func (s steinmetzDisk) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s steinmetzDisk) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s steinmetzDisk) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !s.center.IsFinite() || !finite(s.radius) || s.radius <= 0 || !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < -1 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	sin, cos := math.Sincos(u)
	if u == -math.Pi/2 || u == math.Pi/2 {
		cos = 0
	}
	p := s.center.Add(geom.Vec3{Y: s.radius * sin, Z: v * s.radius * cos})
	du := geom.Vec3{Y: s.radius * cos, Z: -v * s.radius * sin}
	dv := geom.Vec3{Z: s.radius * cos}
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return p, du, dv, nil
}
func (s steinmetzDisk) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	b := math.Nextafter(2*s.radius, math.Inf(1))
	if !finite(b) {
		return 0, 0, 0, geom.ErrInvalid
	}
	return b, b, 0, nil
}
