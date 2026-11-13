package plaincadbridge

import (
	"context"
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (b *Bridge) revolve(q request) (Shape, error) {
	if q.Origin == nil || q.Direction == nil {
		return Shape{}, errors.New("revolve requires a resolved axis")
	}
	angle := float64(q.Angle)
	if !finite(angle) || angle == 0 || math.Abs(angle) > 2*math.Pi {
		return Shape{}, errors.New("revolve angle must be nonzero and at most 360 degrees")
	}
	if len(q.Holes) != 0 {
		return Shape{}, &failureError{"UNSUPPORTED", "revolve requires a single meridian loop"}
	}
	if q.Op == "revolveCircle" {
		return b.revolveCircle(q, angle)
	}
	if len(q.Outer) > MaxProfileVertices {
		return Shape{}, &failureError{"LIMIT", "revolve profile exceeds 128 vertices"}
	}
	ctx := context.Background()
	p, err := trimmed.PreparePlanarPolygonRevolution(ctx, loop(q.Outer), q.Origin.vec(), q.Direction.vec())
	if err != nil {
		return Shape{}, err
	}
	frame, profile := p.Frame, p.Meridian
	if radius, lo, hi, ok := cylinderMeridian(profile); ok {
		if math.Abs(angle) == 2*math.Pi {
			return b.circle(frame.ToWorld(geom.Vec3{Z: lo}), frame.ZAxis(), radius, hi-lo)
		}
		f, err := sweepStart(frame, lo, angle)
		if err != nil {
			return Shape{}, err
		}
		return b.putRadialSector(radialSector{frame: f, radius: radius, height: hi - lo, angle: math.Abs(angle)})
	}
	if math.Abs(angle) == 2*math.Pi {
		if shaft, ok := steppedMeridian(frame, profile); ok {
			return b.putSteppedShaft(shaft)
		}
	}
	s, err := trimmed.RevolvePolygon(ctx, frame, profile, angle)
	if err != nil {
		return Shape{}, err
	}
	pMass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil {
		return Shape{}, err
	}
	return b.putAnalytic(s, pMass.Volume, pMass.Area)
}

func (b *Bridge) revolveCircle(q request, angle float64) (Shape, error) {
	if q.Center == nil || q.Normal == nil || len(q.Outer) != 0 {
		return Shape{}, errors.New("circular revolve requires one meridian disk and its plane normal")
	}
	axis, err := q.Direction.vec().Unit()
	if err != nil {
		return Shape{}, err
	}
	origin, center, normal := q.Origin.vec(), q.Center.vec(), q.Normal.vec()
	if normal == (geom.Vec3{}) || exactDotDifference(axis, geom.Vec3{}, normal) != 0 || exactDotDifference(center, origin, normal) != 0 {
		return Shape{}, errors.New("revolve axis must lie in the disk plane")
	}
	delta := center.Sub(origin)
	axial := delta.Dot(axis)
	radial := delta.Sub(axis.Scale(axial))
	major, minor := radial.Length(), float64(q.Radius)
	if !finite(major) || !finite(minor) || minor <= 0 || major <= minor {
		return Shape{}, errors.New("revolve disk must not cross or touch the selected axis")
	}
	f, err := geom.NewFrame(origin, radial, axis)
	if err != nil {
		return Shape{}, err
	}
	if !cardinalFrame(f) || f.ToWorld(geom.Vec3{X: major, Z: axial}) != center {
		return Shape{}, &failureError{"UNSUPPORTED", "circular revolution requires an exactly represented coordinate meridian"}
	}
	f, err = sweepStart(f, axial, angle)
	if err != nil {
		return Shape{}, err
	}
	var s parametric.Solid
	if math.Abs(angle) == 2*math.Pi {
		s, err = parametric.Torus(f, major, minor)
	} else {
		s, err = parametric.TorusElbow(f, major, minor, math.Abs(angle))
	}
	if err != nil {
		return Shape{}, err
	}
	body, err := revolutionPatches(s)
	if err != nil {
		return Shape{}, err
	}
	return b.putAnalyticBody(body)
}

func revolutionPatches(p parametric.Solid) (*analyticBody, error) {
	s, err := trimmed.FromParametric(p)
	if err != nil {
		return nil, err
	}
	mass, err := p.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if err != nil {
		return nil, err
	}
	return &analyticBody{solid: s, patches: &p, volume: mass.Volume, area: mass.Area}, nil
}

func sweepStart(f geom.Frame, axial, angle float64) (geom.Frame, error) {
	u := min(0, angle)
	x := f.XAxis().Scale(math.Cos(u)).Add(f.YAxis().Scale(math.Sin(u)))
	return geom.NewFrame(f.ToWorld(geom.Vec3{Z: axial}), x, f.ZAxis())
}

func cylinderMeridian(points []geom.Vec2) (radius, lo, hi float64, ok bool) {
	if len(points) != 4 {
		return
	}
	lo, hi = points[0].Y, points[0].Y
	for _, p := range points {
		radius = max(radius, p.X)
		lo = min(lo, p.Y)
		hi = max(hi, p.Y)
	}
	if radius <= 0 || hi <= lo {
		return
	}
	seen := map[geom.Vec2]bool{}
	for _, p := range points {
		if seen[p] || p.X != 0 && p.X != radius || p.Y != lo && p.Y != hi {
			return
		}
		seen[p] = true
	}
	for i, p := range points {
		q := points[(i+1)%4]
		if p.X != q.X && p.Y != q.Y {
			return
		}
	}
	return radius, lo, hi, true
}
