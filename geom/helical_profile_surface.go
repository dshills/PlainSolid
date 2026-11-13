package geom

import (
	"math"
	"math/big"
)

// HelicalProfileSurface sweeps a straight radius/axial-offset profile with
// constant lead. U is measured in turns; V interpolates start→end in [0,1].
// S(U,V)=O+r(V)*(X*cos(2πhU)+Y*sin(2πhU))+Z*(lead*U+z(V)).
// A flat radial profile is a helicoid; sloping profiles describe thread flanks.
// This is analytic geometry, not a spline or a global embedding certificate.
// In particular, sufficiently tall profiles can overlap neighboring turns.
type HelicalProfileSurface struct {
	frame      Frame
	start, end Vec2
	lead       float64
	handedness HelixHandedness
	domain     ParameterRectangle
}

// NewHelicalProfileSurface requires finite positive endpoint radii/lead, a valid
// frame/handedness, distinct profile endpoints and finite uMin<uMax.
func NewHelicalProfileSurface(frame Frame, start, end Vec2, lead float64, handedness HelixHandedness, uMin, uMax float64) (HelicalProfileSurface, error) {
	if _, e := NewHelix(frame, start.X, lead, handedness); e != nil {
		return HelicalProfileSurface{}, e
	}
	if !finite(start.Y) || !finite(end.X) || !finite(end.Y) || end.X <= 0 || start == end || !finite(uMin) || !finite(uMax) || uMin >= uMax {
		return HelicalProfileSurface{}, ErrInvalid
	}
	return HelicalProfileSurface{frame, start, end, lead, handedness, ParameterRectangle{uMin, uMax, 0, 1}}, nil
}

// Domain returns the closed turn/profile parameter rectangle.
func (s HelicalProfileSurface) Domain() ParameterRectangle { return s.domain }

// Parameters returns copies of the frame, profile endpoints, lead and handedness.
func (s HelicalProfileSurface) Parameters() (Frame, Vec2, Vec2, float64, HelixHandedness) {
	return s.frame, s.start, s.end, s.lead, s.handedness
}
func (s HelicalProfileSurface) check(u, v float64) error {
	if !s.frame.valid() || !s.domain.Valid() || s.lead <= 0 || s.start.X <= 0 || s.end.X <= 0 || !finite(u) || !finite(v) {
		return ErrInvalid
	}
	if u < s.domain.UMin || u > s.domain.UMax || v < 0 || v > 1 {
		return ErrDomain
	}
	return nil
}

// Eval returns a numerical analytic representative. Exact bounds are available
// separately through Bounds; no certified roundoff is attached to Eval.
func (s HelicalProfileSurface) Eval(u, v float64) (Vec3, error) {
	if e := s.check(u, v); e != nil {
		return Vec3{}, e
	}
	sn, cs := math.Sincos(2 * math.Pi * float64(s.handedness) * math.Remainder(u, 1))
	r := (1-v)*s.start.X + v*s.end.X
	z := s.lead*u + (1-v)*s.start.Y + v*s.end.Y
	p := s.frame.ToWorld(Vec3{r * cs, r * sn, z})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}

