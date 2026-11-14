package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func islandPocketBounds(p trimmed.RectangularIslandPocket) (geom.Box, error) {
	return frameMap(p.Frame).ApplyBox(geom.Box{Min: geom.Vec3{X: p.OuterMin.X, Y: p.OuterMin.Y}, Max: geom.Vec3{X: p.OuterMax.X, Y: p.OuterMax.Y, Z: p.Height}})
}
func (b *Bridge) putRectangularIslandPocket(p trimmed.RectangularIslandPocket) (Shape, error) {
	solid, e := p.Solid(context.Background())
	if e != nil {
		return Shape{}, e
	}
	volume, e := p.NominalVolume()
	if e != nil {
		return Shape{}, e
	}
	area, e := p.SurfaceArea()
	if e != nil {
		return Shape{}, e
	}
	return b.putAnalyticBody(&analyticBody{solid: solid, volume: volume, area: area, islandPocket: &p})
}

// rectangularIslandPocket proves a finite rectangular-minus-disk tool reaches
// exactly one stock cap and leaves a genuine blind floor. Its physical entrance
// determines the canonical frame, independently of the tool's authored sign.
func (b *Bridge) rectangularIslandPocket(a, tool stored) (Shape, bool, error) {
	if !a.planar() || tool.analytic == nil || tool.analytic.threadStock == nil {
		return Shape{}, false, nil
	}
	cutter := tool.analytic.threadStock
	if !cardinalFrame(cutter.frame) {
		return Shape{}, false, nil
	}
	stock, ok := orthogonalPrism(a.solid, cutter.frame)
	if !ok {
		return Shape{}, false, nil
	}
	reject := func(message string) (Shape, bool, error) { return Shape{}, true, &failureError{"UNSUPPORTED", message} }
	p := trimmed.RectangularIslandPocket{OuterMin: geom.Vec2{X: stock.Min.X, Y: stock.Min.Y}, OuterMax: geom.Vec2{X: stock.Max.X, Y: stock.Max.Y}, PocketMin: cutter.footprint.Min, PocketMax: cutter.footprint.Max, IslandRadius: cutter.radius, Height: stock.Max.Z - stock.Min.Z}
	floor, entrance := 0., stock.Max.Z
	var e error
	switch {
	case stock.Min.Z < 0 && 0 < stock.Max.Z && cutter.height >= stock.Max.Z:
		p.Frame, e = geom.NewFrame(cutter.frame.ToWorld(geom.Vec3{Z: stock.Min.Z}), cutter.frame.XAxis(), cutter.frame.ZAxis())
		p.Depth = stock.Max.Z
	case 0 <= stock.Min.Z && stock.Min.Z < cutter.height && cutter.height < stock.Max.Z:
		p.Frame, e = geom.NewFrame(cutter.frame.ToWorld(geom.Vec3{Z: stock.Max.Z}), cutter.frame.XAxis(), cutter.frame.ZAxis().Scale(-1))
		p.OuterMin.Y, p.OuterMax.Y = -stock.Max.Y, -stock.Min.Y
		p.PocketMin.Y, p.PocketMax.Y = -cutter.footprint.Max.Y, -cutter.footprint.Min.Y
		p.Depth = cutter.height - stock.Min.Z
		floor, entrance = cutter.height, stock.Min.Z
	default:
		return reject("island pocket requires a finite tool entering one stock cap with its other endpoint strictly inside")
	}
	if e != nil {
		return Shape{}, true, e
	}
	if p.Frame.ToWorld(geom.Vec3{Z: p.Height - p.Depth}) != cutter.frame.ToWorld(geom.Vec3{Z: floor}) || p.Frame.ToWorld(geom.Vec3{Z: p.Height}) != cutter.frame.ToWorld(geom.Vec3{Z: entrance}) {
		return reject("island pocket cap and floor cannot be represented exactly")
	}
	volume, e := p.NominalVolume()
	if e != nil {
		return reject("island pocket footprint is not strictly inside the uncut stock")
	}
	before := a.result.GeometryAssertions.Volume
	if !finite(volume) || before-volume <= math.Max(1e-9, math.Abs(before)*1e-10) {
		return reject("island pocket must remove measurable material")
	}
	result, e := b.putRectangularIslandPocket(p)
	return result, true, e
}
