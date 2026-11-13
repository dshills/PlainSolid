package geom

import (
	"math"
	"math/big"
)

// HelixHandedness determines angular travel as the parameter increases.
// Viewed toward the origin from the positive frame axis, RightHanded rotates
// counterclockwise while advancing along that axis; LeftHanded rotates clockwise.
type HelixHandedness int

const (
	RightHanded HelixHandedness = 1
	LeftHanded  HelixHandedness = -1
)

// Helix is the analytic curve O+R*(X*cos(2πht)+Y*sin(2πht))+Z*lead*t.
// Its parameter t is measured in turns, lead is positive distance per turn,
// and h is handedness. It is not a rational spline. Frame coefficients and
// dimensions denote their exact represented values; trig evaluation and
// derivatives return ordinary float64 numerical approximations.
type Helix struct {
	frame        Frame
	radius, lead float64
	handedness   HelixHandedness
}

// NewHelix requires a valid frame, finite positive radius/lead, and explicit handedness.
func NewHelix(frame Frame, radius, lead float64, handedness HelixHandedness) (Helix, error) {
	if !frame.valid() || !finite(radius) || !finite(lead) || radius <= 0 || lead <= 0 || (handedness != RightHanded && handedness != LeftHanded) {
		return Helix{}, ErrInvalid
	}
	return Helix{frame, radius, lead, handedness}, nil
}

// Parameters returns the immutable frame, radius, lead and handedness.
func (h Helix) Parameters() (Frame, float64, float64, HelixHandedness) {
	return h.frame, h.radius, h.lead, h.handedness
}
func (h Helix) valid() bool {
	return h.frame.valid() && finite(h.radius) && finite(h.lead) && h.radius > 0 && h.lead > 0 && (h.handedness == RightHanded || h.handedness == LeftHanded)
}

// Eval reduces turns before angular evaluation, preserving the represented
// fractional turn even for large integral t. Axial overflow returns ErrInvalid.
func (h Helix) Eval(t float64) (Vec3, error) {
	if !h.valid() || !finite(t) {
		return Vec3{}, ErrInvalid
	}
	s, c := math.Sincos(2 * math.Pi * float64(h.handedness) * math.Remainder(t, 1))
	p := h.frame.ToWorld(Vec3{h.radius * c, h.radius * s, h.lead * t})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}

// Derivative differentiates with respect to turns, not radians or arc length.
func (h Helix) Derivative(t float64) (Vec3, error) {
	if !h.valid() || !finite(t) {
		return Vec3{}, ErrInvalid
	}
	w := 2 * math.Pi * float64(h.handedness)
	s, c := math.Sincos(w * math.Remainder(t, 1))
	p := h.frame.x.Scale(-h.radius * w * s).Add(h.frame.y.Scale(h.radius * w * c)).Add(h.frame.z.Scale(h.lead))
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}

// SecondDerivative returns the second derivative with respect to turns.
func (h Helix) SecondDerivative(t float64) (Vec3, error) {
	if !h.valid() || !finite(t) {
		return Vec3{}, ErrInvalid
	}
	w := 2 * math.Pi * float64(h.handedness)
	s, c := math.Sincos(w * math.Remainder(t, 1))
	p := h.frame.x.Scale(-h.radius * w * w * c).Add(h.frame.y.Scale(-h.radius * w * w * s))
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}

// Bounds encloses the exact analytic curve on the closed turn interval. Radial
// ranges use full sine/cosine bounds, so small intervals can have loose boxes.
// Exact rational endpoint arithmetic and outward rounding include subnormals.
func (h Helix) Bounds(t0, t1 float64) (Box, error) {
	if !h.valid() || !finite(t0) || !finite(t1) || t0 > t1 {
		return Box{}, ErrInvalid
	}
	var lo, hi Vec3
	origin := [3]float64{h.frame.origin.X, h.frame.origin.Y, h.frame.origin.Z}
	x := [3]float64{h.frame.x.X, h.frame.x.Y, h.frame.x.Z}
	y := [3]float64{h.frame.y.X, h.frame.y.Y, h.frame.y.Z}
	z := [3]float64{h.frame.z.X, h.frame.z.Y, h.frame.z.Z}
	var lower, upper [3]float64
	for axis := 0; axis < 3; axis++ {
		radial := new(big.Rat).Mul(new(big.Rat).SetFloat64(h.radius), new(big.Rat).Add(new(big.Rat).SetFloat64(math.Abs(x[axis])), new(big.Rat).SetFloat64(math.Abs(y[axis]))))
		slope := new(big.Rat).Mul(new(big.Rat).SetFloat64(z[axis]), new(big.Rat).SetFloat64(h.lead))
		a := new(big.Rat).Mul(slope, new(big.Rat).SetFloat64(t0))
		b := new(big.Rat).Mul(slope, new(big.Rat).SetFloat64(t1))
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		o := new(big.Rat).SetFloat64(origin[axis])
		lower[axis] = rationalFloatBound(new(big.Rat).Sub(new(big.Rat).Add(o, a), radial), true)
		upper[axis] = rationalFloatBound(new(big.Rat).Add(new(big.Rat).Add(o, b), radial), false)
	}
	lo = Vec3{lower[0], lower[1], lower[2]}
	hi = Vec3{upper[0], upper[1], upper[2]}
	if !lo.IsFinite() || !hi.IsFinite() {
		return Box{}, ErrConvergence
	}
	return Box{lo, hi}, nil
}

// ChordDeviationBound bounds Euclidean deviation from the exact linear chord
// at matching normalized parameters on [t0,t1]. The interpolation remainder is
// ||C”||∞*(t1-t0)^2/8; 2π<7 and component L1 norms supply an exact conservative
// bound without assuming the stored normalized frame is perfectly orthogonal.
// It excludes roundoff in endpoint values returned by Eval; it is not a mesh
// certification. A zero-width interval has zero deviation.
func (h Helix) ChordDeviationBound(t0, t1 float64) (float64, error) {
	if !h.valid() || !finite(t0) || !finite(t1) || t0 > t1 {
		return 0, ErrInvalid
	}
	sum := new(big.Rat)
	for _, q := range []float64{h.frame.x.X, h.frame.x.Y, h.frame.x.Z, h.frame.y.X, h.frame.y.Y, h.frame.y.Z} {
		sum.Add(sum, new(big.Rat).SetFloat64(math.Abs(q)))
	}
	dt := new(big.Rat).Sub(new(big.Rat).SetFloat64(t1), new(big.Rat).SetFloat64(t0))
	bound := new(big.Rat).Mul(big.NewRat(49, 8), new(big.Rat).Mul(new(big.Rat).SetFloat64(h.radius), new(big.Rat).Mul(sum, new(big.Rat).Mul(dt, dt))))
	value := rationalFloatBound(bound, false)
	if !finite(value) {
		return 0, ErrConvergence
	}
	return value, nil
}
