package plaincadbridge

import (
	"cmp"
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

func (bore sideStockBore) measures(length float64) (volume, surfaceDelta float64) {
	depth, radius := bore.high-bore.low, bore.circle.Radius
	disk := math.Pi * radius * radius
	delta := 2 * math.Pi * radius * depth
	if bore.low == 0 && bore.high == length {
		delta -= 2 * disk
	}
	return disk * depth, delta
}

func sideNotchBore(p step.SideNotchBody, tool *cylinderBody) (trimmed.ExtrusionCircle, float64, float64, bool) {
	return circularStockSpan(step.DiskHoleExtrusionBody{Frame: p.Frame, Height: p.Length}, tool)
}

func (b *Bridge) cutSideNotchBores(a stored, tools []stored) (Shape, bool, error) {
	var p step.SideNotchBody
	var bores []sideStockBore
	var added []sideStockBore
	var notchSpans [][2]float64
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
				section, wall := sideCylinderSection(p.Inset, p.Radius)
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
		if ok && hole == (trimmed.ExtrusionCircle{Radius: p.Radius}) && high > low {
			notchSpans = append(notchSpans, [2]float64{low, high})
			continue
		}
		if !ok || low != 0 && high != p.Length || high <= low || !exactAxisDistance(high, low, high-low) {
			return Shape{}, true, unsupportedEdge("additional side-notch bores must open on an exterior cap")
		}
		added = append(added, sideStockBore{hole, low, high})
	}
	slices.SortFunc(notchSpans, func(a, b [2]float64) int { return cmp.Compare(a[0], b[0]) })
	end := p.Depth
	for _, span := range notchSpans {
		if span[0] > end {
			return Shape{}, true, unsupportedEdge("coaxial notch cuts require connected axial spans")
		}
		end = max(end, span[1])
	}
	if end > p.Depth {
		extra := end - p.Depth
		if !exactAxisDistance(end, p.Depth, extra) {
			return Shape{}, true, geom.ErrUnrepresentable
		}
		section, wall := notchSection(p)
		volume -= section * extra
		area += wall * extra
		if end == p.Length {
			area -= 2 * section
		}
		p.Depth = end
	}
	// Merge connected spans across the whole batch. A separated pair retains
	// its material web and two independent cap openings.
	merged := make([]sideStockBore, 0, len(bores)+len(added))
	spans := slices.Concat(bores, added)
	slices.SortFunc(spans, compareSideStockBores)
	for _, bore := range spans {
		if len(merged) == 0 || merged[len(merged)-1].circle != bore.circle || merged[len(merged)-1].high < bore.low {
			merged = append(merged, bore)
			continue
		}
		last := &merged[len(merged)-1]
		last.high = max(last.high, bore.high)
	}
	var addedVolume, addedArea float64
	for _, bore := range merged {
		depth := bore.high - bore.low
		if !exactAxisDistance(bore.high, bore.low, depth) {
			return Shape{}, true, geom.ErrUnrepresentable
		}
		removed, delta := bore.measures(p.Length)
		addedVolume, addedArea = addedVolume+removed, addedArea+delta
	}
	for _, bore := range bores {
		removed, delta := bore.measures(p.Length)
		addedVolume, addedArea = addedVolume-removed, addedArea-delta
	}
	volume, area = volume-addedVolume, area+addedArea
	if !removesMaterial(a.result.GeometryAssertions.Volume, volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	p.Through, p.Blind = nil, nil
	for _, bore := range merged {
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

func compareSideStockBores(a, b sideStockBore) int {
	if order := cmp.Compare(a.circle.Center.X, b.circle.Center.X); order != 0 {
		return order
	}
	if order := cmp.Compare(a.circle.Center.Y, b.circle.Center.Y); order != 0 {
		return order
	}
	if order := cmp.Compare(a.circle.Radius, b.circle.Radius); order != 0 {
		return order
	}
	return cmp.Compare(a.low, b.low)
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
	if hole == (trimmed.ExtrusionCircle{Radius: p.Radius}) {
		section, _ := notchSection(p)
		return section * max(0, high-max(low, p.Depth)), true, nil
	}
	if x-r <= -p.Left || x+r >= p.Right || y-r <= -p.Inset || y+r >= p.Back ||
		compareCircularFootprints(trimmed.ExtrusionCircle{Radius: p.Radius}, hole, false) <= 0 {
		return 0, true, unsupportedEdge("additional bore footprint must clear the side notch and stock walls")
	}
	depth := high - low
	for _, prior := range sideStockBores(p) {
		if prior.circle == hole {
			depth -= max(0, min(high, prior.high)-max(low, prior.low))
			continue
		}
		if compareCircularFootprints(prior.circle, hole, false) <= 0 {
			return 0, true, unsupportedEdge("additional bore footprints must remain disjoint")
		}
	}
	return math.Pi * r * r * max(0, depth), true, nil
}

func notchSection(p step.SideNotchBody) (area, wallDelta float64) {
	if p.Left == 0 {
		return math.Pi * p.Radius * p.Radius / 4, (math.Pi/2 - 2) * p.Radius
	}
	return sideCylinderSection(p.Inset, p.Radius)
}
