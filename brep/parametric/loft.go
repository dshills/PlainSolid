package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// EllipticLoft connects two concentric, axis-aligned elliptical profiles at
// z=0 and z=height in frame coordinates. Corresponding angles are connected
// by straight generators. Both ends have exact planar caps.
func EllipticLoft(frame geom.Frame, lower, upper geom.Vec2, height float64) (Solid, error) {
	if lower.X <= 0 || lower.Y <= 0 || upper.X <= 0 || upper.Y <= 0 || height <= 0 ||
		!finite(lower.X) || !finite(lower.Y) || !finite(upper.X) || !finite(upper.Y) || !finite(height) || frame.ZAxis().Length() == 0 {
		return Solid{}, geom.ErrInvalid
	}
	t := frame.ToWorld(geom.Vec3{Z: height})
	top, err := geom.NewFrame(t, frame.XAxis(), frame.ZAxis())
	if err != nil {
		return Solid{}, err
	}
	s := ellipticPatch{frame: frame, lower: lower, upper: upper, height: height}
	return build([]geom.Vec3{
		frame.ToWorld(geom.Vec3{X: lower.X}),
		frame.ToWorld(geom.Vec3{X: upper.X, Z: height}),
		frame.Origin(), t,
	}, math.Max(height, math.Max(math.Max(lower.X, lower.Y), math.Max(upper.X, upper.Y))),
		patch{face: Face{Surface: s, U: circleDomain, V: normalized, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}}, corners: [4]int{0, 0, 1, 1}},
		patch{Face{Surface: ellipticPatch{frame: frame, upper: lower}, U: circleDomain, V: normalized, Boundary: [4]Use{{3, false}, {4, false}, {0, true}, {4, true}}},
			[4]int{2, 2, 0, 0}, [4]bool{true, false, false, false}},
		patch{Face{Surface: ellipticPatch{frame: top, upper: upper}, U: circleDomain, V: normalized, Boundary: [4]Use{{5, false}, {6, false}, {2, false}, {6, true}}, Reversed: true},
			[4]int{3, 3, 1, 1}, [4]bool{true, false, false, false}})
}

type ellipticPatch struct {
	frame        geom.Frame
	lower, upper geom.Vec2
	height       float64
}

func (s ellipticPatch) Eval(u, v float64) (geom.Vec3, error) {
	if !finite(u) || !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	x, y := (1-v)*s.lower.X+v*s.upper.X, (1-v)*s.lower.Y+v*s.upper.Y
	p := s.frame.ToWorld(geom.Vec3{X: x * math.Cos(u), Y: y * math.Sin(u), Z: s.height * v})
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}
func (s ellipticPatch) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	x, y := (1-v)*s.lower.X+v*s.upper.X, (1-v)*s.lower.Y+v*s.upper.Y
	c, si := math.Cos(u), math.Sin(u)
	a := s.frame.XAxis().Scale(-x * si).Add(s.frame.YAxis().Scale(y * c))
	b := s.frame.XAxis().Scale((s.upper.X - s.lower.X) * c).Add(s.frame.YAxis().Scale((s.upper.Y - s.lower.Y) * si)).Add(s.frame.ZAxis().Scale(s.height))
	if !a.IsFinite() || !b.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, fmt.Errorf("parametric: nonfinite elliptic partials")
	}
	return a, b, nil
}
