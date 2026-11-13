// Package elbow authors bounded circular torus segments on one retained chart.
package elbow

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("elbow: invalid ring or sweep")
var ErrUnresolved = errors.New("elbow: angular limit or representation unresolved")

// Segment retains an immutable shared chart. The native constructor normalizes
// its cap frames independently; exact measures/bounds here target the affine
// torus image under Frame's represented axes, not exact native chart sewing.
type Segment struct {
	native              parametric.Solid
	frame               geom.Frame
	major, minor, angle float64
}

func (s Segment) Solid() parametric.Solid { return s.native }
func (s Segment) SignedAngle() float64    { return s.angle }
func (s Segment) Frame() geom.Frame       { return s.frame }
func rat(x float64) *big.Rat              { return new(big.Rat).SetFloat64(x) }
func add(a, b *big.Rat) *big.Rat          { return new(big.Rat).Add(a, b) }
func mul(a, b *big.Rat) *big.Rat          { return new(big.Rat).Mul(a, b) }
func sub(a, b *big.Rat) *big.Rat          { return new(big.Rat).Sub(a, b) }
func abs(a *big.Rat) *big.Rat             { return new(big.Rat).Abs(a) }
func finite(x float64) bool               { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// NewSigned accepts either sweep sign, excludes zero and unresolved near-full
// sweeps, and requires a ring (major>minor>0). Negative sweeps reverse the
// frame's Y and Z directions; circular meridians make this the same material
// sweep orientation. A conservative rational upper limit6<2π proves angular
// injectivity without treating binary64 math.Pi as exact. Native near-zero
// conditioning checks still apply. No result is returned on failure.
func NewSigned(ctx context.Context, frame geom.Frame, major, minor, angle float64) (Segment, error) {
	if ctx == nil {
		return Segment{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Segment{}, e
	}
	if !finite(major) || !finite(minor) || !finite(angle) || minor <= 0 || major <= minor || angle == 0 {
		return Segment{}, ErrInvalid
	}
	if math.Abs(angle) > 6 {
		return Segment{}, ErrUnresolved
	}
	if angle < 0 {
		f, e := geom.NewFrame(frame.Origin(), frame.XAxis(), frame.ZAxis().Scale(-1))
		if e != nil {
			return Segment{}, e
		}
		frame = f
	}
	native, e := parametric.TorusElbow(frame, major, minor, math.Abs(angle))
	if e != nil {
		return Segment{}, e
	}
	s := Segment{native, frame, major, minor, angle}
	if s.determinant().Sign() <= 0 {
		return Segment{}, ErrInvalid
	}
	return s, nil
}
func (s Segment) determinant() *big.Rat {
	x, y, z := s.frame.XAxis(), s.frame.YAxis(), s.frame.ZAxis()
	a, b, c := [3]float64{x.X, x.Y, x.Z}, [3]float64{y.X, y.Y, y.Z}, [3]float64{z.X, z.Y, z.Z}
	v := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		v.Add(v, mul(rat(a[i]), sub(mul(rat(b[j]), rat(c[k])), mul(rat(b[k]), rat(c[j])))))
	}
	return v
}

// VolumePiCoefficient is exact: ideal affine volume = π times the returned
// rational, including the exact determinant of represented frame axes. The
// value is caller-owned; the transcendental constant is not rounded here.
func (s Segment) VolumePiCoefficient() (*big.Rat, error) {
	if s.angle == 0 {
		return nil, ErrInvalid
	}
	return mul(mul(mul(mul(rat(s.major), rat(s.minor)), rat(s.minor)), rat(math.Abs(s.angle))), abs(s.determinant())), nil
}

// IdealBounds is a conservative, deliberately full-ring affine hull. Each
// endpoint is rounded outward from an exact rational coordinate bound; it
// covers any accepted sweep and both end disks, including all rounding of the
// represented axes. It does not claim a tight angular box or normalized native
// cap error bounds.
func (s Segment) IdealBounds() (geom.Box, error) {
	if s.angle == 0 {
		return geom.Box{}, ErrInvalid
	}
	o, x, y, z := s.frame.Origin(), s.frame.XAxis(), s.frame.YAxis(), s.frame.ZAxis()
	orig := [3]float64{o.X, o.Y, o.Z}
	xx, yy, zz := [3]float64{x.X, x.Y, x.Z}, [3]float64{y.X, y.Y, y.Z}, [3]float64{z.X, z.Y, z.Z}
	var lo, hi [3]float64
	for i := 0; i < 3; i++ {
		d := add(mul(add(rat(s.major), rat(s.minor)), add(abs(rat(xx[i])), abs(rat(yy[i])))), mul(rat(s.minor), abs(rat(zz[i]))))
		a, b := sub(rat(orig[i]), d), add(rat(orig[i]), d)
		lo[i], _ = a.Float64()
		hi[i], _ = b.Float64()
		if rat(lo[i]) == nil || rat(hi[i]) == nil {
			return geom.Box{}, ErrUnresolved
		}
		if rat(lo[i]).Cmp(a) > 0 {
			lo[i] = math.Nextafter(lo[i], math.Inf(-1))
		}
		if rat(hi[i]).Cmp(b) < 0 {
			hi[i] = math.Nextafter(hi[i], math.Inf(1))
		}
		if !finite(lo[i]) || !finite(hi[i]) {
			return geom.Box{}, ErrUnresolved
		}
	}
	return geom.Box{Min: geom.Vec3{X: lo[0], Y: lo[1], Z: lo[2]}, Max: geom.Vec3{X: hi[0], Y: hi[1], Z: hi[2]}}, nil
}

// Continuation identifies two adjacent closed angular intervals of the same
// original chart. Their interiors are disjoint and their intersection is the
// single common meridian disk. Interval coordinates increase in the retained
// chart, even for a negative original sweep. Returned rationals are owned.
type Continuation struct {
	Union       Segment
	Intervals   [2][2]*big.Rat
	SharedAngle *big.Rat
}

// Continue extends in the original sweep direction; it is a constrained union,
// not an arbitrary-frame Boolean. Shared support identity comes from the private
// Segment, not approximate frame matching. The represented sum must be exact.
func Continue(ctx context.Context, s Segment, delta float64) (Continuation, error) {
	if ctx == nil || s.angle == 0 || !finite(delta) || delta == 0 || math.Signbit(delta) != math.Signbit(s.angle) {
		return Continuation{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Continuation{}, e
	}
	end := add(rat(math.Abs(s.angle)), rat(math.Abs(delta)))
	if end.Cmp(rat(6)) > 0 {
		return Continuation{}, ErrUnresolved
	}
	value, e := geom.RepresentedExactScalar(end)
	if e != nil {
		return Continuation{}, e
	}
	native, e := parametric.TorusElbow(s.frame, s.major, s.minor, value)
	if e != nil {
		return Continuation{}, e
	}
	angle := value
	if s.angle < 0 {
		angle = -angle
	}
	union := Segment{native, s.frame, s.major, s.minor, angle}
	joint := rat(math.Abs(s.angle))
	return Continuation{Union: union, Intervals: [2][2]*big.Rat{{new(big.Rat), new(big.Rat).Set(joint)}, {new(big.Rat).Set(joint), new(big.Rat).Set(end)}}, SharedAngle: joint}, nil
}
