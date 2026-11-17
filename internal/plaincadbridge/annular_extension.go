package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/geom"
)

func (b *Bridge) fuseAnnularUpperExtension(a, c stored) (Shape, bool, error) {
	if a.analytic == nil || c.analytic == nil {
		return Shape{}, false, nil
	}
	if a.analytic.annular == nil {
		a, c = c, a
	}
	if a.analytic.annular == nil || c.analytic.sector == nil {
		return Shape{}, false, nil
	}
	p, r := *a.analytic.annular, *c.analytic.sector
	axis, extensionAxis := p.Frame.ZAxis(), r.frame.ZAxis()
	if p.cut || p.extended || p.UpperChamfer != 0 || r.inner != p.InnerRadius || r.radius >= p.OuterRadius ||
		r.angle != 2*math.Pi || !cardinalFrame(p.Frame) || (extensionAxis != axis && extensionAxis != axis.Scale(-1)) {
		return Shape{}, false, nil
	}
	origin := r.frame.Origin()
	start := p.Frame.ToLocal(origin)
	if start.X != 0 || start.Y != 0 {
		return Shape{}, false, nil
	}
	height := r.height * extensionAxis.Dot(axis)
	end := start.Z + height
	lo, hi := min(start.Z, end), max(start.Z, end)
	if !exactAxisDistance(origin.Dot(axis), p.Frame.Origin().Dot(axis), start.Z) ||
		!exactAxisDistance(end, start.Z, height) ||
		!exactAxisDistance(p.Frame.ToWorld(geom.Vec3{Z: hi}).Dot(axis), p.Frame.Origin().Dot(axis), hi) {
		return Shape{}, true, geom.ErrUnrepresentable
	}
	if lo < p.LowerChamfer || lo >= p.Height || hi <= p.Height {
		return Shape{}, true, unsupportedEdge("annular extension must overlap the constant-radius flange and extend above its upper cap")
	}
	built, err := p.BuildUpperExtension(context.Background(), r.radius, hi)
	if err != nil {
		return Shape{}, true, err
	}
	p.lowerFace, p.upperFace, p.extended = built.LowerFace, built.ShoulderFace, true
	added := hi - p.Height
	volume := a.analytic.volume + math.Pi*(r.radius*r.radius-r.inner*r.inner)*added
	area := a.analytic.area + 2*math.Pi*(r.radius+r.inner)*added
	result, err := b.putAnalyticBody(&analyticBody{solid: built.Solid, annular: &p, volume: volume, area: area})
	return result, true, err
}
