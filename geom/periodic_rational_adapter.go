package geom

import (
	"context"
	"math/big"
)

// ControlCount reports the number of original controls in one cycle.
func (c *ExactPeriodicSplineCurve) ControlCount() int {
	if c == nil {
		return 0
	}
	return len(c.net)
}

// Eval adapts the exact homogeneous curve to Curve. t is interpreted as
// its represented binary64 value; use EvaluateRational at nonbinary knots.
func (c *ExactPeriodicSplineCurve) Eval(t float64) (Vec3, error) {
	jet, err := c.evaluateRepresented(t)
	if err != nil {
		return Vec3{}, err
	}
	return representedPeriodicVector(jet.Point)
}

// Derivative rounds the exact physical first derivative at the represented t.
func (c *ExactPeriodicSplineCurve) Derivative(t float64) (Vec3, error) {
	jet, err := c.evaluateRepresented(t)
	if err != nil {
		return Vec3{}, err
	}
	return representedPeriodicVector(jet.Derivative)
}

func (c *ExactPeriodicSplineCurve) evaluateRepresented(t float64) (RationalPeriodicCurveJet, error) {
	if !finite(t) {
		return RationalPeriodicCurveJet{}, ErrDomain
	}
	return c.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(t), SurfacePatchOptions{})
}

// ControlShape reports the number of controls in both original directions.
func (s *ExactPeriodicSplineSurface) ControlShape() (u, v int) {
	if s == nil {
		return 0, 0
	}
	return s.axis[0].count, s.axis[1].count
}

// Eval adapts the exact homogeneous tensor to Surface. It rounds the
// exact point evaluated at the two represented binary64 parameters.
func (s *ExactPeriodicSplineSurface) Eval(u, v float64) (Vec3, error) {
	jet, err := s.evaluateRepresented(u, v)
	if err != nil {
		return Vec3{}, err
	}
	return representedPeriodicVector(jet.Point)
}

// Partials rounds the exact physical derivatives at represented u and v.
func (s *ExactPeriodicSplineSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	jet, err := s.evaluateRepresented(u, v)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	a, err := representedPeriodicVector(jet.Du)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	b, err := representedPeriodicVector(jet.Dv)
	if err != nil {
		return Vec3{}, Vec3{}, err
	}
	return a, b, nil
}

func (s *ExactPeriodicSplineSurface) evaluateRepresented(u, v float64) (RationalPeriodicSurfaceJet, error) {
	if !finite(u) || !finite(v) {
		return RationalPeriodicSurfaceJet{}, ErrDomain
	}
	return s.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), SurfacePatchOptions{})
}

func representedPeriodicVector(v [3]*big.Rat) (Vec3, error) {
	x, _ := v[0].Float64()
	y, _ := v[1].Float64()
	z, _ := v[2].Float64()
	p := Vec3{x, y, z}
	if !p.IsFinite() {
		return Vec3{}, ErrUnrepresentable
	}
	return p, nil
}

var _ Curve = (*ExactPeriodicSplineCurve)(nil)
var _ Surface = (*ExactPeriodicSplineSurface)(nil)
