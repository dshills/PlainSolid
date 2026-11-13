package plaincadbridge

import (
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type planarThreadStock struct {
	frame          geom.Frame
	footprint      trimmed.ExtrusionRectangle
	height, radius float64
}

func rectangularThreadStock(p mixedProfileRequest) *planarThreadStock {
	if len(p.Outer) != 4 || len(p.Circles) != 1 {
		return nil
	}
	circle := p.Circles[0]
	center, err := mixedPoint(circle.Center)
	if err != nil {
		return nil
	}
	lo, hi := geom.Vec2{X: math.Inf(1), Y: math.Inf(1)}, geom.Vec2{X: math.Inf(-1), Y: math.Inf(-1)}
	var points [4]geom.Vec2
	for i, xy := range p.Outer {
		point, err := mixedPoint(xy)
		if err != nil {
			return nil
		}
		points[i] = point
		lo.X, lo.Y = min(lo.X, point.X), min(lo.Y, point.Y)
		hi.X, hi.Y = max(hi.X, point.X), max(hi.Y, point.Y)
	}
	seen := map[geom.Vec2]bool{}
	for _, point := range points {
		if seen[point] || point.X != lo.X && point.X != hi.X || point.Y != lo.Y && point.Y != hi.Y {
			return nil
		}
		seen[point] = true
	}
	frame, err := geom.NewFrame(p.Origin.vec(), p.XDirection.vec(), p.Normal.vec())
	if err != nil || !cardinalFrame(frame) {
		return nil
	}
	h := float64(p.Height)
	base := frame.ToWorld(geom.Vec3{X: center.X, Y: center.Y, Z: min(0, h)})
	if frame.ToLocal(base) != (geom.Vec3{X: center.X, Y: center.Y, Z: min(0, h)}) {
		return nil
	}
	f, err := geom.NewFrame(base, frame.XAxis(), frame.ZAxis())
	if err != nil {
		return nil
	}
	r := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: lo.X - center.X, Y: lo.Y - center.Y}, Max: geom.Vec2{X: hi.X - center.X, Y: hi.Y - center.Y}}
	for _, q := range [][2]geom.Vec2{{r.Min, lo}, {r.Max, hi}} {
		if (geom.Vec2{X: q[0].X + center.X, Y: q[0].Y + center.Y}) != q[1] {
			return nil
		}
	}
	return &planarThreadStock{f, r, math.Abs(h), float64(circle.Radius)}
}
