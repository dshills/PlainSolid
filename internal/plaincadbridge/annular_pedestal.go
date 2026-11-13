package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type pedestalStock struct {
	center       geom.Vec2
	inner, outer float64
	centerFace   int
}

func (b *Bridge) fuseAnnularPedestal(a, tool stored) (Shape, bool, error) {
	if a.analytic == nil || a.analytic.rounded == nil || tool.analytic == nil || tool.analytic.sector == nil {
		return Shape{}, false, nil
	}
	p, ring := *a.analytic.rounded, *tool.analytic.sector
	if p.height <= 0 || p.cut || p.pedestal != nil || ring.inner <= 0 || ring.angle != 2*math.Pi || !cardinalFrame(p.frame) {
		return Shape{}, false, nil
	}
	n := p.frame.ZAxis()
	if ring.frame.ZAxis() != n && ring.frame.ZAxis() != n.Scale(-1) {
		return Shape{}, false, nil
	}
	start := p.frame.ToLocal(ring.frame.Origin())
	end := start.Z + ring.height*ring.frame.ZAxis().Dot(n)
	lo, hi := min(start.Z, end), max(start.Z, end)
	if p.frame.ToWorld(start) != ring.frame.Origin() || lo < 0 || lo > p.height || hi <= p.height {
		return Shape{}, true, &failureError{"UNSUPPORTED", "annular pedestal must overlap or touch the upper rounded cap and extend above it"}
	}
	// The complete outer disk is proved strictly inside the upper cap.
	// Lower sections of this stock have the same or a wider footprint.
	frame, err := geom.NewFrame(p.frame.ToWorld(geom.Vec3{X: start.X, Y: start.Y}), p.frame.XAxis(), n)
	if err != nil {
		return Shape{}, true, err
	}
	solid, centerFace, err := trimmed.FuseAnnularPedestal(context.Background(), a.analytic.solid, 1, frame,
		ring.inner, ring.radius, p.height, hi)
	if err != nil {
		return Shape{}, true, err
	}
	p.pedestal = &pedestalStock{center: geom.Vec2{X: start.X, Y: start.Y}, inner: ring.inner, outer: ring.radius, centerFace: centerFace}
	height := hi - p.height
	volume := a.analytic.volume + math.Pi*(ring.radius*ring.radius-ring.inner*ring.inner)*height
	area := a.analytic.area + 2*math.Pi*(ring.radius+ring.inner)*height
	shape, err := b.putAnalyticBody(&analyticBody{solid: solid, rounded: &p, volume: volume, area: area})
	return shape, true, err
}
