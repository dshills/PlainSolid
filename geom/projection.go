package geom

import "math"

// ProjectionStatus distinguishes a unique closest point from nonunique
// geometry and from a unique point with a singular angular parameterization.
type ProjectionStatus uint8

const (
	ProjectionUnique ProjectionStatus = iota
	ProjectionNonUnique
	ProjectionSingularParameters
)

// SurfaceProjection is an analytic closest-point result. Parameters follow the
// surface's Eval convention. Residual is the largest absolute dot product of
// (query-Point) with a normalized nonzero parametric tangent; it has length
// units. It is a numerical diagnostic, not a certified error bound.
// For nonunique results, Point is one deterministic representative.
type SurfaceProjection struct {
	U, V               float64
	Point              Vec3
	Distance, Residual float64
	Status             ProjectionStatus
}

func finishProjection(surface Surface, p Vec3, u, v float64, status ProjectionStatus) (SurfaceProjection, error) {
	q, e := surface.Eval(u, v)
	if e != nil {
		return SurfaceProjection{}, e
	}
	du, dv, e := surface.Partials(u, v)
	if e != nil {
		return SurfaceProjection{}, e
	}
	delta := p.Sub(q)
	distance := delta.Length()
	if !finite(distance) {
		return SurfaceProjection{}, ErrInvalid
	}
	residual := 0.0
	for _, tangent := range []Vec3{du, dv} {
		unit, e := tangent.Unit()
		if e == ErrDegenerate {
			continue
		}
		if e != nil {
			return SurfaceProjection{}, e
		}
		dot := delta.Dot(unit)
		if !finite(dot) {
			return SurfaceProjection{}, ErrInvalid
		}
		residual = math.Max(residual, math.Abs(dot))
	}
	return SurfaceProjection{u, v, q, distance, residual, status}, nil
}
func (s PlaneSurface) Project(p Vec3) (SurfaceProjection, error) {
	if !s.frame.valid() || !p.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	local := s.frame.ToLocal(p)
	if !local.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	return finishProjection(s, p, local.X, local.Y, ProjectionUnique)
}
func (s CylinderSurface) Project(p Vec3) (SurfaceProjection, error) {
	if !s.frame.valid() || !p.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	local := s.frame.ToLocal(p)
	if !local.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	u := math.Atan2(local.Y, local.X)
	status := ProjectionUnique
	if local.X == 0 && local.Y == 0 {
		u = 0
		status = ProjectionNonUnique
	}
	return finishProjection(s, p, u, local.Z, status)
}
func (s SphereSurface) Project(p Vec3) (SurfaceProjection, error) {
	if !s.frame.valid() || !p.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	local := s.frame.ToLocal(p)
	if !local.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	radial := math.Hypot(local.X, local.Y)
	u, v := math.Atan2(local.Y, local.X), math.Atan2(local.Z, radial)
	status := ProjectionUnique
	if radial == 0 {
		u = 0
		status = ProjectionSingularParameters
		if local.Z == 0 {
			v = 0
			status = ProjectionNonUnique
		}
	}
	return finishProjection(s, p, u, v, status)
}
func (s TorusSurface) Project(p Vec3) (SurfaceProjection, error) {
	if !s.frame.valid() || !p.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	local := s.frame.ToLocal(p)
	if !local.IsFinite() {
		return SurfaceProjection{}, ErrInvalid
	}
	radial := math.Hypot(local.X, local.Y)
	u := math.Atan2(local.Y, local.X)
	status := ProjectionUnique
	if radial == 0 {
		u = 0
		status = ProjectionNonUnique
	}
	v := math.Atan2(local.Z, radial-s.major)
	if radial == s.major && local.Z == 0 {
		v = 0
		status = ProjectionNonUnique
	}
	return finishProjection(s, p, u, v, status)
}
