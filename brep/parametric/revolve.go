package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// Revolve turns a simple closed meridian polygon about the frame's Z axis.
// Each Vec2 contains a nonnegative radius (X) and an axial coordinate (Y).
// Closing is implicit. Either winding is accepted. An axis-contacting profile
// must have exactly one side on the axis; other profiles must stay strictly
// off-axis. The result retains planar, cylindrical and conical patches exactly.
func Revolve(frame geom.Frame, profile []geom.Vec2) (Solid, error) {
	p, scale, e := PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return Solid{}, e
	}
	vertices := make([]geom.Vec3, len(p))
	edges := make([]Edge, len(p))
	for i, a := range p {
		vertices[i] = frame.ToWorld(geom.Vec3{X: a.X, Z: a.Y})
		edges[i] = Edge{Start: i, End: i, Range: normalized}
		if a.X > 0 {
			edges[i].Curve = iso{Face{Surface: revolvedSide{frame, a, a}, U: circleDomain, V: normalized}, 0}
		}
	}
	faces := make([]Face, 0, len(p))
	for i, a := range p {
		j := (i + 1) % len(p)
		b := p[j]
		if a.X == 0 && b.X == 0 {
			continue
		}
		seam := len(edges)
		f := Face{Surface: revolvedSide{frame, a, b}, U: circleDomain, V: normalized, Boundary: [4]Use{{i, false}, {seam, false}, {j, true}, {seam, true}}}
		faces = append(faces, f)
		edges = append(edges, Edge{Start: i, End: j, Range: normalized, Curve: iso{f, 1}})
	}
	return NewSolid(vertices, edges, faces, scale*1e-12)
}

// PrepareRevolutionMeridian validates and copies a simple (radius, axial)
// polygon, returning it counterclockwise with a characteristic model scale.
// An axis contact must be precisely one polygon side.
func PrepareRevolutionMeridian(frame geom.Frame, profile []geom.Vec2) ([]geom.Vec2, float64, error) {
	if len(profile) < 3 || len(profile) > 1024 || frame.ZAxis().Length() == 0 {
		return nil, 0, fmt.Errorf("parametric: invalid revolution frame or profile size")
	}
	p := append([]geom.Vec2(nil), profile...)
	axis := []int{}
	scale := 0.
	for i, a := range p {
		if a.X < 0 || !finite(a.X) || !finite(a.Y) {
			return nil, 0, fmt.Errorf("parametric: invalid meridian point %d", i)
		}
		if a.X == 0 {
			axis = append(axis, i)
		}
		scale = math.Max(scale, math.Max(a.X, math.Abs(a.Y-p[0].Y)))
	}
	if len(axis) != 0 && (len(axis) != 2 || axis[1]-axis[0] != 1 && axis[1]-axis[0] != len(p)-1) {
		return nil, 0, fmt.Errorf("parametric: expected one contiguous axis side")
	}
	if scale <= 0 || !finite(scale) || scale*1e-12 == 0 {
		return nil, 0, fmt.Errorf("parametric: unsupported meridian scale")
	}
	// In a simple polygon, the turn at the lexicographically smallest
	// noncollinear vertex determines the winding. Reject straight consecutive
	// segments, zero edges and non-adjacent intersections before using it.
	left := 0
	for i, a := range p {
		b, c := p[(i+len(p)-1)%len(p)], p[(i+1)%len(p)]
		sign, _ := geom.Orient2D(b, a, c)
		if sign == 0 {
			return nil, 0, fmt.Errorf("parametric: degenerate meridian corner %d", i)
		}
		if a.X < p[left].X || a.X == p[left].X && a.Y < p[left].Y {
			left = i
		}
	}
	for i, a := range p {
		for j := i + 2; j < len(p); j++ {
			if i == 0 && j == len(p)-1 {
				continue
			}
			if meridianIntersects(a, p[i+1], p[j], p[(j+1)%len(p)]) {
				return nil, 0, fmt.Errorf("parametric: meridian sides %d and %d intersect", i, j)
			}
		}
	}
	winding, _ := geom.Orient2D(p[(left+len(p)-1)%len(p)], p[left], p[(left+1)%len(p)])
	if winding < 0 {
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
		}
	}
	return p, scale, nil
}

// RevolutionSide constructs the analytic sweep of one non-axis meridian
// segment. U is angle in radians and V runs from a to b over [0,1].
// The result participates in native persistence and analytic bounds.
func RevolutionSide(frame geom.Frame, a, b geom.Vec2) (geom.Surface, error) {
	if frame.ZAxis().Length() == 0 || a == b || a.X < 0 || b.X < 0 || a.X == 0 && b.X == 0 ||
		!finite(a.X) || !finite(a.Y) || !finite(b.X) || !finite(b.Y) {
		return nil, geom.ErrInvalid
	}
	return revolvedSide{frame, a, b}, nil
}
func meridianIntersects(a, b, c, d geom.Vec2) bool {
	if math.Max(a.X, b.X) < math.Min(c.X, d.X) || math.Max(c.X, d.X) < math.Min(a.X, b.X) || math.Max(a.Y, b.Y) < math.Min(c.Y, d.Y) || math.Max(c.Y, d.Y) < math.Min(a.Y, b.Y) {
		return false
	}
	s, _ := geom.Orient2D(a, b, c)
	t, _ := geom.Orient2D(a, b, d)
	u, _ := geom.Orient2D(c, d, a)
	v, _ := geom.Orient2D(c, d, b)
	on := func(a, b, p geom.Vec2) bool {
		return p.X >= math.Min(a.X, b.X) && p.X <= math.Max(a.X, b.X) && p.Y >= math.Min(a.Y, b.Y) && p.Y <= math.Max(a.Y, b.Y)
	}
	return s*t < 0 && u*v < 0 || s == 0 && on(a, b, c) || t == 0 && on(a, b, d) || u == 0 && on(c, d, a) || v == 0 && on(c, d, b)
}

type revolvedSide struct {
	frame geom.Frame
	a, b  geom.Vec2
}

func (s revolvedSide) Eval(u, v float64) (geom.Vec3, error) {
	if !finite(u) || !finite(v) || v < 0 || v > 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	r := (1-v)*s.a.X + v*s.b.X
	z := (1-v)*s.a.Y + v*s.b.Y
	p := s.frame.ToWorld(geom.Vec3{X: r * math.Cos(u), Y: r * math.Sin(u), Z: z})
	if !p.IsFinite() {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return p, nil
}
func (s revolvedSide) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	if _, err := s.Eval(u, v); err != nil {
		return geom.Vec3{}, geom.Vec3{}, err
	}
	r := (1-v)*s.a.X + v*s.b.X
	dr, dz := s.b.X-s.a.X, s.b.Y-s.a.Y
	c, si := math.Cos(u), math.Sin(u)
	return s.frame.XAxis().Scale(-r * si).Add(s.frame.YAxis().Scale(r * c)), s.frame.XAxis().Scale(dr * c).Add(s.frame.YAxis().Scale(dr * si)).Add(s.frame.ZAxis().Scale(dz)), nil
}
