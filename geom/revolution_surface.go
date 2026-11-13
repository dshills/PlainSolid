package geom

import (
	"math"
	"math/big"
)

// RevolutionSurface is an immutable bounded analytic sweep. U is radians and
// V is the original B-spline parameter; no tensor reparameterization
// or periodic closure is introduced. The supported meridian occupies one
// fixed meridian half-plane about a signed-coordinate axis, with positive radii
// and strictly monotone axial control heights. These conditions keep its
// angular and meridian tangents independent throughout each smooth knot span.
type RevolutionSurface struct {
	profile      *BSpline
	origin, axis Vec3
	start, end   float64
}

// NewRevolutionSurface accepts a continuous clamped positive-weight meridian of at
// most 4096 poles, an exact signed-coordinate unit axis, and an increasing
// angular interval within [-3,3] radians of width at most 3. These conservative
// bounds exclude complete turns and periodic ambiguity. It retains the source
// profile and axis without snapping or normalizing them. Nonconstant weights
// additionally require degree <=16, <=256 poles, <=64 knot spans and an exact
// strict Bernstein sign proof of the axial rational derivative on every span.
func NewRevolutionSurface(profile *BSpline, origin, axis Vec3, start, end float64) (RevolutionSurface, error) {
	fail := func() (RevolutionSurface, error) { return RevolutionSurface{}, ErrDomain }
	if profile == nil || !origin.IsFinite() || !axis.IsFinite() || !finite(start) || !finite(end) || start < -3 || end > 3 || start >= end || new(big.Rat).Sub(new(big.Rat).SetFloat64(end), new(big.Rat).SetFloat64(start)).Cmp(big.NewRat(3, 1)) > 0 || len(profile.poles) > 4096 {
		return fail()
	}
	lo, hi := profile.Domain()
	if !finite(lo) || !finite(hi) || lo >= hi {
		return fail()
	}
	aa := [3]float64{axis.X, axis.Y, axis.Z}
	ax := -1
	for i, x := range aa {
		if x != 0 {
			if ax >= 0 || (x != 1 && x != -1) {
				return fail()
			}
			ax = i
		}
	}
	if ax < 0 {
		return fail()
	}
	rational := false
	for _, w := range profile.weights {
		if w != 1 {
			rational = true
		}
	}
	for i := 0; i <= profile.degree; i++ {
		if profile.knots[i] != lo || profile.knots[len(profile.knots)-1-i] != hi {
			return fail()
		}
	}
	run := 0
	for i, k := range profile.knots {
		if i == 0 || k != profile.knots[i-1] {
			run = 1
		} else {
			run++
		}
		if k != lo && k != hi && run > profile.degree {
			return fail()
		}
	}
	oo := [3]float64{origin.X, origin.Y, origin.Z}
	perp0, perp1 := (ax+1)%3, (ax+2)%3
	var first0, first1 *big.Rat
	heightSign := 0.
	previous := 0.
	for i, p := range profile.poles {
		pp := [3]float64{p.X, p.Y, p.Z}
		if !p.Sub(origin).IsFinite() {
			return fail()
		}
		q0 := new(big.Rat).Sub(new(big.Rat).SetFloat64(pp[perp0]), new(big.Rat).SetFloat64(oo[perp0]))
		q1 := new(big.Rat).Sub(new(big.Rat).SetFloat64(pp[perp1]), new(big.Rat).SetFloat64(oo[perp1]))
		if i == 0 {
			first0, first1 = q0, q1
		}
		cross := new(big.Rat).Sub(new(big.Rat).Mul(q0, first1), new(big.Rat).Mul(q1, first0))
		dot := new(big.Rat).Add(new(big.Rat).Mul(q0, first0), new(big.Rat).Mul(q1, first1))
		if cross.Sign() != 0 || dot.Sign() <= 0 {
			return fail()
		}
		// Compare original coordinates rather than a rounded projected height.
		if i > 0 {
			if pp[ax] == previous {
				return fail()
			}
			sign := math.Copysign(1, pp[ax]-previous)
			if i == 1 {
				heightSign = sign
			} else if sign != heightSign {
				return fail()
			}
		}
		previous = pp[ax]
	}
	if rational {
		if e := certifyRevolutionRationalHeight(profile, ax, heightSign); e != nil {
			return RevolutionSurface{}, e
		}
	}
	owned, e := NewBSpline(profile.degree, profile.poles, profile.knots, profile.weights)
	if e != nil {
		return fail()
	}
	return RevolutionSurface{profile: owned, origin: origin, axis: axis, start: start, end: end}, nil
}
func (s RevolutionSurface) Profile() *BSpline {
	if s.profile == nil {
		return nil
	}
	p, _ := NewBSpline(s.profile.degree, s.profile.poles, s.profile.knots, s.profile.weights)
	return p
}
func (s RevolutionSurface) Axis() (Vec3, Vec3) { return s.origin, s.axis }
func (s RevolutionSurface) Domain() (float64, float64, float64, float64) {
	if s.profile == nil {
		return math.NaN(), math.NaN(), math.NaN(), math.NaN()
	}
	a, b := s.profile.Domain()
	return s.start, s.end, a, b
}
func (s RevolutionSurface) valid(u, v float64) bool {
	if s.profile == nil || !finite(u) || !finite(v) || u < s.start || u > s.end {
		return false
	}
	a, b := s.profile.Domain()
	return v >= a && v <= b
}
func (s RevolutionSurface) rotate(p Vec3, u float64) Vec3 {
	sn, cs := math.Sincos(u)
	axial := s.axis.Scale(s.axis.Dot(p))
	radial := p.Sub(axial)
	return axial.Add(radial.Scale(cs)).Add(s.axis.Cross(radial).Scale(sn))
}
func (s RevolutionSurface) Eval(u, v float64) (Vec3, error) {
	if !s.valid(u, v) {
		return Vec3{}, ErrDomain
	}
	p, e := s.profile.Eval(v)
	if e != nil {
		return Vec3{}, e
	}
	q := s.origin.Add(s.rotate(p.Sub(s.origin), u))
	if !q.IsFinite() {
		return Vec3{}, ErrDomain
	}
	return q, nil
}
func (s RevolutionSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if !s.valid(u, v) {
		return Vec3{}, Vec3{}, ErrDomain
	}
	p, e := s.profile.Eval(v)
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	d, e := s.profile.Derivative(v)
	if e != nil {
		return Vec3{}, Vec3{}, e
	}
	du := s.axis.Cross(s.rotate(p.Sub(s.origin), u))
	dv := s.rotate(d, u)
	if !du.IsFinite() || !dv.IsFinite() {
		return Vec3{}, Vec3{}, ErrDomain
	}
	return du, dv, nil
}
