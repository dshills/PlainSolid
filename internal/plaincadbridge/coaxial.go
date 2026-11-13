package plaincadbridge

import (
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type radialSector struct {
	frame                        geom.Frame
	inner, radius, height, angle float64
}

func (b *Bridge) putRadialSector(r radialSector) (Shape, error) {
	var p parametric.Solid
	var err error
	switch {
	case r.angle == 2*math.Pi:
		p, err = parametric.Tube(r.frame, r.radius, r.height, r.radius-r.inner)
	case r.inner == 0:
		p, err = parametric.CylinderSector(r.frame, r.radius, r.height, r.angle)
	default:
		p, err = parametric.AnnularSector(r.frame, r.inner, r.radius, r.height, r.angle)
	}
	if err != nil {
		return Shape{}, err
	}
	body, err := revolutionPatches(p)
	if err != nil {
		return Shape{}, err
	}
	body.sector = &r
	return b.putAnalyticBody(body)
}

// coaxialThrough proves one whole axial interval is removed. Positions are
// tested against the actual retained axis, including reversed through tools.
func coaxialThrough(a stored, c *cylinderBody) (radialSector, bool) {
	var r radialSector
	switch {
	case a.cylinder != nil:
		p := a.cylinder
		r = radialSector{frame: p.frame, radius: p.radius, height: p.height, angle: 2 * math.Pi}
	case a.analytic != nil && a.analytic.sector != nil:
		r = *a.analytic.sector
	default:
		return r, false
	}
	if c == nil || c.radius <= r.inner || c.radius >= r.radius {
		return r, false
	}
	z := r.frame.ZAxis()
	cz := c.frame.ZAxis()
	if z != cz && z != cz.Scale(-1) {
		return r, false
	}
	local := r.frame.ToLocal(c.frame.Origin())
	if local.X != 0 || local.Y != 0 || r.frame.ToWorld(local) != c.frame.Origin() {
		return r, false
	}
	end := local.Z + c.height
	if z != cz {
		end = local.Z - c.height
	}
	return r, min(local.Z, end) <= 0 && max(local.Z, end) >= r.height
}
