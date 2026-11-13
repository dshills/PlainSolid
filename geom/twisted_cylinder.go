package geom

import (
	"math"
	"math/big"
)

// TwistedCylinderSurface is an analytic cylinder parameterized by an axial
// coordinate V and angular turns U+Twist*V+Phase. An integral step in U is a
// full turn. The resulting seam is a helix rather than a straight generator.
type TwistedCylinderSurface struct {
	frame                Frame
	radius, twist, phase float64
}

func NewTwistedCylinderSurface(frame Frame, radius, twist, phase float64) (TwistedCylinderSurface, error) {
	if !frame.valid() || !finite(radius) || radius <= 0 || !finite(twist) || !finite(phase) {
		return TwistedCylinderSurface{}, ErrInvalid
	}
	return TwistedCylinderSurface{frame, radius, twist, phase}, nil
}

func (s TwistedCylinderSurface) Parameters() (Frame, float64, float64, float64) {
	return s.frame, s.radius, s.twist, s.phase
}

func (s TwistedCylinderSurface) polar(u, v float64) (float64, float64, error) {
	if !s.frame.valid() || s.radius <= 0 || !finite(u) || !finite(v) {
		return 0, 0, ErrInvalid
	}
	t := u + s.twist*v + s.phase
	if !finite(t) {
		return 0, 0, ErrInvalid
	}
	a, b := math.Sincos(2 * math.Pi * math.Remainder(t, 1))
	return b, a, nil
}

func (s TwistedCylinderSurface) Eval(u, v float64) (Vec3, error) {
	c, q, e := s.polar(u, v)
	if e != nil {
		return Vec3{}, e
	}
	p := s.frame.ToWorld(Vec3{s.radius * c, s.radius * q, v})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}

func (s TwistedCylinderSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	c, q, e := s.polar(u, v)
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	a := s.frame.XAxis().Scale(-q * s.radius * 2 * math.Pi).Add(s.frame.YAxis().Scale(c * s.radius * 2 * math.Pi))
	b := a.Scale(s.twist).Add(s.frame.ZAxis())
	if !a.IsFinite() || !b.IsFinite() {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	return a, b, nil
}

// SecondDerivativeBounds bounds the model norm of UU, UV and VV over the
// whole chart using exact represented arithmetic and 2π<7.
func (s TwistedCylinderSurface) SecondDerivativeBounds() (float64, float64, float64, error) {
	if !s.frame.valid() || s.radius <= 0 {
		return 0, 0, 0, ErrInvalid
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x, y := s.frame.XAxis(), s.frame.YAxis()
	n := new(big.Rat)
	for _, v := range []float64{x.X, x.Y, x.Z, y.X, y.Y, y.Z} {
		n.Add(n, rat(math.Abs(v)))
	}
	n.Mul(n, new(big.Rat).Mul(big.NewRat(49, 1), rat(s.radius)))
	t := rat(math.Abs(s.twist))
	var out [3]float64
	for i := range out {
		q, e := RationalOutwardRange(n)
		if e != nil || !finite(q.Max) {
			return 0, 0, 0, ErrConvergence
		}
		out[i] = q.Max
		n.Mul(n, t)
	}
	return out[0], out[1], out[2], nil
}
