package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type roundedStock struct {
	frame    geom.Frame
	profile  trimmed.RoundedRectangle
	height   float64
	fillet   float64
	chamfer  float64
	stack    *roundedStack
	cut      bool
	pedestal *pedestalStock
}

func (p *roundedStock) isPlainExtrusion() bool {
	return p.fillet == 0 && p.chamfer == 0 && p.stack == nil && !p.cut && p.pedestal == nil
}

func (b *Bridge) filletRoundedStock(s stored, q request) (Shape, error) {
	p := *s.analytic.rounded
	if p.cut || p.pedestal != nil || p.stack != nil || p.chamfer != 0 || p.fillet != 0 || p.height <= 0 || q.Normal == nil || q.Origin == nil || q.Radius <= 0 || len(q.Edges) != 0 ||
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

func (b *Bridge) chamferRoundedStock(s stored, q request) (Shape, error) {
	p := *s.analytic.rounded
	if p.cut || p.pedestal != nil || p.stack != nil || p.chamfer != 0 ||
		q.Normal == nil || q.Origin == nil || q.Distance <= 0 || len(q.Edges) != 0 ||
		q.Normal.vec() != p.frame.ZAxis().Scale(-1) || exactDotDifference(q.Origin.vec(), p.frame.Origin(), p.frame.ZAxis()) != 0 {
		return Shape{}, unsupportedEdge("rounded chamfer requires the original lower cap")
	}
	c := float64(q.Distance)
	ctx := context.Background()
	tol := s.analytic.solid.Tolerance()
	var solid trimmed.Solid
	var err error
	if p.fillet > 0 {
		solid, err = trimmed.TopFilletedBottomChamferedRoundedRectangle(ctx, p.frame, p.profile, p.height, p.fillet, c, tol)
	} else {
		solid, err = trimmed.BottomChamferedRoundedRectangle(ctx, p.frame, p.profile, p.height, c, tol)
	}
	if err != nil {
		return Shape{}, err
	}
	w, d := p.profile.Max.X-p.profile.Min.X, p.profile.Max.Y-p.profile.Min.Y
	perimeter := 2*(w+d) + (2*math.Pi-8)*p.profile.Radii[0]
	p.chamfer = c
	removed := perimeter*c*c/2 - math.Pi*c*c*c/3
	areaChange := perimeter*c*(math.Sqrt2-2) + math.Pi*c*c*(1-math.Sqrt2)
	return b.putAnalyticBody(&analyticBody{solid: solid, rounded: &p, volume: s.analytic.volume - removed, area: s.analytic.area + areaChange})
}
