package geom

import "math"

// Curve evaluates a parametric curve and its first derivative.
type Curve interface {
	Eval(float64) (Vec3, error)
	Derivative(float64) (Vec3, error)
}

// Surface evaluates a parametric surface and its first partial derivatives.
type Surface interface {
	Eval(float64, float64) (Vec3, error)
	Partials(float64, float64) (Vec3, Vec3, error)
}

// Frame is an immutable right-handed orthonormal coordinate system.
type Frame struct{ origin, x, y, z Vec3 }

// NewFrame normalizes z and projects xHint perpendicular to it.
func NewFrame(origin, xHint, z Vec3) (Frame, error) {
	if !origin.IsFinite() {
		return Frame{}, ErrInvalid
	}
	n, e := z.Unit()
	if e != nil {
		return Frame{}, e
	}
	x, e := xHint.Unit()
	if e != nil {
		return Frame{}, e
	}
	y, e := n.Cross(x).Unit()
	if e != nil {
		return Frame{}, e
	}
	x = y.Cross(n)
	return Frame{origin, x, y, n}, nil
}

// FrameFromNormal picks a stable x direction, preferring the world x axis.
func FrameFromNormal(origin, normal Vec3) (Frame, error) {
	n, e := normal.Unit()
	if e != nil {
		return Frame{}, e
	}
	hint := Vec3{1, 0, 0}
	if math.Abs(n.X) > .9 {
		hint = Vec3{0, 1, 0}
	}
	return NewFrame(origin, hint, n)
}
func (f Frame) Origin() Vec3 { return f.origin }
func (f Frame) XAxis() Vec3  { return f.x }
func (f Frame) YAxis() Vec3  { return f.y }
func (f Frame) ZAxis() Vec3  { return f.z }
func (f Frame) ToWorld(p Vec3) Vec3 {
	return f.origin.Add(f.x.Scale(p.X)).Add(f.y.Scale(p.Y)).Add(f.z.Scale(p.Z))
}
func (f Frame) ToLocal(p Vec3) Vec3 {
	d := p.Sub(f.origin)
	return Vec3{d.Dot(f.x), d.Dot(f.y), d.Dot(f.z)}
}
func (f Frame) valid() bool {
	return f.origin.IsFinite() && f.x.Length() > 0 && f.y.Length() > 0 && f.z.Length() > 0
}

// Line uses distance as its parameter along a unit direction.
type Line struct{ origin, direction Vec3 }

func NewLine(origin, direction Vec3) (Line, error) {
	if !origin.IsFinite() {
		return Line{}, ErrInvalid
	}
	d, e := direction.Unit()
	return Line{origin, d}, e
}
func (l Line) Eval(t float64) (Vec3, error) {
	if !finite(t) || l.direction.Length() == 0 {
		return Vec3{}, ErrInvalid
	}
	p := l.origin.Add(l.direction.Scale(t))
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func (l Line) Derivative(t float64) (Vec3, error) {
	if !finite(t) || l.direction.Length() == 0 {
		return Vec3{}, ErrInvalid
	}
	return l.direction, nil
}

// Ellipse uses an angle in radians; evaluation is periodic for every finite t.
// A circle is an ellipse with equal radii.
type Ellipse struct {
	frame  Frame
	rx, ry float64
}

func NewEllipse(frame Frame, rx, ry float64) (Ellipse, error) {
	if !frame.valid() || !finite(rx) || !finite(ry) || rx <= 0 || ry <= 0 {
		return Ellipse{}, ErrInvalid
	}
	return Ellipse{frame, rx, ry}, nil
}
func NewCircle(center, normal Vec3, radius float64) (Ellipse, error) {
	f, e := FrameFromNormal(center, normal)
	if e != nil {
		return Ellipse{}, e
	}
	return NewEllipse(f, radius, radius)
}
func (e Ellipse) Eval(t float64) (Vec3, error) {
	if !finite(t) || !e.frame.valid() {
		return Vec3{}, ErrInvalid
	}
	p := e.frame.ToWorld(Vec3{e.rx * math.Cos(t), e.ry * math.Sin(t), 0})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func (e Ellipse) Derivative(t float64) (Vec3, error) {
	if !finite(t) || !e.frame.valid() {
		return Vec3{}, ErrInvalid
	}
	p := e.frame.x.Scale(-e.rx * math.Sin(t)).Add(e.frame.y.Scale(e.ry * math.Cos(t)))
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}

// RationalBSpline returns an exact rational representation of the full ellipse,
// with domain [0,4]. Its parameter is not the ellipse's angular parameter.
func (e Ellipse) RationalBSpline() (*BSpline, error) {
	if !e.frame.valid() {
		return nil, ErrInvalid
	}
	local := []Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}, {-1, 1, 0}, {-1, 0, 0}, {-1, -1, 0}, {0, -1, 0}, {1, -1, 0}, {1, 0, 0}}
	for i, p := range local {
		local[i] = e.frame.ToWorld(Vec3{p.X * e.rx, p.Y * e.ry, 0})
	}
	w := math.Sqrt(.5)
	return NewBSpline(2, local, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{1, w, 1, w, 1, w, 1, w, 1})
}

