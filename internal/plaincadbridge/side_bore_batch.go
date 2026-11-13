package plaincadbridge

import (
	"context"
	"math"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type sideStockBore struct {
	circle    trimmed.ExtrusionCircle
	low, high float64
}

func sideNotchBore(p step.SideNotchBody, tool *cylinderBody) (trimmed.ExtrusionCircle, float64, float64, bool) {
	return circularStockSpan(step.DiskHoleExtrusionBody{Frame: p.Frame, Height: p.Length}, tool)
}

func (b *Bridge) cutSideNotchBores(a stored, tools []stored) (Shape, bool, error) {
	var p step.SideNotchBody
	var bores []sideStockBore
	var skip *cylinderBody
	volume, area := a.result.GeometryAssertions.Volume, a.result.GeometryAssertions.SurfaceArea
	if a.analytic != nil && a.analytic.sideNotch != nil {
		p = *a.analytic.sideNotch
		bores = sideStockBores(p)
	} else {
		if !a.planar() || len(tools) < 2 {
			return Shape{}, false, nil
		}
		for _, tool := range tools {
			if f, box, ok := sideCylinderBlindRecipe(a.solid, tool.cylinder); ok {
				skip = tool.cylinder
				p = sideNotchRecipe(f, box, skip.radius, skip.height)
				section, wall := sideCylinderSection(box, p.Radius)
				volume, area = volume-section*p.Depth, area+wall*p.Depth
				break
			}
			if f, box, ok := sideCylinderThroughRecipe(a.solid, tool.cylinder); ok {
				skip = tool.cylinder
				p = sideNotchRecipe(f, box, skip.radius, box.Max.Z)
				removed, delta := sideCylinderMeasures(box, p.Radius)
				volume, area = volume-removed, area+delta
				break
			}
			if f, size, ok := cornerCylinderThroughRecipe(a.solid, tool.cylinder); ok {
				skip = tool.cylinder
				r := skip.radius
				p = cornerNotchRecipe(f, size, r, size.Z)
				volume -= math.Pi * r * r * size.Z / 4
				area += (math.Pi/2-2)*r*size.Z - math.Pi*r*r/2
				break
			}
			if f, size, ok := cornerCylinderRecipe(a.solid, tool.cylinder); ok {
				skip = tool.cylinder
				r, depth := skip.radius, skip.height
				p = cornerNotchRecipe(f, size, r, depth)
				volume -= math.Pi * r * r * depth / 4
				area += (math.Pi/2 - 2) * r * depth
				break
			}
		}
		if skip == nil {
			return Shape{}, false, nil
		}
	}
	for _, tool := range tools {
		if tool.cylinder == nil {
			return Shape{}, false, nil
		}
		if tool.cylinder == skip {
			continue
		}
		if boundsSeparated(a.result.Bounds, tool.result.Bounds) {
			continue
		}
		hole, low, high, ok := sideNotchBore(p, tool.cylinder)
		if ok && hole == (trimmed.ExtrusionCircle{Radius: p.Radius}) && low == 0 && high <= p.Depth {
			continue
		}
		if !ok || low != 0 && high != p.Length || high <= low || !exactAxisDistance(high, low, high-low) {
			return Shape{}, true, unsupportedEdge("additional side-notch bores must open on an exterior cap")
		}
		depth := high - low
		prior := slices.IndexFunc(bores, func(bore sideStockBore) bool { return bore.circle == hole })
		if prior >= 0 {
			previous := bores[prior]
			if previous.low > high || previous.high < low {
				return Shape{}, true, unsupportedEdge("opposed cuts at one side-notch bore site require connected axial spans")
			}
			newLow, newHigh := min(previous.low, low), max(previous.high, high)
			if newLow == previous.low && newHigh == previous.high {
				continue
			}
			bores[prior] = sideStockBore{hole, newLow, newHigh}
			depth = newHigh - newLow - (previous.high - previous.low)
			low, high = newLow, newHigh
		} else {
			bores = append(bores, sideStockBore{hole, low, high})
		}
		diskArea := math.Pi * hole.Radius * hole.Radius
		volume -= diskArea * depth
		area += 2 * math.Pi * hole.Radius * depth
		if low == 0 && high == p.Length {
			area -= 2 * diskArea
		}
	}
	if !removesMaterial(a.result.GeometryAssertions.Volume, volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	p.Through, p.Blind = nil, nil
	for _, bore := range bores {
		if bore.low == 0 && bore.high == p.Length {
			p.Through = append(p.Through, bore.circle)
		} else {
			p.Blind = append(p.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: bore.circle, Depth: bore.high - bore.low, OppositeCap: bore.low != 0})
		}
	}
	solid, err := p.Solid(context.Background())
	if err != nil {
		return Shape{}, true, err
	}
	result, err := b.putAnalyticBody(&analyticBody{solid: solid, sideNotch: &p, volume: volume, area: area})
	return result, true, err
}

func sideStockBores(p step.SideNotchBody) []sideStockBore {
	bores := make([]sideStockBore, 0, len(p.Through)+len(p.Blind))
	for _, h := range p.Through {
		bores = append(bores, sideStockBore{h, 0, p.Length})
	}
	for _, h := range p.Blind {
		low, high := circularBlindSpan(p.Length, h)
		bores = append(bores, sideStockBore{h.ExtrusionCircle, low, high})
	}
	return bores
}

func overlapSideNotchBores(a, c stored) (float64, bool, error) {
	if a.analytic == nil || a.analytic.sideNotch == nil || c.cylinder == nil {
		return 0, false, nil
	}
	p := *a.analytic.sideNotch
	hole, low, high, ok := sideNotchBore(p, c.cylinder)
	if !ok {
		return 0, true, geom.ErrUnrepresentable
	}
	if high <= low {
		return 0, true, nil
	}
	x, y, r := hole.Center.X, hole.Center.Y, hole.Radius
	if hole == (trimmed.ExtrusionCircle{Radius: p.Radius}) && low >= 0 && high <= p.Depth {
		return 0, true, nil
	}
	if x-r <= -p.Left || x+r >= p.Right || y-r <= -p.Inset || y+r >= p.Back ||
		compareCircularFootprints(trimmed.ExtrusionCircle{Radius: p.Radius}, hole, false) <= 0 {
		return 0, true, unsupportedEdge("additional bore footprint must clear the side notch and stock walls")
	}
	for _, prior := range sideStockBores(p) {
		if prior.circle == hole {
			return math.Pi * r * r * (high - low - max(0, min(high, prior.high)-max(low, prior.low))), true, nil
		}
		if compareCircularFootprints(prior.circle, hole, false) <= 0 {
			return 0, true, unsupportedEdge("additional bore footprints must remain disjoint")
		}
	}
	return math.Pi * r * r * (high - low), true, nil
}
