package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"slices"
	"sort"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func sideOpeningStockFromNotch(p step.SideNotchBody) (step.SideOpeningCutsBody, bool) {
	if p.Left <= 0 || len(p.Through)+len(p.Blind) != 0 {
		return step.SideOpeningCutsBody{}, false
	}
	f, err := geom.NewFrame(p.Frame.ToWorld(geom.Vec3{X: -p.Left, Y: -p.Inset}), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return step.SideOpeningCutsBody{}, false
	}
	return step.SideOpeningCutsBody{
		Name: p.Name, Frame: f, Width: p.Left + p.Right, Back: p.Back + p.Inset, Length: p.Length,
		Cuts: []trimmed.SideOpeningCut{{ExtrusionCircle: trimmed.ExtrusionCircle{
			Center: geom.Vec2{X: p.Left, Y: p.Inset}, Radius: p.Radius,
		}, Depth: p.Depth}},
	}, true
}

func sideOpeningCylinderSpan(p step.SideOpeningCutsBody, tool *cylinderBody) (trimmed.ExtrusionCircle, float64, float64, bool) {
	return circularStockSpan(step.DiskHoleExtrusionBody{Frame: p.Frame, Height: p.Length}, tool)
}

func crossesOneStockSide(p step.SideOpeningCutsBody, c trimmed.ExtrusionCircle) bool {
	crossings := 0
	for _, distance := range [...]float64{c.Center.Y, p.Width - c.Center.X, p.Back - c.Center.Y, c.Center.X} {
		if distance > c.Radius {
			continue
		}
		if math.Abs(distance) >= c.Radius {
			return false
		}
		crossings++
	}
	return crossings == 1
}

// sideOpeningStockFromDualDisk recovers the original analytic stock and
// cutters after a strict equal-depth pair. A later group is rebuilt from
// those descriptors just as it would be if all tools were supplied together.
func sideOpeningStockFromDualDisk(p *sideDualDiskBody) (step.SideOpeningCutsBody, bool) {
	f, e := geom.NewFrame(p.frame.ToWorld(geom.Vec3{X: p.min.X, Y: p.min.Y}), p.frame.XAxis(), p.frame.ZAxis())
	if e != nil {
		return step.SideOpeningCutsBody{}, false
	}
	cuts := make([]trimmed.SideOpeningCut, len(p.discs))
	for i, disc := range p.discs {
		disc.Center.X -= p.min.X
		disc.Center.Y -= p.min.Y
		cuts[i] = trimmed.SideOpeningCut{ExtrusionCircle: disc, Depth: p.depth}
	}
	return step.SideOpeningCutsBody{Name: "PlainSolid", Frame: f, Width: p.max.X - p.min.X, Back: p.max.Y - p.min.Y, Length: p.height, Cuts: cuts}, true
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
	if result, ok, second := b.cutSideOpeningCuts(a, tools); ok {
		return result, true, second
	}
	return first, handled, err
}

