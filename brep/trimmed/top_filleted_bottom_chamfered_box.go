package trimmed

import (
	"context"
	"plainsolid/geom"
)

// TopFilletedBottomChamferedBox composes a complete upper perimeter fillet
// with a complete lower equal-setback chamfer. The two treated axial bands
// must be strictly separated. Vertical edges remain sharp. Cap faces retain
// indices zero (bottom) and one (top), matching TopFilletedBox.
func TopFilletedBottomChamferedBox(ctx context.Context, frame geom.Frame, width, depth, height, filletRadius, chamferSetback, tolerance float64) (Solid, error) {
	if !finite(chamferSetback) || !finite(tolerance) || chamferSetback <= 8*tolerance {
		return Solid{}, geom.ErrDomain
	}
	return topFilletedBox(ctx, frame, width, depth, height, filletRadius, chamferSetback, tolerance)
}
func (b *cornerCylinderBuilder) bottomChamferFace(a, c, p, q geom.Vec2, height float64) error {
	points := []geom.Vec3{{X: a.X, Y: a.Y}, {X: c.X, Y: c.Y}, {X: q.X, Y: q.Y, Z: height}, {X: p.X, Y: p.Y, Z: height}}
	origin := b.frame.ToWorld(points[0])
	x := b.frame.ToWorld(points[1]).Sub(origin)
	normal := x.Cross(b.frame.ToWorld(points[3]).Sub(origin))
	f, e := geom.NewFrame(origin, x, normal)
	if e != nil {
		return e
	}
	surface, e := geom.NewPlaneSurface(f)
	if e != nil {
		return e
	}
	uv := make([]geom.Vec2, len(points))
	for i, p := range points {
		local := f.ToLocal(b.frame.ToWorld(p))
		uv[i] = geom.Vec2{X: local.X, Y: local.Y}
	}
	return b.face(surface, points, uv, nil, false)
}
