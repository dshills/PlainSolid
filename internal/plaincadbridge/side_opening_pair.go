package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"sort"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func pairFromSideNotch(p step.SideNotchBody) (step.SideOpeningPairBody, bool) {
	if p.Left <= 0 || len(p.Through)+len(p.Blind) != 0 {
		return step.SideOpeningPairBody{}, false
	}
	f, err := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{X: -p.Left, Y: -p.Inset}), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return step.SideOpeningPairBody{}, false
	}
	return step.SideOpeningPairBody{
		Name: p.Name, Frame: f, Width: p.Left + p.Right, Back: p.Back + p.Inset, Length: p.Length,
		Cuts: [2]trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{
			Center: geom.Vec2{X: p.Left, Y: p.Inset}, Radius: p.Radius,
		}, Depth: p.Depth}},
	}, true
}

func pairCylinderSpan(p step.SideOpeningPairBody, tool *cylinderBody) (trimmed.ExtrusionCircle, float64, float64, bool) {
	return circularStockSpan(step.DiskHoleExtrusionBody{Frame: p.Frame, Height: p.Length}, tool)
}

func crossesPairSide(p step.SideOpeningPairBody, c trimmed.ExtrusionCircle) bool {
	return math.Abs(c.Center.Y) < c.Radius && c.Center.X-c.Radius > 0 &&
		c.Center.X+c.Radius < p.Width && c.Center.Y+c.Radius < p.Back
}

// Keep the precise equal-depth family when it can represent the exact
// section. Its narrower representability gate can reject an irrational
// circle intersection which the analytic two-opening path proves directly.
// Retry only that exact representability failure and publish at most one body.
func (b *Bridge) cutSideOpeningFamilies(a stored, tools []stored) (Shape, bool, error) {
	first, handled, err := b.cutSideDualDisk(a, tools)
	if handled && !errors.Is(err, geom.ErrUnrepresentable) {
		return first, true, err
	}
	if result, ok, second := b.cutSideOpeningPair(a, tools); ok {
		return result, true, second
	}
	return first, handled, err
}

