package plaincadbridge

import (
	"context"
	"math"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func circularStockRecipe(a stored) (step.DiskHoleExtrusionBody, bool) {
	if a.analytic != nil && a.analytic.stepDisk != nil {
		p := *a.analytic.stepDisk
		return p, cardinalFrame(p.Frame)
	}
	p := step.DiskHoleExtrusionBody{Name: "PlainSolid"}
	switch {
	case a.cylinder != nil:
		stock := a.cylinder
		p.Frame, p.Height, p.Outer.Radius = stock.frame, stock.height, stock.radius
	case a.analytic != nil && a.analytic.sector != nil && a.analytic.sector.angle == 2*math.Pi:
		stock := a.analytic.sector
		p.Frame, p.Height, p.Outer.Radius = stock.frame, stock.height, stock.radius
		if stock.inner > 0 {
			p.Holes = []trimmed.ExtrusionCircle{{Radius: stock.inner}}
		}
	default:
		return p, false
	}
	return p, cardinalFrame(p.Frame)
}

func circularStockSpan(p step.DiskHoleExtrusionBody, tool *cylinderBody) (trimmed.ExtrusionCircle, float64, float64, bool) {
	axis, drill := p.Frame.ZAxis(), tool.frame.ZAxis()
	if drill != axis && drill != axis.Scale(-1) {
		return trimmed.ExtrusionCircle{}, 0, 0, false
	}
	origin := p.Frame.Origin()
	start := p.Frame.ToLocal(tool.frame.Origin())
	if p.Frame.ToWorld(start) != tool.frame.Origin() ||
		!exactAxisDistance(tool.frame.Origin().Dot(p.Frame.XAxis()), origin.Dot(p.Frame.XAxis()), start.X) ||
		!exactAxisDistance(tool.frame.Origin().Dot(p.Frame.YAxis()), origin.Dot(p.Frame.YAxis()), start.Y) ||
		!exactAxisDistance(tool.frame.Origin().Dot(axis), origin.Dot(axis), start.Z) {
		return trimmed.ExtrusionCircle{}, 0, 0, false
	}
	end := start.Z + tool.height
	if drill != axis {
		end = start.Z - tool.height
	}
	finish := p.Frame.ToWorld(geom.Vec3{X: start.X, Y: start.Y, Z: end})
	if finish != tool.frame.ToWorld(geom.Vec3{Z: tool.height}) ||
		!exactAxisDistance(finish.Dot(axis), tool.frame.Origin().Dot(axis), end-start.Z) ||
		!exactAxisDistance(max(start.Z, end), min(start.Z, end), tool.height) {
		return trimmed.ExtrusionCircle{}, 0, 0, false
	}
	hole := trimmed.ExtrusionCircle{Center: geom.Vec2{X: start.X, Y: start.Y}, Radius: tool.radius}
	lo := max(min(0, p.Height), min(start.Z, end))
	hi := min(max(0, p.Height), max(start.Z, end))
	return hole, lo, hi, true
}

// A negative result places the center distance strictly below the exact
// sum of the two radii, or their signed difference for containment.
func compareCircularFootprints(a, c trimmed.ExtrusionCircle, difference bool) int {
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x := new(big.Rat).Sub(rat(a.Center.X), rat(c.Center.X))
	y := new(big.Rat).Sub(rat(a.Center.Y), rat(c.Center.Y))
	distanceSquared := x.Mul(x, x).Add(x, y.Mul(y, y))
	radius := rat(a.Radius)
	if difference {
		radius.Sub(radius, rat(c.Radius))
		if radius.Sign() < 0 {
			return 1
		}
	} else {
		radius.Add(radius, rat(c.Radius))
	}
	return distanceSquared.Cmp(radius.Mul(radius, radius))
}

func circularBlindSpan(height float64, hole trimmed.ExtrusionBlindHole) (float64, float64) {
	depth := math.Copysign(hole.Depth, height)
	if hole.OppositeCap {
		return min(height, height-depth), max(height, height-depth)
	}
	return min(0, depth), max(0, depth)
}

func (b *Bridge) cutCircularStockBores(a stored, tools []stored) (Shape, bool, error) {
	p, ok := circularStockRecipe(a)
	if !ok {
		return Shape{}, false, nil
	}
	for _, tool := range tools {
		if tool.cylinder == nil {
			return Shape{}, false, nil
		}
	}
	p.Holes, p.Blind = slices.Clone(p.Holes), slices.Clone(p.Blind)
	before := a.result.GeometryAssertions.Volume
	volume, area := before, a.result.GeometryAssertions.SurfaceArea
	layered := a.analytic != nil && a.analytic.layeredDisk
	changed := false
	type boreSpan struct {
		tool   *cylinderBody
		circle trimmed.ExtrusionCircle
		lo, hi float64
	}
	candidates := make([]boreSpan, 0, len(tools))
	for _, tool := range tools {
		hole, lo, hi, ok := circularStockSpan(p, tool.cylinder)
		if !ok {
			return Shape{}, true, unsupportedEdge("circular stock requires an exactly represented parallel tool")
		}
		if hi <= lo || compareCircularFootprints(p.Outer, hole, false) >= 0 {
			continue
		}
		if compareCircularFootprints(p.Outer, hole, true) >= 0 {
			return Shape{}, true, unsupportedEdge("circular stock requires the bore strictly inside its outside wall")
		}
		candidates = append(candidates, boreSpan{tool.cylinder, hole, lo, hi})
	}
	for i, candidate := range candidates {
		covered := false
		for j, other := range candidates {
			if i == j || other.lo > candidate.lo || other.hi < candidate.hi ||
				compareCircularFootprints(other.circle, candidate.circle, true) > 0 {
				continue
			}
			if other.circle != candidate.circle || other.lo != candidate.lo || other.hi != candidate.hi || j < i {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		hole, lo, hi, ok := circularStockSpan(p, candidate.tool)
		if !ok {
			return Shape{}, true, geom.ErrUnrepresentable
		}
		redundant, contact := circularBoreContact(p, hole, lo, hi)
		if redundant {
			continue
		}
		layered = layered || contact
		changed = true
		depth := hi - lo
		if !exactAxisDistance(hi, lo, depth) {
			return Shape{}, true, geom.ErrUnrepresentable
		}
		diskArea := math.Pi * hole.Radius * hole.Radius
		volume -= diskArea * depth
		area += 2 * math.Pi * hole.Radius * depth
		if lo == min(0, p.Height) && hi == max(0, p.Height) {
			p.Holes = append(p.Holes, hole)
			area -= 2 * diskArea
			continue
		}
		entry := lo
		if p.Height < 0 {
			entry = hi
		}
		opposite := entry != 0
		if opposite && lo != p.Height && hi != p.Height {
			return Shape{}, true, unsupportedEdge("circular stock requires blind bores to open at an exterior cap")
		}
		if opposite && len(p.Blind) == 0 {
			axis := p.Frame.ZAxis()
			origin := p.Frame.ToWorld(geom.Vec3{Z: p.Height})
			frame, err := geom.NewFrame(origin, p.Frame.XAxis(), axis)
			if err != nil || !exactAxisDistance(origin.Dot(axis), p.Frame.Origin().Dot(axis), p.Height) {
				return Shape{}, true, geom.ErrUnrepresentable
			}
			p.Frame, p.Height = frame, -p.Height
			opposite = false
		}
		p.Blind = append(p.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: hole, Depth: depth, OppositeCap: opposite})
	}
	if !changed {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	limit := 16
	if layered {
		limit = 32
	}
	if len(p.Holes)+len(p.Blind) > limit {
		return Shape{}, true, &failureError{"LIMIT", "circular stock bore count exceeds the qualified limit"}
	}
	ctx := context.Background()
	var s trimmed.Solid
	var err error
	if layered {
		var proof trimmed.LayeredBoreHistory
		s, proof, err = trimmed.BoreDiskDepthUnion(ctx, p.Frame, p.Outer, p.Holes, p.Blind, p.Height)
		stockHeight := math.Abs(p.Height)
		volume = math.Pi*p.Outer.Radius*p.Outer.Radius*stockHeight - proof.RemovedVolume
		area = 2*math.Pi*p.Outer.Radius*(p.Outer.Radius+stockHeight) + proof.SurfaceAreaDelta
	} else {
		s, err = trimmed.ExtrudeDiskWithHoles(ctx, p.Frame, p.Outer, p.Holes, p.Blind, p.Height)
	}
	if err != nil {
		return Shape{}, true, err
	}
	if !removesMaterial(before, volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	out, err := b.putAnalyticBody(&analyticBody{solid: s, stepDisk: &p, layeredDisk: layered, volume: volume, area: area})
	return out, true, err
}

func circularBoreContact(p step.DiskHoleExtrusionBody, hole trimmed.ExtrusionCircle, lo, hi float64) (redundant, contact bool) {
	for _, prior := range p.Holes {
		if compareCircularFootprints(prior, hole, true) <= 0 {
			return true, true
		}
		contact = contact || compareCircularFootprints(prior, hole, false) <= 0
	}
	for _, prior := range p.Blind {
		start, end := circularBlindSpan(p.Height, prior)
		if lo >= start && hi <= end && compareCircularFootprints(prior.ExtrusionCircle, hole, true) <= 0 {
			return true, true
		}
		contact = contact || compareCircularFootprints(prior.ExtrusionCircle, hole, false) <= 0
	}
	return false, contact
}

func overlapCircularStockBores(a, tool stored) (float64, bool, error) {
	if a.analytic == nil || tool.cylinder == nil {
		return 0, false, nil
	}
	p, ok := circularStockRecipe(a)
	if !ok {
		return 0, false, nil
	}
	hole, lo, hi, ok := circularStockSpan(p, tool.cylinder)
	if !ok {
		return 0, true, unsupportedEdge("circular stock requires an exactly represented parallel tool")
	}
	if hi <= lo || compareCircularFootprints(p.Outer, hole, false) >= 0 {
		return 0, true, nil
	}
	if compareCircularFootprints(p.Outer, hole, true) >= 0 {
		return 0, true, unsupportedEdge("circular stock requires the bore strictly inside its outside wall")
	}
	if redundant, _ := circularBoreContact(p, hole, lo, hi); redundant {
		return 0, true, nil
	}
	if a.analytic.layeredDisk {
		removed, err := trimmed.DiskDepthUnionOverlap(p.Frame, p.Outer, p.Holes, p.Blind, p.Height, hole, lo, hi)
		return removed, true, err
	}
	volume := math.Pi * hole.Radius * hole.Radius * (hi - lo)
	removed := func(prior trimmed.ExtrusionCircle, depth float64) (float64, error) {
		if depth <= 0 || compareCircularFootprints(prior, hole, false) >= 0 {
			return 0, nil
		}
		profile := func(h trimmed.ExtrusionCircle) parametric.DiskProfile {
			return parametric.DiskProfile{Center: h.Center, Radius: h.Radius}
		}
		ctx := context.Background()
		solid, err := parametric.CombineParallelCylinders(p.Frame, profile(prior), profile(hole), depth, parametric.Intersection, 1e-9)
		if err != nil || len(solid.Faces()) == 0 {
			return 0, err
		}
		properties, err := solid.MassProperties(ctx, parametric.IntegrationOptions{})
		return properties.Volume, err
	}
	for _, prior := range p.Holes {
		v, err := removed(prior, hi-lo)
		if err != nil {
			return 0, true, err
		}
		volume -= v
	}
	for _, prior := range p.Blind {
		start, end := circularBlindSpan(p.Height, prior)
		v, err := removed(prior.ExtrusionCircle, min(hi, end)-max(lo, start))
		if err != nil {
			return 0, true, err
		}
		volume -= v
	}
	return max(0, volume), true, nil
}
