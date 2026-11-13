package trimmed

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RevolveCircle sweeps a meridian disk centered at (major, axial) with the
// stated minor radius about frame Z. The disk stays strictly off the axis.
// Signed partial turns retain a torus support and two planar end disks;
// a full turn produces one closed torus. All circular edges remain analytic.
func RevolveCircle(ctx context.Context, frame geom.Frame, major, axial, minor, angle float64) (Solid, error) {
	if ctx == nil || !finite(major) || !finite(axial) || !finite(minor) || !finite(angle) ||
		minor <= 0 || major <= minor || angle == 0 || math.Abs(angle) > 2*math.Pi {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	lo := min(0, angle)
	x := frame.XAxis().Scale(math.Cos(lo)).Add(frame.YAxis().Scale(math.Sin(lo)))
	f, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: axial}), x, frame.ZAxis())
	if e != nil {
		return Solid{}, e
	}
	var s parametric.Solid
	if math.Abs(angle) == 2*math.Pi {
		s, e = parametric.Torus(f, major, minor)
	} else {
		s, e = parametric.TorusElbow(f, major, minor, math.Abs(angle))
	}
	if e != nil {
		return Solid{}, e
	}
	return FromParametric(s)
}
