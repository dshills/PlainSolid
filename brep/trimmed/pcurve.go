package trimmed

import (
	"math"

	"plainsolid/geom"
)

// EllipseUV is an angular arc Center+Cos*cos(a)+Sin*sin(a)+Linear*a.
// Leave Linear zero for an ellipse; a nonzero value describes the
// corresponding trim in an unwrapped cylinder chart.
// Angles can increase or decrease. A single turn, including its seam, is
// represented by [0, 2π]; negative angles are also allowed.
type EllipseUV struct {
	Center, Cos, Sin, Linear geom.Vec2
	From, To                 float64
}

// EllipseParameters returns the immutable represented coefficient data. Curve
// implementations with exact endpoint overrides may embed EllipseUV.
func (c EllipseUV) EllipseParameters() EllipseUV { return c }

func (c EllipseUV) EvalUV(t float64) (geom.Vec2, error) {
	a, e := c.angle(t)
	if e != nil {
		return geom.Vec2{}, e
	}
	s, k := math.Sincos(math.Remainder(a, 2*math.Pi))
	p := add2(add2(c.Center, add2(scale2(c.Cos, k), scale2(c.Sin, s))), scale2(c.Linear, a))
	if !uvFinite(p) {
		return geom.Vec2{}, geom.ErrInvalid
	}
	return p, nil
}

func (c EllipseUV) DerivativeUV(t float64) (geom.Vec2, error) {
	a, e := c.angle(t)
	if e != nil {
		return geom.Vec2{}, e
	}
	s, k := math.Sincos(math.Remainder(a, 2*math.Pi))
	d := scale2(add2(add2(scale2(c.Cos, -s), scale2(c.Sin, k)), c.Linear), c.To-c.From)
	if !uvFinite(d) {
		return geom.Vec2{}, geom.ErrInvalid
	}
	return d, nil
}

func (c EllipseUV) angle(t float64) (float64, error) {
	if !finite(t) || t < 0 || t > 1 || !finite(c.From) || !finite(c.To) || c.From == c.To ||
		math.Abs(c.To-c.From) > 2*math.Pi || !uvFinite(c.Center) || !uvFinite(c.Cos) || !uvFinite(c.Sin) || !uvFinite(c.Linear) {
		return 0, geom.ErrDomain
	}
	return (1-t)*c.From + t*c.To, nil
}

// QuadraticUV is a quadratic Bézier trim with explicit endpoints.
type QuadraticUV struct{ Start, Control, End geom.Vec2 }

func (c QuadraticUV) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 || !uvFinite(c.Start) || !uvFinite(c.Control) || !uvFinite(c.End) {
		return geom.Vec2{}, geom.ErrDomain
	}
	return mix(mix(c.Start, c.Control, t), mix(c.Control, c.End, t), t), nil
}

// SplineUV maps a rational spline in the XY plane into a parameter-space trim.
// The active subinterval is traversed from Min to Max, or in reverse.
type SplineUV struct {
	Curve    *geom.BSpline
	Range    UVInterval
	Reversed bool
}

// UVInterval is the active parameter range of a SplineUV.
type UVInterval struct{ Min, Max float64 }

func (c SplineUV) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 || c.Curve == nil || !finite(c.Range.Max-c.Range.Min) || c.Range.Min >= c.Range.Max {
		return geom.Vec2{}, geom.ErrDomain
	}
	if c.Reversed {
		t = 1 - t
	}
	u := (1-t)*c.Range.Min + t*c.Range.Max
	p, e := c.Curve.Eval(u)
	if e != nil {
		return geom.Vec2{}, e
	}
	if p.Z != 0 {
		return geom.Vec2{}, geom.ErrDomain
	}
	return geom.Vec2{X: p.X, Y: p.Y}, nil
}

// DerivativeUV returns the first derivative with respect to normalized t.
func (c QuadraticUV) DerivativeUV(t float64) (geom.Vec2, error) {
	if _, e := c.EvalUV(t); e != nil {
		return geom.Vec2{}, e
	}
	return geom.Vec2{
		X: 2 * ((1-t)*(c.Control.X-c.Start.X) + t*(c.End.X-c.Control.X)),
		Y: 2 * ((1-t)*(c.Control.Y-c.Start.Y) + t*(c.End.Y-c.Control.Y)),
	}, nil
}

func (c SplineUV) DerivativeUV(t float64) (geom.Vec2, error) {
	if _, e := c.EvalUV(t); e != nil {
		return geom.Vec2{}, e
	}
	h := c.Range.Max - c.Range.Min
	if c.Reversed {
		t = 1 - t
		h = -h
	}
	u := (1-t)*c.Range.Min + t*c.Range.Max
	v, e := c.Curve.Derivative(u)
	if e != nil {
		return geom.Vec2{}, e
	}
	v = v.Scale(h)
	if !v.IsFinite() || v.Z != 0 {
		return geom.Vec2{}, geom.ErrDomain
	}
	return geom.Vec2{X: v.X, Y: v.Y}, nil
}