func (b *Bridge) cutSideOpeningCuts(a stored, tools []stored) (Shape, bool, error) {
	var p step.SideOpeningCutsBody
	var skip *cylinderBody
	switch {
	case a.analytic != nil && a.analytic.sideOpeningCuts != nil:
		p = *a.analytic.sideOpeningCuts
		p.Cuts = slices.Clone(p.Cuts)
	case a.analytic != nil && a.analytic.sideDualDisk != nil:
		var ok bool
		p, ok = sideOpeningStockFromDualDisk(a.analytic.sideDualDisk)
		if !ok {
			return Shape{}, false, nil
		}
	case a.analytic != nil && a.analytic.sideNotch != nil:
		var ok bool
		p, ok = sideOpeningStockFromNotch(*a.analytic.sideNotch)
		if !ok {
			return Shape{}, false, nil
		}
	case a.planar() && len(tools) >= 2:
		for _, input := range tools {
			c := input.cylinder
			if f, box, ok := sideCylinderBlindRecipe(a.solid, c); ok {
				p, _ = sideOpeningStockFromNotch(sideNotchRecipe(f, box, c.radius, c.height))
				skip = c
				break
			}
			if f, box, ok := sideCylinderThroughRecipe(a.solid, c); ok {
				p, _ = sideOpeningStockFromNotch(sideNotchRecipe(f, box, c.radius, box.Max.Z))
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
	spans := make([][][2]float64, len(p.Cuts))
	for i, cut := range p.Cuts {
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
		circle, low, high, ok := sideOpeningCylinderSpan(p, c)
		if !ok || high <= low || !crossesOneStockSide(p, circle) {
			return Shape{}, false, nil
		}
		matched := false
		for i := range p.Cuts {
			if p.Cuts[i].ExtrusionCircle == circle {
				spans[i] = append(spans[i], [2]float64{low, high})
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if len(p.Cuts) == trimmed.MaxSideOpeningCuts {
			return Shape{}, true, &failureError{"LIMIT", "Too many distinct side-opening Hole cutters"}
		}
		p.Cuts = append(p.Cuts, trimmed.SideOpeningCut{ExtrusionCircle: circle})
		spans = append(spans, [][2]float64{{low, high}})
	}
	if len(p.Cuts) < 2 {
		return Shape{}, false, nil
	}
	if len(p.Cuts) == 2 && compareCircularFootprints(p.Cuts[0].ExtrusionCircle, p.Cuts[1].ExtrusionCircle, false) >= 0 {
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
	body, err := sideOpeningCutsBody(p)
	if err != nil {
		return Shape{}, true, err
	}
	if !removesMaterial(a.result.GeometryAssertions.Volume, body.volume) {
		return Shape{}, true, unsupportedEdge("Boolean cut removed no volume")
	}
	out, err := b.putAnalyticBody(body)
	return out, true, err
}

func sideOpeningCutsBody(p step.SideOpeningCutsBody) (*analyticBody, error) {
	result, err := trimmed.BoxSideOpeningCuts(context.Background(), p.Frame, p.Width, p.Back, p.Length, p.Cuts, 1e-9)
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
	return &analyticBody{solid: result.Solid, sideOpeningCuts: &p, polygonExtrema: corners,
		volume: p.Width*p.Back*p.Length - result.RemovedVolume,
		area:   2*(p.Width*p.Back+(p.Width+p.Back)*p.Length) + result.SurfaceAreaDelta}, nil
}

func overlapSideOpeningCuts(a, tool stored) (float64, bool, error) {
	if a.analytic == nil || tool.cylinder == nil {
		return 0, false, nil
	}
	var p step.SideOpeningCutsBody
	if a.analytic.sideOpeningCuts != nil {
		p = *a.analytic.sideOpeningCuts
	} else if a.analytic.sideDualDisk != nil {
		var ok bool
		p, ok = sideOpeningStockFromDualDisk(a.analytic.sideDualDisk)
		if !ok {
			return 0, false, nil
		}
	} else if a.analytic.sideNotch != nil {
		var ok bool
		p, ok = sideOpeningStockFromNotch(*a.analytic.sideNotch)
		if !ok {
			return 0, false, nil
		}
	} else {
		return 0, false, nil
	}
	c, low, high, ok := sideOpeningCylinderSpan(p, tool.cylinder)
	if !ok || high <= low || !crossesOneStockSide(p, c) {
		return 0, false, nil
	}
	if a.analytic.sideOpeningCuts == nil && a.analytic.sideDualDisk == nil {
		prior := p.Cuts[0].ExtrusionCircle
		if c == prior || compareCircularFootprints(prior, c, false) >= 0 {
			return 0, false, nil
		}
	}
	volume, err := trimmed.BoxSideOpeningCutsOverlap(p.Frame, p.Width, p.Back, p.Length, p.Cuts, c, low, high, 1e-9)
	return volume, true, err
}
