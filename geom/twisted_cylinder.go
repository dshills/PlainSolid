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
	direction            float64
}

func NewTwistedCylinderSurface(frame Frame, radius, twist, phase float64) (TwistedCylinderSurface, error) {
	if !frame.valid() || !finite(radius) || radius <= 0 || !finite(twist) || !finite(phase) {
		return TwistedCylinderSurface{}, ErrInvalid
	}
	return TwistedCylinderSurface{frame, radius, twist, phase, 1}, nil
}

// ReversedAngular flips the orientation of the angular coordinate. The model
// support is unchanged and U remains periodic with period one.
func (s TwistedCylinderSurface) ReversedAngular() TwistedCylinderSurface {
	s.direction = -s.direction
	return s
}

func (s TwistedCylinderSurface) AngularReversed() bool { return s.direction < 0 }

func (s TwistedCylinderSurface) Parameters() (Frame, float64, float64, float64) {
	return s.frame, s.radius, s.twist, s.phase
}

func (s TwistedCylinderSurface) polar(u, v float64) (float64, float64, error) {
	if !s.frame.valid() || s.radius <= 0 || !finite(u) || !finite(v) {
		return 0, 0, ErrInvalid
	}
	t := s.direction*u + s.twist*v + s.phase
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
	vary := s.frame.XAxis().Scale(-q * s.radius * 2 * math.Pi).Add(s.frame.YAxis().Scale(c * s.radius * 2 * math.Pi))
	a := vary.Scale(s.direction)
	b := vary.Scale(s.twist).Add(s.frame.ZAxis())
	if !a.IsFinite() || !b.IsFinite() {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	return a, b, nil
}

// TwistedAnnularSurface parameterizes a planar annulus by turns U and radius
// V. The inner and outer circular rims have independent angular seam phases.
type TwistedAnnularSurface struct {
	frame                       Frame
	inner, outer, height, phase float64
}

func NewTwistedAnnularSurface(frame Frame, inner, outer, height, phase float64) (TwistedAnnularSurface, error) {
	if !frame.valid() || !finite(inner) || !finite(outer) || !finite(height) || !finite(phase) ||
		inner <= 0 || outer <= inner || math.Abs(phase) > .5 {
		return TwistedAnnularSurface{}, ErrInvalid
	}
	return TwistedAnnularSurface{frame, inner, outer, height, phase}, nil
}

func (s TwistedAnnularSurface) Parameters() (Frame, float64, float64, float64, float64) {
	return s.frame, s.inner, s.outer, s.height, s.phase
}

func (s TwistedAnnularSurface) polar(u, v float64) (float64, float64, error) {
	if !s.frame.valid() || s.inner <= 0 || !finite(u) || !finite(v) || v < s.inner || v > s.outer {
		return 0, 0, ErrDomain
	}
	t := math.Remainder(u+s.phase*(s.outer-v)/(s.outer-s.inner), 1)
	a, b := math.Sincos(2 * math.Pi * t)
	return b, a, nil
}

func (s TwistedAnnularSurface) Eval(u, v float64) (Vec3, error) {
	c, q, e := s.polar(u, v)
	if e != nil {
		return Vec3{}, e
	}
	p := s.frame.ToWorld(Vec3{v * c, v * q, s.height})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}

func (s TwistedAnnularSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	c, q, e := s.polar(u, v)
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	x, y := s.frame.XAxis(), s.frame.YAxis()
	a := x.Scale(-q * v * 2 * math.Pi).Add(y.Scale(c * v * 2 * math.Pi))
	b := x.Scale(c).Add(y.Scale(q)).Add(a.Scale(-s.phase / (s.outer - s.inner)))
	if !a.IsFinite() || !b.IsFinite() {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	return a, b, nil
}

func (s TwistedAnnularSurface) SecondDerivativeBounds() (float64, float64, float64, error) {
	if !s.frame.valid() || s.inner <= 0 || s.outer <= s.inner {
		return 0, 0, 0, ErrInvalid
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x, y := s.frame.XAxis(), s.frame.YAxis()
	n := new(big.Rat)
	for _, v := range []float64{x.X, x.Y, x.Z, y.X, y.Y, y.Z} {
		n.Add(n, rat(math.Abs(v)))
	}
	d := new(big.Rat).Quo(rat(math.Abs(s.phase)), new(big.Rat).Sub(rat(s.outer), rat(s.inner)))
	uu := new(big.Rat).Mul(n, new(big.Rat).Mul(big.NewRat(49, 1), rat(s.outer)))
	base := new(big.Rat).Mul(n, big.NewRat(7, 1))
	uv := new(big.Rat).Add(base, new(big.Rat).Mul(uu, d))
	vv := new(big.Rat).Mul(d, new(big.Rat).Add(new(big.Rat).Mul(base, big.NewRat(2, 1)), new(big.Rat).Mul(uu, d)))
	var out [3]float64
	for i, p := range []*big.Rat{uu, uv, vv} {
		q, e := RationalOutwardRange(p)
		if e != nil || !finite(q.Max) {
			return 0, 0, 0, ErrConvergence
		}
		out[i] = q.Max
	}
	return out[0], out[1], out[2], nil
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
