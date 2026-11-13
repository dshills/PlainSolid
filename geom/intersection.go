package geom

import "math"

// IntersectionKind identifies the geometry of an analytic intersection.
type IntersectionKind uint8

const (
	IntersectionNone IntersectionKind = iota
	IntersectionPoint
	IntersectionLine
	IntersectionCircle
	IntersectionCoincident
)

// IntersectionTolerance separates distance and angular tolerances. Angular is
// a bound on the absolute sine/cosine used for parallel tests, in [0,1).
// Zero requests exact float64 comparisons rather than a hidden global epsilon.
type IntersectionTolerance struct{ Linear, Angular float64 }

func (t IntersectionTolerance) valid() bool {
	return finite(t.Linear) && finite(t.Angular) && t.Linear >= 0 && t.Angular >= 0 && t.Angular < 1
}
func (l Line) Origin() Vec3    { return l.origin }
func (l Line) Direction() Vec3 { return l.direction }
func (l Line) valid() bool {
	return l.origin.IsFinite() && l.direction.IsFinite() && l.direction.Length() > 0
}
func (p Plane) valid() bool {
	return p.origin.IsFinite() && p.normal.IsFinite() && p.normal.Length() > 0
}

// Closest projects onto an infinite line, returning signed distance parameter.
func (l Line) Closest(p Vec3) (Vec3, float64, error) {
	if !l.valid() || !p.IsFinite() {
		return Vec3{}, 0, ErrInvalid
	}
	t := p.Sub(l.origin).Dot(l.direction)
	q, e := l.Eval(t)
	return q, t, e
}

// LinePlaneResult carries a point only for IntersectionPoint.
type LinePlaneResult struct {
	Kind      IntersectionKind
	Point     Vec3
	Parameter float64
}

func IntersectLinePlane(line Line, plane Plane, tolerance IntersectionTolerance) (LinePlaneResult, error) {
	r := LinePlaneResult{}
	if !line.valid() || !plane.valid() || !tolerance.valid() {
		return r, ErrInvalid
	}
	den := line.direction.Dot(plane.normal)
	distance := plane.SignedDistance(line.origin)
	if !finite(distance) {
		return r, ErrInvalid
	}
	if math.Abs(den) <= tolerance.Angular {
		if math.Abs(distance) <= tolerance.Linear {
			r.Kind = IntersectionCoincident
		}
		return r, nil
	}
	t := -distance / den
	p, e := line.Eval(t)
	if e != nil {
		return r, e
	}
	return LinePlaneResult{IntersectionPoint, p, t}, nil
}

// PlanePlaneResult carries a line only for IntersectionLine.
type PlanePlaneResult struct {
	Kind IntersectionKind
	Line Line
}

func IntersectPlanes(a, b Plane, tolerance IntersectionTolerance) (PlanePlaneResult, error) {
	r := PlanePlaneResult{}
	if !a.valid() || !b.valid() || !tolerance.valid() {
		return r, ErrInvalid
	}
	cross := a.normal.Cross(b.normal)
	sine := cross.Length()
	if sine <= tolerance.Angular {
		d := b.SignedDistance(a.origin)
		if !finite(d) {
			return r, ErrInvalid
		}
		if math.Abs(d) <= tolerance.Linear {
			r.Kind = IntersectionCoincident
		}
		return r, nil
	}
	q := b.normal.Sub(a.normal.Scale(a.normal.Dot(b.normal)))
	scale := -b.SignedDistance(a.origin) / q.Dot(q)
	point := a.origin.Add(q.Scale(scale))
	line, e := NewLine(point, cross)
	if e != nil {
		return r, e
	}
	return PlanePlaneResult{IntersectionLine, line}, nil
}

// Sphere is an immutable analytic sphere with strictly positive radius.
type Sphere struct {
	center Vec3
	radius float64
}

