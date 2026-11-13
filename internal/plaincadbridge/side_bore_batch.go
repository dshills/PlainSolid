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
	circle trimmed.ExtrusionCircle
	depth  float64
}

func sideNotchBore(p step.SideBlindNotchBody, tool *cylinderBody) (trimmed.ExtrusionCircle, float64, float64, bool) {
	return circularStockSpan(step.DiskHoleExtrusionBody{Frame: p.Frame, Height: p.Length}, tool)
}

func (b *Bridge) cutSideNotchBores(a stored, tools []stored) (Shape, bool, error) {
	var p step.SideBlindNotchBody
	var bores []sideStockBore
	var skip *cylinderBody
	volume, area := a.result.GeometryAssertions.Volume, a.result.GeometryAssertions.SurfaceArea
	if a.analytic != nil && a.analytic.sideBlindNotch != nil {
		p = *a.analytic.sideBlindNotch
		bores = sideStockBores(p)
	} else {
		if !a.planar() || len(tools) < 2 {
			return Shape{}, false, nil
		}
		for _, tool := range tools {
			if f, box, ok := sideCylinderBlindRecipe(a.solid, tool.cylinder); ok {
				skip = tool.cylinder
				p = sideBlindNotchRecipe(f, box, skip.radius, skip.height)
				section, wall := sideCylinderSection(box, p.Radius)
				volume, area = volume-section*p.Depth, area+wall*p.Depth
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
		if !ok || low != 0 || high <= low {
			return Shape{}, true, unsupportedEdge("additional side-notch bores must enter the same cap or cut through")
		}
		prior := slices.IndexFunc(bores, func(bore sideStockBore) bool { return bore.circle == hole })
		if prior >= 0 {
			if bores[prior].depth >= high {
				continue
			}
			old := bores[prior].depth
			bores[prior].depth = high
			volume -= math.Pi * hole.Radius * hole.Radius * (high - old)
			area += 2 * math.Pi * hole.Radius * (high - old)
		} else {
			bores = append(bores, sideStockBore{hole, high})
			volume -= math.Pi * hole.Radius * hole.Radius * high
			area += 2 * math.Pi * hole.Radius * high
		}
		if high == p.Length {
			area -= 2 * math.Pi * hole.Radius * hole.Radius
		}
	}
	if !removesMaterial(a.result.GeometryAssertions.Volume, volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	p.Through, p.Blind = nil, nil
	for _, bore := range bores {
		if bore.depth == p.Length {
			p.Through = append(p.Through, bore.circle)
		} else {
			p.Blind = append(p.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: bore.circle, Depth: bore.depth})
		}
	}
	solid, err := trimmed.BoxSideCylinderBlindWithBores(context.Background(), p.Frame, p.Left, p.Right, p.Back, p.Inset, p.Length, p.Radius, p.Depth, p.Through, p.Blind, 1e-9)
	if err != nil {
		return Shape{}, true, err
	}
	result, err := b.putAnalyticBody(&analyticBody{solid: solid, sideBlindNotch: &p, volume: volume, area: area})
	return result, true, err
}

func sideStockBores(p step.SideBlindNotchBody) []sideStockBore {
	bores := make([]sideStockBore, 0, len(p.Through)+len(p.Blind))
	for _, h := range p.Through {
		bores = append(bores, sideStockBore{h, p.Length})
	}
	for _, h := range p.Blind {
		bores = append(bores, sideStockBore{h.ExtrusionCircle, h.Depth})
	}
	return bores
}

func overlapSideNotchBores(a, c stored) (float64, bool, error) {
	if a.analytic == nil || a.analytic.sideBlindNotch == nil || c.cylinder == nil {
		return 0, false, nil
	}
	p := *a.analytic.sideBlindNotch
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
			return math.Pi * r * r * max(0, high-max(low, prior.depth)), true, nil
		}
		if compareCircularFootprints(prior.circle, hole, false) <= 0 {
			return 0, true, unsupportedEdge("additional bore footprints must remain disjoint")
		}
	}
	return math.Pi * r * r * (high - low), true, nil
}
