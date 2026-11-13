// Package geom provides dependency-free Euclidean geometry and rational splines.
// Coordinates and parameters use float64. Units are chosen by the caller.
package geom

import (
	"errors"
	"math"
	"math/big"
)

var ErrInvalid = errors.New("geom: invalid or non-finite geometry")
var ErrDegenerate = errors.New("geom: degenerate geometry")
var ErrDomain = errors.New("geom: parameter outside domain")

// Vec2 is a point or vector in the plane.
type Vec2 struct{ X, Y float64 }

// Vec3 is a point or vector in three-dimensional Euclidean space.
type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vec3) Length() float64         { return math.Hypot(math.Hypot(a.X, a.Y), a.Z) }
func (a Vec3) Distance(b Vec3) float64 { return a.Sub(b).Length() }
func (a Vec3) IsFinite() bool          { return finite(a.X) && finite(a.Y) && finite(a.Z) }
func finite(x float64) bool            { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// Unit returns a unit vector, rejecting zero and non-finite input.
func (a Vec3) Unit() (Vec3, error) {
	if !a.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	m := math.Max(math.Abs(a.X), math.Max(math.Abs(a.Y), math.Abs(a.Z)))
	if m == 0 {
		return Vec3{}, ErrDegenerate
	}
	a = Vec3{a.X / m, a.Y / m, a.Z / m}
	return a.Scale(1 / a.Length()), nil
}

// Box is a closed axis-aligned box. Zero thickness is allowed.
type Box struct{ Min, Max Vec3 }

func (b Box) Valid() bool {
	return b.Min.IsFinite() && b.Max.IsFinite() && b.Min.X <= b.Max.X && b.Min.Y <= b.Max.Y && b.Min.Z <= b.Max.Z
}
func (b Box) Contains(p Vec3) bool {
	return b.Valid() && p.IsFinite() && p.X >= b.Min.X && p.X <= b.Max.X && p.Y >= b.Min.Y && p.Y <= b.Max.Y && p.Z >= b.Min.Z && p.Z <= b.Max.Z
}
func (b Box) Size() Vec3 { return b.Max.Sub(b.Min) }
func (b Box) Volume() float64 {
	if !b.Valid() {
		return math.NaN()
	}
	s := b.Size()
	return s.X * s.Y * s.Z
}
func (b Box) Intersect(c Box) (Box, bool) {
	d := Box{Vec3{math.Max(b.Min.X, c.Min.X), math.Max(b.Min.Y, c.Min.Y), math.Max(b.Min.Z, c.Min.Z)}, Vec3{math.Min(b.Max.X, c.Max.X), math.Min(b.Max.Y, c.Max.Y), math.Min(b.Max.Z, c.Max.Z)}}
	return d, b.Valid() && c.Valid() && d.Valid()
}
func (b Box) Union(c Box) (Box, error) {
	if !b.Valid() || !c.Valid() {
		return Box{}, ErrInvalid
	}
	return Box{Vec3{math.Min(b.Min.X, c.Min.X), math.Min(b.Min.Y, c.Min.Y), math.Min(b.Min.Z, c.Min.Z)}, Vec3{math.Max(b.Max.X, c.Max.X), math.Max(b.Max.Y, c.Max.Y), math.Max(b.Max.Z, c.Max.Z)}}, nil
}

// Bounds rejects an empty list, instead of silently making an origin box.
func Bounds(points []Vec3) (Box, error) {
	if len(points) == 0 {
		return Box{}, ErrInvalid
	}
	b := Box{points[0], points[0]}
	for _, p := range points {
		if !p.IsFinite() {
			return Box{}, ErrInvalid
		}
		b, _ = b.Union(Box{p, p})
	}
	return b, nil
}

// Orient2D returns the exact sign of the determinant for finite float64 inputs.
// An adaptive exact rational fallback resolves nearly collinear configurations.
func Orient2D(a, b, c Vec2) (int, error) {
	for _, v := range []float64{a.X, a.Y, b.X, b.Y, c.X, c.Y} {
		if !finite(v) {
			return 0, ErrInvalid
		}
	}
	l := (a.X - c.X) * (b.Y - c.Y)
	r := (a.Y - c.Y) * (b.X - c.X)
	d := l - r
	if finite(d) && math.Abs(l)+math.Abs(r) > 1e-292 && math.Abs(d) > 3.3306690738754716e-16*(math.Abs(l)+math.Abs(r)) {
		if d > 0 {
			return 1, nil
		}
		return -1, nil
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sub := func(x, y float64) *big.Rat { return new(big.Rat).Sub(rat(x), rat(y)) }
	left := new(big.Rat).Mul(sub(a.X, c.X), sub(b.Y, c.Y))
	right := new(big.Rat).Mul(sub(a.Y, c.Y), sub(b.X, c.X))
	return new(big.Rat).Sub(left, right).Sign(), nil
}

// ClosestOnSegment returns the nearest point and normalized segment parameter.
func ClosestOnSegment(p, a, b Vec3) (Vec3, float64, error) {
	if !p.IsFinite() || !a.IsFinite() || !b.IsFinite() {
		return Vec3{}, 0, ErrInvalid
	}
	d := b.Sub(a)
	length := d.Length()
	if !finite(length) {
		return Vec3{}, 0, ErrInvalid
	}
	if length == 0 {
		return a, 0, nil
	}
	u, _ := d.Unit()
	t := p.Sub(a).Dot(u) / length
	if !finite(t) {
		return Vec3{}, 0, ErrInvalid
	}
	t = math.Max(0, math.Min(1, t))
	return a.Add(d.Scale(t)), t, nil
}

// Plane is a point and unit normal, created with NewPlane.
type Plane struct {
	origin, normal Vec3
	raw            *exactPlaneEquation
}

func NewPlane(origin, normal Vec3) (Plane, error) {
	if !origin.IsFinite() {
		return Plane{}, ErrInvalid
	}
	n, e := normal.Unit()
	return Plane{origin: origin, normal: n}, e
}
func (p Plane) Origin() Vec3                  { return p.origin }
func (p Plane) Normal() Vec3                  { return p.normal }
func (p Plane) SignedDistance(v Vec3) float64 { return v.Sub(p.origin).Dot(p.normal) }
func (p Plane) Project(v Vec3) Vec3           { return v.Sub(p.normal.Scale(p.SignedDistance(v))) }

// IntersectRay solves origin+t*direction; false denotes a parallel ray or t<0.
func (p Plane) IntersectRay(origin, direction Vec3, tolerance float64) (Vec3, float64, bool, error) {
	if !origin.IsFinite() || !direction.IsFinite() || !finite(tolerance) || tolerance < 0 || p.normal.Length() == 0 {
		return Vec3{}, 0, false, ErrInvalid
	}
	u, e := direction.Unit()
	if e != nil {
		return Vec3{}, 0, false, e
	}
	den := p.normal.Dot(u)
	if math.Abs(den) <= tolerance {
		return Vec3{}, 0, false, nil
	}
	distance := -p.SignedDistance(origin) / den
	length := direction.Length()
	if !finite(length) {
		return Vec3{}, 0, false, ErrInvalid
	}
	t := distance / length
	if !finite(t) {
		return Vec3{}, 0, false, ErrInvalid
	}
	return origin.Add(u.Scale(distance)), t, t >= 0, nil
}