// PlaneSurface uses distances along the frame's x and y axes as parameters.
type PlaneSurface struct{ frame Frame }

func NewPlaneSurface(frame Frame) (PlaneSurface, error) {
	if !frame.valid() {
		return PlaneSurface{}, ErrInvalid
	}
	return PlaneSurface{frame}, nil
}
func (s PlaneSurface) Eval(u, v float64) (Vec3, error) {
	if !s.frame.valid() || !finite(u) || !finite(v) {
		return Vec3{}, ErrInvalid
	}
	p := s.frame.ToWorld(Vec3{u, v, 0})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func (s PlaneSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return Vec3{}, Vec3{}, e
	}
	return s.frame.x, s.frame.y, nil
}

// CylinderSurface uses angle u and signed axial distance v.
type CylinderSurface struct {
	frame  Frame
	radius float64
}

func NewCylinderSurface(frame Frame, radius float64) (CylinderSurface, error) {
	if !frame.valid() || !finite(radius) || radius <= 0 {
		return CylinderSurface{}, ErrInvalid
	}
	return CylinderSurface{frame, radius}, nil
}
func (s CylinderSurface) Eval(u, v float64) (Vec3, error) {
	if !s.frame.valid() || !finite(u) || !finite(v) {
		return Vec3{}, ErrInvalid
	}
	p := s.frame.ToWorld(Vec3{s.radius * math.Cos(u), s.radius * math.Sin(u), v})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func (s CylinderSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return Vec3{}, Vec3{}, e
	}
	du := s.frame.x.Scale(-s.radius * math.Sin(u)).Add(s.frame.y.Scale(s.radius * math.Cos(u)))
	return du, s.frame.z, nil
}

// ConeSurface uses angle u and axial distance v measured from the apex.
// Positive v is required; the apex is accepted, with a degenerate u derivative.
type ConeSurface struct {
	frame Frame
	slope float64
}

func NewConeSurface(frame Frame, halfAngle float64) (ConeSurface, error) {
	if !frame.valid() || !finite(halfAngle) || halfAngle <= 0 || halfAngle >= math.Pi/2 {
		return ConeSurface{}, ErrInvalid
	}
	return NewConeSurfaceSlope(frame, math.Tan(halfAngle))
}