func NewSphere(center Vec3, radius float64) (Sphere, error) {
	if !center.IsFinite() || !finite(radius) || radius <= 0 {
		return Sphere{}, ErrInvalid
	}
	return Sphere{center, radius}, nil
}
func (s Sphere) Center() Vec3    { return s.center }
func (s Sphere) Radius() float64 { return s.radius }
func (s Sphere) valid() bool     { return s.center.IsFinite() && finite(s.radius) && s.radius > 0 }
func (s Sphere) SignedDistance(p Vec3) float64 {
	if !s.valid() || !p.IsFinite() {
		return math.NaN()
	}
	return p.Distance(s.center) - s.radius
}

// Closest returns ErrDegenerate at the center, where projection is not unique.
func (s Sphere) Closest(p Vec3) (Vec3, error) {
	if !s.valid() || !p.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	d, e := p.Sub(s.center).Unit()
	if e != nil {
		return Vec3{}, e
	}
	q := s.center.Add(d.Scale(s.radius))
	if !q.IsFinite() {
		return Vec3{}, ErrInvalid
	}
	return q, nil
}

// IntersectLineSphere returns zero, one (tangent), or two points sorted by line
// parameter. Within the linear tolerance, near tangency returns the closest
// line point; this point can differ from the sphere by at most that tolerance.
func IntersectLineSphere(line Line, sphere Sphere, tolerance float64) ([]Vec3, error) {
	if !line.valid() || !sphere.valid() || !finite(tolerance) || tolerance < 0 {
		return nil, ErrInvalid
	}
	closest, t, e := line.Closest(sphere.center)
	if e != nil {
		return nil, e
	}
	d := closest.Distance(sphere.center)
	if !finite(d) {
		return nil, ErrInvalid
	}
	if d-sphere.radius > tolerance {
		return nil, nil
	}
	if math.Abs(d-sphere.radius) <= tolerance {
		return []Vec3{closest}, nil
	}
	h := math.Sqrt(sphere.radius-d) * math.Sqrt(sphere.radius+d)
	if !finite(h) {
		return nil, ErrInvalid
	}
	a, e := line.Eval(t - h)
	if e != nil {
		return nil, e
	}
	b, e := line.Eval(t + h)
	if e != nil {
		return nil, e
	}
	return []Vec3{a, b}, nil
}

// SphereSphereResult carries either a tangent Point or a Circle. Coincident
// spheres are explicitly reported, avoiding an arbitrary intersection circle.
type SphereSphereResult struct {
	Kind   IntersectionKind
	Point  Vec3
	Circle Ellipse
}

func IntersectSpheres(a, b Sphere, tolerance float64) (SphereSphereResult, error) {
	r := SphereSphereResult{}
	if !a.valid() || !b.valid() || !finite(tolerance) || tolerance < 0 {
		return r, ErrInvalid
	}
	delta := b.center.Sub(a.center)
	d := delta.Length()
	if !finite(d) || !finite(a.radius+b.radius) {
		return r, ErrInvalid
	}
	if d <= tolerance {
		if math.Abs(a.radius-b.radius) <= tolerance {
			r.Kind = IntersectionCoincident
		}
		return r, nil
	}
	if d-(a.radius+b.radius) > tolerance || math.Abs(a.radius-b.radius)-d > tolerance {
		return r, nil
	}
	axis, _ := delta.Unit()
	x := .5 * (d + (a.radius-b.radius)*((a.radius+b.radius)/d))
	center := a.center.Add(axis.Scale(x))
	if !center.IsFinite() {
		return r, ErrInvalid
	}
	if math.Abs(d-(a.radius+b.radius)) <= tolerance || math.Abs(d-math.Abs(a.radius-b.radius)) <= tolerance {
		return SphereSphereResult{Kind: IntersectionPoint, Point: center}, nil
	}
	radius := math.Sqrt(a.radius-x) * math.Sqrt(a.radius+x)
	if !finite(radius) || radius <= 0 {
		return r, ErrDegenerate
	}
	circle, e := NewCircle(center, axis, radius)
	if e != nil {
		return r, e
	}
	return SphereSphereResult{Kind: IntersectionCircle, Circle: circle}, nil
}