func (b *Bridge) cutSideOpeningPair(a stored, tools []stored) (Shape, bool, error) {
	var p step.SideOpeningPairBody
	var skip *cylinderBody
	n := 1
	switch {
	case a.analytic != nil && a.analytic.sideOpeningPair != nil:
		p = *a.analytic.sideOpeningPair
		n = 2
	case a.analytic != nil && a.analytic.sideNotch != nil:
		var ok bool
		p, ok = pairFromSideNotch(*a.analytic.sideNotch)
		if !ok {
			return Shape{}, false, nil
		}
	case a.planar() && len(tools) >= 2:
		for _, input := range tools {
			c := input.cylinder
			if f, box, ok := sideCylinderBlindRecipe(a.solid, c); ok {
				p, _ = pairFromSideNotch(sideNotchRecipe(f, box, c.radius, c.height))
				skip = c
				break
			}
			if f, box, ok := sideCylinderThroughRecipe(a.solid, c); ok {
				p, _ = pairFromSideNotch(sideNotchRecipe(f, box, c.radius, box.Max.Z))
				skip = c
				break
			}
		}
		if skip == nil || p.Length == 0 {
			return Shape{}, false, nil
		}
	default:
		return Shape{}, false, nil
	}
	var spans [2][][2]float64
	for i, cut := range p.Cuts[:n] {
		if cut.OppositeCap {
			spans[i] = append(spans[i], [2]float64{p.Length - cut.Depth, p.Length})
		} else {
			spans[i] = append(spans[i], [2]float64{0, cut.Depth})
		}
	}
	for _, input := range tools {
		c := input.cylinder
		if c == nil {
			return Shape{}, false, nil
		}
		if c == skip || boundsSeparated(a.result.Bounds, input.result.Bounds) {
			continue
		}
		circle, low, high, ok := pairCylinderSpan(p, c)
		if !ok || high <= low || !crossesPairSide(p, circle) {
			return Shape{}, false, nil
		}
		matched := false
		for i := range p.Cuts[:n] {
			if p.Cuts[i].ExtrusionCircle == circle {
				spans[i] = append(spans[i], [2]float64{low, high})
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if n == 2 {
			return Shape{}, true, unsupportedEdge("side-opening cut supports two distinct intersecting footprints")
		}
		if compareCircularFootprints(p.Cuts[0].ExtrusionCircle, circle, false) >= 0 {
			return Shape{}, false, nil
		}
		p.Cuts[n].ExtrusionCircle = circle
		spans[n] = append(spans[n], [2]float64{low, high})
		n++
	}
	if n < 2 {
		return Shape{}, false, nil
	}
	for i, site := range spans {
		sort.Slice(site, func(a, b int) bool { return site[a][0] < site[b][0] })
		low, high := site[0][0], site[0][1]
		for _, span := range site[1:] {
			if span[0] > high {
				return Shape{}, true, unsupportedEdge("opposed cuts at a side-opening site must connect")
			}
			high = max(high, span[1])
		}
		if low != 0 && high != p.Length {
			return Shape{}, true, unsupportedEdge("side-opening cuts must open on an exterior cap")
		}
		if !exactAxisDistance(high, low, high-low) {
			return Shape{}, true, geom.ErrUnrepresentable
		}
		p.Cuts[i].Depth, p.Cuts[i].OppositeCap = high-low, low != 0
	}
	body, err := sideOpeningPairBody(p)
	if err != nil {
		return Shape{}, true, err
	}
	if !removesMaterial(a.result.GeometryAssertions.Volume, body.volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	out, err := b.putAnalyticBody(body)
	return out, true, err
}

func sideOpeningPairBody(p step.SideOpeningPairBody) (*analyticBody, error) {
	result, err := trimmed.BoxSideOpeningPair(context.Background(), p.Frame, p.Width, p.Back, p.Length, p.Cuts, 1e-9)
	if err != nil {
		return nil, err
	}
	var corners []geom.Vec3
	for _, x := range []float64{0, p.Width} {
		for _, y := range []float64{0, p.Back} {
			for _, z := range []float64{0, p.Length} {
				corners = append(corners, p.Frame.ToWorld(geom.Vec3{X: x, Y: y, Z: z}))
			}
		}
	}
	return &analyticBody{solid: result.Solid, sideOpeningPair: &p, polygonExtrema: corners,
		volume: p.Width*p.Back*p.Length - result.RemovedVolume,
		area:   2*(p.Width*p.Back+(p.Width+p.Back)*p.Length) + result.SurfaceAreaDelta}, nil
}

func overlapSideOpeningPair(a, tool stored) (float64, bool, error) {
	if a.analytic == nil || tool.cylinder == nil {
		return 0, false, nil
	}
	var p step.SideOpeningPairBody
	var prior trimmed.SideOpeningCut
	if a.analytic.sideOpeningPair != nil {
		p = *a.analytic.sideOpeningPair
	} else if a.analytic.sideNotch != nil {
		var ok bool
		p, ok = pairFromSideNotch(*a.analytic.sideNotch)
		if !ok {
			return 0, false, nil
		}
		prior = p.Cuts[0]
	} else {
		return 0, false, nil
	}
	c, low, high, ok := pairCylinderSpan(p, tool.cylinder)
	if !ok || high <= low || !crossesPairSide(p, c) {
		return 0, false, nil
	}
	if a.analytic.sideOpeningPair != nil {
		var matched trimmed.SideOpeningCut
		switch c {
		case p.Cuts[0].ExtrusionCircle:
			matched, prior = p.Cuts[0], p.Cuts[1]
		case p.Cuts[1].ExtrusionCircle:
			matched, prior = p.Cuts[1], p.Cuts[0]
		default:
			return 0, false, nil
		}
		if matched.OppositeCap {
			high = min(high, p.Length-matched.Depth)
		} else {
			low = max(low, matched.Depth)
		}
		if high <= low {
			return 0, true, nil
		}
	} else if c == prior.ExtrusionCircle || compareCircularFootprints(prior.ExtrusionCircle, c, false) >= 0 {
		return 0, false, nil
	}
	v, err := trimmed.BoxSideOpeningPairOverlap(p.Frame, p.Width, p.Back, p.Length, prior, c, low, high, 1e-9)
	return v, true, err
}
