package plaincadbridge

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func circularStockBoreRecipe(stock, tool *cylinderBody) (step.DiskHoleExtrusionBody, float64, float64, bool) {
	if stock == nil || tool == nil || !cardinalFrame(stock.frame) {
		return step.DiskHoleExtrusionBody{}, 0, 0, false
	}
	axis, drill := stock.frame.ZAxis(), tool.frame.ZAxis()
	if drill != axis && drill != axis.Scale(-1) {
		return step.DiskHoleExtrusionBody{}, 0, 0, false
	}
	start := stock.frame.ToLocal(tool.frame.Origin())
	if stock.frame.ToWorld(start) != tool.frame.Origin() ||
		!exactAxisDistance(tool.frame.Origin().Dot(stock.frame.XAxis()), stock.frame.Origin().Dot(stock.frame.XAxis()), start.X) ||
		!exactAxisDistance(tool.frame.Origin().Dot(stock.frame.YAxis()), stock.frame.Origin().Dot(stock.frame.YAxis()), start.Y) ||
		!exactAxisDistance(tool.frame.Origin().Dot(axis), stock.frame.Origin().Dot(axis), start.Z) {
		return step.DiskHoleExtrusionBody{}, 0, 0, false
	}
	// Check that both local and world axial translations preserve the same
	// represented endpoint before clipping the finite cylindrical tool.
	end := start.Z + tool.height
	if drill != axis {
		end = start.Z - tool.height
	}
	finish := stock.frame.ToWorld(geom.Vec3{X: start.X, Y: start.Y, Z: end})
	if finish != tool.frame.ToWorld(geom.Vec3{Z: tool.height}) ||
		!exactAxisDistance(finish.Dot(axis), tool.frame.Origin().Dot(axis), end-start.Z) ||
		!exactAxisDistance(max(start.Z, end), min(start.Z, end), tool.height) {
		return step.DiskHoleExtrusionBody{}, 0, 0, false
	}
	lo, hi := max(0, min(start.Z, end)), min(stock.height, max(start.Z, end))
	if hi <= lo || lo > 0 && hi < stock.height {
		return step.DiskHoleExtrusionBody{}, 0, 0, false
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	gap := new(big.Rat).Sub(rat(stock.radius), rat(tool.radius))
	x, y := rat(start.X), rat(start.Y)
	distanceSquared := new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
	if gap.Sign() <= 0 || distanceSquared.Cmp(gap.Mul(gap, gap)) >= 0 {
		return step.DiskHoleExtrusionBody{}, 0, 0, false
	}
	depth := hi - lo
	if !exactAxisDistance(hi, lo, depth) {
		return step.DiskHoleExtrusionBody{}, 0, 0, false
	}
	diskArea := math.Pi * tool.radius * tool.radius
	removed, areaChange := diskArea*depth, 2*math.Pi*tool.radius*depth
	hole := trimmed.ExtrusionCircle{Center: geom.Vec2{X: start.X, Y: start.Y}, Radius: tool.radius}
	p := step.DiskHoleExtrusionBody{Name: "PlainSolid", Frame: stock.frame, Height: stock.height, Outer: trimmed.ExtrusionCircle{Radius: stock.radius}}
	if lo == 0 && hi == stock.height {
		p.Holes = []trimmed.ExtrusionCircle{hole}
		areaChange -= 2 * diskArea
	} else {
		if lo > 0 {
			// A negative height starts from the original upper cap while
			// retaining the same transverse chart and outer boundary.
			origin := stock.frame.ToWorld(geom.Vec3{Z: stock.height})
			frame, err := geom.NewFrame(origin, stock.frame.XAxis(), axis)
			if err != nil || !exactAxisDistance(origin.Dot(axis), stock.frame.Origin().Dot(axis), stock.height) {
				return step.DiskHoleExtrusionBody{}, 0, 0, false
			}
			p.Frame, p.Height = frame, -stock.height
		}
		p.Blind = []trimmed.ExtrusionBlindHole{{ExtrusionCircle: hole, Depth: depth}}
	}
	return p, removed, areaChange, true
}

func (b *Bridge) cutCircularStockBore(a, c stored) (Shape, bool, error) {
	p, removed, areaChange, ok := circularStockBoreRecipe(a.cylinder, c.cylinder)
	if !ok {
		return Shape{}, false, nil
	}
	s, err := trimmed.ExtrudeDiskWithHoles(context.Background(), p.Frame, p.Outer, p.Holes, p.Blind, p.Height)
	if err != nil {
		return Shape{}, true, err
	}
	volume := a.result.GeometryAssertions.Volume - removed
	if !removesMaterial(a.result.GeometryAssertions.Volume, volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	out, err := b.putAnalyticBody(&analyticBody{solid: s, stepDisk: &p, volume: volume, area: a.result.GeometryAssertions.SurfaceArea + areaChange})
	return out, true, err
}
