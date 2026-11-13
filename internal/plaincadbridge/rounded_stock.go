package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type roundedStock struct {
	frame   geom.Frame
	profile trimmed.RoundedRectangle
	height  float64
	fillet  float64
	cut     bool
}

func (b *Bridge) filletRoundedStock(s stored, q request) (Shape, error) {
	p := *s.analytic.rounded
	if p.cut || p.fillet != 0 || p.height <= 0 || q.Normal == nil || q.Origin == nil || q.Radius <= 0 || len(q.Edges) != 0 ||
		q.Normal.vec() != p.frame.ZAxis() || exactDotDifference(q.Origin.vec(), p.frame.ToWorld(geom.Vec3{Z: p.height}), p.frame.ZAxis()) != 0 {
		return Shape{}, unsupportedEdge("rounded perimeter requires one original upper cap")
	}
	radius := float64(q.Radius)
	solid, err := trimmed.TopFilletedRoundedRectangle(context.Background(), p.frame, p.profile, p.height, radius, s.analytic.solid.Tolerance())
	if err != nil {
		return Shape{}, err
	}
	width, depth := p.profile.Max.X-p.profile.Min.X, p.profile.Max.Y-p.profile.Min.Y
	perimeter := 2*(width+depth) + (2*math.Pi-8)*p.profile.Radii[0]
	removed := perimeter*radius*radius*(1-math.Pi/4) - math.Pi*radius*radius*radius*(5./3-math.Pi/2)
	// Replace the original top perimeter strip by the inset cap and the
	// swept quarter-circle blend over its straight and circular supports.
	areaChange := perimeter*radius*(math.Pi/2-2) + math.Pi*radius*radius*(3-math.Pi)
	p.fillet = radius
	return b.putAnalyticBody(&analyticBody{solid: solid, rounded: &p, volume: s.analytic.volume - removed, area: s.analytic.area + areaChange})
}
