package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

type mappedCurve struct {
	curve     geom.Curve
	transform geom.Affine
}

func (c mappedCurve) Eval(t float64) (geom.Vec3, error) {
	p, e := c.curve.Eval(t)
	if e != nil {
		return p, e
	}
	return checked(c.transform.Apply(p))
}
func (c mappedCurve) Derivative(t float64) (geom.Vec3, error) {
	p, e := c.curve.Derivative(t)
	if e != nil {
		return p, e
	}
	return checked(c.transform.ApplyVector(p))
}

type mappedSurface struct {
	surface   geom.Surface
	transform geom.Affine
}

func (s mappedSurface) Eval(u, v float64) (geom.Vec3, error) {
	p, e := s.surface.Eval(u, v)
	if e != nil {
		return p, e
	}
	return checked(s.transform.Apply(p))
}
func (s mappedSurface) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	a, b, e := s.surface.Partials(u, v)
	if e != nil {
		return a, b, e
	}
	a, b = s.transform.ApplyVector(a), s.transform.ApplyVector(b)
	if !a.IsFinite() || !b.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return a, b, nil
}
func checked(p geom.Vec3) (geom.Vec3, error) {
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}

// MapCurve applies an affine map to a curve evaluator, preserving its
// parameter. Callers should validate the transform for their use case.
func MapCurve(c geom.Curve, m geom.Affine) geom.Curve {
	if c == nil {
		return nil
	}
	if old, ok := c.(mappedCurve); ok {
		return mappedCurve{old.curve, m.Mul(old.transform)}
	}
	return mappedCurve{c, m}
}

// MapSurface applies an affine map to a surface evaluator, preserving UV.
func MapSurface(s geom.Surface, m geom.Affine) geom.Surface {
	if s == nil {
		return nil
	}
	if old, ok := s.(mappedSurface); ok {
		return mappedSurface{old.surface, m.Mul(old.transform)}
	}
	return mappedSurface{s, m}
}

// SurfaceMapping exposes the base evaluator and affine placement of a
// surface produced by MapSurface. Unmapped surfaces return the input and
// the identity transform.
func SurfaceMapping(s geom.Surface) (geom.Surface, geom.Affine) {
	if m, ok := s.(mappedSurface); ok {
		return m.surface, m.transform
	}
	return s, geom.Identity()
}

// CurveMapping exposes the original curve and affine placement of a mapped
// curve. Unmapped curves have identity placement.
func CurveMapping(c geom.Curve) (geom.Curve, geom.Affine) {
	if m, ok := c.(mappedCurve); ok {
		return m.curve, m.transform
	}
	return c, geom.Identity()
}

// Transformed retains the exact evaluated geometry and topology under a
// nonsingular affine map. Reflections reverse oriented face normals. The
// stored edge agreement tolerance is scaled by a conservative linear norm.
func (s Solid) Transformed(m geom.Affine) (Solid, error) {
	if _, e := m.Inverse(); e != nil {
		return Solid{}, e
	}
	det := m.Determinant()
	if det == 0 || !finite(det) {
		return Solid{}, fmt.Errorf("parametric: determinant outside float64 range")
	}
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	for i, p := range v {
		v[i] = m.Apply(p)
	}
	for i, x := range e {
		e[i].Curve = MapCurve(x.Curve, m)
	}
	for i, x := range f {
		f[i].Surface = MapSurface(x.Surface, m)
		if det < 0 {
			f[i].Reversed = !x.Reversed
		}
	}
	bound := 0.
	for _, r := range m {
		bound = math.Hypot(bound, math.Hypot(math.Hypot(r[0], r[1]), r[2]))
	}
	return NewSolid(v, e, f, s.tolerance*bound)
}
