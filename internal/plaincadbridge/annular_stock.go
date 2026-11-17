package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type annularStock struct {
	trimmed.ChamferedAnnularPrism
	lowerFace, upperFace int
	cut, extended        bool
}

func (b *Bridge) chamferAnnularStock(s stored, q request) (Shape, error) {
	if s.analytic == nil || s.analytic.sector == nil || q.Origin == nil || q.Normal == nil || q.Distance <= 0 || len(q.Edges) != 0 {
		return Shape{}, unsupportedEdge("annular chamfer requires a complete original planar cap")
	}
	stock := s.analytic.sector
	if stock.inner <= 0 || stock.angle != 2*math.Pi || !cardinalFrame(stock.frame) {
		return Shape{}, unsupportedEdge("annular chamfer requires an unmodified full circular annulus")
	}
	p := annularStock{ChamferedAnnularPrism: trimmed.ChamferedAnnularPrism{Frame: stock.frame, OuterRadius: stock.radius,
		InnerRadius: stock.inner, Height: stock.height}}
	n, origin := q.Normal.vec(), q.Origin.vec()
	axis := stock.frame.ZAxis()
	switch {
	case n == axis.Scale(-1) && exactDotDifference(origin, stock.frame.Origin(), axis) == 0:
		p.LowerChamfer = float64(q.Distance)
	case n == axis && exactDotDifference(origin, stock.frame.ToWorld(geom.Vec3{Z: stock.height}), axis) == 0:
		p.UpperChamfer = float64(q.Distance)
	default:
		return Shape{}, unsupportedEdge("annular cap was lost or does not match the original support")
	}
	built, err := p.Build(context.Background())
	if err != nil {
		return Shape{}, err
	}
	volume, err := p.NominalVolume()
	if err != nil {
		return Shape{}, err
	}
	area, err := p.SurfaceArea()
	if err != nil {
		return Shape{}, err
	}
	p.lowerFace, p.upperFace = built.LowerFace, built.UpperFace
	return b.putAnalyticBody(&analyticBody{solid: built.Solid, annular: &p, volume: volume, area: area})
}
