package step

import (
	"errors"
	"math"
	"plainsolid/geom"
)

// ConicalSurface retains STEP's reference-circle and axial parameter convention:
// radius(v)=ReferenceRadius+v*tan(SemiAngle), local axial coordinate=v.
// Evaluation is restricted to the nonnegative-radius sheet.
type ConicalSurface struct {
	frame         geom.Frame
	radius, angle float64
}

func (s ConicalSurface) Parameters() (geom.Frame, float64, float64) {
	return s.frame, s.radius, s.angle
}
func (s ConicalSurface) Eval(u, v float64) (geom.Vec3, error) {
	if s.angle <= 0 || s.angle >= math.Pi/2 || s.frame.ZAxis().Length() == 0 || !finiteNumber(u) || !finiteNumber(v) {
		return geom.Vec3{}, errors.New("step: invalid cone parameter")
	}
	r := s.radius + v*math.Tan(s.angle)
	if !finiteNumber(r) || r < 0 {
		return geom.Vec3{}, errors.New("step: cone parameter outside sheet")
	}
	p := s.frame.ToWorld(geom.Vec3{X: r * math.Cos(u), Y: r * math.Sin(u), Z: v})
	if !p.IsFinite() {
		return geom.Vec3{}, errors.New("step: cone evaluation overflow")
	}
	return p, nil
}
func (s ConicalSurface) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	r := s.radius + v*math.Tan(s.angle)
	du := s.frame.XAxis().Scale(-r * math.Sin(u)).Add(s.frame.YAxis().Scale(r * math.Cos(u)))
	dv := s.frame.XAxis().Scale(math.Tan(s.angle) * math.Cos(u)).Add(s.frame.YAxis().Scale(math.Tan(s.angle) * math.Sin(u))).Add(s.frame.ZAxis())
	if !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, errors.New("step: cone partial overflow")
	}
	return du, dv, nil
}