// NewConeSurfaceSlope specifies radial growth per unit axial distance directly.
// It avoids the rounding incurred by converting an exactly representable slope
// through an angle, which matters when constructing exact generator lines.
func NewConeSurfaceSlope(frame Frame, slope float64) (ConeSurface, error) {
	if !frame.valid() || !finite(slope) || slope <= 0 {
		return ConeSurface{}, ErrInvalid
	}
	return ConeSurface{frame, slope}, nil
}
func (s ConeSurface) Eval(u, v float64) (Vec3, error) {
	if !s.frame.valid() || !finite(u) || !finite(v) || v < 0 {
		return Vec3{}, ErrDomain
	}
	r := v * s.slope
	p := s.frame.ToWorld(Vec3{r * math.Cos(u), r * math.Sin(u), v})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func (s ConeSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return Vec3{}, Vec3{}, e
	}
	du := s.frame.x.Scale(-v * s.slope * math.Sin(u)).Add(s.frame.y.Scale(v * s.slope * math.Cos(u)))
	dv := s.frame.x.Scale(s.slope * math.Cos(u)).Add(s.frame.y.Scale(s.slope * math.Sin(u))).Add(s.frame.z)
	return du, dv, nil
}

// SphereSurface uses longitude u and latitude v in [-pi/2,pi/2].
// The longitude derivative vanishes at the poles.
type SphereSurface struct {
	frame  Frame
	radius float64
}

func NewSphereSurface(frame Frame, radius float64) (SphereSurface, error) {
	if !frame.valid() || !finite(radius) || radius <= 0 {
		return SphereSurface{}, ErrInvalid
	}
	return SphereSurface{frame, radius}, nil
}
func (s SphereSurface) Eval(u, v float64) (Vec3, error) {
	if !s.frame.valid() || !finite(u) || !finite(v) || math.Abs(v) > math.Pi/2 {
		return Vec3{}, ErrDomain
	}
	p := s.frame.ToWorld(Vec3{s.radius * math.Cos(v) * math.Cos(u), s.radius * math.Cos(v) * math.Sin(u), s.radius * math.Sin(v)})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func (s SphereSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return Vec3{}, Vec3{}, e
	}
	r := s.radius
	du := s.frame.x.Scale(-r * math.Cos(v) * math.Sin(u)).Add(s.frame.y.Scale(r * math.Cos(v) * math.Cos(u)))
	dv := s.frame.x.Scale(-r * math.Sin(v) * math.Cos(u)).Add(s.frame.y.Scale(-r * math.Sin(v) * math.Sin(u))).Add(s.frame.z.Scale(r * math.Cos(v)))
	return du, dv, nil
}

// TorusSurface is a ring torus (major radius > minor radius > 0).
// u follows the major circle and v follows the tube circle, both in radians.
type TorusSurface struct {
	frame        Frame
	major, minor float64
}

func NewTorusSurface(frame Frame, major, minor float64) (TorusSurface, error) {
	if !frame.valid() || !finite(major) || !finite(minor) || minor <= 0 || major <= minor || !finite(major+minor) {
		return TorusSurface{}, ErrInvalid
	}
	return TorusSurface{frame, major, minor}, nil
}
func (s TorusSurface) Eval(u, v float64) (Vec3, error) {
	if !s.frame.valid() || !finite(u) || !finite(v) {
		return Vec3{}, ErrInvalid
	}
	r := s.major + s.minor*math.Cos(v)
	p := s.frame.ToWorld(Vec3{r * math.Cos(u), r * math.Sin(u), s.minor * math.Sin(v)})
	if !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return p, nil
}
func (s TorusSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if _, e := s.Eval(u, v); e != nil {
		return Vec3{}, Vec3{}, e
	}
	r := s.major + s.minor*math.Cos(v)
	du := s.frame.x.Scale(-r * math.Sin(u)).Add(s.frame.y.Scale(r * math.Cos(u)))
	dv := s.frame.x.Scale(-s.minor * math.Sin(v) * math.Cos(u)).Add(s.frame.y.Scale(-s.minor * math.Sin(v) * math.Sin(u))).Add(s.frame.z.Scale(s.minor * math.Cos(v)))
	return du, dv, nil
}