// Partials returns derivatives in turns and normalized profile position.
func (s HelicalProfileSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if e := s.check(u, v); e != nil {
		return Vec3{}, Vec3{}, e
	}
	w := 2 * math.Pi * float64(s.handedness)
	sn, cs := math.Sincos(w * math.Remainder(u, 1))
	r := (1-v)*s.start.X + v*s.end.X
	dr, dz := s.end.X-s.start.X, s.end.Y-s.start.Y
	du := s.frame.x.Scale(-r * w * sn).Add(s.frame.y.Scale(r * w * cs)).Add(s.frame.z.Scale(s.lead))
	dv := s.frame.x.Scale(dr * cs).Add(s.frame.y.Scale(dr * sn)).Add(s.frame.z.Scale(dz))
	if !du.IsFinite() || !dv.IsFinite() {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	return du, dv, nil
}

// SecondPartials returns S_UU,S_UV,S_VV; S_VV is identically zero.
func (s HelicalProfileSurface) SecondPartials(u, v float64) (Vec3, Vec3, Vec3, error) {
	if e := s.check(u, v); e != nil {
		return Vec3{}, Vec3{}, Vec3{}, e
	}
	w := 2 * math.Pi * float64(s.handedness)
	sn, cs := math.Sincos(w * math.Remainder(u, 1))
	r := (1-v)*s.start.X + v*s.end.X
	dr := s.end.X - s.start.X
	uu := s.frame.x.Scale(-r * w * w * cs).Add(s.frame.y.Scale(-r * w * w * sn))
	uv := s.frame.x.Scale(-dr * w * sn).Add(s.frame.y.Scale(dr * w * cs))
	if !uu.IsFinite() || !uv.IsFinite() {
		return Vec3{}, Vec3{}, Vec3{}, ErrInvalid
	}
	return uu, uv, Vec3{}, nil
}

// Bounds encloses the exact analytic surface on a contained parameter rectangle.
// Full angular sine/cosine ranges deliberately trade tightness for a simple
// unconditional bound. Rational corner arithmetic and outward float conversion
// preserve tiny values; unrepresentable finite bounds return ErrConvergence.
func (s HelicalProfileSurface) Bounds(rect ParameterRectangle) (Box, error) {
	if !rect.Valid() {
		return Box{}, ErrInvalid
	}
	if e := s.check(rect.UMin, rect.VMin); e != nil {
		return Box{}, e
	}
	if e := s.check(rect.UMax, rect.VMax); e != nil {
		return Box{}, e
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	lerp := func(a, b, v float64) *big.Rat {
		return new(big.Rat).Add(rat(a), new(big.Rat).Mul(rat(v), new(big.Rat).Sub(rat(b), rat(a))))
	}
	r0, r1 := lerp(s.start.X, s.end.X, rect.VMin), lerp(s.start.X, s.end.X, rect.VMax)
	if r0.Cmp(r1) > 0 {
		r0, r1 = r1, r0
	}
	z0, z1 := lerp(s.start.Y, s.end.Y, rect.VMin), lerp(s.start.Y, s.end.Y, rect.VMax)
	if z0.Cmp(z1) > 0 {
		z0, z1 = z1, z0
	}
	z0.Add(z0, new(big.Rat).Mul(rat(s.lead), rat(rect.UMin)))
	z1.Add(z1, new(big.Rat).Mul(rat(s.lead), rat(rect.UMax)))
	origin := [3]float64{s.frame.origin.X, s.frame.origin.Y, s.frame.origin.Z}
	x := [3]float64{s.frame.x.X, s.frame.x.Y, s.frame.x.Z}
	y := [3]float64{s.frame.y.X, s.frame.y.Y, s.frame.y.Z}
	z := [3]float64{s.frame.z.X, s.frame.z.Y, s.frame.z.Z}
	var lo, hi [3]float64
	for i := 0; i < 3; i++ {
		radial := new(big.Rat).Mul(r1, new(big.Rat).Add(rat(math.Abs(x[i])), rat(math.Abs(y[i]))))
		a, b := new(big.Rat).Mul(rat(z[i]), z0), new(big.Rat).Mul(rat(z[i]), z1)
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		lo[i] = rationalFloatBound(new(big.Rat).Sub(new(big.Rat).Add(rat(origin[i]), a), radial), true)
		hi[i] = rationalFloatBound(new(big.Rat).Add(new(big.Rat).Add(rat(origin[i]), b), radial), false)
	}
	box := Box{Vec3{lo[0], lo[1], lo[2]}, Vec3{hi[0], hi[1], hi[2]}}
	if !box.Min.IsFinite() || !box.Max.IsFinite() {
		return Box{}, ErrConvergence
	}
	return box, nil
}

// SecondDerivativeBounds returns conservative Euclidean norm upper bounds for
// S_UU,S_UV,S_VV throughout rect. Exact rational arithmetic uses 2π<7 and
// component L1 norms of the represented frame axes. These ideal derivatives
// support interpolation deflection estimates, excluding Eval roundoff.
func (s HelicalProfileSurface) SecondDerivativeBounds(rect ParameterRectangle) (uu, uv, vv float64, err error) {
	if !rect.Valid() {
		return 0, 0, 0, ErrInvalid
	}
	if e := s.check(rect.UMin, rect.VMin); e != nil {
		return 0, 0, 0, e
	}
	if e := s.check(rect.UMax, rect.VMax); e != nil {
		return 0, 0, 0, e
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	dr := new(big.Rat).Sub(rat(s.end.X), rat(s.start.X))
	r0 := new(big.Rat).Add(rat(s.start.X), new(big.Rat).Mul(dr, rat(rect.VMin)))
	r1 := new(big.Rat).Add(rat(s.start.X), new(big.Rat).Mul(dr, rat(rect.VMax)))
	if r0.Cmp(r1) > 0 {
		r1 = r0
	}
	sum := new(big.Rat)
	for _, x := range []float64{s.frame.x.X, s.frame.x.Y, s.frame.x.Z, s.frame.y.X, s.frame.y.Y, s.frame.y.Z} {
		sum.Add(sum, rat(math.Abs(x)))
	}
	uu = rationalFloatBound(new(big.Rat).Mul(big.NewRat(49, 1), new(big.Rat).Mul(r1, sum)), false)
	uv = rationalFloatBound(new(big.Rat).Mul(big.NewRat(7, 1), new(big.Rat).Mul(new(big.Rat).Abs(dr), sum)), false)
	if !finite(uu) || !finite(uv) {
		return 0, 0, 0, ErrConvergence
	}
	return uu, uv, 0, nil
}
